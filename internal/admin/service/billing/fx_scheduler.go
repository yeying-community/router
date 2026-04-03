package billing

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

const (
	fxAutoSyncLoopIntervalSeconds = 30
	fxAutoSyncMinIntervalSeconds  = 60
)

var startFXAutoSyncWorkerOnce sync.Once

func StartFXAutoSyncWorker() {
	startFXAutoSyncWorkerOnce.Do(func() {
		go runFXAutoSyncWorker()
	})
}

func runFXAutoSyncWorker() {
	logger.SysLog("[billing.fx] auto sync worker started")
	ticker := time.NewTicker(fxAutoSyncLoopIntervalSeconds * time.Second)
	defer ticker.Stop()

	for {
		if shouldRunFXAutoSyncNow() {
			runFXAutoSyncOnce()
		}
		<-ticker.C
	}
}

func shouldRunFXAutoSyncNow() bool {
	if !config.FXAutoSyncEnabled {
		return false
	}
	if strings.TrimSpace(strings.ToLower(config.FXAutoSyncProvider)) != fxProviderFrankfurter {
		return false
	}
	now := helper.GetTimestamp()
	interval := normalizedFXAutoSyncIntervalSeconds()
	if config.FXAutoSyncLastRunAt <= 0 {
		return true
	}
	return now-config.FXAutoSyncLastRunAt >= int64(interval)
}

func normalizedFXAutoSyncIntervalSeconds() int {
	interval := config.FXAutoSyncIntervalSeconds
	if interval < fxAutoSyncMinIntervalSeconds {
		return fxAutoSyncMinIntervalSeconds
	}
	return interval
}

func runFXAutoSyncOnce() {
	runAt := helper.GetTimestamp()
	_ = model.UpdateOption("FXAutoSyncLastRunAt", strconv.FormatInt(runAt, 10))

	result, err := SyncFXMarketRates(context.Background())
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if len(message) > 1024 {
			message = message[:1024]
		}
		_ = model.UpdateOption("FXAutoSyncLastError", message)
		logger.SysWarnf("[billing.fx] auto sync failed: %s", message)
		return
	}
	_ = model.UpdateOption("FXAutoSyncLastSuccessAt", strconv.FormatInt(runAt, 10))
	_ = model.UpdateOption("FXAutoSyncLastError", "")
	logger.SysLogf("[billing.fx] auto sync success provider=%s updated=%d skipped=%d date=%s",
		result.Provider, result.UpdatedCount, result.SkippedCount, result.Date)
}
