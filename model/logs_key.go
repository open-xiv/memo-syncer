package model

import "time"

// LogsKey mirrors the discord-bot schema. Schema is owned by memo-discord-bot;
// syncer only reads (client/secret) and writes back stats (last_use_at/use_count/err_count).
type LogsKey struct {
	ID uint `gorm:"primaryKey"`

	UserID uint `gorm:"uniqueIndex"`

	Client string `gorm:"uniqueIndex"`
	Secret string

	UpdatedAt time.Time
	LastUseAt time.Time

	UseCount uint
	ErrCount uint
}
