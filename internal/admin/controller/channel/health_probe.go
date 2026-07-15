package channel

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

const (
	channelHealthProbeScanInterval   = 5 * time.Minute
	channelHealthProbeInitialSilence = 2 * time.Hour
	channelHealthProbeSuccessSilence = 6 * time.Hour
	channelHealthProbeFailureRetry   = 15 * time.Minute
	channelHealthProbeBatchSize      = 10
)

var channelHealthProbeWorkerOnce sync.Once

type channelHealthSignal struct {
	at     int64
	failed bool
}

func latestChannelHealthSignal(db *gorm.DB, logDB *gorm.DB, row model.ChannelModel) (channelHealthSignal, error) {
	result := channelHealthSignal{}
	models := model.NormalizeChannelModelIDsPreserveOrder([]string{row.Model, row.UpstreamModel, row.PublishedModel})
	if logDB != nil && len(models) > 0 {
		logRow := model.Log{}
		err := logDB.Model(&model.Log{}).
			Where("channel_id = ?", strings.TrimSpace(row.ChannelId)).
			Where("type = ? OR (type = ? AND LOWER(TRIM(relay_error_type)) <> ? AND LOWER(TRIM(relay_error_code)) <> ?)", model.LogTypeConsume, model.LogTypeRelayFailure, "client_abort", "request_aborted").
			Where("request_model_name IN ? OR actual_model_name IN ? OR model_name IN ?", models, models, models).
			Order("created_at desc").
			First(&logRow).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}
		if err == nil {
			result.at = logRow.CreatedAt
			result.failed = logRow.Type == model.LogTypeRelayFailure &&
				strings.TrimSpace(strings.ToLower(logRow.RelayErrorType)) != "client_abort" &&
				strings.TrimSpace(strings.ToLower(logRow.RelayErrorCode)) != "request_aborted"
		}
	}
	if db == nil || len(models) == 0 {
		return result, nil
	}
	testRow := model.ChannelTest{}
	err := db.Model(&model.ChannelTest{}).
		Where("channel_id = ? AND model IN ?", strings.TrimSpace(row.ChannelId), models).
		Order("tested_at desc, round desc").
		First(&testRow).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}
	if err == nil && testRow.TestedAt > result.at {
		result.at = testRow.TestedAt
		result.failed = model.NormalizeChannelTestStatus(testRow.Status) != model.ChannelTestStatusSupported || !testRow.Supported
	}
	return result, nil
}

func enqueueDueChannelHealthProbes(db *gorm.DB, logDB *gorm.DB, now int64, limit int, enqueue func(string, string) (bool, error)) (int, error) {
	if db == nil || enqueue == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = channelHealthProbeBatchSize
	}
	rows := make([]model.ChannelModel, 0)
	if err := db.Model(&model.ChannelModel{}).
		Where("publish_enabled = ? AND selected = ? AND type = ?", true, true, model.ProviderModelTypeText).
		Order("published_at asc, channel_id asc, model asc").
		Limit(1000).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, row := range rows {
		if created >= limit {
			break
		}
		signal, err := latestChannelHealthSignal(db, logDB, row)
		if err != nil {
			return created, err
		}
		due := false
		if signal.at <= 0 {
			publishedAt := row.PublishedAt
			if publishedAt <= 0 {
				publishedAt = now - int64(channelHealthProbeInitialSilence/time.Second)
			}
			due = now-publishedAt >= int64(channelHealthProbeInitialSilence/time.Second)
		} else {
			interval := channelHealthProbeSuccessSilence
			if signal.failed {
				interval = channelHealthProbeFailureRetry
			}
			due = now-signal.at >= int64(interval/time.Second)
		}
		if !due {
			continue
		}
		createdNow, err := enqueue(strings.TrimSpace(row.ChannelId), strings.TrimSpace(row.Model))
		if err != nil {
			logger.SysError("failed to enqueue channel health probe: " + err.Error())
			continue
		}
		if createdNow {
			created++
		}
	}
	return created, nil
}

func runChannelHealthProbeScan() {
	created, err := enqueueDueChannelHealthProbes(model.DB, model.LOG_DB, time.Now().Unix(), channelHealthProbeBatchSize, func(channelID string, modelID string) (bool, error) {
		_, createdCount, _, err := CreateChannelModelTestTasks(
			channelID,
			"health_probe",
			modelID,
			[]string{modelID},
			nil,
			"health-probe",
			"",
			"",
			"",
		)
		return createdCount > 0, err
	})
	if err != nil {
		logger.SysError("channel health probe scan failed: " + err.Error())
		return
	}
	if created > 0 {
		logger.SysLogf("channel health probe scan enqueued %d tasks", created)
	}
}

func StartChannelHealthProbeWorker() {
	channelHealthProbeWorkerOnce.Do(func() {
		go func() {
			timer := time.NewTimer(time.Minute)
			defer timer.Stop()
			<-timer.C
			runChannelHealthProbeScan()
			ticker := time.NewTicker(channelHealthProbeScanInterval)
			defer ticker.Stop()
			for range ticker.C {
				runChannelHealthProbeScan()
			}
		}()
	})
}
