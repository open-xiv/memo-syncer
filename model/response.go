package model

import "time"

// ProgressResponse is the payload for GET /progress/.
//
// Shape:
//   - top-level state + one-sentence totals
//   - scan:       live counters for the in-flight scan (resets at scan start)
//   - last_scan:  frozen snapshot of the previous completed scan
//                 (null before the very first scan finishes)
//   - next_scan_at:    only populated when state == "idle"
//   - keys_recover_at: only populated when state == "waiting_for_keys"
//   - workers:    worker pool stats
//   - keys:       FFLogs donated-key pool stats
//
// Refer to openapi.yaml for the full schema.
type ProgressResponse struct {
	State string `json:"state"`

	TotalMembers int64 `json:"total_members"`

	Scan     CurrentScanResponse `json:"scan"`
	LastScan *LastScanResponse   `json:"last_scan,omitempty"`

	NextScanAt    *time.Time `json:"next_scan_at,omitempty"`
	KeysRecoverAt *time.Time `json:"keys_recover_at,omitempty"`

	Workers WorkerStatsResponse `json:"workers"`
	Keys    *KeyStatsResponse   `json:"keys,omitempty"`
}

// CurrentScanResponse holds the live state of the scan in progress. When the
// loop is idle, these fields still carry the most recently observed values
// (they reset only at the next scan start), but the authoritative view of
// the previous scan is LastScanResponse.
type CurrentScanResponse struct {
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	Walked         int64            `json:"walked"`
	ExpectedQueued int64            `json:"expected_queued"`
	AtMember       *MemberRefDTO    `json:"at_member,omitempty"`
	Counters       CountersResponse `json:"counters"`
}

// LastScanResponse is a frozen snapshot of the previous completed scan.
type LastScanResponse struct {
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     time.Time        `json:"finished_at"`
	DurationMs     int64            `json:"duration_ms"`
	Walked         int64            `json:"walked"`
	ExpectedQueued int64            `json:"expected_queued"`
	Counters       CountersResponse `json:"counters"`
}

// CountersResponse is the breakdown of outcomes inside a scan.
type CountersResponse struct {
	FilteredNonCN   int64 `json:"filtered_non_cn"`
	FilteredRecent  int64 `json:"filtered_recent"`
	Queued          int64 `json:"queued"`
	NoFFLogsChar    int64 `json:"no_fflogs_char"`
	MembersWithData int64 `json:"members_with_data"`
	FightsUploaded  int64 `json:"fights_uploaded"`
	Errors          int64 `json:"errors"`
}

// MemberRefDTO is a minimal member identity for display in /progress.
type MemberRefDTO struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Server string `json:"server"`
}

// WorkerStatsResponse is the per-scan worker pool state.
type WorkerStatsResponse struct {
	Count   int   `json:"count"`
	Waiting int32 `json:"waiting"`
}

// KeyStatsResponse is the FFLogs donated key pool health.
type KeyStatsResponse struct {
	Total          int        `json:"total"`
	Active         int        `json:"active"`
	Disabled       int        `json:"disabled"`
	TotalLimit     int        `json:"total_limit"`
	TotalRemaining int        `json:"total_remaining"`
	EarliestReset  *time.Time `json:"earliest_reset,omitempty"`
}

// MemberResponse is the payload for GET /member/:name.
type MemberResponse struct {
	Name         string     `json:"name"`
	Server       string     `json:"server"`
	LastSyncTime *time.Time `json:"last_sync_time"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Status string `json:"status"`
}
