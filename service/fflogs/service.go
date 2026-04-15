package fflogs

import (
	"context"
	"errors"
	"memo-syncer/flow"
	"memo-syncer/model"
	"memo-syncer/util"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// CharIDCacheTTL is how long a resolved FFLogs character ID is remembered in
// Redis. The ID itself never changes, so the only reason to expire is to let
// deleted / renamed characters self-heal.
const CharIDCacheTTL = 30 * 24 * time.Hour

// ErrNoProgress is returned by BuildMemberZoneProgress when FFLogs has no
// parse for the requested (charID, encounterID).
var ErrNoProgress = errors.New("fflogs: no progress for zone")

// ResolveCharacterID returns the character ID for (name, server, region),
// consulting Redis first. Cache hits skip the FFLogs round trip entirely.
// Writes back to Redis on a successful lookup.
func ResolveCharacterID(ctx context.Context, c *Client, name, server, region string) (int, error) {
	key := charIDCacheKey(name, server, region)

	if v, err := flow.Redis.Get(ctx, key).Result(); err == nil {
		if id, convErr := strconv.Atoi(v); convErr == nil && id > 0 {
			return id, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		// redis is down — continue to FFLogs, don't fail the sync
	}

	id, err := FetchCharacterID(ctx, c, name, server, region)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, nil
	}

	_ = flow.Redis.Set(ctx, key, strconv.Itoa(id), CharIDCacheTTL).Err()
	return id, nil
}

func charIDCacheKey(name, server, region string) string {
	return "fflogs:char:" + region + ":" + name + "@" + server
}

// BuildMemberZoneProgress takes the result of a single aliased best-fight
// ranking and follows through to fight_detail, returning the fight ready for
// upload. Returns ErrNoProgress when the member has no kills in that zone or
// the underlying report is incomplete.
func BuildMemberZoneProgress(ctx context.Context, c *Client, rank *EncounterRanking) (*model.Fight, error) {
	if rank == nil || len(rank.Ranks) == 0 {
		return nil, ErrNoProgress
	}

	reportCode := rank.Ranks[0].Report.Code
	fightID := rank.Ranks[0].Report.FightID

	detail, err := FetchFightDetail(ctx, c, reportCode, fightID)
	if err != nil {
		return nil, err
	}

	fight := MapToMemo(*detail)
	if fight == nil {
		return nil, ErrNoProgress
	}
	return fight, nil
}

// GroupServer builds a name→server map from report.masterData.actors (fight
// Composition entries don't carry server info, so we have to cross-reference).
func GroupServer(fight FightDetail) map[string]string {
	nameToServer := make(map[string]string)
	for _, actor := range fight.ReportData.Report.MasterData.Actors {
		if actor.Server != nil {
			nameToServer[actor.Name] = *actor.Server
		}
	}
	return nameToServer
}

func GroupDeath(fight FightDetail) map[string]int {
	deathCounts := make(map[string]int)
	for _, event := range fight.ReportData.Report.Table.Data.DeathEvents {
		deathCounts[event.Name]++
	}
	return deathCounts
}

// MapToMemo translates a FFLogs fight report into the memo-server Fight DTO.
// Returns nil when the report is incomplete (empty fights or empty composition)
// — the caller should treat nil the same as ErrNoProgress and skip the member.
func MapToMemo(detail FightDetail) *model.Fight {
	report := detail.ReportData.Report

	// defensive checks: FFLogs may return rankings that point to a deleted or
	// partially-indexed report, in which case Fights[] or Composition[] is empty.
	if len(report.Fights) == 0 {
		return nil
	}
	if len(report.Table.Data.Composition) == 0 {
		return nil
	}

	serverMap := GroupServer(detail)
	deathMap := GroupDeath(detail)

	players := make([]model.Player, 0, len(report.Table.Data.Composition))
	for _, p := range report.Table.Data.Composition {
		players = append(players, model.Player{
			Name:       p.Name,
			Server:     serverMap[p.Name],
			JobID:      util.GetJobID(p.Type),
			Level:      100,
			DeathCount: uint(deathMap[p.Name]),
		})
	}

	first := report.Fights[0]
	isClear := first.Kill
	enemyHP := first.BossPercentage
	if isClear {
		enemyHP = 0
	}

	return &model.Fight{
		StartTime: time.UnixMilli(int64(report.StartTime + first.StartTime)),
		Duration:  time.Duration(report.Table.Data.CombatTime) * time.Millisecond,

		ZoneID:  uint(report.Zone.ID),
		Players: players,

		Clear: isClear,
		Progress: model.Progress{
			Phase:    0,
			Subphase: 0,
			EnemyID:  uint(first.EncounterID),
			EnemyHp:  enemyHP,
		},
	}
}
