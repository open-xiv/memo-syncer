package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		stop := time.Now()
		latency := stop.Sub(start)
		if raw != "" {
			path = path + "?" + raw
		}

		var errMsg string
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}

		logger := log.Info()

		status := c.Writer.Status()

		isProgress := strings.Contains(path, "/progress")

		if isProgress {
			logger = log.Debug()
		} else {
			if status >= 500 {
				logger = log.Error().Str("error", errMsg)
			} else if status >= 400 {
				logger = log.Warn().Str("error", errMsg)
			} else {
				logger = log.Info()
			}
		}

		logger.Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Str("ip", c.ClientIP()).
			Dur("latency", latency).
			Msg("Request")
	}
}
