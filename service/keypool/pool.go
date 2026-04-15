package keypool

import (
	"context"
	"errors"
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/service/fflogs"
	"strings"
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

	// DefaultLimitPerHour is assumed when FFLogs returns 0 from its initial
	// rateLimitData probe (which it does for freshly-created OAuth clients
	// before they've ever made a real query). FFLogs v2 default for new
	// clients appears to be ~5000 points/hour; erring low is safer because
	// Reconcile will correct upward after the first real query warms the
	// account, while starting too high would cause 429s.
	DefaultLimitPerHour = 5000

	// ProbeStagger adds a delay between consecutive key probes at startup
	// so 20+ keys don't burst /oauth/token and trip FFLogs' OAuth rate limit.
	ProbeStagger = 300 * time.Millisecond

	// ProbeRetryStagger is the slower spacing used for the second attempt
	// on keys that failed phase-1 probing — CloudFlare's OAuth rate window
	// has usually reset by the time we get here, but we go slower to avoid
	// tripping it again.
	ProbeRetryStagger = 2 * time.Second

	// ProbeRetryCooldown is how long we wait after phase 1 before starting
	// phase 2 retries, giving FFLogs' OAuth rate window time to reset.
	ProbeRetryCooldown = 10 * time.Second

	// ForbiddenErrThreshold: any logs_keys row whose err_count has reached
	// (or passed) this value is treated as permanently broken — Load skips
	// it, and probe/MarkError bump straight to this number when they see a
	// definitive invalid_client response.
	ForbiddenErrThreshold = uint(100)
)

var (
	ErrNoKey        = errors.New("keypool: no key available")
	ErrAllExhausted = errors.New("keypool: all keys exhausted")
)

// isInvalidClient detects FFLogs' permanent "your OAuth credentials are
// wrong/revoked" error — distinct from transient 429 / 5xx failures.
func isInvalidClient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid_client") ||
		strings.Contains(msg, "unauthorized_client") ||
		strings.Contains(msg, "Client authentication failed")
}

// forbidInDB writes err_count = ForbiddenErrThreshold directly so the next
// Pool.Load filters this row out. Used as a hard-kill for keys that we're
// confident will never work again (invalid_client / 401 / 403).
func forbidInDB(ctx context.Context, keyID uint) {
	if keyID == 0 {
		return // env key, no DB row
	}
	err := flow.DB.WithContext(ctx).
		Model(&model.LogsKey{}).
		Where("id = ?", keyID).
		Update("err_count", ForbiddenErrThreshold).Error
	if err != nil {
		log.Warn().Err(err).Uint("key_id", keyID).Msg("failed to mark key forbidden")
	}
}

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
// for every row. Rows whose OAuth / rate-limit probe fails in phase 1 get
// a second-chance retry in phase 2 after a cooldown — FFLogs' CloudFlare
// layer often 429s the first burst of OAuth token requests, which is not
// a real key failure.
func (p *Pool) Load(ctx context.Context) error {
	var rows []model.LogsKey
	// skip keys already marked forbidden (err_count reached the threshold),
	// so discord donors whose OAuth credentials were revoked don't cost us a
	// probe on every restart
	if err := flow.DB.Where("err_count < ?", ForbiddenErrThreshold).Find(&rows).Error; err != nil {
		return err
	}

	// Phase 1: initial probe with fast stagger.
	states := make([]*KeyState, 0, len(rows))
	for i, row := range rows {
		if i > 0 {
			select {
			case <-time.After(ProbeStagger):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		st := &KeyState{
			ID:       row.ID,
			ClientID: row.Client,
			Client:   fflogs.NewClient(row.Client, row.Secret),
		}
		p.probe(ctx, st)
		states = append(states, st)
	}

	// Phase 2: collect keys that were disabled for transient reasons (i.e.
	// NOT permanently forbidden) and retry them after a cooldown. This
	// gracefully recovers from FFLogs OAuth burst 429s that wrongly killed
	// good keys in phase 1.
	var retryable []*KeyState
	for _, st := range states {
		// Skip keys that hit invalid_client in phase 1 — forbidInDB already
		// bumped their err_count and retrying won't help.
		if st.Disabled && !st.Forbidden {
			retryable = append(retryable, st)
		}
	}
	if len(retryable) > 0 {
		log.Info().
			Int("retryable", len(retryable)).
			Dur("cooldown", ProbeRetryCooldown).
			Msg("phase-1 probes had transient failures, scheduling phase-2 retry")

		select {
		case <-time.After(ProbeRetryCooldown):
		case <-ctx.Done():
			return ctx.Err()
		}

		for i, st := range retryable {
			if i > 0 {
				select {
				case <-time.After(ProbeRetryStagger):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			// Reset the session-disabled flag so probe can re-arm the key
			// on success. Leave ErrCount as-is (it already reflects the
			// failed attempt and will drain via FlushStats normally).
			st.Disabled = false
			p.probe(ctx, st)
		}
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
// Permanent failures (invalid_client) also write err_count=forbidden to DB
// so future Load calls skip the key entirely.
func (p *Pool) probe(ctx context.Context, st *KeyState) {
	data, err := fflogs.FetchRateLimit(ctx, st.Client)
	if err != nil {
		if isInvalidClient(err) {
			log.Warn().Err(err).Uint("key_id", st.ID).Msg("key permanently invalid, marking forbidden")
			st.Disabled = true
			st.Forbidden = true
			st.ErrCount++
			forbidInDB(ctx, st.ID)
			return
		}
		log.Warn().Err(err).Uint("key_id", st.ID).Msg("rate limit probe failed (transient)")
		st.Disabled = true
		st.ErrCount++
		return
	}

	// success: clear the session-disabled flag if this was a phase-2 retry
	st.Disabled = false

	now := time.Now()
	st.Limit = int(data.LimitPerHour)
	// FFLogs returns limitPerHour=0 for freshly-created OAuth clients that
	// haven't made any real queries yet; fall back to the documented default
	// so these keys can participate. Reconcile will replace this with the
	// true limit after the first few queries warm them up.
	if st.Limit == 0 {
		st.Limit = DefaultLimitPerHour
	}
	// round up spent so Remaining() is slightly conservative
	st.SpentEstimate = int(data.PointsSpentThisHour)
	if data.PointsSpentThisHour > float64(st.SpentEstimate) {
		st.SpentEstimate++
	}
	st.ResetAt = now.Add(time.Duration(data.PointsResetIn * float64(time.Second)))
	// if reset window is missing (e.g. Limit defaulted above and server
	// returned 0 seconds), project 1 hour forward so the key isn't stuck
	if !st.ResetAt.After(now) {
		st.ResetAt = now.Add(time.Hour)
	}
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
	st := lease.State
	st.ErrCount++

	var forbid bool

	switch kind {
	case ErrRateLimited:
		// force Remaining() to zero until the known reset
		st.SpentEstimate = st.Limit
		log.Warn().Uint("key_id", st.ID).Time("reset_at", st.ResetAt).Msg("key rate limited")

	case ErrUnauthorized:
		st.Disabled = true
		st.CooldownUntil = time.Now().Add(DisabledCooldown)
		forbid = true
		log.Warn().Uint("key_id", st.ID).Msg("key unauthorized, marking forbidden")

	case ErrTransient:
		// nothing permanent; caller retries with next Acquire
	}

	keyID := st.ID
	p.mu.Unlock()

	if forbid {
		forbidInDB(context.Background(), keyID)
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
		// Only overwrite Limit with a non-zero value. FFLogs sometimes
		// returns limitPerHour=0 mid-session (same quirk as the initial
		// probe). Preserving the prior Limit lets the key keep serving
		// traffic; the spent/reset figures are always trustworthy.
		if incomingLimit := int(data.LimitPerHour); incomingLimit > 0 {
			k.Limit = incomingLimit
		}
		k.SpentEstimate = int(data.PointsSpentThisHour)
		if data.PointsSpentThisHour > float64(k.SpentEstimate) {
			k.SpentEstimate++
		}
		if data.PointsResetIn > 0 {
			k.ResetAt = now.Add(time.Duration(data.PointsResetIn * float64(time.Second)))
		}
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
