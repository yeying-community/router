package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	rootapp "github.com/yeying-community/router"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/i18n"
	"github.com/yeying-community/router/common/logger"
	channelcontroller "github.com/yeying-community/router/internal/admin/controller/channel"
	task "github.com/yeying-community/router/internal/admin/controller/task"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"
	_ "github.com/yeying-community/router/internal/admin/repository/bootstrap"
	billingsvc "github.com/yeying-community/router/internal/admin/service/billing"
	notificationsvc "github.com/yeying-community/router/internal/admin/service/notification"
	publishersvc "github.com/yeying-community/router/internal/admin/service/publisher"
	topupsvc "github.com/yeying-community/router/internal/admin/service/topup"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/transport/http/middleware"
	"github.com/yeying-community/router/internal/transport/http/router"
)

// Run starts the HTTP server.
func Run() {
	common.Init()
	logger.SetupLogger()
	logger.SysLogf("Router %s started", common.Version)
	validateStartupAuthConfig()

	gin.SetMode(common.GinMode)
	if config.DebugEnabled {
		logger.SysLog("running in debug mode")
	}

	// Initialize SQL Database
	model.InitDB()
	model.InitLogDB()

	var err error
	err = model.CreateRootAccountIfNeed()
	if err != nil {
		logger.FatalLog("database init error: " + err.Error())
	}
	defer func() {
		err := model.CloseDB()
		if err != nil {
			logger.FatalLog("failed to close database: " + err.Error())
		}
	}()

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		logger.FatalLog("failed to initialize Redis: " + err.Error())
	}

	// Initialize options
	model.InitOptionMap()
	if config.MemoryCacheEnabled {
		logger.SysLog("memory cache enabled")
		logger.SysLog(fmt.Sprintf("sync frequency: %d seconds", config.SyncFrequency))
		model.InitChannelCache()
	}
	if config.MemoryCacheEnabled {
		go model.SyncOptions(config.SyncFrequency)
		go model.SyncChannelCache(config.SyncFrequency)
	}
	if config.BatchUpdateEnabled {
		logger.SysLog("batch update enabled with interval " + strconv.Itoa(config.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}
	if config.EnableMetric {
		logger.SysLog("metric enabled, will disable channel if too much request failed")
		monitor.StartMetricMonitor()
	}
	openai.InitTokenEncoders()
	client.Init()
	if config.IsMasterNode {
		task.StartAsyncTaskWorkers()
		channelcontroller.StartChannelHealthProbeWorker()
		billingsvc.StartFXAutoSyncWorker()
		billingsvc.StartChannelBillingAutoRefreshWorker()
		billingsvc.StartChannelCostQuoteSyncWorker()
		topupsvc.StartTopupReconcileWorker()
		notificationsvc.StartUserNotificationWorker()
		billingsvc.StartProcurementRetryWorker()
		publishersvc.StartPublisherSettlementDeliveryWorker()
	}

	// Initialize i18n
	if err := i18n.Init(); err != nil {
		logger.FatalLog("failed to initialize i18n: " + err.Error())
	}

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.Recovery())
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.TraceID())
	server.Use(middleware.Language())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(config.CookieSecret))
	server.Use(sessions.Sessions("session", store))

	router.SetRouter(server, rootapp.BuildFS)
	var port = strconv.Itoa(*common.Port)
	logger.SysLogf("server started on http://localhost:%s", port)

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: server,
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	stopSignals := make(chan os.Signal, 1)
	signal.Notify(stopSignals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stopSignals)

	select {
	case err = <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.FatalLog("failed to start HTTP server: " + err.Error())
		}
	case sig := <-stopSignals:
		logger.SysLogf("received %s, shutting down HTTP server", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.SysError("HTTP server graceful shutdown failed: " + err.Error())
			_ = httpServer.Close()
		}
	}
}

func validateStartupAuthConfig() {
	if strings.TrimSpace(config.JWTSecret) == "" {
		logger.SysError("auth.jwt_secret is empty; wallet login routes remain enabled, but wallet access/refresh token issuance and verification will fail until it is configured.")
	}
}
