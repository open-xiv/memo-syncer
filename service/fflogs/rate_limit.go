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

type RateLimitData struct {
	LimitPerHour        int `json:"limitPerHour"`
	PointsSpentThisHour int `json:"pointsSpentThisHour"`
	PointsResetIn       int `json:"pointsResetIn"`
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
