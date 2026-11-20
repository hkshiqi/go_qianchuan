package middleware

import (
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactedQuery(c.Request.URL.RawQuery)

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[%s] %d | %13v | %15s | %-7s %s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)
	}
}

// redactedQuery removes OAuth and API credentials from access logs while
// preserving non-sensitive query parameters for troubleshooting.
func redactedQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "<redacted>"
	}
	for key := range values {
		switch strings.ToLower(key) {
		case "auth_code", "code", "access_token", "refresh_token", "token", "secret":
			values[key] = []string{"REDACTED"}
		}
	}
	return values.Encode()
}
