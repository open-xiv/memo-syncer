package fflogs

import (
	"context"

	"github.com/machinebox/graphql"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	TokenURL = "https://www.fflogs.com/oauth/token"
	APIURL   = "https://www.fflogs.com/api/v2/client"
)

// Client wraps an authenticated FFLogs v2 GraphQL client tied to a single (clientID, clientSecret) pair.
// the keypool constructs one per donated key and reuses it so the OAuth token stays cached inside the transport.
type Client struct {
	graphql *graphql.Client
}

func NewClient(clientID, clientSecret string) *Client {
	cfg := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     TokenURL,
	}
	oauthClient := cfg.Client(context.Background())

	return &Client{
		graphql: graphql.NewClient(APIURL, graphql.WithHTTPClient(oauthClient)),
	}
}

func (c *Client) Query(ctx context.Context, query string, vars map[string]any, res any) error {
	req := graphql.NewRequest(query)
	for k, v := range vars {
		req.Var(k, v)
	}
	return c.graphql.Run(ctx, req, res)
}
