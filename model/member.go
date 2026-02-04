package model

import "time"

type Member struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `gorm:"uniqueIndex:idx_member_name_server" json:"name"`
	Server string `gorm:"uniqueIndex:idx_member_name_server" json:"server"`

	LogsSyncTime *time.Time `json:"logs_sync_time"`
	Hidden       bool       `gorm:"default:false" json:"hidden"`
}
