package fflogs

import "context"

const rateLimitQuery = `
query {
    rateLimitData {
        limitPerHour
        pointsSpentThisHour
        pointsResetIn
    }
}
`

// RateLimitData mirrors FFLogs' rateLimitData payload.
// all three fields come back as JSON numbers with fractional parts (e.g. pointsSpentThisHour can be 26.05), so they must be float64. integer casts happen at the KeyState boundary.
type RateLimitData struct {
	LimitPerHour        float64 `json:"limitPerHour"`
	PointsSpentThisHour float64 `json:"pointsSpentThisHour"`
	PointsResetIn       float64 `json:"pointsResetIn"`
}

type rateLimitResponse struct {
	RateLimitData RateLimitData `json:"rateLimitData"`
}

func FetchRateLimit(ctx context.Context, c *Client) (*RateLimitData, error) {
	var resp rateLimitResponse
	if err := c.Query(ctx, rateLimitQuery, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.RateLimitData, nil
}
