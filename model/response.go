package model

import "time"

// ProgressResponse is the `/progress` payload.
//
// Semantics:
//
//   - state:            "idle" | "scanning" | "waiting_for_keys"
//   - total / current:  members counted vs walked by producer in the CURRENT
//     scan. current is a monotonic counter — NOT a DB PK.
//   - last_id:          PK of the most recently produced member (debug aid).
//   - name / server:    identity of the member at last_id (if resolvable).
//   - scan_started_at:  wall-clock of when the current scan began (or last
//     scan if state == idle).
//   - scan_finished_at: wall-clock of when the last scan completed. Zero
//     until the first scan finishes.
//   - next_scan_at:     wall-clock of when the next scan will fire. Only
//     populated while state == idle.
//   - keys_recover_at:  wall-clock of the earliest key reset a waiting
//     worker is blocked on. Only populated while state == waiting_for_keys.
//   - waiting_workers:  how many workers are currently blocked on Pool.Acquire.
//   - keys:             pool digest.
type ProgressResponse struct {
	State string `json:"state"`

	Total   int64 `json:"total"`
	Current int64 `json:"current"`

	// Per-scan breakdown. Resets at the start of each scan so the numbers
	// always reflect "what has this scan done so far". Useful for polling
	// /progress to see real-time activity.
	Counters ScanCounters `json:"counters"`

	LastID uint   `json:"last_id,omitempty"`
	Name   string `json:"name,omitempty"`
	Server string `json:"server,omitempty"`

	ScanStartedAt  *time.Time `json:"scan_started_at,omitempty"`
	ScanFinishedAt *time.Time `json:"scan_finished_at,omitempty"`
	NextScanAt     *time.Time `json:"next_scan_at,omitempty"`
	KeysRecoverAt  *time.Time `json:"keys_recover_at,omitempty"`

	WaitingWorkers int32 `json:"waiting_workers"`

	Keys *KeySummary `json:"keys,omitempty"`
}

type ScanCounters struct {
	FilteredNonCN    int64 `json:"filtered_non_cn"`     // skipped because server is EN/JP/EU
	FilteredRecent   int64 `json:"filtered_recent"`     // skipped because synced within 1h
	Queued           int64 `json:"queued"`              // passed filters and handed to workers
	NoFFLogsChar     int64 `json:"no_fflogs_char"`      // worker resolved charID=0
	MembersWithData  int64 `json:"members_with_data"`   // worker uploaded at least one fight
	FightsUploaded   int64 `json:"fights_uploaded"`     // total POST /fight/ 2xx count
	MemberSyncErrors int64 `json:"member_sync_errors"`  // transient FFLogs/net errors
}

type KeySummary struct {
	Total          int        `json:"total"`
	Active         int        `json:"active"`
	Disabled       int        `json:"disabled"`
	TotalLimit     int        `json:"total_limit"`
	TotalRemaining int        `json:"total_remaining"`
	EarliestReset  *time.Time `json:"earliest_reset,omitempty"`
}

type MemberResponse struct {
	Name         string     `json:"name"`
	Server       string     `json:"server"`
	LastSyncTime *time.Time `json:"last_sync_time"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type StatusResponse struct {
	Status string `json:"status"`
}
