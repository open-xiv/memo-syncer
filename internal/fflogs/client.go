package fflogs

import (
	"context"

	"github.com/machinebox/graphql"
	"golang.org/x/oauth2/clientcredentials"
)

type Client struct {
	gqlClient *graphql.Client
}

func NewClient(clientID, clientSecret string) *Client {
	config := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     "https://www.fflogs.com/oauth/token",
	}

	oauthClient := config.Client(context.Background())

	gqlClient := graphql.NewClient(
		"https://www.fflogs.com/api/v2/client",
		graphql.WithHTTPClient(oauthClient),
	)

	return &Client{
		gqlClient: gqlClient,
	}
}

func (c *Client) do(ctx context.Context, query string, vars map[string]any, res any) error {
	req := graphql.NewRequest(query)
	for k, v := range vars {
		req.Var(k, v)
	}

	return c.gqlClient.Run(ctx, req, res)
}

func (c *Client) FetchCharacterID(ctx context.Context, name, server, region string) (int, error) {
	vars := map[string]any{
		"server": server,
		"name":   name,
		"region": region,
	}
	var res CharacterID

	err := c.do(ctx, characterIDQuery, vars, &res)

	return res.Data.CharacterData.Character.Id, err
}

func (c *Client) FetchBestFightByEncounter(ctx context.Context, id, encounter int) (*EncounterRanked, error) {
	vars := map[string]any{
		"encounter": encounter,
		"id":        id,
	}
	var res EncounterRanked

	err := c.do(ctx, bestFightQuery, vars, &res)

	return &res, err
}

func (c *Client) FetchFightDetail(ctx context.Context, report string, fight int) (*FightDetail, error) {
	vars := map[string]any{
		"report": report,
		"fight":  fight,
	}
	var res FightDetail

	err := c.do(ctx, fightDetailQuery, vars, &res)

	return &res, err
}

func (c *Client) FetchJobs(ctx context.Context) (*Jobs, error) {
	var res Jobs

	err := c.do(ctx, jobsQuery, nil, &res)

	return &res, err
}
