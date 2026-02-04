package main

import (
	"memo-syncer/flow"
	"memo-syncer/logger"
	"memo-syncer/router"
	"memo-syncer/service/memo"
	"time"

	"github.com/rs/zerolog/log"
)

func main() {
	// logger
	logger.InitLogger()

	// database
	flow.InitDB()
	flow.InitRedis()
	flow.InitGraphQL()

	// sync in background
	go func() {
		for {
			if err := memo.SyncMembers(); err != nil {
				log.Error().Err(err).Msg("sync members failed")
			} else {
				log.Info().Msg("sync members completed")
			}
			time.Sleep(10 * time.Minute)
		}
	}()

	// setup router
	r := router.SetupRouter()

	// start server
	if err := r.Run(":8080"); err != nil {
		log.Fatal().Msgf("failed to run server: %v", err)
	}
}
