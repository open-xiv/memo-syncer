package memo

import (
	"sync"
	"sync/atomic"
	"time"
)

// SyncState is the coarse-grained state of the background sync loop.
// Reads and writes go through an atomic Int32.
type SyncState int32

const (
	StateIdle            SyncState = 0 // between scans, sleeping
	StateScanning        SyncState = 1 // actively syncing members
	StateWaitingForKeys  SyncState = 2 // scan is blocked on key exhaustion
)

func (s SyncState) String() string {
	switch s {
	case StateScanning:
		return "scanning"
	case StateWaitingForKeys:
		return "waiting_for_keys"
	default:
		return "idle"
	}
}

// stateStore centralizes the variable bag so handlers can take a consistent
// snapshot without a dozen independent atomic reads.
var stateStore = struct {
	mu sync.Mutex

	state         atomic.Int32 // SyncState
	waitingCount  atomic.Int32 // # workers currently blocked in waitForKey

	scanStartedAt  time.Time
	scanFinishedAt time.Time
	nextScanAt     time.Time
	keysRecoverAt  time.Time // set whenever a worker actually blocks on Acquire
}{}

// Snapshot is a point-in-time view of the sync loop exposed to /progress.
type Snapshot struct {
	State           string    // "idle" | "scanning" | "waiting_for_keys"
	WaitingWorkers  int32     // >0 means at least one worker is blocked on the pool
	ScanStartedAt   time.Time // zero when never started
	ScanFinishedAt  time.Time // zero during first scan
	NextScanAt      time.Time // zero while scanning
	KeysRecoverAt   time.Time // zero unless all keys exhausted right now
}

func CurrentSnapshot() Snapshot {
	stateStore.mu.Lock()
	defer stateStore.mu.Unlock()

	return Snapshot{
		State:          SyncState(stateStore.state.Load()).String(),
		WaitingWorkers: stateStore.waitingCount.Load(),
		ScanStartedAt:  stateStore.scanStartedAt,
		ScanFinishedAt: stateStore.scanFinishedAt,
		NextScanAt:     stateStore.nextScanAt,
		KeysRecoverAt:  stateStore.keysRecoverAt,
	}
}

func markScanStarted() {
	stateStore.mu.Lock()
	defer stateStore.mu.Unlock()
	stateStore.state.Store(int32(StateScanning))
	stateStore.scanStartedAt = time.Now()
	stateStore.nextScanAt = time.Time{}
}

// MarkIdle transitions the loop to idle and records when the next scan will
// fire. Called by the outer loop after SyncMembers returns.
func MarkIdle(nextScanAt time.Time) {
	stateStore.mu.Lock()
	defer stateStore.mu.Unlock()
	stateStore.state.Store(int32(StateIdle))
	stateStore.scanFinishedAt = time.Now()
	stateStore.nextScanAt = nextScanAt
	stateStore.keysRecoverAt = time.Time{}
}

// markWaitingForKeys transitions the loop into WaitingForKeys when at least
// one worker blocks on Acquire. Called with waitingDelta=+1 on entry and -1
// on exit — once waitingCount drops back to zero, we flip back to Scanning.
func markWaitingForKeys(delta int32, recoverAt time.Time) {
	stateStore.mu.Lock()
	defer stateStore.mu.Unlock()

	count := stateStore.waitingCount.Add(delta)
	if delta > 0 {
		stateStore.keysRecoverAt = recoverAt
		if count == 1 {
			stateStore.state.Store(int32(StateWaitingForKeys))
		}
	} else if count == 0 {
		stateStore.state.Store(int32(StateScanning))
		stateStore.keysRecoverAt = time.Time{}
	}
}
