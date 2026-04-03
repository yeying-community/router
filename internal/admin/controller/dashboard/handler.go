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
	sectionAll         = "all"
	sectionOverview    = "overview"
	sectionTrend       = "trend"
	sectionHealth      = "health"
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
	Balance            float64  `json:"balance"`
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

type dashboardPayload struct {
	Section     string              `json:"section"`
	Period      string              `json:"period"`
	Granularity string              `json:"granularity"`
	StartAt     int64               `json:"start_timestamp"`
	EndAt       int64               `json:"end_timestamp"`
	Summary     summaryData         `json:"summary"`
	Trend       []trendPoint        `json:"trend"`
	TopChannels []channelHealthItem `json:"top_channels"`
	RecentTasks []model.AsyncTask   `json:"recent_tasks"`
	GeneratedAt int64               `json:"generated_at"`
}

func normalizeSection(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case sectionOverview, sectionTrend, sectionHealth:
		return strings.TrimSpace(strings.ToLower(raw))
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
	for _, row := range channel.GetModelConfigs() {
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
	for _, row := range channel.GetModelConfigs() {
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
			Balance:            row.Balance,
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

// GetDashboard godoc
// @Summary Get admin dashboard aggregate data
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param period query string false "today|last_7_days|last_30_days|this_month|last_month|this_year|last_year|last_12_months|all_time"
// @Param section query string false "all|overview|trend|health"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/dashboard [get]
func GetDashboard(c *gin.Context) {
	period := normalizePeriod(c.DefaultQuery("period", periodLast7Days))
	section := normalizeSection(c.Query("section"))
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

	if section == sectionAll || section == sectionOverview {
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
	}

	if section == sectionAll || section == sectionTrend {
		trend, err := buildTrend(startAt, endAt, granularity)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		payload.Trend = trend
	}

	if section == sectionAll || section == sectionHealth {
		topChannels, _, _, _, err := listTopChannels()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		payload.TopChannels = topChannels
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payload,
	})
}
