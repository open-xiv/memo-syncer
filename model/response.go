package model

import "time"

type ProgressResponse struct {
	Total   int64  `json:"total"`
	Current uint   `json:"current"`
	Name    string `json:"name"`
	Server  string `json:"server"`
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
