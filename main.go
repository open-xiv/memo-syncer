package main

import (
	"context"
	"memo-syncer/flow"
	"memo-syncer/logger"
	"memo-syncer/model"
	"memo-syncer/router"
	"memo-syncer/service/keypool"
	"memo-syncer/service/memo"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

func main() {
	logger.InitLogger()

	flow.InitDB()
	flow.InitRedis()

	// one-time reset of stale logs_sync_time timestamps from earlier broken runs
	if os.Getenv("RESET_STALE_SYNC") == "true" {
		resetStaleSync()
	}

	// build the FFLogs key pool (donated keys + optional env fallback)
	ctx := context.Background()
	memo.Pool = keypool.New()
	if err := memo.Pool.Load(ctx); err != nil {
		log.Warn().Err(err).Msg("no donated logs keys loaded")

		id := os.Getenv("GRAPH_ID")
		secret := os.Getenv("GRAPH_SECRET")
		if id != "" && secret != "" {
			if err := memo.Pool.AddEnvKey(ctx, id, secret); err != nil {
				log.Fatal().Err(err).Msg("env fallback key is invalid; refusing to start with no keys")
			}
			log.Info().Msg("using GRAPH_ID/GRAPH_SECRET env fallback")
		} else {
			log.Fatal().Msg("no keys available; set GRAPH_ID/GRAPH_SECRET or populate logs_keys")
		}
	}

	// let tunables be overridden by env without a rebuild
	if v := os.Getenv("SYNCER_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			memo.WorkerCount = n
		}
	}

	// background maintenance: reconcile pool every minute, flush stats every 2
	go poolMaintenance(ctx)

	// main sync loop
	go runSyncLoop()

	r := router.SetupRouter()
	if err := r.Run(":8080"); err != nil {
		log.Fatal().Msgf("failed to run server: %v", err)
	}
}

// IdleInterval is how long the sync loop sleeps between complete scans.
const IdleInterval = 30 * time.Minute

func runSyncLoop() {
	for {
		if err := memo.SyncMembers(); err != nil {
			log.Error().Err(err).Msg("sync members failed")
		} else {
			log.Info().Msg("sync members completed")
		}

		nextAt := time.Now().Add(IdleInterval)
		memo.MarkIdle(nextAt)
		time.Sleep(IdleInterval)
	}
}

func poolMaintenance(ctx context.Context) {
	reconcileTick := time.NewTicker(1 * time.Minute)
	flushTick := time.NewTicker(2 * time.Minute)
	defer reconcileTick.Stop()
	defer flushTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcileTick.C:
			memo.Pool.Reconcile(ctx)
		case <-flushTick.C:
			memo.Pool.FlushStats(ctx)
		}
	}
}

func resetStaleSync() {
	res := flow.DB.Model(&model.Member{}).
		Where("logs_sync_time IS NOT NULL").
		Update("logs_sync_time", nil)
	if res.Error != nil {
		log.Error().Err(res.Error).Msg("reset stale sync failed")
		return
	}
	log.Info().Int64("rows", res.RowsAffected).Msg("reset stale logs_sync_time")
}
