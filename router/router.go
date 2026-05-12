package router

import (
	"github.com/open-xiv/memo-syncer/api"
	"github.com/open-xiv/memo-syncer/flow"
	"github.com/open-xiv/memo-syncer/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func SetupRouter() *gin.Engine {
	r := gin.New()

	r.TrustedPlatform = gin.PlatformFlyIO
	err := r.SetTrustedProxies(nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set trusted proxies")
	}

	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Prometheus())

	r.Use(cors.New(middleware.CorsConfig()))

	limiter := redis_rate.NewLimiter(flow.Redis)
	publicLimit := redis_rate.PerMinute(800)
	publicRateLimiter := middleware.CreateLimiter(limiter, publicLimit)

	r.GET("/status", api.Status)
	r.GET("/status/live", api.StatusLive)
	r.GET("/status/ready", api.StatusReady)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	progress := r.Group("/progress")
	{
		progress.GET("/", publicRateLimiter, api.GetProgress)
		progress.GET("/:name", publicRateLimiter, api.GetMemberProgress)
	}

	return r
}
