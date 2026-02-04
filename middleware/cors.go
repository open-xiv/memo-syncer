package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
)

func CorsConfig() cors.Config {
	return cors.Config{
		AllowMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
		},

		AllowOriginFunc: func(origin string) bool {

			allowedDomains := []string{
				"https://sumemo.dev",
				"https://diemoe.net",
				"http://localhost:3000",
				"http://localhost:5173",
			}

			// domain check
			for _, domain := range allowedDomains {
				if origin == domain {
					return true
				}
			}

			if strings.HasSuffix(origin, ".diemoe.net") {
				return true
			}

			// no match -> header check
			return false
		},

		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}
