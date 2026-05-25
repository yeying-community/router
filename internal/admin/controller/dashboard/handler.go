package dashboard

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
)

const (
	channelTopLimit    = 8
	taskRecentLimit    = 8
	modelTopLimit      = 12
	sectionAll         = "all"
	sectionSpending    = "spending"
	sectionChannels    = "channels"
	sectionUsers       = "users"
	sectionModels      = "models"
	periodToday        = "today"
	periodLast7Days    = "last_7_days"
	periodLast30Days   = "last_30_days"
	periodThisMonth    = "this_month"
	periodLastMonth    = "last_month"
	periodThisYear     = "this_year"
	periodLastYear     = "last_year"
	periodLast12Months = "last_12_months"
	periodAllTime      = "all_time"
	granularityHour    = "hour"
	granularityDay     = "day"
	granularityMonth   = "month"

	channelHealthLevelHealthy  = "healthy"
	channelHealthLevelWarning  = "warning"
	channelHealthLevelCritical = "critical"
	channelHealthLevelUnknown  = "unknown"
)

type summaryData struct {
	ConsumeQuota    int64 `json:"consume_quota"`
	ConsumeYYC      int64 `json:"consume_yyc"`
	TopupQuota      int64 `json:"topup_quota"`
	TopupYYC        int64 `json:"topup_yyc"`
	NetQuota        int64 `json:"net_quota"`
	NetYYC          int64 `json:"net_yyc"`
	RequestCount    int64 `json:"request_count"`
	ActiveUserCount int64 `json:"active_user_count"`

	ChannelTotal    int64 `json:"channel_total"`
	ChannelEnabled  int64 `json:"channel_enabled"`
	ChannelDisabled int64 `json:"channel_disabled"`

	GroupTotal    int64 `json:"group_total"`
	ProviderTotal int64 `json:"provider_total"`

	TaskActiveTotal int64 `json:"task_active_total"`
	TaskFailedTotal int64 `json:"task_failed_total"`
}

type trendPoint struct {
	Bucket          string `json:"bucket"`
	ConsumeQuota    int64  `json:"consume_quota"`
	ConsumeYYC      int64  `json:"consume_yyc"`
	TopupQuota      int64  `json:"topup_quota"`
	TopupYYC        int64  `json:"topup_yyc"`
	RequestCount    int64  `json:"request_count"`
	ActiveUserCount int64  `json:"active_user_count"`
}

type channelHealthItem struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Protocol           string   `json:"protocol"`
	Status             int      `json:"status"`
	Capabilities       []string `json:"capabilities"`
	UsedQuota          int64    `json:"used_quota"`
	YYCUsed            int64    `json:"yyc_used"`
	Priority           int64    `json:"priority"`
	SelectedModelCount int      `json:"selected_model_count"`
	TestedModelCount   int      `json:"tested_model_count"`
	TestedEndpointCnt  int      `json:"tested_endpoint_count"`
	SupportedCount     int      `json:"supported_count"`
	UnsupportedCount   int      `json:"unsupported_count"`
	PassRate           float64  `json:"pass_rate"`
	CoverageRate       float64  `json:"coverage_rate"`
	AvgLatencyMs       int64    `json:"avg_latency_ms"`
	LastTestedAt       int64    `json:"last_tested_at"`
	HasTestData        bool     `json:"has_test_data"`
	HealthScore        int      `json:"health_score"`
	HealthLevel        string   `json:"health_level"`
}

type usageRankingItem struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	SpendQuota   int64   `json:"spend_quota"`
	SpendYYC     int64   `json:"spend_yyc"`
	ShareRate    float64 `json:"share_rate"`
	LastUsedAt   int64   `json:"last_used_at"`
}

type usageRankSummary struct {
	UserCount    int64   `json:"user_count"`
	RequestCount int64   `json:"request_count"`
	TotalTokens  int64   `json:"total_tokens"`
	SpendQuota   int64   `json:"spend_quota"`
	SpendYYC     int64   `json:"spend_yyc"`
	TopUsername  string  `json:"top_username"`
	TopUserShare float64 `json:"top_user_share"`
}

type usageTotalSummary struct {
	UserCount    int64 `json:"user_count"`
	RequestCount int64 `json:"request_count"`
	TotalTokens  int64 `json:"total_tokens"`
	SpendQuota   int64 `json:"spend_quota"`
	SpendYYC     int64 `json:"spend_yyc"`
}

type modelHealthItem struct {
	Model                string   `json:"model"`
	Provider             string   `json:"provider"`
	Tags                 []string `json:"tags"`
	RequestCount         int64    `json:"request_count"`
	TotalTokens          int64    `json:"total_tokens"`
	SpendQuota           int64    `json:"spend_quota"`
	SpendYYC             int64    `json:"spend_yyc"`
	ChannelCount         int      `json:"channel_count"`
	TestedChannelCount   int      `json:"tested_channel_count"`
	TestedEndpointCount  int      `json:"tested_endpoint_count"`
	SupportedCount       int      `json:"supported_count"`
	UnsupportedCount     int      `json:"unsupported_count"`
	SupportedEndpointCnt int      `json:"supported_endpoint_count"`
	PassRate             float64  `json:"pass_rate"`
	AvgLatencyMs         int64    `json:"avg_latency_ms"`
	LastTestedAt         int64    `json:"last_tested_at"`
	HealthScore          int      `json:"health_score"`
	HealthLevel          string   `json:"health_level"`
}

type modelSummaryData struct {
	SelectedModelCount int64   `json:"selected_model_count"`
	TestedModelCount   int64   `json:"tested_model_count"`
	HealthyModelCount  int64   `json:"healthy_model_count"`
	WarningModelCount  int64   `json:"warning_model_count"`
	CriticalModelCount int64   `json:"critical_model_count"`
	RequestCount       int64   `json:"request_count"`
	TotalTokens        int64   `json:"total_tokens"`
	SpendQuota         int64   `json:"spend_quota"`
	SpendYYC           int64   `json:"spend_yyc"`
	AvgPassRate        float64 `json:"avg_pass_rate"`
	AvgLatencyMs       int64   `json:"avg_latency_ms"`
}

type dashboardPayload struct {
	Section      string              `json:"section"`
	Period       string              `json:"period"`
	Granularity  string              `json:"granularity"`
	StartAt      int64               `json:"start_timestamp"`
	EndAt        int64               `json:"end_timestamp"`
	Summary      summaryData         `json:"summary"`
	Trend        []trendPoint        `json:"trend"`
	TopChannels  []channelHealthItem `json:"top_channels"`
	UsageSummary usageRankSummary    `json:"usage_summary"`
	UsageTotals  usageTotalSummary   `json:"usage_totals"`
	UsageRank    []usageRankingItem  `json:"usage_rank"`
	ModelSummary modelSummaryData    `json:"model_summary"`
	TopModels    []modelHealthItem   `json:"top_models"`
	RecentTasks  []model.AsyncTask   `json:"recent_tasks"`
	GeneratedAt  int64               `json:"generated_at"`
}

type usageRankingRow struct {
	UserID       string `gorm:"column:user_id"`
	Username     string `gorm:"column:username"`
	RequestCount int64  `gorm:"column:request_count"`
	PromptTokens int64  `gorm:"column:prompt_tokens"`
	CompletionTs int64  `gorm:"column:completion_tokens"`
	SpendQuota   int64  `gorm:"column:spend_quota"`
	LastUsedAt   int64  `gorm:"column:last_used_at"`
}

func normalizeSection(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case sectionSpending, sectionChannels, sectionUsers, sectionModels:
		return strings.TrimSpace(strings.ToLower(raw))
	case "overview", "trend":
		return sectionSpending
	case "health":
		return sectionChannels
	default:
		return sectionAll
	}
}

func normalizePeriod(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case periodToday,
		periodLast7Days,
		periodLast30Days,
		periodThisMonth,
		periodLastMonth,
		periodThisYear,
		periodLastYear,
		periodLast12Months,
		periodAllTime:
		return strings.TrimSpace(strings.ToLower(raw))
	case "last_week":
		return periodLast7Days
	default:
		return periodLast7Days
	}
}

func periodRange(period string, now time.Time) (start time.Time, end time.Time) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	switch period {
	case periodToday:
		return startOfDay, endOfDay
	case periodLast7Days:
		return startOfDay.AddDate(0, 0, -6), endOfDay
	case periodLast30Days:
		return startOfDay.AddDate(0, 0, -29), endOfDay
	case periodThisMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), endOfDay
	case periodLastMonth:
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		lastMonthEnd := monthStart.Add(-time.Second)
		return time.Date(lastMonthEnd.Year(), lastMonthEnd.Month(), 1, 0, 0, 0, 0, now.Location()), lastMonthEnd
	case periodThisYear:
		return time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()), endOfDay
	case periodLastYear:
		start := time.Date(now.Year()-1, time.January, 1, 0, 0, 0, 0, now.Location())
		end := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
		return start, end
	case periodLast12Months:
		return startOfDay.AddDate(-1, 0, 0), endOfDay
	default:
		return startOfDay.AddDate(0, 0, -6), endOfDay
	}
}

func periodGranularity(period string) string {
	switch period {
	case periodToday:
		return granularityHour
	case periodLast7Days, periodLast30Days, periodThisMonth, periodLastMonth:
		return granularityDay
	default:
		return granularityMonth
	}
}

func sumQuotaByType(logType int, startAt int64, endAt int64) (int64, error) {
	var value int64
	err := model.LOG_DB.Table(model.EventLogsTableName).
		Select("COALESCE(sum(quota),0)").
		Where("type = ? AND created_at BETWEEN ? AND ?", logType, startAt, endAt).
		Scan(&value).Error
	return value, err
}

func countRequests(startAt int64, endAt int64) (int64, error) {
	var value int64
	err := model.LOG_DB.Table(model.EventLogsTableName).
		Where("type = ? AND created_at BETWEEN ? AND ?", model.LogTypeConsume, startAt, endAt).
		Count(&value).Error
	return value, err
}

func countActiveUsers(startAt int64, endAt int64) (int64, error) {
	var value int64
	err := model.LOG_DB.Table(model.EventLogsTableName).
		Select("COUNT(DISTINCT user_id)").
		Where("type = ? AND created_at BETWEEN ? AND ? AND COALESCE(user_id, '') <> ''", model.LogTypeConsume, startAt, endAt).
		Scan(&value).Error
	return value, err
}

func countByModel(table any) (int64, error) {
	count := int64(0)
	err := model.DB.Model(table).Count(&count).Error
	return count, err
}

func countTasksByStatuses(statuses []string) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	count := int64(0)
	err := model.DB.Model(&model.AsyncTask{}).Where("status IN ?", statuses).Count(&count).Error
	return count, err
}

type channelHealthMetrics struct {
	SelectedModelCount int
	TestedModelCount   int
	TestedEndpointCnt  int
	SupportedCount     int
	UnsupportedCount   int
	PassRate           float64
	CoverageRate       float64
	AvgLatencyMs       int64
	LastTestedAt       int64
	HasTestData        bool
	HealthScore        int
	HealthLevel        string
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func channelHealthLevelByScore(score int) string {
	switch {
	case score >= 85:
		return channelHealthLevelHealthy
	case score >= 65:
		return channelHealthLevelWarning
	case score > 0:
		return channelHealthLevelCritical
	default:
		return channelHealthLevelUnknown
	}
}

func calcChannelHealth(channel *model.Channel, nowTs int64) channelHealthMetrics {
	metrics := channelHealthMetrics{
		HealthLevel: channelHealthLevelUnknown,
	}
	if channel == nil {
		return metrics
	}
	selectedModels := make(map[string]struct{})
	for _, row := range channel.GetChannelModels() {
		if !row.Selected || row.Inactive || strings.TrimSpace(row.Model) == "" {
			continue
		}
		selectedModels[row.Model] = struct{}{}
	}
	metrics.SelectedModelCount = len(selectedModels)

	testedModelSet := make(map[string]struct{})
	latencyTotal := int64(0)
	latencyCount := int64(0)
	assertCount := 0
	for _, row := range channel.Tests {
		modelID := strings.TrimSpace(row.Model)
		if modelID == "" {
			continue
		}
		// 仅统计已启用模型，避免历史残留测试噪音干扰健康判断。
		if len(selectedModels) > 0 {
			if _, ok := selectedModels[modelID]; !ok {
				continue
			}
		}
		metrics.TestedEndpointCnt++
		testedModelSet[modelID] = struct{}{}
		if row.TestedAt > metrics.LastTestedAt {
			metrics.LastTestedAt = row.TestedAt
		}
		switch strings.TrimSpace(strings.ToLower(row.Status)) {
		case model.ChannelTestStatusSupported:
			metrics.SupportedCount++
			assertCount++
		case model.ChannelTestStatusUnsupported:
			metrics.UnsupportedCount++
			assertCount++
		}
		if row.LatencyMs > 0 {
			latencyTotal += row.LatencyMs
			latencyCount++
		}
	}
	metrics.TestedModelCount = len(testedModelSet)
	metrics.HasTestData = metrics.TestedEndpointCnt > 0
	if latencyCount > 0 {
		metrics.AvgLatencyMs = latencyTotal / latencyCount
	}
	if metrics.SelectedModelCount > 0 {
		metrics.CoverageRate = clamp01(float64(metrics.TestedModelCount) / float64(metrics.SelectedModelCount))
	}
	if assertCount > 0 {
		metrics.PassRate = clamp01(float64(metrics.SupportedCount) / float64(assertCount))
	}

	score := 100.0
	if channel.Status != model.ChannelStatusEnabled {
		if channel.Status == model.ChannelStatusCreating {
			score -= 20
		} else {
			score -= 40
		}
	}
	if metrics.SelectedModelCount > 0 {
		score -= (1 - metrics.CoverageRate) * 25
	} else {
		score -= 12
	}
	if assertCount > 0 {
		score -= (1 - metrics.PassRate) * 25
	} else {
		score -= 18
	}
	switch {
	case metrics.AvgLatencyMs >= 30000:
		score -= 20
	case metrics.AvgLatencyMs >= 15000:
		score -= 14
	case metrics.AvgLatencyMs >= 8000:
		score -= 8
	case metrics.AvgLatencyMs >= 3000:
		score -= 4
	default:
		if metrics.AvgLatencyMs <= 0 {
			score -= 6
		}
	}
	if metrics.HasTestData {
		age := nowTs - metrics.LastTestedAt
		if age > 30*24*3600 {
			score -= 15
		} else if age > 7*24*3600 {
			score -= 8
		}
	} else {
		score -= 10
	}
	score = math.Max(0, math.Min(100, score))
	metrics.HealthScore = int(math.Round(score))
	metrics.HealthLevel = channelHealthLevelByScore(metrics.HealthScore)
	return metrics
}

func collectCapabilities(channel *model.Channel) []string {
	if channel == nil {
		return []string{}
	}
	selected := map[string]struct{}{}
	for _, row := range channel.GetChannelModels() {
		if !row.Selected || row.Inactive {
			continue
		}
		modelType := strings.TrimSpace(strings.ToLower(row.Type))
		switch modelType {
		case "image", "audio", "video":
			selected[modelType] = struct{}{}
		default:
			selected["text"] = struct{}{}
		}
	}
	order := []string{"text", "image", "audio", "video"}
	result := make([]string, 0, len(order))
	for _, item := range order {
		if _, ok := selected[item]; ok {
			result = append(result, item)
		}
	}
	return result
}

func listTopChannels() ([]channelHealthItem, int64, int64, int64, error) {
	total, enabled, disabled, err := countChannelSummary()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	rows := make([]*model.Channel, 0, channelTopLimit)
	err = model.DB.Model(&model.Channel{}).
		Order("used_quota desc, created_time desc").
		Limit(channelTopLimit).
		Omit("key").
		Find(&rows).Error
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if err := model.HydrateChannelsWithModels(model.DB, rows); err != nil {
		return nil, 0, 0, 0, err
	}
	if err := model.HydrateChannelsWithTests(model.DB, rows); err != nil {
		return nil, 0, 0, 0, err
	}
	nowTs := helper.GetTimestamp()
	items := make([]channelHealthItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.NormalizeProtocol()
		health := calcChannelHealth(row, nowTs)
		items = append(items, channelHealthItem{
			ID:                 strings.TrimSpace(row.Id),
			Name:               strings.TrimSpace(row.Name),
			Protocol:           strings.TrimSpace(row.Protocol),
			Status:             row.Status,
			Capabilities:       collectCapabilities(row),
			UsedQuota:          row.UsedQuota,
			YYCUsed:            row.UsedQuota,
			Priority:           row.GetPriority(),
			SelectedModelCount: health.SelectedModelCount,
			TestedModelCount:   health.TestedModelCount,
			TestedEndpointCnt:  health.TestedEndpointCnt,
			SupportedCount:     health.SupportedCount,
			UnsupportedCount:   health.UnsupportedCount,
			PassRate:           health.PassRate,
			CoverageRate:       health.CoverageRate,
			AvgLatencyMs:       health.AvgLatencyMs,
			LastTestedAt:       health.LastTestedAt,
			HasTestData:        health.HasTestData,
			HealthScore:        health.HealthScore,
			HealthLevel:        health.HealthLevel,
		})
	}
	return items, total, enabled, disabled, nil
}

func countChannelSummary() (int64, int64, int64, error) {
	total, err := countByModel(&model.Channel{})
	if err != nil {
		return 0, 0, 0, err
	}
	enabled := int64(0)
	err = model.DB.Model(&model.Channel{}).Where("status = ?", model.ChannelStatusEnabled).Count(&enabled).Error
	if err != nil {
		return 0, 0, 0, err
	}
	disabled := int64(0)
	err = model.DB.Model(&model.Channel{}).Where("status IN ?", []int{model.ChannelStatusManuallyDisabled, model.ChannelStatusAutoDisabled}).Count(&disabled).Error
	if err != nil {
		return 0, 0, 0, err
	}
	return total, enabled, disabled, nil
}

type dayQuotaRow struct {
	Bucket string `gorm:"column:bucket"`
	Type   int    `gorm:"column:type"`
	Quota  int64  `gorm:"column:quota"`
}

type dayCountRow struct {
	Bucket string `gorm:"column:bucket"`
	Count  int64  `gorm:"column:count"`
}

func buildTimeBucket(ts int64, granularity string) string {
	t := time.Unix(ts, 0)
	switch granularity {
	case granularityHour:
		return t.Format("2006-01-02 15")
	case granularityMonth:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

func nextBucket(ts int64, granularity string) int64 {
	t := time.Unix(ts, 0)
	switch granularity {
	case granularityHour:
		return t.Add(time.Hour).Unix()
	case granularityMonth:
		return t.AddDate(0, 1, 0).Unix()
	default:
		return t.AddDate(0, 0, 1).Unix()
	}
}

func normalizeBucketTimestamp(ts int64, granularity string) int64 {
	t := time.Unix(ts, 0)
	switch granularity {
	case granularityHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Unix()
	case granularityMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).Unix()
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	}
}

func sqlGroupExpr(granularity string) string {
	switch granularity {
	case granularityHour:
		return "TO_CHAR(date_trunc('hour', to_timestamp(created_at)), 'YYYY-MM-DD HH24')"
	case granularityMonth:
		return "TO_CHAR(date_trunc('month', to_timestamp(created_at)), 'YYYY-MM')"
	default:
		return "TO_CHAR(date_trunc('day', to_timestamp(created_at)), 'YYYY-MM-DD')"
	}
}

func buildTrend(startAt int64, endAt int64, granularity string) ([]trendPoint, error) {
	if startAt <= 0 || endAt <= 0 || endAt < startAt {
		return []trendPoint{}, nil
	}
	groupExpr := sqlGroupExpr(granularity)
	quotaRows := make([]dayQuotaRow, 0)
	err := model.LOG_DB.Table(model.EventLogsTableName).
		Select(groupExpr+" AS bucket, type, COALESCE(sum(quota),0) AS quota").
		Where("type IN ? AND created_at BETWEEN ? AND ?", []int{model.LogTypeConsume, model.LogTypeTopup}, startAt, endAt).
		Group("bucket, type").
		Order("bucket asc").
		Scan(&quotaRows).Error
	if err != nil {
		return nil, err
	}
	requestRows := make([]dayCountRow, 0)
	err = model.LOG_DB.Table(model.EventLogsTableName).
		Select(groupExpr+" AS bucket, COUNT(1) AS count").
		Where("type = ? AND created_at BETWEEN ? AND ?", model.LogTypeConsume, startAt, endAt).
		Group("bucket").
		Order("bucket asc").
		Scan(&requestRows).Error
	if err != nil {
		return nil, err
	}
	activeRows := make([]dayCountRow, 0)
	err = model.LOG_DB.Table(model.EventLogsTableName).
		Select(groupExpr+" AS bucket, COUNT(DISTINCT user_id) AS count").
		Where("type = ? AND created_at BETWEEN ? AND ? AND COALESCE(user_id, '') <> ''", model.LogTypeConsume, startAt, endAt).
		Group("bucket").
		Order("bucket asc").
		Scan(&activeRows).Error
	if err != nil {
		return nil, err
	}
	points := make(map[string]*trendPoint, 128)
	start := normalizeBucketTimestamp(startAt, granularity)
	end := normalizeBucketTimestamp(endAt, granularity)
	for current := start; current <= end; current = nextBucket(current, granularity) {
		bucket := buildTimeBucket(current, granularity)
		points[bucket] = &trendPoint{Bucket: bucket}
	}
	for _, row := range quotaRows {
		bucket := strings.TrimSpace(row.Bucket)
		if bucket == "" {
			continue
		}
		if _, ok := points[bucket]; !ok {
			points[bucket] = &trendPoint{Bucket: bucket}
		}
		if row.Type == model.LogTypeConsume {
			points[bucket].ConsumeQuota += row.Quota
			points[bucket].ConsumeYYC += row.Quota
		} else if row.Type == model.LogTypeTopup {
			points[bucket].TopupQuota += row.Quota
			points[bucket].TopupYYC += row.Quota
		}
	}
	for _, row := range requestRows {
		bucket := strings.TrimSpace(row.Bucket)
		if bucket == "" {
			continue
		}
		if _, ok := points[bucket]; !ok {
			points[bucket] = &trendPoint{Bucket: bucket}
		}
		points[bucket].RequestCount = row.Count
	}
	for _, row := range activeRows {
		bucket := strings.TrimSpace(row.Bucket)
		if bucket == "" {
			continue
		}
		if _, ok := points[bucket]; !ok {
			points[bucket] = &trendPoint{Bucket: bucket}
		}
		points[bucket].ActiveUserCount = row.Count
	}
	list := make([]trendPoint, 0, len(points))
	for _, point := range points {
		if point == nil {
			continue
		}
		list = append(list, *point)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Bucket < list[j].Bucket
	})
	return list, nil
}

func buildUsageRanking(startAt int64, endAt int64, totalConsumeQuota int64, limit int) ([]usageRankingItem, error) {
	return buildUsageRankingWithKeyword(startAt, endAt, totalConsumeQuota, limit, "")
}

func summarizeUsageTotals(startAt int64, endAt int64, userKeyword string) (usageTotalSummary, error) {
	summary := usageTotalSummary{}
	if startAt <= 0 || endAt <= 0 || endAt < startAt {
		return summary, nil
	}
	keyword := strings.TrimSpace(userKeyword)
	query := model.LOG_DB.Table(model.EventLogsTableName).
		Where("type = ? AND created_at BETWEEN ? AND ? AND COALESCE(NULLIF(TRIM(user_id), ''), '') <> ''", model.LogTypeConsume, startAt, endAt)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(user_id ILIKE ? OR username ILIKE ?)", like, like)
	}
	type aggregateRow struct {
		UserCount    int64 `gorm:"column:user_count"`
		RequestCount int64 `gorm:"column:request_count"`
		PromptTokens int64 `gorm:"column:prompt_tokens"`
		CompletionTs int64 `gorm:"column:completion_tokens"`
		SpendQuota   int64 `gorm:"column:spend_quota"`
	}
	row := aggregateRow{}
	err := query.Select(`
		COUNT(DISTINCT user_id) AS user_count,
		COUNT(1) AS request_count,
		COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		COALESCE(SUM(quota), 0) AS spend_quota
	`).Scan(&row).Error
	if err != nil {
		return summary, err
	}
	summary.UserCount = row.UserCount
	summary.RequestCount = row.RequestCount
	summary.TotalTokens = row.PromptTokens + row.CompletionTs
	summary.SpendQuota = row.SpendQuota
	summary.SpendYYC = row.SpendQuota
	return summary, nil
}

func summarizeUsageRanking(items []usageRankingItem) usageRankSummary {
	summary := usageRankSummary{}
	if len(items) == 0 {
		return summary
	}
	summary.UserCount = int64(len(items))
	var topSpendQuota int64
	for index, item := range items {
		summary.RequestCount += item.RequestCount
		summary.TotalTokens += item.TotalTokens
		summary.SpendQuota += item.SpendQuota
		summary.SpendYYC += item.SpendYYC
		if index == 0 || item.SpendQuota > topSpendQuota {
			topSpendQuota = item.SpendQuota
			summary.TopUsername = strings.TrimSpace(item.Username)
			if summary.TopUsername == "" {
				summary.TopUsername = strings.TrimSpace(item.UserID)
			}
			summary.TopUserShare = item.ShareRate
		}
	}
	return summary
}

func buildUsageRankingWithKeyword(startAt int64, endAt int64, totalConsumeQuota int64, limit int, userKeyword string) ([]usageRankingItem, error) {
	if startAt <= 0 || endAt <= 0 || endAt < startAt || limit <= 0 {
		return []usageRankingItem{}, nil
	}
	keyword := strings.TrimSpace(userKeyword)
	rows := make([]usageRankingRow, 0, limit)
	query := model.LOG_DB.Table(model.EventLogsTableName).
		Select(`
			COALESCE(NULLIF(TRIM(user_id), ''), '') AS user_id,
			COALESCE(NULLIF(MAX(TRIM(username)), ''), COALESCE(NULLIF(TRIM(user_id), ''), '-')) AS username,
			COUNT(1) AS request_count,
			COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(quota), 0) AS spend_quota,
			COALESCE(MAX(created_at), 0) AS last_used_at
		`).
		Where("type = ? AND created_at BETWEEN ? AND ? AND COALESCE(NULLIF(TRIM(user_id), ''), '') <> ''", model.LogTypeConsume, startAt, endAt)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(user_id ILIKE ? OR username ILIKE ?)", like, like)
	}
	err := query.Group("user_id").
		Order("spend_quota DESC, request_count DESC, last_used_at DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]usageRankingItem, 0, len(rows))
	for _, row := range rows {
		totalTokens := row.PromptTokens + row.CompletionTs
		shareRate := 0.0
		if totalConsumeQuota > 0 && row.SpendQuota > 0 {
			shareRate = float64(row.SpendQuota) / float64(totalConsumeQuota)
		}
		items = append(items, usageRankingItem{
			UserID:       strings.TrimSpace(row.UserID),
			Username:     strings.TrimSpace(row.Username),
			RequestCount: row.RequestCount,
			TotalTokens:  totalTokens,
			SpendQuota:   row.SpendQuota,
			SpendYYC:     row.SpendQuota,
			ShareRate:    clamp01(shareRate),
			LastUsedAt:   row.LastUsedAt,
		})
	}
	return items, nil
}

func resolveAllTimeRange(now time.Time) (time.Time, time.Time, error) {
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	minTimestamp := int64(0)
	err := model.LOG_DB.Table(model.EventLogsTableName).
		Select("COALESCE(min(created_at),0)").
		Where("type IN ?", []int{model.LogTypeConsume, model.LogTypeTopup}).
		Scan(&minTimestamp).Error
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if minTimestamp > 0 {
		minTime := time.Unix(minTimestamp, 0).In(now.Location())
		start = time.Date(minTime.Year(), minTime.Month(), minTime.Day(), 0, 0, 0, 0, now.Location())
	}
	return start, end, nil
}

type modelUsageRow struct {
	ModelName        string `gorm:"column:model_name"`
	RequestCount     int64  `gorm:"column:request_count"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	SpendQuota       int64  `gorm:"column:spend_quota"`
	LastUsedAt       int64  `gorm:"column:last_used_at"`
}

type modelDashboardAggregate struct {
	item               modelHealthItem
	channelIDs         map[string]struct{}
	testedChannelIDs   map[string]struct{}
	testedEndpoints    map[string]struct{}
	supportedEndpoints map[string]struct{}
	latencyTotal       int64
	latencyCount       int64
}

func newModelDashboardAggregate(modelName string) *modelDashboardAggregate {
	return &modelDashboardAggregate{
		item: modelHealthItem{
			Model:       strings.TrimSpace(modelName),
			HealthLevel: channelHealthLevelUnknown,
			Tags:        []string{},
		},
		channelIDs:         make(map[string]struct{}),
		testedChannelIDs:   make(map[string]struct{}),
		testedEndpoints:    make(map[string]struct{}),
		supportedEndpoints: make(map[string]struct{}),
	}
}

func calcModelHealth(item *modelHealthItem) {
	if item == nil {
		return
	}
	score := 100.0
	if item.ChannelCount > 0 {
		coverageRate := float64(item.TestedChannelCount) / float64(item.ChannelCount)
		score -= (1 - clamp01(coverageRate)) * 30
	} else {
		score -= 25
	}
	assertCount := item.SupportedCount + item.UnsupportedCount
	if assertCount > 0 {
		score -= (1 - clamp01(item.PassRate)) * 30
	} else {
		score -= 20
	}
	switch {
	case item.AvgLatencyMs >= 30000:
		score -= 20
	case item.AvgLatencyMs >= 15000:
		score -= 14
	case item.AvgLatencyMs >= 8000:
		score -= 8
	case item.AvgLatencyMs >= 3000:
		score -= 4
	default:
		if item.AvgLatencyMs <= 0 {
			score -= 6
		}
	}
	if item.LastTestedAt <= 0 {
		score -= 12
	}
	score = math.Max(0, math.Min(100, score))
	item.HealthScore = int(math.Round(score))
	item.HealthLevel = channelHealthLevelByScore(item.HealthScore)
}

func buildModelDashboard(startAt int64, endAt int64, limit int) (modelSummaryData, []modelHealthItem, error) {
	summary := modelSummaryData{}

	selectedRows := make([]model.ChannelModel, 0)
	if err := model.DB.Model(&model.ChannelModel{}).
		Where("selected = ? AND inactive = ?", true, false).
		Find(&selectedRows).Error; err != nil {
		return summary, nil, err
	}

	aggs := make(map[string]*modelDashboardAggregate, len(selectedRows))
	modelNames := make([]string, 0, len(selectedRows))
	providerCandidates := make(map[string]map[string]struct{}, len(selectedRows))
	channelIDs := make([]string, 0, len(selectedRows))
	channelSeen := make(map[string]struct{}, len(selectedRows))

	for _, row := range selectedRows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			modelName = strings.TrimSpace(row.UpstreamModel)
		}
		if modelName == "" {
			continue
		}
		agg, ok := aggs[modelName]
		if !ok {
			agg = newModelDashboardAggregate(modelName)
			aggs[modelName] = agg
			modelNames = append(modelNames, modelName)
		}
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID != "" {
			agg.channelIDs[channelID] = struct{}{}
			if _, exists := channelSeen[channelID]; !exists {
				channelSeen[channelID] = struct{}{}
				channelIDs = append(channelIDs, channelID)
			}
		}
		provider := model.NormalizeGroupModelProviderValue(row.Provider)
		if provider != "" {
			if _, ok := providerCandidates[modelName]; !ok {
				providerCandidates[modelName] = make(map[string]struct{}, 1)
			}
			providerCandidates[modelName][provider] = struct{}{}
		}
	}

	if len(modelNames) == 0 {
		return summary, []modelHealthItem{}, nil
	}

	providerByModel, err := model.LoadUniqueProviderMapByModelsWithDB(model.DB, modelNames)
	if err != nil {
		return summary, nil, err
	}
	for modelName, candidates := range providerCandidates {
		if len(candidates) != 1 {
			continue
		}
		for provider := range candidates {
			providerByModel[modelName] = provider
		}
	}

	tagMap, err := model.LoadProviderModelTagMapByModelsWithDB(model.DB, providerByModel, modelNames)
	if err != nil {
		return summary, nil, err
	}
	modelNamesByProvider := make(map[string][]string)
	for _, modelName := range modelNames {
		provider := model.ResolveProviderFromModelMap(providerByModel, modelName)
		if provider == "" {
			continue
		}
		modelNamesByProvider[provider] = append(modelNamesByProvider[provider], modelName)
	}
	endpointMap := make(map[string][]string, len(modelNames))
	for provider, names := range modelNamesByProvider {
		next, loadErr := model.LoadProviderModelEndpointMapByModelsWithDB(model.DB, provider, names)
		if loadErr != nil {
			return summary, nil, loadErr
		}
		for modelName, endpoints := range next {
			endpointMap[modelName] = endpoints
		}
	}

	if len(channelIDs) > 0 {
		testRows := make([]model.ChannelTest, 0)
		if err := model.DB.Model(&model.ChannelTest{}).
			Where("channel_id IN ? AND model IN ?", channelIDs, modelNames).
			Find(&testRows).Error; err != nil {
			return summary, nil, err
		}
		for _, row := range model.NormalizeChannelTestRows(testRows) {
			modelName := strings.TrimSpace(row.Model)
			agg, ok := aggs[modelName]
			if !ok {
				continue
			}
			channelID := strings.TrimSpace(row.ChannelId)
			if channelID != "" {
				agg.testedChannelIDs[channelID] = struct{}{}
			}
			endpoint := strings.TrimSpace(row.Endpoint)
			if endpoint != "" {
				agg.testedEndpoints[endpoint] = struct{}{}
			}
			agg.item.TestedEndpointCount++
			if row.TestedAt > agg.item.LastTestedAt {
				agg.item.LastTestedAt = row.TestedAt
			}
			switch model.NormalizeChannelTestStatus(row.Status) {
			case model.ChannelTestStatusSupported:
				if row.Supported {
					agg.item.SupportedCount++
				}
			case model.ChannelTestStatusUnsupported:
				agg.item.UnsupportedCount++
			}
			if row.LatencyMs > 0 {
				agg.latencyTotal += row.LatencyMs
				agg.latencyCount++
			}
		}
	}

	usageRows := make([]modelUsageRow, 0, len(modelNames))
	if startAt > 0 && endAt > 0 && endAt >= startAt {
		err = model.LOG_DB.Table(model.EventLogsTableName).
			Select(`
				COALESCE(NULLIF(TRIM(model_name), ''), '-') AS model_name,
				COUNT(1) AS request_count,
				COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
				COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
				COALESCE(SUM(quota), 0) AS spend_quota,
				COALESCE(MAX(created_at), 0) AS last_used_at
			`).
			Where("type = ? AND created_at BETWEEN ? AND ? AND COALESCE(NULLIF(TRIM(model_name), ''), '') <> ''", model.LogTypeConsume, startAt, endAt).
			Group("model_name").
			Scan(&usageRows).Error
		if err != nil {
			return summary, nil, err
		}
	}

	for _, row := range usageRows {
		modelName := strings.TrimSpace(row.ModelName)
		if modelName == "" || modelName == "-" {
			continue
		}
		agg, ok := aggs[modelName]
		if !ok {
			continue
		}
		agg.item.RequestCount = row.RequestCount
		agg.item.TotalTokens = row.PromptTokens + row.CompletionTokens
		agg.item.SpendQuota = row.SpendQuota
		agg.item.SpendYYC = row.SpendQuota
		if row.LastUsedAt > agg.item.LastTestedAt {
			// keep usage recency separate from test recency; test time remains authoritative for health
		}
	}

	items := make([]modelHealthItem, 0, len(aggs))
	for _, modelName := range modelNames {
		agg := aggs[modelName]
		if agg == nil {
			continue
		}
		agg.item.Provider = model.ResolveProviderFromModelMap(providerByModel, modelName)
		agg.item.Tags = tagMap[modelName]
		for _, endpoint := range endpointMap[modelName] {
			normalized := strings.TrimSpace(endpoint)
			if normalized == "" {
				continue
			}
			agg.supportedEndpoints[normalized] = struct{}{}
		}
		agg.item.ChannelCount = len(agg.channelIDs)
		agg.item.TestedChannelCount = len(agg.testedChannelIDs)
		agg.item.SupportedEndpointCnt = len(agg.supportedEndpoints)
		if agg.item.SupportedCount+agg.item.UnsupportedCount > 0 {
			agg.item.PassRate = clamp01(float64(agg.item.SupportedCount) / float64(agg.item.SupportedCount+agg.item.UnsupportedCount))
		}
		if agg.latencyCount > 0 {
			agg.item.AvgLatencyMs = agg.latencyTotal / agg.latencyCount
		}
		calcModelHealth(&agg.item)
		items = append(items, agg.item)
	}

	summary.SelectedModelCount = int64(len(aggs))
	latencyItems := int64(0)
	for _, item := range items {
		if item.TestedChannelCount > 0 {
			summary.TestedModelCount++
		}
		summary.RequestCount += item.RequestCount
		summary.TotalTokens += item.TotalTokens
		summary.SpendQuota += item.SpendQuota
		summary.SpendYYC += item.SpendYYC
		summary.AvgPassRate += item.PassRate
		if item.AvgLatencyMs > 0 {
			summary.AvgLatencyMs += item.AvgLatencyMs
			latencyItems++
		}
		switch item.HealthLevel {
		case channelHealthLevelHealthy:
			summary.HealthyModelCount++
		case channelHealthLevelWarning:
			summary.WarningModelCount++
		case channelHealthLevelCritical:
			summary.CriticalModelCount++
		}
	}
	if len(items) > 0 {
		summary.AvgPassRate = summary.AvgPassRate / float64(len(items))
	}
	if latencyItems > 0 {
		summary.AvgLatencyMs = summary.AvgLatencyMs / latencyItems
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].SpendYYC != items[j].SpendYYC {
			return items[i].SpendYYC > items[j].SpendYYC
		}
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].HealthScore != items[j].HealthScore {
			return items[i].HealthScore > items[j].HealthScore
		}
		return items[i].Model < items[j].Model
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	return summary, items, nil
}

func GetDashboard(c *gin.Context) {
	period := normalizePeriod(c.DefaultQuery("period", periodLast7Days))
	section := normalizeSection(c.Query("section"))
	userKeyword := strings.TrimSpace(c.Query("user_keyword"))
	now := time.Now()
	start, end := periodRange(period, now)
	if period == periodAllTime {
		allTimeStart, allTimeEnd, err := resolveAllTimeRange(now)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		start = allTimeStart
		end = allTimeEnd
	}
	startAt := start.Unix()
	endAt := end.Unix()
	granularity := periodGranularity(period)
	payload := dashboardPayload{
		Section:     section,
		Period:      period,
		Granularity: granularity,
		StartAt:     startAt,
		EndAt:       endAt,
		GeneratedAt: helper.GetTimestamp(),
	}

	if section == sectionAll || section == sectionSpending || section == sectionChannels || section == sectionUsers {
		consumeQuota, err := sumQuotaByType(model.LogTypeConsume, startAt, endAt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		topupQuota, err := sumQuotaByType(model.LogTypeTopup, startAt, endAt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		requestCount, err := countRequests(startAt, endAt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		activeUserCount, err := countActiveUsers(startAt, endAt)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		channelTotal, channelEnabled, channelDisabled, err := countChannelSummary()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		groupTotal, err := countByModel(&model.GroupCatalog{})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		providerTotal, err := countByModel(&model.Provider{})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		taskActiveTotal, err := countTasksByStatuses([]string{model.AsyncTaskStatusPending, model.AsyncTaskStatusRunning})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		taskFailedTotal, err := countTasksByStatuses([]string{model.AsyncTaskStatusFailed})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		if section == sectionAll {
			recentTasks, _, err := model.ListAsyncTasksPageWithDB(model.DB, model.AsyncTaskFilter{}, 1, taskRecentLimit)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			payload.RecentTasks = recentTasks
		}
		payload.Summary = summaryData{
			ConsumeQuota:    consumeQuota,
			ConsumeYYC:      consumeQuota,
			TopupQuota:      topupQuota,
			TopupYYC:        topupQuota,
			NetQuota:        topupQuota - consumeQuota,
			NetYYC:          topupQuota - consumeQuota,
			RequestCount:    requestCount,
			ActiveUserCount: activeUserCount,
			ChannelTotal:    channelTotal,
			ChannelEnabled:  channelEnabled,
			ChannelDisabled: channelDisabled,
			GroupTotal:      groupTotal,
			ProviderTotal:   providerTotal,
			TaskActiveTotal: taskActiveTotal,
			TaskFailedTotal: taskFailedTotal,
		}
		if section == sectionAll || section == sectionUsers {
			usageTotals, err := summarizeUsageTotals(startAt, endAt, userKeyword)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			usageRank, err := buildUsageRankingWithKeyword(startAt, endAt, usageTotals.SpendQuota, 10, userKeyword)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			payload.UsageSummary = summarizeUsageRanking(usageRank)
			payload.UsageTotals = usageTotals
			payload.UsageRank = usageRank
		}
	}

	if section == sectionAll || section == sectionSpending {
		trend, err := buildTrend(startAt, endAt, granularity)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		payload.Trend = trend
	}

	if section == sectionAll || section == sectionChannels {
		topChannels, _, _, _, err := listTopChannels()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		payload.TopChannels = topChannels
	}

	if section == sectionAll || section == sectionModels {
		modelSummary, topModels, err := buildModelDashboard(startAt, endAt, modelTopLimit)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		payload.ModelSummary = modelSummary
		payload.TopModels = topModels
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payload,
	})
}
