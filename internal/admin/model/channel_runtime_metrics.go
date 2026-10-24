package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/yeying-community/router/internal/admin/healthtrend"
)

const (
	ChannelMetricPeriodLastHour    = "last_hour"
	ChannelMetricPeriodLast24Hours = "last_24_hours"
	ChannelMetricPeriodLast7Days   = "last_7_days"
	ChannelMetricPeriodLast30Days  = "last_30_days"
)

type ChannelMetricWindow struct {
	Period          string `json:"period"`
	StartAt         int64  `json:"start_at"`
	EndAt           int64  `json:"end_at"`
	IntervalSeconds int64  `json:"interval_seconds"`
}

type ChannelRuntimeMetrics struct {
	SuccessCount   int64               `json:"success_count"`
	FailureCount   int64               `json:"failure_count"`
	RequestCount   int64               `json:"request_count"`
	SuccessRate    float64             `json:"success_rate"`
	AvgLatencyMs   int64               `json:"avg_latency_ms"`
	LastObservedAt int64               `json:"last_observed_at"`
	PromptTokens   int64               `json:"prompt_tokens"`
	OutputTokens   int64               `json:"output_tokens"`
	TotalTokens    int64               `json:"total_tokens"`
	SpendQuota     int64               `json:"spend_quota"`
	Points         []healthtrend.Point `json:"points"`
}

type ChannelRuntimeMetricQuery struct {
	Window              ChannelMetricWindow
	ChannelIDs          []string
	PersonalProviderIDs []string
	UserID              string
}

type channelRuntimeMetricRow struct {
	ChannelID          string `gorm:"column:channel_id"`
	PersonalProviderID string `gorm:"column:personal_provider_id"`
	BucketStart        int64  `gorm:"column:bucket_start"`
	SuccessCount       int64  `gorm:"column:success_count"`
	FailureCount       int64  `gorm:"column:failure_count"`
	LatencyTotal       int64  `gorm:"column:latency_total"`
	LatencyCount       int64  `gorm:"column:latency_count"`
	PromptTokens       int64  `gorm:"column:prompt_tokens"`
	OutputTokens       int64  `gorm:"column:output_tokens"`
	SpendQuota         int64  `gorm:"column:spend_quota"`
	LastObservedAt     int64  `gorm:"column:last_observed_at"`
}

func ResolveChannelMetricWindow(raw string, now time.Time) ChannelMetricWindow {
	period := strings.TrimSpace(strings.ToLower(raw))
	if now.IsZero() {
		now = time.Now()
	}
	window := ChannelMetricWindow{Period: ChannelMetricPeriodLast24Hours, EndAt: now.Unix(), IntervalSeconds: 60 * 60}
	switch period {
	case ChannelMetricPeriodLastHour, "1h", "hour":
		window.Period = ChannelMetricPeriodLastHour
		window.StartAt = now.Add(-time.Hour).Unix() + 1
		window.IntervalSeconds = 5 * 60
	case ChannelMetricPeriodLast7Days, "7d", "week":
		window.Period = ChannelMetricPeriodLast7Days
		window.StartAt = now.Add(-7*24*time.Hour).Unix() + 1
		window.IntervalSeconds = 6 * 60 * 60
	case ChannelMetricPeriodLast30Days, "30d", "month":
		window.Period = ChannelMetricPeriodLast30Days
		window.StartAt = now.Add(-30*24*time.Hour).Unix() + 1
		window.IntervalSeconds = 24 * 60 * 60
	default:
		window.StartAt = now.Add(-24*time.Hour).Unix() + 1
	}
	return window
}

func NewChannelMetricWindow(period string, startAt int64, endAt int64) ChannelMetricWindow {
	if endAt <= 0 {
		endAt = time.Now().Unix()
	}
	if startAt <= 0 || startAt > endAt {
		return ResolveChannelMetricWindow(period, time.Unix(endAt, 0))
	}
	duration := endAt - startAt + 1
	interval := int64(24 * 60 * 60)
	switch {
	case duration <= 2*60*60:
		interval = 5 * 60
	case duration <= 2*24*60*60:
		interval = 60 * 60
	case duration <= 8*24*60*60:
		interval = 6 * 60 * 60
	}
	return ChannelMetricWindow{Period: strings.TrimSpace(period), StartAt: startAt, EndAt: endAt, IntervalSeconds: interval}
}

func LoadChannelRuntimeMetrics(query ChannelRuntimeMetricQuery) (map[string]ChannelRuntimeMetrics, error) {
	result := make(map[string]ChannelRuntimeMetrics)
	if LOG_DB == nil {
		return result, nil
	}
	window := query.Window
	if window.StartAt <= 0 || window.EndAt < window.StartAt {
		return result, fmt.Errorf("渠道指标时间范围无效")
	}
	if window.IntervalSeconds <= 0 {
		window.IntervalSeconds = 60 * 60
	}

	channelIDs := normalizeTrimmedValuesPreserveOrder(query.ChannelIDs)
	personalIDs := normalizeTrimmedValuesPreserveOrder(query.PersonalProviderIDs)
	personalChannelIDs := make([]string, 0, len(personalIDs))
	for _, id := range personalIDs {
		personalChannelIDs = append(personalChannelIDs, PersonalProviderChannelPrefix+id)
	}
	for _, id := range channelIDs {
		result[id] = emptyChannelRuntimeMetrics(window)
	}
	for _, id := range personalIDs {
		result[PersonalProviderChannelPrefix+id] = emptyChannelRuntimeMetrics(window)
	}
	if len(channelIDs) == 0 && len(personalIDs) == 0 {
		return result, nil
	}

	bucketExpr := healthtrend.SQLBucketExpressionForInterval(LOG_DB, "created_at", window.IntervalSeconds)
	dbQuery := LOG_DB.Table(EventLogsTableName).
		Select(fmt.Sprintf(`
			channel_id,
			personal_provider_id,
			%s AS bucket_start,
			SUM(CASE WHEN type = ? THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN type = ? AND ((relay_error_type IS NULL OR LOWER(TRIM(relay_error_type)) <> ?) AND (relay_error_code IS NULL OR LOWER(TRIM(relay_error_code)) <> ?)) THEN 1 ELSE 0 END) AS failure_count,
			SUM(CASE WHEN elapsed_time > 0 AND (type = ? OR (type = ? AND COALESCE(LOWER(TRIM(relay_error_type)), '') <> ? AND COALESCE(LOWER(TRIM(relay_error_code)), '') <> ?)) THEN elapsed_time ELSE 0 END) AS latency_total,
			SUM(CASE WHEN elapsed_time > 0 AND (type = ? OR (type = ? AND COALESCE(LOWER(TRIM(relay_error_type)), '') <> ? AND COALESCE(LOWER(TRIM(relay_error_code)), '') <> ?)) THEN 1 ELSE 0 END) AS latency_count,
			SUM(CASE WHEN type = ? THEN prompt_tokens ELSE 0 END) AS prompt_tokens,
			SUM(CASE WHEN type = ? THEN completion_tokens ELSE 0 END) AS output_tokens,
			SUM(CASE WHEN type = ? THEN quota ELSE 0 END) AS spend_quota,
			MAX(CASE WHEN type = ? OR (type = ? AND COALESCE(LOWER(TRIM(relay_error_type)), '') <> ? AND COALESCE(LOWER(TRIM(relay_error_code)), '') <> ?) THEN created_at ELSE 0 END) AS last_observed_at
		`, bucketExpr),
			LogTypeConsume,
			LogTypeRelayFailure, "client_abort", "request_aborted",
			LogTypeConsume, LogTypeRelayFailure, "client_abort", "request_aborted",
			LogTypeConsume, LogTypeRelayFailure, "client_abort", "request_aborted",
			LogTypeConsume, LogTypeConsume, LogTypeConsume,
			LogTypeConsume, LogTypeRelayFailure, "client_abort", "request_aborted").
		Where("type IN ?", []int{LogTypeConsume, LogTypeRelayFailure}).
		Where("created_at BETWEEN ? AND ?", window.StartAt, window.EndAt)
	if userID := strings.TrimSpace(query.UserID); userID != "" {
		dbQuery = dbQuery.Where("user_id = ?", userID)
	}
	switch {
	case len(channelIDs) > 0 && len(personalIDs) > 0:
		dbQuery = dbQuery.Where("channel_id IN ? OR personal_provider_id IN ? OR channel_id IN ?", channelIDs, personalIDs, personalChannelIDs)
	case len(channelIDs) > 0:
		dbQuery = dbQuery.Where("channel_id IN ?", channelIDs)
	case len(personalIDs) > 0:
		dbQuery = dbQuery.Where("personal_provider_id IN ? OR channel_id IN ?", personalIDs, personalChannelIDs)
	}
	rows := make([]channelRuntimeMetricRow, 0)
	if err := dbQuery.Group("channel_id, personal_provider_id, bucket_start").Order("bucket_start asc").Scan(&rows).Error; err != nil {
		return nil, err
	}

	aggregates := make(map[string][]healthtrend.Aggregate)
	for _, row := range rows {
		key := strings.TrimSpace(row.ChannelID)
		if personalID := strings.TrimSpace(row.PersonalProviderID); personalID != "" {
			key = PersonalProviderChannelPrefix + personalID
		}
		if key == "" {
			continue
		}
		metrics, ok := result[key]
		if !ok {
			continue
		}
		metrics.PromptTokens += row.PromptTokens
		metrics.OutputTokens += row.OutputTokens
		metrics.SpendQuota += row.SpendQuota
		result[key] = metrics
		aggregates[key] = append(aggregates[key], healthtrend.Aggregate{
			BucketStart: row.BucketStart, SuccessCount: row.SuccessCount, FailureCount: row.FailureCount,
			LatencyTotal: row.LatencyTotal, LatencyCount: row.LatencyCount, LastObservedAt: row.LastObservedAt,
		})
	}
	for key, metrics := range result {
		metrics.Points = healthtrend.BuildPointsForWindow(window.StartAt, window.EndAt, window.IntervalSeconds, aggregates[key])
		summary := healthtrend.Summarize(metrics.Points)
		metrics.SuccessCount = summary.SuccessCount
		metrics.FailureCount = summary.FailureCount
		metrics.RequestCount = summary.TotalCount
		metrics.AvgLatencyMs = summary.AvgLatencyMs
		metrics.LastObservedAt = summary.LastObservedAt
		metrics.TotalTokens = metrics.PromptTokens + metrics.OutputTokens
		if metrics.RequestCount > 0 {
			metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.RequestCount)
		}
		result[key] = metrics
	}
	return result, nil
}

// MergeChannelRuntimeMetrics combines offer-level runtime observations into a
// stable product channel. Logs keep their original channel ids for routing and
// settlement attribution; only the read projection is merged here.
func MergeChannelRuntimeMetrics(window ChannelMetricWindow, values ...ChannelRuntimeMetrics) ChannelRuntimeMetrics {
	result := emptyChannelRuntimeMetrics(window)
	aggregates := make([]healthtrend.Aggregate, 0)
	for _, metrics := range values {
		result.PromptTokens += metrics.PromptTokens
		result.OutputTokens += metrics.OutputTokens
		result.SpendQuota += metrics.SpendQuota
		for _, point := range metrics.Points {
			aggregates = append(aggregates, healthtrend.Aggregate{
				BucketStart:    point.BucketStart,
				SuccessCount:   point.SuccessCount,
				FailureCount:   point.FailureCount,
				LatencyTotal:   point.AvgLatencyMs * point.LatencyCount,
				LatencyCount:   point.LatencyCount,
				LastObservedAt: point.LastObservedAt,
			})
		}
	}
	result.Points = healthtrend.BuildPointsForWindow(window.StartAt, window.EndAt, window.IntervalSeconds, aggregates)
	summary := healthtrend.Summarize(result.Points)
	result.SuccessCount = summary.SuccessCount
	result.FailureCount = summary.FailureCount
	result.RequestCount = summary.TotalCount
	result.AvgLatencyMs = summary.AvgLatencyMs
	result.LastObservedAt = summary.LastObservedAt
	result.TotalTokens = result.PromptTokens + result.OutputTokens
	if result.RequestCount > 0 {
		result.SuccessRate = float64(result.SuccessCount) / float64(result.RequestCount)
	}
	return result
}

func emptyChannelRuntimeMetrics(window ChannelMetricWindow) ChannelRuntimeMetrics {
	return ChannelRuntimeMetrics{
		Points: healthtrend.BuildPointsForWindow(window.StartAt, window.EndAt, window.IntervalSeconds, nil),
	}
}
