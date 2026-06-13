package channel

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
)

type channelAlertFeedItem struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Detail         string `json:"detail"`
	Level          string `json:"level"`
	ChannelID      string `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	CreatedAt      int64  `json:"created_at"`
	Status         string `json:"status"`
	AcknowledgedAt int64  `json:"acknowledged_at"`
	AcknowledgedBy string `json:"acknowledged_by"`
	ResolvedAt     int64  `json:"resolved_at"`
	ResolvedBy     string `json:"resolved_by"`
	OperatorNote   string `json:"operator_note"`
}

type channelAlertFeedData struct {
	Items    []channelAlertFeedItem `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type acknowledgeChannelAlertRequest struct {
	AlertType string `json:"alert_type"`
	AlertKey  string `json:"alert_key"`
	ChannelID string `json:"channel_id"`
	Note      string `json:"note"`
}

type channelAlertFeedFilters struct {
	Status  string
	Type    string
	Level   string
	Keyword string
	Time    string
}

func parseAlertFeedLimit(c *gin.Context) int {
	limit := 20
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return limit
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return limit
	}
	if parsed > 200 {
		return 200
	}
	return parsed
}

func parseAlertFeedPage(c *gin.Context) int {
	page := 1
	raw := strings.TrimSpace(c.Query("page"))
	if raw == "" {
		return page
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return page
	}
	return parsed
}

func parseAlertFeedPageSize(c *gin.Context) int {
	pageSize := 20
	raw := strings.TrimSpace(c.Query("page_size"))
	if raw == "" {
		return pageSize
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return pageSize
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func normalizeAlertFeedStatusFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "all":
		return "all"
	case model.ChannelAlertStatusAcknowledged:
		return model.ChannelAlertStatusAcknowledged
	case model.ChannelAlertStatusResolved:
		return model.ChannelAlertStatusResolved
	case "unacknowledged":
		return "unacknowledged"
	default:
		return model.ChannelAlertStatusActive
	}
}

func normalizeAlertFeedTypeFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "billing":
		return model.ChannelAlertTypeBilling
	case "circuit":
		return model.ChannelAlertTypeCircuitBreaker
	case "model_disabled":
		return model.ChannelAlertTypeModelDisabled
	case "endpoint_disabled":
		return model.ChannelAlertTypeEndpointDisabled
	default:
		return "all"
	}
}

func normalizeAlertFeedLevelFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "error":
		return "critical"
	case "warning", "warn":
		return "warning"
	case "info":
		return "info"
	default:
		return "all"
	}
}

func normalizeAlertFeedTimeFilter(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "24h", "7d", "30d":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "all"
	}
}

func GetRecentChannelAlerts(c *gin.Context) {
	filters := channelAlertFeedFilters{
		Status:  normalizeAlertFeedStatusFilter(c.Query("status")),
		Type:    normalizeAlertFeedTypeFilter(c.Query("type")),
		Level:   normalizeAlertFeedLevelFilter(c.Query("level")),
		Keyword: strings.TrimSpace(strings.ToLower(c.Query("keyword"))),
		Time:    normalizeAlertFeedTimeFilter(c.Query("time")),
	}
	page := parseAlertFeedPage(c)
	pageSize := parseAlertFeedPageSize(c)
	limit := parseAlertFeedLimit(c)
	if strings.TrimSpace(c.Query("page_size")) != "" {
		limit = page * pageSize
		if limit > 500 {
			limit = 500
		}
	}
	billingRows, err := model.ListRecentChannelBillingAlertEventsWithDB(model.DB, limit)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "billing"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	circuitRows, err := model.ListRecentChannelCircuitBreakerEventsWithDB(model.DB, limit)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "circuit"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	modelRows, err := model.ListRecentDisabledChannelModelsWithDB(model.DB, limit)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "models"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	endpointRows, err := model.ListRecentDisabledChannelModelEndpointsWithDB(model.DB, limit)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "endpoints"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelNameByID, err := loadChannelNameByIDForAlertFeed(channelIDsForAlertFeed(billingRows, circuitRows, modelRows, endpointRows))
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "channels"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	items := buildChannelAlertFeedItems(billingRows, circuitRows, modelRows, endpointRows, channelNameByID)
	refs := make([]model.ChannelAlertStateRef, 0, len(items))
	for _, item := range items {
		refs = append(refs, model.ChannelAlertStateRef{
			AlertType: item.Type,
			AlertKey:  item.ID,
		})
	}
	stateByRef, err := model.GetChannelAlertStatesByRefsWithDB(model.DB, refs)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_channel_alerts", stringField("source", "states"), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	for i := range items {
		if state, ok := stateByRef[items[i].Type+"::"+items[i].ID]; ok {
			items[i].Status = state.Status
			items[i].AcknowledgedAt = state.AcknowledgedAt
			items[i].AcknowledgedBy = state.AcknowledgedBy
			items[i].ResolvedAt = state.ResolvedAt
			items[i].ResolvedBy = state.ResolvedBy
			items[i].OperatorNote = state.LastOperatorNote
		} else {
			items[i].Status = model.ChannelAlertStatusActive
		}
	}
	items = filterChannelAlertFeedItems(items, filters)
	total := len(items)
	items = paginateChannelAlertFeedItems(items, page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelAlertFeedData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func ResolveChannelAlert(c *gin.Context) {
	req := acknowledgeChannelAlertRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	alertType := strings.TrimSpace(req.AlertType)
	alertKey := strings.TrimSpace(req.AlertKey)
	channelID := strings.TrimSpace(req.ChannelID)
	if alertType == "" || alertKey == "" || channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "告警参数无效"})
		return
	}
	row, err := model.ResolveChannelAlertStateWithDB(
		model.DB,
		model.ChannelAlertStateRef{AlertType: alertType, AlertKey: alertKey},
		channelID,
		strings.TrimSpace(c.GetString(ctxkey.Id)),
		req.Note,
	)
	if err != nil {
		logChannelAdminWarn(c, "resolve_channel_alert", stringField("alert_type", alertType), stringField("alert_key", alertKey), stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func AcknowledgeChannelAlert(c *gin.Context) {
	req := acknowledgeChannelAlertRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	alertType := strings.TrimSpace(req.AlertType)
	alertKey := strings.TrimSpace(req.AlertKey)
	channelID := strings.TrimSpace(req.ChannelID)
	if alertType == "" || alertKey == "" || channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "告警参数无效"})
		return
	}
	row, err := model.AcknowledgeChannelAlertStateWithDB(
		model.DB,
		model.ChannelAlertStateRef{AlertType: alertType, AlertKey: alertKey},
		channelID,
		strings.TrimSpace(c.GetString(ctxkey.Id)),
		req.Note,
	)
	if err != nil {
		logChannelAdminWarn(c, "acknowledge_channel_alert", stringField("alert_type", alertType), stringField("alert_key", alertKey), stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func channelIDsForAlertFeed(billingRows []model.ChannelBillingAlertEvent, circuitRows []model.ChannelCircuitBreakerEvent, modelRows []model.ChannelModel, endpointRows []model.ChannelModelEndpoint) []string {
	result := make([]string, 0, len(billingRows)+len(circuitRows)+len(modelRows)+len(endpointRows))
	seen := make(map[string]struct{}, len(result))
	appendID := func(raw string) {
		id := strings.TrimSpace(raw)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	for _, row := range billingRows {
		appendID(row.ChannelId)
	}
	for _, row := range circuitRows {
		appendID(row.ChannelId)
	}
	for _, row := range modelRows {
		appendID(row.ChannelId)
	}
	for _, row := range endpointRows {
		appendID(row.ChannelId)
	}
	return result
}

func loadChannelNameByIDForAlertFeed(channelIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(channelIDs))
	if len(channelIDs) == 0 {
		return result, nil
	}
	channels := make([]model.Channel, 0, len(channelIDs))
	if err := model.DB.Select("id", "name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, row := range channels {
		result[strings.TrimSpace(row.Id)] = strings.TrimSpace(row.DisplayName())
	}
	return result, nil
}

func buildChannelAlertFeedItems(billingRows []model.ChannelBillingAlertEvent, circuitRows []model.ChannelCircuitBreakerEvent, modelRows []model.ChannelModel, endpointRows []model.ChannelModelEndpoint, channelNameByID map[string]string) []channelAlertFeedItem {
	items := make([]channelAlertFeedItem, 0, len(billingRows)+len(circuitRows)+len(modelRows)+len(endpointRows))
	for _, row := range billingRows {
		channelID := strings.TrimSpace(row.ChannelId)
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = "渠道账务告警"
		}
		items = append(items, channelAlertFeedItem{
			ID:          strings.TrimSpace(row.Id),
			Type:        model.ChannelAlertTypeBilling,
			Title:       title,
			Summary:     strings.TrimSpace(channelNameByID[channelID]),
			Detail:      strings.TrimSpace(row.Content),
			Level:       normalizeAlertFeedLevel(row.Severity),
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(channelNameByID[channelID]),
			CreatedAt:   row.CreatedAt,
		})
	}
	for _, row := range circuitRows {
		channelID := strings.TrimSpace(row.ChannelId)
		state := strings.TrimSpace(strings.ToLower(row.State))
		title := "渠道熔断事件"
		switch state {
		case model.ChannelCircuitBreakerStateOpen:
			title = "渠道已熔断"
		case model.ChannelCircuitBreakerStateHalfOpen:
			title = "渠道进入半开恢复"
		case model.ChannelCircuitBreakerStateRecovered:
			title = "渠道已恢复"
		case model.ChannelCircuitBreakerStateCanceled:
			title = "渠道已取消熔断"
		}
		level := "info"
		if state == model.ChannelCircuitBreakerStateOpen {
			level = "critical"
		} else if state == model.ChannelCircuitBreakerStateHalfOpen || state == model.ChannelCircuitBreakerStateCanceled {
			level = "warning"
		}
		items = append(items, channelAlertFeedItem{
			ID:          strconv.FormatUint(uint64(row.ID), 10),
			Type:        model.ChannelAlertTypeCircuitBreaker,
			Title:       title,
			Summary:     joinAlertParts(strings.TrimSpace(channelNameByID[channelID]), strings.TrimSpace(row.Reason)),
			Detail:      strings.TrimSpace(row.Reason),
			Level:       level,
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(channelNameByID[channelID]),
			CreatedAt:   row.CreatedAt,
		})
	}
	for _, row := range modelRows {
		channelID := strings.TrimSpace(row.ChannelId)
		items = append(items, channelAlertFeedItem{
			ID:          strings.TrimSpace(channelID + ":" + row.Model + ":" + strconv.FormatInt(row.DisabledAt, 10)),
			Type:        model.ChannelAlertTypeModelDisabled,
			Title:       "渠道模型已自动暂停",
			Summary:     joinAlertParts(strings.TrimSpace(channelNameByID[channelID]), strings.TrimSpace(row.Model), strings.TrimSpace(row.DisabledBy)),
			Detail:      strings.TrimSpace(row.DisabledReason),
			Level:       "warning",
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(channelNameByID[channelID]),
			CreatedAt:   row.DisabledAt,
		})
	}
	for _, row := range endpointRows {
		channelID := strings.TrimSpace(row.ChannelId)
		items = append(items, channelAlertFeedItem{
			ID:          strings.TrimSpace(channelID + ":" + row.Model + ":" + row.Endpoint + ":" + strconv.FormatInt(row.DisabledAt, 10)),
			Type:        model.ChannelAlertTypeEndpointDisabled,
			Title:       "渠道端点已自动暂停",
			Summary:     joinAlertParts(strings.TrimSpace(channelNameByID[channelID]), strings.TrimSpace(row.Model), strings.TrimSpace(row.Endpoint), strings.TrimSpace(row.DisabledBy)),
			Detail:      strings.TrimSpace(row.DisabledReason),
			Level:       "warning",
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(channelNameByID[channelID]),
			CreatedAt:   row.DisabledAt,
		})
	}
	sortChannelAlertFeedItems(items)
	return items
}

func sortChannelAlertFeedItems(items []channelAlertFeedItem) {
	if len(items) <= 1 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt > items[i].CreatedAt {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func filterChannelAlertFeedItems(items []channelAlertFeedItem, filters channelAlertFeedFilters) []channelAlertFeedItem {
	if len(items) == 0 {
		return items
	}
	now := unixNow()
	threshold := int64(0)
	switch filters.Time {
	case "24h":
		threshold = now - 24*60*60
	case "7d":
		threshold = now - 7*24*60*60
	case "30d":
		threshold = now - 30*24*60*60
	}
	result := make([]channelAlertFeedItem, 0, len(items))
	for _, item := range items {
		status := strings.TrimSpace(strings.ToLower(item.Status))
		statusMatched := false
		switch filters.Status {
		case "all":
			statusMatched = true
		case model.ChannelAlertStatusResolved:
			statusMatched = status == model.ChannelAlertStatusResolved
		case model.ChannelAlertStatusAcknowledged:
			statusMatched = status == model.ChannelAlertStatusAcknowledged
		case "unacknowledged":
			statusMatched = status == "" || status == model.ChannelAlertStatusActive
		default:
			statusMatched = status != model.ChannelAlertStatusResolved
		}
		if !statusMatched {
			continue
		}
		if filters.Type != "all" && strings.TrimSpace(item.Type) != filters.Type {
			continue
		}
		if filters.Level != "all" && normalizeAlertFeedLevel(item.Level) != filters.Level {
			continue
		}
		if threshold > 0 && item.CreatedAt < threshold {
			continue
		}
		if filters.Keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.Title,
				item.Summary,
				item.Detail,
				item.ChannelID,
				item.ChannelName,
				item.OperatorNote,
			}, " "))
			if !strings.Contains(haystack, filters.Keyword) {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

func unixNow() int64 {
	return helper.GetTimestamp()
}

func paginateChannelAlertFeedItems(items []channelAlertFeedItem, page int, pageSize int) []channelAlertFeedItem {
	if len(items) == 0 {
		return items
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []channelAlertFeedItem{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func joinAlertParts(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return strings.Join(values, " · ")
}

func normalizeAlertFeedLevel(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "error":
		return "critical"
	case "warning", "warn":
		return "warning"
	default:
		return "info"
	}
}
