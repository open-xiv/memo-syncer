package fflogs

import "context"

// FetchCharacterID resolves (name, server, region) → FFLogs character ID.
// Callers should cache the result (Redis) since the ID is immutable.
func FetchCharacterID(ctx context.Context, c *Client, name, server, region string) (int, error) {
	vars := map[string]any{
		"name":   name,
		"server": server,
		"region": region,
	}
	var res CharacterID
	if err := c.Query(ctx, characterIDQuery, vars, &res); err != nil {
		return 0, err
	}
	return res.CharacterData.Character.ID, nil
}

// FetchBestFights batches four encounter lookups into a single GraphQL call
// using aliases (m1..m4). Order of the encounter IDs matches BestFights.Zone(i).
func FetchBestFights(ctx context.Context, c *Client, charID int, enc1, enc2, enc3, enc4 int) (*BestFights, error) {
	vars := map[string]any{
		"charID": charID,
		"enc1":   enc1,
		"enc2":   enc2,
		"enc3":   enc3,
		"enc4":   enc4,
	}
	var res BestFights
	if err := c.Query(ctx, bestFightsQuery, vars, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// FetchFightDetail fetches the report + summary table for a single fight.
func FetchFightDetail(ctx context.Context, c *Client, reportCode string, fightID int) (*FightDetail, error) {
	vars := map[string]any{
		"code":    reportCode,
		"fightID": fightID,
	}
	var res FightDetail
	if err := c.Query(ctx, fightDetailQuery, vars, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
