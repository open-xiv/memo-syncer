package keypool

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/open-xiv/memo-syncer/flow"
	"github.com/open-xiv/memo-syncer/model"
	"github.com/open-xiv/memo-syncer/service/fflogs"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var (
	// per-key point reserve kept untouched so concurrent consumers (discord-bot validateKey, periodic rateLimitData) don't push us past FFLogs' hourly limit.
	SafetyBudget = 500

	// initial per-sync point budget subtracted locally before the periodic rateLimitData refresh reconciles drift.
	EstCostPerMember = 20

	ReconcileEvery = 5 * time.Minute

	ReconcileUseThreshold = uint(50)

	DisabledCooldown = 48 * time.Hour

	// FFLogs returns limitPerHour=0 from the initial rateLimitData probe on freshly-created OAuth clients; this is the fallback.
	// erring low is safer because Reconcile will correct upward after the first real query.
	DefaultLimitPerHour = 5000

	// delay between consecutive key probes at startup so 20+ keys don't burst /oauth/token and trip FFLogs' OAuth rate limit.
	ProbeStagger = 300 * time.Millisecond

	// slower spacing used for the second attempt on keys that failed phase-1 probing.
	ProbeRetryStagger = 2 * time.Second

	// wait after phase 1 before starting phase 2 retries, giving FFLogs' OAuth rate window time to reset.
	ProbeRetryCooldown = 10 * time.Second

	// logs_keys rows at or above this err_count are treated as permanently broken — Load skips them.
	// err_count reflects consecutive failures since the last success (see KeyState.ErrCount),
	// so crossing the threshold means a key has failed N times in a row without ever recovering.
	// the threshold is also a runtime kill switch: MarkError disables the key inline once reached.
	// forbidInDB writes this value directly for unambiguously permanent errors (invalid_client / 401 / 403).
	ForbiddenErrThreshold = uint(100)

	// cap on the reset-window duration we accept from FFLogs.
	// FFLogs is documented to use 1h windows; rare malformed responses returning 10⁶s would otherwise back off for days.
	MaxResetIn = time.Hour
)

// clampResetIn turns a (possibly malformed) seconds value from FFLogs into a safe Duration capped at MaxResetIn.
// negative or NaN-ish values fall back to MaxResetIn so the next Reconcile gets a chance to overwrite.
func clampResetIn(secs float64) time.Duration {
	if secs <= 0 || secs > MaxResetIn.Seconds() {
		return MaxResetIn
	}
	return time.Duration(secs * float64(time.Second))
}

var (
	ErrNoKey        = errors.New("keypool: no key available")
	ErrAllExhausted = errors.New("keypool: all keys exhausted")
)

// isInvalidClient detects FFLogs' permanent "your OAuth credentials are wrong/revoked" error — distinct from transient 429 / 5xx failures.
func isInvalidClient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid_client") ||
		strings.Contains(msg, "unauthorized_client") ||
		strings.Contains(msg, "Client authentication failed")
}

// forbidInDB writes err_count = ForbiddenErrThreshold directly so the next Pool.Load filters this row out.
// hard-kill for keys that won't work again (invalid_client / 401 / 403).
func forbidInDB(ctx context.Context, keyID uint) {
	if keyID == 0 {
		return
	}
	err := flow.DB.WithContext(ctx).
		Model(&model.LogsKey{}).
		Where("id = ?", keyID).
		Update("err_count", ForbiddenErrThreshold).Error
	if err != nil {
		log.Warn().Err(err).Uint("key_id", keyID).Msg("failed to mark key forbidden")
	}
}

// Pool is the set of live FFLogs keys shared by all workers. Acquire selects the key with the most remaining quota and returns a lease that must be Released or MarkError'd.
type Pool struct {
	mu   sync.Mutex
	keys []*KeyState
}

func New() *Pool {
	return &Pool{}
}

type Lease struct {
	State  *KeyState
	Client *fflogs.Client
}

// Load reads logs_keys rows and creates clients / fetches rateLimitData for every row.
// rows whose probe fails in phase 1 get a second-chance retry in phase 2 — FFLogs' CloudFlare layer often 429s the first burst of OAuth token requests.
func (p *Pool) Load(ctx context.Context) error {
	var rows []model.LogsKey
	if err := flow.DB.Where("err_count < ?", ForbiddenErrThreshold).Find(&rows).Error; err != nil {
		return err
	}

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

	// phase 2: retry keys disabled for transient reasons (not permanently forbidden) after a cooldown.
	var retryable []*KeyState
	for _, st := range states {
		if st.Disabled && !st.Forbidden {
			retryable = append(retryable, st)
		}
	}
	if len(retryable) > 0 {
		log.Info().
			Int("retryable", len(retryable)).
			Dur("cooldown_ms", ProbeRetryCooldown).
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
			// re-arm so probe can flip Disabled back on success. probe itself resets ErrCount on success
			// (consecutive-failure semantics), so a phase-2 success effectively forgets the phase-1 failure.
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

// AddEnvKey adds a single key built from GRAPH_ID/GRAPH_SECRET as a fallback when the donated pool is empty.
func (p *Pool) AddEnvKey(ctx context.Context, clientID, clientSecret string) error {
	st := &KeyState{
		ID:       0,
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

// probe fetches the initial rateLimitData for a key.
// on failure the key is marked disabled; invalid_client also writes err_count=forbidden to DB so future Load calls skip it.
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

	st.Disabled = false
	// success resets the consecutive-failure counter so a key that had transient hiccups
	// stops drifting toward the forbidden threshold.
	st.ErrCount = 0

	now := time.Now()
	st.Limit = int(data.LimitPerHour)
	// FFLogs returns limitPerHour=0 for freshly-created OAuth clients that haven't made any real queries yet; Reconcile will replace this with the true limit later.
	if st.Limit == 0 {
		st.Limit = DefaultLimitPerHour
	}
	// round up spent so Remaining() is slightly conservative
	st.SpentEstimate = int(data.PointsSpentThisHour)
	if data.PointsSpentThisHour > float64(st.SpentEstimate) {
		st.SpentEstimate++
	}
	st.ResetAt = now.Add(clampResetIn(data.PointsResetIn))
	st.LastRefreshAt = now

	log.Info().
		Uint("key_id", st.ID).
		Int("limit", st.Limit).
		Int("spent", st.SpentEstimate).
		Time("reset_at", st.ResetAt).
		Msg("key probe ok")
}

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

type Summary struct {
	Total          int       `json:"total"`
	Active         int       `json:"active"`
	Disabled       int       `json:"disabled"`
	TotalRemaining int       `json:"total_remaining"`
	TotalLimit     int       `json:"total_limit"`
	EarliestReset  time.Time `json:"earliest_reset"`
}

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
// on exhaustion it returns (nil, nextResetAt, ErrAllExhausted) so the caller can sleep precisely until the earliest reset.
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

// Release settles a successful lease. extraCost adjusts the estimate up or down when the caller knows the actual cost.
// success here resets ErrCount — it's the consecutive-failure counter, so any prior streak is broken.
func (p *Pool) Release(lease *Lease, extraCost int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	st := lease.State
	st.ErrCount = 0
	if extraCost != 0 {
		st.SpentEstimate += extraCost
		if st.SpentEstimate < 0 {
			st.SpentEstimate = 0
		}
	}
}

type ErrorKind int

const (
	ErrTransient ErrorKind = iota
	ErrRateLimited
	ErrUnauthorized
)

// MarkError reports a per-lease failure. the lease is spent — the caller should not use its client again.
// per consecutive-failure semantics, ErrCount accumulates only across UNBROKEN failure runs;
// any Release in between resets it. Reaching ForbiddenErrThreshold here means N failures in a row,
// strong enough evidence to disable the key inline and forbid it in DB so the next Load skips it.
func (p *Pool) MarkError(lease *Lease, kind ErrorKind) {
	p.mu.Lock()
	st := lease.State
	st.ErrCount++

	var forbid bool

	switch kind {
	case ErrRateLimited:
		st.SpentEstimate = st.Limit
		log.Warn().Uint("key_id", st.ID).Time("reset_at", st.ResetAt).Msg("key rate limited")

	case ErrUnauthorized:
		st.Disabled = true
		st.CooldownUntil = time.Now().Add(DisabledCooldown)
		forbid = true
		log.Warn().Uint("key_id", st.ID).Msg("key unauthorized, marking forbidden")

	case ErrTransient:
	}

	if !forbid && st.ErrCount >= ForbiddenErrThreshold {
		st.Disabled = true
		st.CooldownUntil = time.Now().Add(DisabledCooldown)
		forbid = true
		log.Warn().
			Uint("key_id", st.ID).
			Uint("err_count", st.ErrCount).
			Msg("key hit consecutive-failure threshold, marking forbidden")
	}

	keyID := st.ID
	p.mu.Unlock()

	if forbid {
		forbidInDB(context.Background(), keyID)
	}
}

// Reconcile re-queries rateLimitData for keys whose local estimate has drifted (wall clock or usage count) and corrects SpentEstimate.
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
		// FFLogs sometimes returns limitPerHour=0 mid-session (same quirk as the initial probe); preserve prior Limit so the key keeps serving traffic.
		if incomingLimit := int(data.LimitPerHour); incomingLimit > 0 {
			k.Limit = incomingLimit
		}
		k.SpentEstimate = int(data.PointsSpentThisHour)
		if data.PointsSpentThisHour > float64(k.SpentEstimate) {
			k.SpentEstimate++
		}
		if data.PointsResetIn > 0 {
			k.ResetAt = now.Add(clampResetIn(data.PointsResetIn))
		}
		k.LastRefreshAt = now
		k.UsesSinceRefresh = 0
		// successful reconcile counts as a successful FFLogs interaction → break any prior failure streak.
		k.ErrCount = 0
		p.mu.Unlock()
	}
}

// FlushStats persists in-memory counters back to logs_keys.
// use_count is a lifetime monotonic counter, so we flush its delta and zero the local accumulator.
// err_count is the current consecutive-failure run (see KeyState.ErrCount), so we write the absolute
// value and leave the local field alone — it stays as the live counter for the next acquire/release cycle.
func (p *Pool) FlushStats(ctx context.Context) {
	p.mu.Lock()
	type pending struct {
		id       uint
		useDelta uint
		errAbs   uint
	}
	var batch []pending
	for _, k := range p.keys {
		if k.ID == 0 {
			continue
		}
		// skip rows with no activity since last flush. err_count alone changing without any use
		// (e.g. a Reconcile probe failure) waits until a normal lease cycle bumps UseCount,
		// at which point the absolute err_count will be persisted alongside.
		if k.UseCount == 0 {
			continue
		}
		batch = append(batch, pending{id: k.ID, useDelta: k.UseCount, errAbs: k.ErrCount})
	}
	p.mu.Unlock()

	now := time.Now()
	for _, b := range batch {
		err := flow.DB.WithContext(ctx).
			Model(&model.LogsKey{}).
			Where("id = ?", b.id).
			Updates(map[string]any{
				"use_count":   gorm.Expr("use_count + ?", b.useDelta),
				"err_count":   b.errAbs,
				"last_use_at": now,
			}).Error
		if err != nil {
			log.Warn().Err(err).Uint("key_id", b.id).Msg("flush stats failed")
			continue
		}

		p.mu.Lock()
		for _, k := range p.keys {
			if k.ID == b.id {
				if k.UseCount >= b.useDelta {
					k.UseCount -= b.useDelta
				}
				// ErrCount left as-is: it's the live consecutive-failure counter,
				// not a delta to be drained.
				break
			}
		}
		p.mu.Unlock()
	}
}
