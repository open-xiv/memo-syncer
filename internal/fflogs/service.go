package fflogs

import (
	"context"
	"memo-uploader-fflog/internal/memo"
	"time"
)

type Service struct {
	client *Client
}

func NewService(client *Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetBestCleanByZone(ctx context.Context, name, server string, zone int) (*memo.FightRecordPayload, error) {
	id, err := s.client.FetchCharacterID(ctx, name, server, "cn")
	if err != nil {
		return nil, err
	}

	fights, err := s.client.FetchBestFightByEncounter(ctx, id, zone)
	if err != nil {
		return nil, err
	}
	reportCode := fights.Data.CharacterData.Character.EncounterRankings.Ranks[0].Report.Code
	fightId := fights.Data.CharacterData.Character.EncounterRankings.Ranks[0].Report.FightID

	detail, err := s.client.FetchFightDetail(ctx, reportCode, fightId)
	if err != nil {
		return nil, err
	}

	return s.mapToMemo(detail), nil
}

func (s *Service) mapToMemo(detail FightDetail) *memo.FightRecordPayload {
	var report = detail.Data.ReportData.Report

	var absoluteStartTimestamp = report.StartTime + report.Fights[0].StartTime
	var absoluteStartTime = time.UnixMilli(int64(absoluteStartTimestamp))

	var duration = int64(report.Table.Data.CombatTime)
	var zone = uint32(report.Zone.Id)
	var isKill = report.Fights[0].Kill

	var playerPayloads []memo.PlayerPayload
	// TODO: query server, jobId, level, and count the death amount
	for i := 0; i < len(report.Table.Data.Composition); i++ {
		var player = report.Table.Data.Composition[i]
		playerPayloads = append(playerPayloads, memo.PlayerPayload{
			Name:       player.Name,
			Server:     "",
			JobID:      0,
			Level:      0,
			DeathCount: 0,
		})
	}

	var enemyHP = report.Fights[0].BossPercentage
	if isKill {
		enemyHP = 0
	}
	var enemyID = uint32(report.Fights[0].EncounterID)

	return &memo.FightRecordPayload{
		StartTime: absoluteStartTime,
		Duration:  duration,
		ZoneID:    zone,
		Players:   playerPayloads,
		IsClear:   isKill,
		Progress: &memo.FightProgressPayload{
			PhaseID:    0,
			SubphaseID: 0,
			EnemyID:    enemyID,
			EnemyHP:    enemyHP,
		},
	}
}
