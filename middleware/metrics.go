package middleware

import (
	"memo-syncer/metrics"

	"github.com/gin-gonic/gin"
)

func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		path := c.FullPath()
		if path == "" || path == "/metrics" {
			return
		}

		method := c.Request.Method
		metrics.HTTPRequestsTotal.WithLabelValues(path, method).Inc()
	}
}
