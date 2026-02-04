package fflogs

import (
	"context"
	"memo-syncer/model"
	"time"

	"github.com/rs/zerolog/log"
)

var JobMap *Jobs

func InitFFLogs() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jobs, err := FetchJobs(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch jobs")
	}

	JobMap = jobs
}

func GetMemberZoneBestProgress(ctx context.Context, name, server string, zoneID int) (*model.Fight, error) {
	id, err := FetchCharacterID(ctx, name, server, "cn")
	if err != nil {
		return nil, err
	}

	fights, err := FetchBestFightByEncounter(ctx, id, zoneID)
	if err != nil {
		return nil, err
	}

	if len(fights.CharacterData.Character.EncounterRankings.Ranks) == 0 {
		return nil, nil
	}

	reportCode := fights.CharacterData.Character.EncounterRankings.Ranks[0].Report.Code
	fightID := fights.CharacterData.Character.EncounterRankings.Ranks[0].Report.FightID

	detail, err := FetchFightDetail(ctx, reportCode, fightID)
	if err != nil {
		return nil, err
	}

	return MapToMemo(*detail), nil
}

func GroupServer(fight FightDetail) map[string]string {
	nameToServer := make(map[string]string)
	actors := fight.ReportData.Report.MasterData.Actors

	for _, actor := range actors {
		if actor.Server != nil {
			nameToServer[actor.Name] = *actor.Server
		}
	}

	return nameToServer
}

func GroupDeath(fight FightDetail) map[string]int {
	deathCounts := make(map[string]int)

	deaths := fight.ReportData.Report.Table.Data.DeathEvents

	for _, event := range deaths {
		deathCounts[event.Name]++
	}

	return deathCounts
}

func GroupJob() map[string]int {
	slugMap := make(map[string]int)

	for _, class := range JobMap.GameData.Classes {
		for _, spec := range class.Specs {
			slugMap[spec.Slug] = spec.Id
		}
	}
	return slugMap
}

func MapToMemo(detail FightDetail) *model.Fight {
	var report = detail.ReportData.Report

	// group map
	jobMap := GroupJob()
	serverMap := GroupServer(detail)
	deathMap := GroupDeath(detail)

	// players
	var playerPayloads []model.Player
	for _, player := range report.Table.Data.Composition {
		playerPayloads = append(playerPayloads, model.Player{
			Name:       player.Name,
			Server:     serverMap[player.Name],
			JobID:      uint(jobMap[player.Type]),
			Level:      100,
			DeathCount: uint(deathMap[player.Name]),
		})
	}

	// progress
	isClear := report.Fights[0].Kill
	var enemyHP = report.Fights[0].BossPercentage
	if isClear {
		enemyHP = 0
	}

	// fight
	return &model.Fight{
		StartTime: time.UnixMilli(int64(report.StartTime + report.Fights[0].StartTime)),
		Duration:  time.Duration(report.Table.Data.CombatTime) * time.Millisecond,

		ZoneID:  uint(report.Zone.Id),
		Players: playerPayloads,

		Clear: isClear,
		Progress: model.Progress{
			Phase:    0,
			Subphase: 0,
			EnemyID:  uint(report.Fights[0].EncounterID),
			EnemyHp:  enemyHP,
		},
	}
}
