package router

import (
	"memo-syncer/api"
	"memo-syncer/flow"
	"memo-syncer/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func SetupRouter() *gin.Engine {
	r := gin.New()

	// trust proxy headers
	r.TrustedPlatform = gin.PlatformFlyIO
	err := r.SetTrustedProxies(nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set trusted proxies")
	}

	// middleware
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Prometheus())

	// cors
	r.Use(cors.New(middleware.CorsConfig()))

	// limiter
	limiter := redis_rate.NewLimiter(flow.Redis)
	// public limit: 60 requests per minute
	publicLimit := redis_rate.PerMinute(800)
	publicRateLimiter := middleware.CreateLimiter(limiter, publicLimit)

	// health check
	r.GET("/status", api.Status)

	// metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// progress
	progress := r.Group("/progress")
	{
		// public
		progress.GET("/", publicRateLimiter, api.GetProgress)
		progress.GET("/:name", publicRateLimiter, api.GetMemberProgress)
	}

	return r
}
