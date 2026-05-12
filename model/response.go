package model

import "time"

// ProgressResponse is the payload for GET /progress/. shape is defined in /openapi.yaml.
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

type CurrentScanResponse struct {
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	Walked         int64            `json:"walked"`
	ExpectedQueued int64            `json:"expected_queued"`
	AtMember       *MemberRefDTO    `json:"at_member,omitempty"`
	Counters       CountersResponse `json:"counters"`
}

type LastScanResponse struct {
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     time.Time        `json:"finished_at"`
	DurationMs     int64            `json:"duration_ms"`
	Walked         int64            `json:"walked"`
	ExpectedQueued int64            `json:"expected_queued"`
	Counters       CountersResponse `json:"counters"`
}

type CountersResponse struct {
	FilteredNonCN   int64 `json:"filtered_non_cn"`
	FilteredRecent  int64 `json:"filtered_recent"`
	Queued          int64 `json:"queued"`
	NoFFLogsChar    int64 `json:"no_fflogs_char"`
	MembersWithData int64 `json:"members_with_data"`
	FightsUploaded  int64 `json:"fights_uploaded"`
	Errors          int64 `json:"errors"`
}

type MemberRefDTO struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Server string `json:"server"`
}

type WorkerStatsResponse struct {
	Count   int   `json:"count"`
	Waiting int32 `json:"waiting"`
}

type KeyStatsResponse struct {
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
