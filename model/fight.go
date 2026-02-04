package model

import "time"

type Progress struct {
	Phase    uint `json:"phase"`
	Subphase uint `json:"subphase"`

	EnemyID uint    `json:"enemy_id"`
	EnemyHp float64 `json:"enemy_hp"`
}

type Player struct {
	Name       string `json:"name"`
	Server     string `json:"server"`
	JobID      uint   `json:"job_id"`
	Level      uint   `json:"level"`
	DeathCount uint   `json:"death_count"`
}

type Fight struct {
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`

	ZoneID  uint     `json:"zone_id"`
	Players []Player `json:"players"`

	Clear    bool     `json:"clear"`
	Progress Progress `json:"progress"`
}
