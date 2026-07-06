package billing

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
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
	RecordFXSyncRun(runAt)

	result, err := SyncFXMarketRates(context.Background())
	if err != nil {
		message := RecordFXSyncFailure(err)
		logger.SysWarnf("[billing.fx] auto sync failed: %s", message)
		return
	}
	RecordFXSyncSuccess(runAt)
	logger.SysLogf("[billing.fx] auto sync success provider=%s updated=%d skipped=%d date=%s",
		result.Provider, result.UpdatedCount, result.SkippedCount, result.Date)
}
