package keypool

import (
	"time"

	"github.com/open-xiv/memo-syncer/service/fflogs"
)

// KeyState holds live information about a single donated FFLogs key. mutations happen under Pool.mu.
type KeyState struct {
	ID       uint
	ClientID string
	Client   *fflogs.Client

	Limit         int
	SpentEstimate int
	ResetAt       time.Time

	LastRefreshAt    time.Time
	UsesSinceRefresh uint
	UseCount         uint
	ErrCount         uint

	Disabled      bool
	Forbidden     bool
	CooldownUntil time.Time
}

func (k *KeyState) Remaining() int {
	r := k.Limit - k.SpentEstimate
	if r < 0 {
		return 0
	}
	return r
}

func (k *KeyState) Available(now time.Time, minBudget int) bool {
	if k.Disabled {
		return false
	}
	if now.Before(k.CooldownUntil) {
		return false
	}
	return k.Remaining() >= minBudget
}

func (k *KeyState) RollReset(now time.Time) {
	if !k.ResetAt.IsZero() && now.After(k.ResetAt) {
		k.SpentEstimate = 0
		k.ResetAt = now.Add(time.Hour)
	}
}
