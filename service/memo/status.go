package memo

import (
	"sync"
	"sync/atomic"
	"time"
)

type SyncState int32

const (
	StateIdle           SyncState = 0
	StateScanning       SyncState = 1
	StateWaitingForKeys SyncState = 2
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

var stateStore = struct {
	mu sync.Mutex

	state        atomic.Int32
	waitingCount atomic.Int32

	scanStartedAt  time.Time
	scanFinishedAt time.Time
	nextScanAt     time.Time
	keysRecoverAt  time.Time
}{}

type Snapshot struct {
	State          string
	WaitingWorkers int32
	ScanStartedAt  time.Time
	ScanFinishedAt time.Time
	NextScanAt     time.Time
	KeysRecoverAt  time.Time
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

func MarkIdle(nextScanAt time.Time) {
	stateStore.mu.Lock()
	defer stateStore.mu.Unlock()
	stateStore.state.Store(int32(StateIdle))
	stateStore.scanFinishedAt = time.Now()
	stateStore.nextScanAt = nextScanAt
	stateStore.keysRecoverAt = time.Time{}
}

// markWaitingForKeys flips the state to WaitingForKeys when any worker blocks on Acquire.
// called with delta=+1 on entry and -1 on exit; flips back to Scanning when waitingCount hits zero.
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
