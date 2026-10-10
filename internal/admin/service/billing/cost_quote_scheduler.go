package billing

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	channelcontroller "github.com/yeying-community/router/internal/admin/controller/channel"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
)

const (
	costQuoteSyncTickSeconds        = 30
	costQuoteSyncMinIntervalSeconds = 60
)

var (
	startCostQuoteSyncWorkerOnce sync.Once
	costQuoteSyncLastRunAt       int64
)

// StartChannelCostQuoteSyncWorker keeps the local cost-rate cache fresh so the
// online cost floor (when enabled) doesn't fall back to local the moment a manual
// sync goes stale. It only does work when the floor is enabled AND the billing
// service is configured — otherwise it ticks and skips (zero cost when off).
func StartChannelCostQuoteSyncWorker() {
	startCostQuoteSyncWorkerOnce.Do(func() { go runCostQuoteSyncWorker() })
}

func runCostQuoteSyncWorker() {
	logger.SysLog("[billing.cost_quote] sync worker started")
	ticker := time.NewTicker(costQuoteSyncTickSeconds * time.Second)
	defer ticker.Stop()
	for {
		if shouldRunCostQuoteSyncNow() {
			runCostQuoteSyncOnce()
		}
		<-ticker.C
	}
}

func shouldRunCostQuoteSyncNow() bool {
	if !config.BillingServiceCostRateFloorEnabled {
		return false
	}
	if strings.TrimSpace(config.BillingServiceBaseURL) == "" {
		return false
	}
	now := time.Now().Unix()
	if costQuoteSyncLastRunAt <= 0 {
		return true
	}
	return now-costQuoteSyncLastRunAt >= costQuoteSyncIntervalSeconds()
}

// costQuoteSyncIntervalSeconds refreshes at roughly half the freshness TTL so the
// cache never lapses between runs, clamped to a sane minimum.
func costQuoteSyncIntervalSeconds() int64 {
	interval := config.BillingServiceCostRateFloorTTLSeconds / 2
	if interval < costQuoteSyncMinIntervalSeconds {
		interval = costQuoteSyncMinIntervalSeconds
	}
	return interval
}

func runCostQuoteSyncOnce() {
	costQuoteSyncLastRunAt = time.Now().Unix()
	channels, err := channelsvc.GetAllBasic(0, 0, "all", true)
	if err != nil {
		logger.SysWarnf("[billing.cost_quote] list channels failed: %s", err.Error())
		return
	}
	synced, cachedTotal, skippedChannels := 0, 0, 0
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.Id) == "" || channel.Status != model.ChannelStatusEnabled {
			continue
		}
		cached, _, err := channelcontroller.SyncChannelCostQuotesForChannel(context.Background(), channel.Id)
		if err != nil {
			// manual / cost-unsupported / service error: expected for most channels,
			// not worth logging per-channel at warn level.
			skippedChannels++
			continue
		}
		synced++
		cachedTotal += cached
	}
	if synced > 0 || cachedTotal > 0 {
		logger.SysLogf("[billing.cost_quote] sync done channels=%d cached=%d skipped=%d", synced, cachedTotal, skippedChannels)
	}
}
