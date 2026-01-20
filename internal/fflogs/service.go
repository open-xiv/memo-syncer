package fflogs

import (
	"context"
	"memo-uploader-fflog/internal/memo"
	"time"
)

type Service struct {
	client *Client
	jobs   *Jobs
}

func NewService(client *Client) *Service {
	return &Service{client: client}
}

func (s *Service) Init(ctx context.Context) error {
	jobs, err := s.client.FetchJobs(ctx)
	if err != nil {
		return err
	}
	s.jobs = jobs
	return nil
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

	return s.mapToMemo(*detail), nil
}

func getPlayerServerMap(fight FightDetail) map[string]string {
	nameToServer := make(map[string]string)
	actors := fight.Data.ReportData.Report.MasterData.Actors

	for _, actor := range actors {
		if actor.Server != nil {
			nameToServer[actor.Name] = *actor.Server
		}
	}

	return nameToServer
}

func CountDeathsByName(fight FightDetail) map[string]int {
	deathCounts := make(map[string]int)

	deaths := fight.Data.ReportData.Report.Table.Data.DeathEvents

	for _, event := range deaths {
		deathCounts[event.Name]++
	}

	return deathCounts
}

func (j *Jobs) mapSlugToID() map[string]int {
	slugMap := make(map[string]int)

	for _, class := range j.Data.GameData.Classes {
		for _, spec := range class.Specs {
			slugMap[spec.Slug] = spec.Id
		}
	}
	return slugMap
}

func (s *Service) mapToMemo(detail FightDetail) *memo.FightRecordPayload {
	var report = detail.Data.ReportData.Report

	var absoluteStartTimestamp = report.StartTime + report.Fights[0].StartTime
	var absoluteStartTime = time.UnixMilli(int64(absoluteStartTimestamp))

	var duration = int64(report.Table.Data.CombatTime)
	var zone = uint32(report.Zone.Id)
	var isKill = report.Fights[0].Kill

	// Construct player payloads
	var playerPayloads []memo.PlayerPayload

	jobMap := s.jobs.mapSlugToID()
	serverMap := getPlayerServerMap(detail)
	deathMap := CountDeathsByName(detail)
	for _, player := range report.Table.Data.Composition {
		playerPayloads = append(playerPayloads, memo.PlayerPayload{
			Name:       player.Name,
			Server:     serverMap[player.Name],
			JobID:      uint32(jobMap[player.Type]),
			Level:      100,
			DeathCount: uint32(deathMap[player.Name]),
		})
	}

	// Construct progress payloads
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
