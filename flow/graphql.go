package flow

import (
	"context"
	"os"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2/clientcredentials"
)

var GraphQL *graphql.Client

func InitGraphQL() {
	id := os.Getenv("GRAPH_ID")
	secret := os.Getenv("GRAPH_SECRET")
	if id == "" || secret == "" {
		log.Fatal().Msgf("GRAPH_ID or GRAPH_SECRET not set")
	}

	config := &clientcredentials.Config{
		ClientID:     id,
		ClientSecret: secret,
		TokenURL:     "https://www.fflogs.com/oauth/token",
	}

	oauthClient := config.Client(context.Background())

	GraphQL = graphql.NewClient(
		"https://www.fflogs.com/api/v2/client",
		graphql.WithHTTPClient(oauthClient),
	)
}

func Query(ctx context.Context, query string, vars map[string]any, res any) error {
	req := graphql.NewRequest(query)
	for k, v := range vars {
		req.Var(k, v)
	}

	return GraphQL.Run(ctx, req, res)
}
