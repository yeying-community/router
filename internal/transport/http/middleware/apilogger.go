package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/logger"
)

// ApiLogger writes per-request info to logs/api.log with context data set by auth middlewares.
func ApiLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// before
		logger.ApiLogf(
			c.Request.Context(),
			"INFO",
			"REQ method=%s path=%s ip=%s ua=%s",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
			c.Request.UserAgent(),
		)

		c.Next()

		// after
		latency := time.Since(start)
		status := c.Writer.Status()
		userID := c.GetInt("id")
		role := c.GetInt("role")
		tokenId := c.GetInt("token_id")
		channelId := c.GetInt("channel_id")
		logger.ApiLogf(
			c.Request.Context(),
			"INFO",
			"RESP method=%s path=%s status=%d latency=%s user_id=%d role=%d token_id=%d channel_id=%d",
			c.Request.Method,
			c.Request.URL.Path,
			status,
			latency,
			userID,
			role,
			tokenId,
			channelId,
		)
	}
}
