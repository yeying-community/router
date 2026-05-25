package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/logger"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				ctx := c.Request.Context()
				logger.Errorf(ctx, fmt.Sprintf("panic detected: %v", err))
				logger.Errorf(ctx, fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				logger.Errorf(ctx, fmt.Sprintf("request: %s %s", c.Request.Method, c.Request.URL.Path))
				body, _ := common.GetRequestBody(c)
				logger.Errorf(
					ctx,
					fmt.Sprintf("request body summary: %s", common.MarshalPayloadLogFields(body, c.ContentType())),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Panic detected, error: %v. Please submit an issue with the related log here: https://github.com/yeying-community/router", err),
						"type":    "one_api_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
