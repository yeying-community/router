package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func SetRouter(engine *gin.Engine) {
	engine.Use(middleware.CORS())

	SetApiRouter(engine)
	if common.DisableOpenAICompat {
		logger.SysLog("OpenAI-compatible routes disabled via feature.disable_openai_compat")
	} else {
		SetDashboardRouter(engine)
		SetRelayRouter(engine)
	}

	frontendBaseURL := common.FrontendBaseURL
	if config.IsMasterNode && frontendBaseURL != "" {
		frontendBaseURL = ""
		logger.SysLog("feature.frontend_base_url is ignored on master node")
	}
	if frontendBaseURL == "" {
		return
	}

	frontendBaseURL = strings.TrimSuffix(frontendBaseURL, "/")
	engine.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseURL, c.Request.RequestURI))
	})
}
