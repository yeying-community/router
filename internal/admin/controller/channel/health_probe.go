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

func channelHealthProbeEndpointMatchesModel(row model.ChannelModel, endpointModel string) bool {
	normalizedEndpointModel := strings.TrimSpace(endpointModel)
	if normalizedEndpointModel == "" {
		return false
	}
	for _, candidate := range model.NormalizeProviderLookupCandidates(row.Model, row.UpstreamModel) {
		if candidate == normalizedEndpointModel {
			return true
		}
	}
	return false
}

// selectChannelHealthProbeEndpoint keeps automatic probes aligned with the
// endpoint capabilities that are actually enabled for the channel. The
// channel model's endpoint is only a preference; it must not bypass either
// channel endpoint state or the provider catalog.
func selectChannelHealthProbeEndpoint(db *gorm.DB, row model.ChannelModel) (string, error) {
	if db == nil {
		return "", nil
	}
	channelID := strings.TrimSpace(row.ChannelId)
	candidates := model.NormalizeProviderLookupCandidates(row.Model, row.UpstreamModel)
	if channelID == "" || len(candidates) == 0 {
		return "", nil
	}

	enabledRows, err := model.ListEnabledChannelModelEndpointsByCandidatesWithDB(db, channelID, candidates...)
	if err != nil {
		return "", err
	}
	enabledEndpoints := make([]string, 0, len(enabledRows))
	seenEnabled := make(map[string]struct{}, len(enabledRows))
	for _, endpointRow := range enabledRows {
		if !channelHealthProbeEndpointMatchesModel(row, endpointRow.Model) {
			continue
		}
		endpoint := model.NormalizeRequestedChannelModelEndpoint(endpointRow.Endpoint)
		if endpoint == "" {
			continue
		}
		if _, ok := seenEnabled[endpoint]; ok {
			continue
		}
		seenEnabled[endpoint] = struct{}{}
		enabledEndpoints = append(enabledEndpoints, endpoint)
	}
	if len(enabledEndpoints) == 0 {
		return "", nil
	}

	provider := model.NormalizeGroupModelProviderValue(row.Provider)
	if provider == "" {
		providerByModel, err := model.LoadUniqueProviderMapByModelsWithDB(db, candidates)
		if err != nil {
			return "", err
		}
		provider = model.ResolveProviderFromModelMap(providerByModel, row.UpstreamModel, row.Model)
	}
	if provider == "" {
		return "", nil
	}

	endpointMap, err := model.LoadProviderModelEndpointMapByModelsWithDB(db, provider, candidates)
	if err != nil {
		return "", err
	}
	officialEndpoints := make([]string, 0)
	for _, candidate := range candidates {
		allowed, ok := endpointMap[candidate]
		if !ok {
			continue
		}
		officialEndpoints = allowed
		break
	}
	if len(officialEndpoints) == 0 {
		return "", nil
	}
	officialSet := make(map[string]struct{}, len(officialEndpoints))
	for _, endpoint := range officialEndpoints {
		normalized := model.NormalizeRequestedChannelModelEndpoint(endpoint)
		if normalized != "" {
			officialSet[normalized] = struct{}{}
		}
	}
	compatibleEndpoints := make([]string, 0, len(enabledEndpoints))
	for _, endpoint := range enabledEndpoints {
		if _, ok := officialSet[endpoint]; ok {
			compatibleEndpoints = append(compatibleEndpoints, endpoint)
		}
	}
	if len(compatibleEndpoints) == 0 {
		return "", nil
	}

	preferred := model.NormalizeRequestedChannelModelEndpoint(row.Endpoint)
	for _, endpoint := range compatibleEndpoints {
		if endpoint == preferred {
			return endpoint, nil
		}
	}
	return compatibleEndpoints[0], nil
}

func enqueueDueChannelHealthProbes(db *gorm.DB, logDB *gorm.DB, now int64, limit int, enqueue func(string, string, string) (bool, error)) (int, error) {
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
		endpoint, err := selectChannelHealthProbeEndpoint(db, row)
		if err != nil {
			logger.SysError("failed to select channel health probe endpoint: " + err.Error())
			continue
		}
		if endpoint == "" {
			continue
		}
		createdNow, err := enqueue(strings.TrimSpace(row.ChannelId), strings.TrimSpace(row.Model), endpoint)
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
	created, err := enqueueDueChannelHealthProbes(model.DB, model.LOG_DB, time.Now().Unix(), channelHealthProbeBatchSize, func(channelID string, modelID string, endpoint string) (bool, error) {
		return scheduleChannelModelVerification(channelID, modelID, endpoint, "automatic_probe")
	})
	if err != nil {
		logger.SysError("channel health probe scan failed: " + err.Error())
		return
	}
	if created > 0 {
		logger.SysLogf("channel health probe scan scheduled %d verifications", created)
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
