package keypool

import (
	"memo-syncer/service/fflogs"
	"time"
)

// KeyState holds live information about a single donated FFLogs key.
// Mutations happen under Pool.mu.
type KeyState struct {
	ID       uint
	ClientID string
	Client   *fflogs.Client

	// quota (in FFLogs "points")
	Limit         int       // from rateLimitData.limitPerHour
	SpentEstimate int       // local running estimate
	ResetAt       time.Time // when SpentEstimate should roll back to 0

	// bookkeeping
	LastRefreshAt    time.Time // last time we re-queried rateLimitData
	UsesSinceRefresh uint      // resets on Reconcile, triggers threshold refresh
	UseCount         uint      // drained by FlushStats, persisted to DB
	ErrCount         uint      // drained by FlushStats, persisted to DB

	// lifecycle
	Disabled bool      // 401/403 or admin kill switch
	CooldownUntil time.Time // per-error cooldown (e.g. 429 retry-after)
}

// Remaining is how many points this key can still spend before the next reset.
func (k *KeyState) Remaining() int {
	r := k.Limit - k.SpentEstimate
	if r < 0 {
		return 0
	}
	return r
}

// Available reports whether the key can be handed out right now given the
// caller's expected query cost plus the global safety margin.
func (k *KeyState) Available(now time.Time, minBudget int) bool {
	if k.Disabled {
		return false
	}
	if now.Before(k.CooldownUntil) {
		return false
	}
	return k.Remaining() >= minBudget
}

// RollReset clears SpentEstimate if we've crossed into the next reset window.
func (k *KeyState) RollReset(now time.Time) {
	if !k.ResetAt.IsZero() && now.After(k.ResetAt) {
		k.SpentEstimate = 0
		k.ResetAt = now.Add(time.Hour)
	}
}
