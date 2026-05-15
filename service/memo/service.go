package memo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open-xiv/memo-syncer/flow"
	"github.com/open-xiv/memo-syncer/model"
	"github.com/open-xiv/memo-syncer/service/fflogs"
	"github.com/open-xiv/memo-syncer/service/keypool"

	"github.com/rs/zerolog/log"
)

var (
	Pool *keypool.Pool

	// capped regardless of pool size to avoid overloading FFLogs with too many concurrent connections per donated key.
	WorkerCount = 8

	MemberQueueSize = 64

	RecentSyncSkip = 24 * time.Hour

	// bounds a single member's full pipeline (char_id → best_fights → fight_detail × N → CreateFight × N).
	PerMemberTimeout = 60 * time.Second
)

var (
	TotalMembers   atomic.Int64
	ExpectedQueued atomic.Int64

	Walked atomic.Int64

	FilteredNonCN    atomic.Int64
	FilteredRecent   atomic.Int64
	Queued           atomic.Int64
	MembersNoCharID  atomic.Int64
	MembersWithData  atomic.Int64
	FightsUploaded   atomic.Int64
	MemberSyncErrors atomic.Int64

	LastID atomic.Uint64

	CurrentMember atomic.Pointer[MemberRef]

	lastScanSnapshot atomic.Pointer[ScanSnapshot]
)

type MemberRef struct {
	ID     uint
	Name   string
	Server string
}

type ScanSnapshot struct {
	StartedAt      time.Time
	FinishedAt     time.Time
	DurationMs     int64
	Walked         int64
	ExpectedQueued int64

	FilteredNonCN   int64
	FilteredRecent  int64
	Queued          int64
	NoFFLogsChar    int64
	MembersWithData int64
	FightsUploaded  int64
	Errors          int64
}

func LastScan() *ScanSnapshot {
	return lastScanSnapshot.Load()
}

func resetScanCounters() {
	Walked.Store(0)
	ExpectedQueued.Store(0)
	FilteredNonCN.Store(0)
	FilteredRecent.Store(0)
	Queued.Store(0)
	MembersNoCharID.Store(0)
	MembersWithData.Store(0)
	FightsUploaded.Store(0)
	MemberSyncErrors.Store(0)
	LastID.Store(0)
	CurrentMember.Store(nil)
}

func SyncMembers() error {
	if Pool == nil {
		return errors.New("memo: key pool not initialized")
	}

	markScanStarted()
	scanStart := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resetScanCounters()

	var total int64
	if err := flow.DB.Model(&model.Member{}).Count(&total).Error; err != nil {
		return err
	}
	TotalMembers.Store(total)

	// pre-compute the honest denominator for the /progress ring.
	// "CN" = server in cnWorlds (same allowlist as IsCNServer); "stale" = logs_sync_time IS NULL OR < now() - RecentSyncSkip.
	staleCutoff := time.Now().Add(-RecentSyncSkip)
	var expected int64
	if err := flow.DB.Raw(`
		SELECT COUNT(*) FROM members
		WHERE server IN ?
		  AND (logs_sync_time IS NULL OR logs_sync_time < ?)
	`, CNWorldList(), staleCutoff).Scan(&expected).Error; err != nil {
		log.Warn().Err(err).Msg("expected_queued precount failed; falling back to 0")
		expected = 0
	}
	ExpectedQueued.Store(expected)

	log.Info().
		Int64("total", total).
		Int64("expected_queued", expected).
		Int("workers", WorkerCount).
		Msg("scan started")

	memberCh := make(chan model.Member, MemberQueueSize)

	var prodErr error
	var prodWg sync.WaitGroup
	prodWg.Add(1)
	go func() {
		defer prodWg.Done()
		defer close(memberCh)
		prodErr = produceMembers(ctx, memberCh)
	}()

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

	finishedAt := time.Now()
	snap := &ScanSnapshot{
		StartedAt:       scanStart,
		FinishedAt:      finishedAt,
		DurationMs:      finishedAt.Sub(scanStart).Milliseconds(),
		Walked:          Walked.Load(),
		ExpectedQueued:  ExpectedQueued.Load(),
		FilteredNonCN:   FilteredNonCN.Load(),
		FilteredRecent:  FilteredRecent.Load(),
		Queued:          Queued.Load(),
		NoFFLogsChar:    MembersNoCharID.Load(),
		MembersWithData: MembersWithData.Load(),
		FightsUploaded:  FightsUploaded.Load(),
		Errors:          MemberSyncErrors.Load(),
	}
	lastScanSnapshot.Store(snap)

	log.Info().
		Int64("total", TotalMembers.Load()).
		Int64("walked", snap.Walked).
		Int64("expected_queued", snap.ExpectedQueued).
		Int64("filtered_non_cn", snap.FilteredNonCN).
		Int64("filtered_recent", snap.FilteredRecent).
		Int64("queued", snap.Queued).
		Int64("no_fflogs_char", snap.NoFFLogsChar).
		Int64("with_data", snap.MembersWithData).
		Int64("fights_uploaded", snap.FightsUploaded).
		Int64("errors", snap.Errors).
		Dur("duration_ms", time.Since(scanStart)).
		Msg("scan completed")

	CurrentMember.Store(nil)

	return prodErr
}

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
			Walked.Add(1)
			CurrentMember.Store(&MemberRef{
				ID:     m.ID,
				Name:   m.Name,
				Server: m.Server,
			})

			switch filterReason(m) {
			case filterKeep:
				Queued.Add(1)
				select {
				case out <- m:
				case <-ctx.Done():
					return ctx.Err()
				}
			case filterNonCN:
				FilteredNonCN.Add(1)
			case filterRecent:
				FilteredRecent.Add(1)
			}
		}
	}
}

type filterResult int

const (
	filterKeep filterResult = iota
	filterNonCN
	filterRecent
)

func filterReason(m model.Member) filterResult {
	if m.LogsSyncTime != nil && time.Since(*m.LogsSyncTime) < RecentSyncSkip {
		return filterRecent
	}
	if !IsCNServer(m.Server) {
		return filterNonCN
	}
	return filterKeep
}

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

func syncOneMember(ctx context.Context, m model.Member) error {
	memCtx, cancel := context.WithTimeout(ctx, PerMemberTimeout)
	defer cancel()

	lease, err := waitForKey(memCtx)
	if err != nil {
		return err
	}

	charID, err := fflogs.ResolveCharacterID(memCtx, lease.Client, m.Name, m.Server, "cn")
	if err != nil {
		MemberSyncErrors.Add(1)
		classifyAndMark(lease, err)
		return err
	}
	if charID == 0 {
		// no fflogs character — mark as "synced" so we don't re-query for RecentSyncSkip
		MembersNoCharID.Add(1)
		markSynced(m)
		return nil
	}

	enc := [4]int{}
	for i, z := range InterestZones {
		enc[i] = z.LogsID
	}
	best, err := fflogs.FetchBestFights(memCtx, lease.Client, charID, enc[0], enc[1], enc[2], enc[3])
	if err != nil {
		MemberSyncErrors.Add(1)
		classifyAndMark(lease, err)
		return err
	}

	uploadedZones := 0
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
			MemberSyncErrors.Add(1)
			classifyAndMark(lease, err)
			return err
		}

		fight.ZoneID = z.ZoneID
		for pi := range fight.Players {
			fight.Players[pi].Level = z.Level
		}

		if err := CreateFight(memCtx, fight); err != nil {
			MemberSyncErrors.Add(1)
			log.Error().Err(err).
				Str("member", m.Name+"@"+m.Server).
				Uint("zone", z.ZoneID).
				Msg("create fight failed")
			// memo-server errors aren't key errors — don't penalize the key
			continue
		}
		FightsUploaded.Add(1)
		uploadedZones++
	}

	// extra debit: each zone with data costs ~10 points for fight_detail, on top of the pre-debited 20 for char_id + best_fights.
	zonesWithData := 0
	for i := range InterestZones {
		if r := best.Zone(i); r != nil && len(r.Ranks) > 0 {
			zonesWithData++
		}
	}
	Pool.Release(lease, zonesWithData*10)

	if uploadedZones > 0 {
		MembersWithData.Add(1)
		log.Info().
			Str("member", m.Name+"@"+m.Server).
			Int("zones", uploadedZones).
			Msg("member synced")
	}

	markSynced(m)
	return nil
}

// waitForKey blocks until the pool hands out a lease or the context expires.
// flips the global state to `waiting_for_keys` while blocked so /progress can surface the condition.
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

		if !blockedOnce {
			markWaitingForKeys(+1, resetAt)
			blockedOnce = true
		}

		wait := time.Until(resetAt)
		if wait <= 0 {
			wait = 10 * time.Second
		}
		// defensive ceiling: if a key's ResetAt slipped through the clamp in pool.go, don't sleep for days.
		// re-check Acquire on a sane cadence; Reconcile will have refreshed real state by then.
		const maxWait = time.Hour
		if wait > maxWait {
			log.Warn().Dur("raw_wait_ms", wait).Dur("capped_ms", maxWait).Msg("acquire wait absurdly long; capping")
			wait = maxWait
		}
		log.Warn().Dur("wait_ms", wait).Msg("all keys exhausted, sleeping until reset")

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
}

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

func markSynced(m model.Member) {
	now := time.Now()
	err := flow.DB.Model(&model.Member{}).
		Where("id = ?", m.ID).
		Update("logs_sync_time", now).Error
	if err != nil {
		log.Warn().Err(err).Uint("member_id", m.ID).Msg("update logs_sync_time failed")
	}
}
