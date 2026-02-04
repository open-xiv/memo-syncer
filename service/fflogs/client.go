package fflogs

import (
	"context"
	"memo-syncer/flow"
)

func FetchCharacterID(ctx context.Context, name, server, region string) (int, error) {
	vars := map[string]any{
		"server": server,
		"name":   name,
		"region": region,
	}
	var res CharacterID

	err := flow.Query(ctx, characterIDQuery, vars, &res)
	return res.CharacterData.Character.Id, err
}

func FetchBestFightByEncounter(ctx context.Context, id, encounter int) (*EncounterRanked, error) {
	vars := map[string]any{
		"encounterID": encounter,
		"charID":      id,
	}
	var res EncounterRanked

	err := flow.Query(ctx, bestFightQuery, vars, &res)
	return &res, err
}

func FetchFightDetail(ctx context.Context, report string, fight int) (*FightDetail, error) {
	vars := map[string]any{
		"code":    report,
		"fightID": fight,
	}
	var res FightDetail

	err := flow.Query(ctx, fightDetailQuery, vars, &res)
	return &res, err
}

func FetchJobs(ctx context.Context) (*Jobs, error) {
	var res Jobs

	err := flow.Query(ctx, jobsQuery, nil, &res)
	return &res, err
}
