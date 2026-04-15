package keypool

import (
	"context"
	"errors"
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/fflogs"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Tunables. Exposed as vars (not consts) so tests can tweak them.
var (
	// SafetyBudget is the per-key point reserve kept untouched so concurrent
	// consumers (discord-bot validateKey, periodic rateLimitData, etc.) don't
	// drive us past FFLogs' hourly limit.
	SafetyBudget = 500

	// EstCostPerMember is the initial per-sync point budget we subtract
	// locally before the periodic rateLimitData refresh reconciles drift.
	// One member sync is roughly character_id + best_fight + fight_detail.
	EstCostPerMember = 20

	// ReconcileEvery is how often each key re-queries rateLimitData to
	// correct local drift (wall clock).
	ReconcileEvery = 5 * time.Minute

	// ReconcileUseThreshold triggers an earlier reconcile once a key has
	// served this many acquisitions since its last refresh.
	ReconcileUseThreshold = uint(50)

	// DisabledCooldown is how long a key stays dark after 401/403.
	DisabledCooldown = 48 * time.Hour
)

var (
	ErrNoKey       = errors.New("keypool: no key available")
	ErrAllExhausted = errors.New("keypool: all keys exhausted")
)

// Pool is the set of live FFLogs keys shared by all workers. Acquire selects
// the key with the most remaining quota and returns a lease that must be
// Released (or MarkError'd) when the work is done.
type Pool struct {
	mu   sync.Mutex
	keys []*KeyState
}

func New() *Pool {
	return &Pool{}
}

// Lease is handed back to the caller. It carries the client to use plus a
// handle the caller needs to settle the quota with when the work finishes.
type Lease struct {
	State  *KeyState
	Client *fflogs.Client
}

// Load reads logs_keys rows and creates LogsClients / fetches rateLimitData
// for every row. Rows whose OAuth / rate-limit probe fails get flagged
// Disabled and their ErrCount bumped, but they still live in the pool so we
// keep the DB mapping stable.
func (p *Pool) Load(ctx context.Context) error {
	var rows []model.LogsKey
	if err := flow.DB.Find(&rows).Error; err != nil {
		return err
	}

	states := make([]*KeyState, 0, len(rows))
	for _, row := range rows {
		st := &KeyState{
			ID:       row.ID,
			ClientID: row.Client,
			Client:   fflogs.NewClient(row.Client, row.Secret),
		}
		p.probe(ctx, st)
		states = append(states, st)
	}

	p.mu.Lock()
	p.keys = states
	p.mu.Unlock()

	active, disabled := p.Stats()
	log.Info().
		Int("total", len(states)).
		Int("active", active).
		Int("disabled", disabled).
		Msg("keypool loaded")

	if active == 0 {
		return ErrNoKey
	}
	return nil
}

// AddEnvKey adds a single key built from GRAPH_ID/GRAPH_SECRET as a fallback
// when the donated pool is empty.
func (p *Pool) AddEnvKey(ctx context.Context, clientID, clientSecret string) error {
	st := &KeyState{
		ID:       0, // 0 == env key, no DB writeback
		ClientID: clientID,
		Client:   fflogs.NewClient(clientID, clientSecret),
	}
	p.probe(ctx, st)

	p.mu.Lock()
	p.keys = append(p.keys, st)
	p.mu.Unlock()

	if st.Disabled {
		return errors.New("keypool: env key probe failed")
	}
	return nil
}

// probe fetches the initial rateLimitData for a key. On failure the key is
// marked disabled so a single bad donation doesn't take down startup.
func (p *Pool) probe(ctx context.Context, st *KeyState) {
	data, err := fflogs.FetchRateLimit(ctx, st.Client)
	if err != nil {
		log.Warn().Err(err).Uint("key_id", st.ID).Msg("initial rate limit probe failed, disabling")
		st.Disabled = true
		st.ErrCount++
		return
	}

	now := time.Now()
	st.Limit = data.LimitPerHour
	st.SpentEstimate = data.PointsSpentThisHour
	st.ResetAt = now.Add(time.Duration(data.PointsResetIn) * time.Second)
	st.LastRefreshAt = now

	log.Info().
		Uint("key_id", st.ID).
		Int("limit", st.Limit).
		Int("spent", st.SpentEstimate).
		Time("reset_at", st.ResetAt).
		Msg("key probe ok")
}

// Stats reports (active, disabled) counts.
func (p *Pool) Stats() (active, disabled int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, k := range p.keys {
		if k.Disabled {
			disabled++
		} else {
			active++
		}
	}
	return
}

// Summary is a snapshot of the pool used by the /progress handler.
type Summary struct {
	Total          int       `json:"total"`
	Active         int       `json:"active"`
	Disabled       int       `json:"disabled"`
	TotalRemaining int       `json:"total_remaining"`  // sum of (Limit - SpentEstimate) across active keys
	TotalLimit     int       `json:"total_limit"`      // sum of Limit across active keys
	EarliestReset  time.Time `json:"earliest_reset"`   // earliest ResetAt among active keys (empty if none)
}

// PoolSummary returns a cheap snapshot for observability.
func (p *Pool) PoolSummary() Summary {
	p.mu.Lock()
	defer p.mu.Unlock()

	var sum Summary
	sum.Total = len(p.keys)
	for _, k := range p.keys {
		if k.Disabled {
			sum.Disabled++
			continue
		}
		sum.Active++
		sum.TotalLimit += k.Limit
		sum.TotalRemaining += k.Remaining()
		if sum.EarliestReset.IsZero() || k.ResetAt.Before(sum.EarliestReset) {
			sum.EarliestReset = k.ResetAt
		}
	}
	return sum
}

// Acquire returns a lease for the key with the most remaining quota.
// On exhaustion it returns (nil, nextResetAt, ErrAllExhausted) so the caller
// can sleep precisely until the earliest reset.
func (p *Pool) Acquire() (*Lease, time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return nil, time.Time{}, ErrNoKey
	}

	now := time.Now()

	var best *KeyState
	bestRemaining := -1
	earliestReset := time.Time{}

	for _, k := range p.keys {
		k.RollReset(now)

		if !k.Available(now, SafetyBudget+EstCostPerMember) {
			// track earliest recoverable reset for callers to wait on
			if !k.Disabled {
				candidate := k.ResetAt
				if now.Before(k.CooldownUntil) && k.CooldownUntil.Before(candidate) {
					candidate = k.CooldownUntil
				}
				if earliestReset.IsZero() || candidate.Before(earliestReset) {
					earliestReset = candidate
				}
			}
			continue
		}

		if r := k.Remaining(); r > bestRemaining {
			best = k
			bestRemaining = r
		}
	}

	if best == nil {
		return nil, earliestReset, ErrAllExhausted
	}

	// pre-debit so concurrent acquirers see the reduction immediately
	best.SpentEstimate += EstCostPerMember
	best.UseCount++
	best.UsesSinceRefresh++

	return &Lease{State: best, Client: best.Client}, time.Time{}, nil
}

// Release settles a successful lease. The optional extraCost lets callers
// correct the estimate up or down if they know the actual cost (we already
// pre-debited EstCostPerMember in Acquire, so pass the delta).
func (p *Pool) Release(lease *Lease, extraCost int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	st := lease.State
	if extraCost != 0 {
		st.SpentEstimate += extraCost
		if st.SpentEstimate < 0 {
			st.SpentEstimate = 0
		}
	}
}

// ErrorKind categorizes FFLogs failures so the pool can decide what to do
// with the offending key.
type ErrorKind int

const (
	// ErrTransient is a network / 5xx error. The key is fine; the caller
	// should retry (usually with a fresh Acquire).
	ErrTransient ErrorKind = iota
	// ErrRateLimited is a 429. Lock the key until its known reset time.
	ErrRateLimited
	// ErrUnauthorized is a 401/403. Disable the key for DisabledCooldown.
	ErrUnauthorized
)

// MarkError reports a per-lease failure. The lease is spent — the caller
// should not use its client again.
func (p *Pool) MarkError(lease *Lease, kind ErrorKind) {
	p.mu.Lock()
	defer p.mu.Unlock()

	st := lease.State
	st.ErrCount++

	switch kind {
	case ErrRateLimited:
		// force Remaining() to zero until the known reset
		st.SpentEstimate = st.Limit
		log.Warn().Uint("key_id", st.ID).Time("reset_at", st.ResetAt).Msg("key rate limited")

	case ErrUnauthorized:
		st.Disabled = true
		st.CooldownUntil = time.Now().Add(DisabledCooldown)
		log.Warn().Uint("key_id", st.ID).Msg("key unauthorized, disabled for 24h")

	case ErrTransient:
		// nothing permanent; caller retries with next Acquire
	}
}

// Reconcile re-queries rateLimitData for keys whose local estimate has drifted
// (either by wall clock or by usage count) and corrects SpentEstimate.
// Called from a background goroutine on a short tick.
func (p *Pool) Reconcile(ctx context.Context) {
	p.mu.Lock()
	due := make([]*KeyState, 0, len(p.keys))
	now := time.Now()
	for _, k := range p.keys {
		if k.Disabled {
			continue
		}
		if now.Sub(k.LastRefreshAt) >= ReconcileEvery || k.UsesSinceRefresh >= ReconcileUseThreshold {
			due = append(due, k)
		}
	}
	p.mu.Unlock()

	if len(due) == 0 {
		return
	}

	for _, k := range due {
		data, err := fflogs.FetchRateLimit(ctx, k.Client)
		if err != nil {
			log.Warn().Err(err).Uint("key_id", k.ID).Msg("reconcile probe failed")
			p.mu.Lock()
			k.ErrCount++
			p.mu.Unlock()
			continue
		}

		now := time.Now()
		p.mu.Lock()
		k.Limit = data.LimitPerHour
		k.SpentEstimate = data.PointsSpentThisHour
		k.ResetAt = now.Add(time.Duration(data.PointsResetIn) * time.Second)
		k.LastRefreshAt = now
		k.UsesSinceRefresh = 0
		p.mu.Unlock()
	}
}

// FlushStats persists in-memory UseCount / ErrCount increments back to
// logs_keys, then zeros the local counters. Called periodically so DB write
// amplification stays low.
func (p *Pool) FlushStats(ctx context.Context) {
	p.mu.Lock()
	type pending struct {
		id       uint
		useDelta uint
		errDelta uint
	}
	var batch []pending
	for _, k := range p.keys {
		if k.ID == 0 { // env key, no row
			continue
		}
		if k.UseCount == 0 && k.ErrCount == 0 {
			continue
		}
		batch = append(batch, pending{id: k.ID, useDelta: k.UseCount, errDelta: k.ErrCount})
	}
	p.mu.Unlock()

	now := time.Now()
	for _, b := range batch {
		err := flow.DB.WithContext(ctx).
			Model(&model.LogsKey{}).
			Where("id = ?", b.id).
			Updates(map[string]any{
				"use_count":   gorm.Expr("use_count + ?", b.useDelta),
				"err_count":   gorm.Expr("err_count + ?", b.errDelta),
				"last_use_at": now,
			}).Error
		if err != nil {
			log.Warn().Err(err).Uint("key_id", b.id).Msg("flush stats failed")
			continue
		}

		// successfully flushed: zero local counters so we don't double-count
		p.mu.Lock()
		for _, k := range p.keys {
			if k.ID == b.id {
				if k.UseCount >= b.useDelta {
					k.UseCount -= b.useDelta
				}
				if k.ErrCount >= b.errDelta {
					k.ErrCount -= b.errDelta
				}
				break
			}
		}
		p.mu.Unlock()
	}
}
