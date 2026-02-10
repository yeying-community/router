package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/internal/admin/controller"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func SetWebRouter(engine *gin.Engine, buildFS embed.FS, indexPage []byte) {
	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	engine.Use(middleware.GlobalWebRateLimit())
	engine.Use(middleware.Cache())
	engine.Use(static.Serve("/", common.EmbedFolder(buildFS, "web/dist")))

	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})
}
