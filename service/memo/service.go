package memo

import (
	"context"
	"errors"
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/fflogs"
	"memo-syncer/service/keypool"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
)

var (
	// Pool is initialized from main.go after DB + Redis are ready.
	Pool *keypool.Pool

	// WorkerCount is the number of parallel sync workers. Capped regardless
	// of pool size to avoid overloading FFLogs with too many concurrent
	// connections per donated key.
	WorkerCount = 8

	// MemberQueueSize is the buffered channel between producer and workers.
	MemberQueueSize = 64

	// RecentSyncSkip is how long we skip a member after a successful sync.
	RecentSyncSkip = time.Hour

	// PerMemberTimeout bounds a single member's full pipeline
	// (char_id → best_fights → fight_detail × N → CreateFight × N).
	PerMemberTimeout = 60 * time.Second
)

// Progress counters exposed to the /progress handler.
var (
	Total     int64
	Processed atomic.Int64
	LastID    atomic.Uint64
)

// SyncMembers runs a full scan over all members: produces them into a
// channel, spawns WorkerCount workers that pull members, acquire a key from
// the pool, fetch FFLogs data, and push fights into memo-server.
//
// Transitions the global state to `scanning` on entry and `idle` on return.
// The main loop is responsible for setting the subsequent NextScanAt.
func SyncMembers() error {
	if Pool == nil {
		return errors.New("memo: key pool not initialized")
	}

	markScanStarted()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := flow.DB.Model(&model.Member{}).Count(&Total).Error; err != nil {
		return err
	}
	Processed.Store(0)
	LastID.Store(0)

	memberCh := make(chan model.Member, MemberQueueSize)

	// producer
	var prodErr error
	var prodWg sync.WaitGroup
	prodWg.Add(1)
	go func() {
		defer prodWg.Done()
		defer close(memberCh)
		prodErr = produceMembers(ctx, memberCh)
	}()

	// workers
	var workerWg sync.WaitGroup
	for i := 0; i < WorkerCount; i++ {
		workerWg.Add(1)
		go func(id int) {
			defer workerWg.Done()
			runWorker(ctx, id, memberCh)
		}(i)
	}

	workerWg.Wait()
	prodWg.Wait()

	return prodErr
}

// produceMembers walks `members` by ID in batches and pushes candidates onto
// memberCh. Candidates are filtered here (recent sync, server check) so
// workers see only real work.
func produceMembers(ctx context.Context, out chan<- model.Member) error {
	var lastID uint
	batchSize := 1000

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var batch []model.Member
		err := flow.DB.WithContext(ctx).
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(batchSize).
			Find(&batch).Error
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}

		for _, m := range batch {
			lastID = m.ID
			LastID.Store(uint64(m.ID))
			Processed.Add(1)

			if !shouldSync(m) {
				continue
			}

			select {
			case out <- m:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func shouldSync(m model.Member) bool {
	if m.LogsSyncTime != nil && time.Since(*m.LogsSyncTime) < RecentSyncSkip {
		return false
	}
	if IsNonCNServer(m.Server) {
		return false
	}
	return true
}

// IsNonCNServer reports whether the server name is purely ASCII — CN servers
// are always Chinese characters, so an ASCII-only name means EN/JP/etc.
func IsNonCNServer(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// runWorker consumes members from memberCh until the channel is closed, and
// delegates each member to syncOneMember. A per-member recover keeps a panic
// isolated to one member instead of killing the whole worker goroutine.
func runWorker(ctx context.Context, id int, in <-chan model.Member) {
	for {
		select {
		case m, ok := <-in:
			if !ok {
				return
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Error().Int("worker", id).
							Str("member", m.Name+"@"+m.Server).
							Interface("panic", r).
							Msg("sync member panicked")
					}
				}()
				if err := syncOneMember(ctx, m); err != nil {
					log.Error().Err(err).Int("worker", id).
						Str("member", m.Name+"@"+m.Server).
						Msg("sync member failed")
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

// syncOneMember runs the full FFLogs → memo-server pipeline for a single
// member. Acquires a lease from the pool, and on FFLogs rate-limit exhaustion
// sleeps until the pool reports it's recovered.
func syncOneMember(ctx context.Context, m model.Member) error {
	memCtx, cancel := context.WithTimeout(ctx, PerMemberTimeout)
	defer cancel()

	// 1. acquire a key (may sleep until a key becomes available)
	lease, err := waitForKey(memCtx)
	if err != nil {
		return err
	}

	// 2. resolve character id (Redis cache first)
	charID, err := fflogs.ResolveCharacterID(memCtx, lease.Client, m.Name, m.Server, "cn")
	if err != nil {
		classifyAndMark(lease, err)
		return err
	}
	if charID == 0 {
		// character doesn't exist on fflogs — mark member as "synced" so we
		// don't re-query for RecentSyncSkip
		markSynced(m)
		return nil
	}

	// 3. batch-fetch best fights for all zones in one query
	enc := [4]int{}
	for i, z := range InterestZones {
		enc[i] = z.LogsID
	}
	best, err := fflogs.FetchBestFights(memCtx, lease.Client, charID, enc[0], enc[1], enc[2], enc[3])
	if err != nil {
		classifyAndMark(lease, err)
		return err
	}

	// 4. per-zone: if a parse exists, fetch detail and POST to memo-server
	// Note: each fight_detail is still one FFLogs call, so we keep using the
	// same lease but report the extra cost so the pool can account for it.
	for i, z := range InterestZones {
		rank := best.Zone(i)
		if rank == nil || len(rank.Ranks) == 0 {
			continue
		}

		fight, err := fflogs.BuildMemberZoneProgress(memCtx, lease.Client, rank)
		if err != nil {
			if errors.Is(err, fflogs.ErrNoProgress) {
				continue
			}
			classifyAndMark(lease, err)
			return err
		}

		fight.ZoneID = z.ZoneID
		for pi := range fight.Players {
			fight.Players[pi].Level = z.Level
		}

		if err := CreateFight(memCtx, fight); err != nil {
			log.Error().Err(err).
				Str("member", m.Name+"@"+m.Server).
				Uint("zone", z.ZoneID).
				Msg("create fight failed")
			// memo-server errors aren't key errors — don't penalize the key
			continue
		}
	}

	// extra debit: each zone that had data costs ~10 points for fight_detail,
	// on top of the pre-debited 20 for char_id + best_fights. Correct the pool.
	zonesWithData := 0
	for i := range InterestZones {
		if r := best.Zone(i); r != nil && len(r.Ranks) > 0 {
			zonesWithData++
		}
	}
	Pool.Release(lease, zonesWithData*10)

	markSynced(m)
	return nil
}

// waitForKey blocks until the pool hands out a lease or the context expires.
// On exhaustion it sleeps until the earliest key reset returned by Acquire,
// and flips the global state to `waiting_for_keys` while blocked so /progress
// can surface the condition.
func waitForKey(ctx context.Context) (*keypool.Lease, error) {
	blockedOnce := false
	defer func() {
		if blockedOnce {
			markWaitingForKeys(-1, time.Time{})
		}
	}()

	for {
		lease, resetAt, err := Pool.Acquire()
		if err == nil {
			return lease, nil
		}
		if errors.Is(err, keypool.ErrNoKey) {
			return nil, err
		}

		// ErrAllExhausted — wait until the earliest reset or ctx deadline
		if !blockedOnce {
			markWaitingForKeys(+1, resetAt)
			blockedOnce = true
		}

		wait := time.Until(resetAt)
		if wait <= 0 {
			wait = 10 * time.Second
		}
		log.Warn().Dur("wait", wait).Msg("all keys exhausted, sleeping until reset")

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
			// loop and try again
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
}

// classifyAndMark inspects an FFLogs error and tells the pool how to react.
func classifyAndMark(lease *keypool.Lease, err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "429"):
		Pool.MarkError(lease, keypool.ErrRateLimited)
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"):
		Pool.MarkError(lease, keypool.ErrUnauthorized)
	default:
		Pool.MarkError(lease, keypool.ErrTransient)
	}
}

// markSynced updates logs_sync_time so the next scan skips this member for
// RecentSyncSkip.
func markSynced(m model.Member) {
	now := time.Now()
	err := flow.DB.Model(&model.Member{}).
		Where("id = ?", m.ID).
		Update("logs_sync_time", now).Error
	if err != nil {
		log.Warn().Err(err).Uint("member_id", m.ID).Msg("update logs_sync_time failed")
	}
}
