package memo

import "time"

type FightRecordPayload struct {
	StartTime time.Time             `json:"start_time"`
	Duration  int64                 `json:"duration"`
	ZoneID    uint32                `json:"zone_id"`
	Players   []PlayerPayload       `json:"players"`
	IsClear   bool                  `json:"clear"`
	Progress  *FightProgressPayload `json:"progress"`
}

type PlayerPayload struct {
	Name       string `json:"name"`
	Server     string `json:"server"`
	JobID      uint32 `json:"job_id"`
	Level      uint32 `json:"level"`
	DeathCount uint32 `json:"death_count"`
}

type FightProgressPayload struct {
	PhaseID    uint32  `json:"phase"`
	SubphaseID uint32  `json:"subphase"`
	EnemyID    uint32  `json:"enemy_id"`
	EnemyHP    float64 `json:"enemy_hp"`
}
