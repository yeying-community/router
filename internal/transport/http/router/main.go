package router

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func SetRouter(engine *gin.Engine, buildFS embed.FS) {
	indexPage, err := buildFS.ReadFile("web/build/index.html")
	if err != nil {
		panic(err)
	}

	engine.Use(middleware.CORS())

	SetApiRouter(engine)
	SetDashboardRouter(engine)
	SetRelayRouter(engine)

	frontendBaseURL := os.Getenv("FRONTEND_BASE_URL")
	if config.IsMasterNode && frontendBaseURL != "" {
		frontendBaseURL = ""
		logger.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseURL == "" {
		SetWebRouter(engine, buildFS, indexPage)
		return
	}

	frontendBaseURL = strings.TrimSuffix(frontendBaseURL, "/")
	engine.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseURL, c.Request.RequestURI))
	})
}
