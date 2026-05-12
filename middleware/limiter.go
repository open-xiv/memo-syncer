package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/open-xiv/memo-syncer/model"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
)

func CreateLimiter(limiter *redis_rate.Limiter, limit redis_rate.Limit) gin.HandlerFunc {
	return func(c *gin.Context) {

		key := fmt.Sprintf("ratelimit:%s", c.ClientIP())

		res, err := limiter.Allow(context.Background(), key, limit)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, model.ErrorResponse{Error: "Internal server error"})
			return
		}

		if res.Allowed == 0 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, model.ErrorResponse{Error: "Rate limit exceeded"})
			return
		}

		c.Next()
	}
}
