package channel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/monitor"

	"gorm.io/gorm"
)

type collectedChannelBillingSnapshot struct {
	Snapshot       model.ChannelBillingSnapshot
	Items          []model.ChannelBillingSnapshotItem
	PrimaryAmount  float64
	ShouldHardStop bool
}

func determineBillingItemStatus(item model.ChannelBillingSnapshotItem, now time.Time, lowRemainingRatio float64) string {
	if item.ExpiresAt > 0 && item.ExpiresAt <= now.Unix() {
		return model.ChannelBillingItemStatusExpired
	}
	if item.ResetAt > 0 && item.ResetAt <= now.Unix() {
		return model.ChannelBillingItemStatusExpired
	}
	if item.RemainingAmount <= 0 {
		return model.ChannelBillingItemStatusDepleted
	}
	if item.LimitAmount > 0 && item.RemainingAmount/item.LimitAmount <= lowRemainingRatio {
		return model.ChannelBillingItemStatusLow
	}
	return model.ChannelBillingItemStatusActive
}

func finalizeCollectedBillingItems(items []model.ChannelBillingSnapshotItem, notifyConfig model.ChannelBillingNotifyConfig) []model.ChannelBillingSnapshotItem {
	now := time.Now()
	normalized := model.NormalizeChannelBillingSnapshotItems(items)
	for index := range normalized {
		if normalized[index].RemainingAmount == 0 && normalized[index].Amount > 0 {
			normalized[index].RemainingAmount = normalized[index].Amount
		}
		if normalized[index].Amount == 0 && normalized[index].RemainingAmount > 0 {
			normalized[index].Amount = normalized[index].RemainingAmount
		}
		normalized[index].Status = determineBillingItemStatus(normalized[index], now, notifyConfig.LowRemainingRatio)
	}
	return normalized
}

func isPackageBillingQuotaType(quotaType string) bool {
	switch strings.TrimSpace(strings.ToLower(quotaType)) {
	case "daily", "weekly", "monthly":
		return true
	default:
		return false
	}
}

func isUsableBillingEntitlement(item model.ChannelBillingSnapshotItem, now int64) bool {
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(item.Status)) {
	case model.ChannelBillingItemStatusDepleted, model.ChannelBillingItemStatusExpired:
		return false
	}
	return item.RemainingAmount > 0
}

func shouldDisableChannelForBillingEntitlements(collected collectedChannelBillingSnapshot, items []model.ChannelBillingSnapshotItem, now int64) bool {
	normalized := model.NormalizeChannelBillingSnapshotItems(items)
	hasPackageEntitlement := false
	hasUsablePackageEntitlement := false
	for _, item := range normalized {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		quotaType := strings.TrimSpace(strings.ToLower(item.QuotaType))
		isPackageEntitlement := resourceType == model.ChannelBillingResourceTypePlan || isPackageBillingQuotaType(quotaType)
		if !isPackageEntitlement {
			continue
		}
		hasPackageEntitlement = true
		if isUsableBillingEntitlement(item, now) {
			hasUsablePackageEntitlement = true
		}
	}
	if hasPackageEntitlement {
		return !hasUsablePackageEntitlement
	}
	if collected.ShouldHardStop {
		return true
	}
	for _, item := range normalized {
		if strings.TrimSpace(strings.ToLower(item.QuotaType)) == "total" && item.RemainingAmount <= 0 {
			return true
		}
	}
	return false
}

func collectChannelBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	if !billingServiceConfigured() {
		return collectedChannelBillingSnapshot{}, fmt.Errorf("渠道账务未配置 Billing 服务")
	}
	if resolveBillingServiceAdapter(profile) == "" {
		return collectedChannelBillingSnapshot{}, fmt.Errorf("当前渠道不支持 Billing 服务刷新账务")
	}
	return collectBillingServiceSnapshot(channel, profile, messageText)
}

func buildChannelBillingAlertKey(item model.ChannelBillingSnapshotItem) string {
	base := []string{
		strings.TrimSpace(item.ResourceType),
		strings.TrimSpace(item.QuotaType),
		strings.TrimSpace(item.SourceRef),
		strings.TrimSpace(item.QuotaLabel),
	}
	return strings.Join(base, "::")
}

func buildChannelBillingRefreshFailureAlertKey(channel *model.Channel, profile model.ChannelBillingProfile) string {
	return strings.Join([]string{
		"refresh",
		strings.TrimSpace(profile.BillingMode),
		strings.TrimSpace(resolveChannelBillingAPIBaseURL(channel, profile)),
	}, "::")
}

const channelBillingRefreshFailureAlertThresholdSeconds int64 = 30 * 60

func shouldNotifyChannelBillingRefreshFailureForStreak(nowTs int64, lastSuccessAt int64, firstFailureAt int64) bool {
	if firstFailureAt <= 0 || nowTs <= 0 {
		return false
	}
	if lastSuccessAt > 0 && lastSuccessAt >= nowTs-channelBillingRefreshFailureAlertThresholdSeconds {
		return false
	}
	return firstFailureAt <= nowTs-channelBillingRefreshFailureAlertThresholdSeconds
}

func resolveChannelBillingRefreshFailureStreakStart(channelID string, nowTs int64) (int64, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" || nowTs <= 0 {
		return 0, nil
	}
	lastSuccessAt, err := model.GetLatestChannelBillingSnapshotCreatedAtByStatusWithDB(
		model.DB,
		normalizedChannelID,
		model.ChannelBillingSnapshotSourceAPI,
		"ok",
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		lastSuccessAt = 0
	}
	firstFailureAt, err := model.GetEarliestChannelBillingSnapshotCreatedAtByStatusAfterWithDB(
		model.DB,
		normalizedChannelID,
		model.ChannelBillingSnapshotSourceAPI,
		"failed",
		lastSuccessAt,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if !shouldNotifyChannelBillingRefreshFailureForStreak(nowTs, lastSuccessAt, firstFailureAt) {
		return 0, nil
	}
	return firstFailureAt, nil
}

func copyBillingSnapshotItemsForSnapshot(items []model.ChannelBillingSnapshotItem) []model.ChannelBillingSnapshotItem {
	copied := model.NormalizeChannelBillingSnapshotItems(items)
	for index := range copied {
		copied[index].Id = ""
		copied[index].SnapshotId = ""
		copied[index].ChannelId = ""
		copied[index].CreatedAt = 0
	}
	return copied
}

func resolveChannelBillingFailureRequestURLs(channel *model.Channel, profile model.ChannelBillingProfile) []string {
	return resolveBillingServiceRequestURLs()
}

func isPlanBillingAlertItem(item model.ChannelBillingSnapshotItem) bool {
	return strings.TrimSpace(strings.ToLower(item.ResourceType)) == model.ChannelBillingResourceTypePlan
}

func formatBillingAlertAmount(amount float64, currency string) string {
	currency = strings.TrimSpace(strings.ToUpper(currency))
	if currency == "" {
		return fmt.Sprintf("%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}

func formatBillingAlertTime(unixTime int64) string {
	if unixTime <= 0 {
		return "-"
	}
	return time.Unix(unixTime, 0).Format("2006-01-02 15:04:05")
}

func shouldSkipExistingBillingAlert(channelID string, eventType string, alertKey string, notifyDate string) (bool, error) {
	existing, err := model.GetChannelBillingAlertEventByDedupeKeyWithDB(model.DB, channelID, eventType, alertKey, notifyDate)
	if err == nil {
		return strings.TrimSpace(strings.ToLower(existing.Status)) == model.ChannelBillingAlertStatusSent, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func isCDKAuthExpiredBillingError(profile model.ChannelBillingProfile, err error) bool {
	if err == nil {
		return false
	}
	if resolveBillingServiceAdapter(profile) != "aixhan" {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(reason, "认证无效") ||
		strings.Contains(reason, "已过期") ||
		strings.Contains(reason, "过期") ||
		strings.Contains(reason, "expired") ||
		strings.Contains(reason, "invalid")
}

func isBillingResponseParseError(err error) bool {
	if err == nil {
		return false
	}
	reason := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(reason, "invalid character") ||
		strings.Contains(reason, "cannot unmarshal") ||
		strings.Contains(reason, "unexpected end of json") ||
		strings.Contains(reason, "json")
}

func sanitizeBillingAlertReason(err error) string {
	if err == nil {
		return ""
	}
	return model.SanitizeChannelBillingAlertReason(err.Error())
}

func createBillingRefreshFailureContent(channel *model.Channel, profile model.ChannelBillingProfile, err error) (string, string, string, string) {
	if channel == nil || err == nil {
		return "", "", "", ""
	}
	channelName := strings.TrimSpace(channel.DisplayName())
	channelID := strings.TrimSpace(channel.Id)
	reason := sanitizeBillingAlertReason(err)
	if reason == "" {
		reason = "账务接口返回异常"
	}
	if isCDKAuthExpiredBillingError(profile, err) {
		return model.ChannelBillingAlertTypePlanExpired, "套餐到期", "渠道套餐已到期", fmt.Sprintf(`
			<p>类别：套餐到期</p>
			<p>渠道：%s</p>
			<p>标识：%s</p>
			<p>原因：%s</p>
			<p>处理：续费、切换备用渠道或主动下线，避免继续路由到不可用渠道。</p>
		`, channelName, channelID, reason)
	}
	if isBillingResponseParseError(err) {
		return model.ChannelBillingAlertTypeResponseError, "响应异常", "渠道账务接口响应异常", fmt.Sprintf(`
			<p>类别：响应异常</p>
			<p>渠道：%s</p>
			<p>标识：%s</p>
			<p>原因：上游账务接口返回了非预期内容，无法解析为账务数据。</p>
			<p>详情：%s</p>
			<p>处理：检查账务 API 地址、CDK 类型和上游服务状态；如果地址打开的是网页或错误页，请改为正确的账务接口地址。</p>
		`, channelName, channelID, reason)
	}
	return model.ChannelBillingAlertTypeRefreshFailed, "刷新失败", "渠道账务刷新失败", fmt.Sprintf(`
		<p>类别：刷新失败</p>
		<p>渠道：%s</p>
		<p>标识：%s</p>
		<p>原因：%s</p>
		<p>处理：检查 CDK、账务 API 地址或上游账号状态；恢复后下次刷新会重新同步权益项。</p>
	`, channelName, channelID, reason)
}

func createBillingAlertContent(channel *model.Channel, item model.ChannelBillingSnapshotItem, eventType string) (string, string) {
	channelName := ""
	channelID := ""
	if channel != nil {
		channelName = strings.TrimSpace(channel.DisplayName())
		channelID = strings.TrimSpace(channel.Id)
	}
	label := strings.TrimSpace(item.QuotaLabel)
	if label == "" {
		label = strings.TrimSpace(item.QuotaType)
	}
	switch eventType {
	case model.ChannelBillingAlertTypeExpiringSoon:
		var content string
		if isPlanBillingAlertItem(item) {
			subject := "渠道套餐即将到期"
			content = fmt.Sprintf(`
				<p><strong>套餐即将到期</strong>：%s</p>
				<p>类别：套餐到期</p>
				<p>渠道：%s</p>
				<p>标识：%s</p>
				<p>权益：%s</p>
				<p>处理：续费、切换备用渠道或主动下线，避免到期后继续路由。</p>
			`, formatBillingAlertTime(item.ExpiresAt), channelName, channelID, label)
			return subject, content
		} else {
			subject := "渠道额度即将到期"
			content = fmt.Sprintf(`
				<p><strong>额度即将到期</strong>：%s</p>
				<p>类别：额度到期</p>
				<p>渠道：%s</p>
				<p>标识：%s</p>
				<p>额度：%s</p>
				<p>剩余：%s</p>
				<p>处理：续费、升级或充值，避免额度到期后不可用。</p>
			`, formatBillingAlertTime(item.ExpiresAt), channelName, channelID, label, formatBillingAlertAmount(item.RemainingAmount, item.Currency))
			return subject, content
		}
	case model.ChannelBillingAlertTypeLowRemaining:
		ratioText := "-"
		if item.LimitAmount > 0 {
			ratioText = fmt.Sprintf("%.2f%%", item.RemainingAmount/item.LimitAmount*100)
		}
		subject := "渠道额度余额偏低"
		content := fmt.Sprintf(`
			<p><strong>额度余额偏低</strong>：%s</p>
			<p>类别：余额偏低</p>
			<p>渠道：%s</p>
			<p>标识：%s</p>
			<p>剩余：%s / %s（%s）</p>
			<p>处理：充值、升级套餐或切换备用渠道。</p>
		`, label, channelName, channelID, formatBillingAlertAmount(item.RemainingAmount, item.Currency), formatBillingAlertAmount(item.LimitAmount, item.Currency), ratioText)
		return subject, content
	default:
		return "", ""
	}
}

func maybeNotifyChannelBillingAlert(channel *model.Channel, snapshotID string, item model.ChannelBillingSnapshotItem, eventType string) error {
	if channel == nil {
		return nil
	}
	today := time.Now().Format("2006-01-02")
	alertKey := buildChannelBillingAlertKey(item)
	shouldSkip, err := shouldSkipExistingBillingAlert(channel.Id, eventType, alertKey, today)
	if err != nil {
		return err
	}
	if shouldSkip {
		return nil
	}
	title, content := createBillingAlertContent(channel, item, eventType)
	if title == "" || content == "" {
		return nil
	}
	status := model.ChannelBillingAlertStatusSent
	if err := monitor.NotifyRootUser(title, content); err != nil {
		status = model.ChannelBillingAlertStatusFailed
	}
	payloadBody, _ := json.Marshal(map[string]any{
		"resource_type":    item.ResourceType,
		"quota_type":       item.QuotaType,
		"quota_label":      item.QuotaLabel,
		"limit_amount":     item.LimitAmount,
		"used_amount":      item.UsedAmount,
		"remaining_amount": item.RemainingAmount,
		"currency":         item.Currency,
		"reset_at":         item.ResetAt,
		"expires_at":       item.ExpiresAt,
		"status":           item.Status,
	})
	_, saveErr := model.SaveChannelBillingAlertEventWithDB(model.DB, model.ChannelBillingAlertEvent{
		ChannelId:  channel.Id,
		SnapshotId: snapshotID,
		EventType:  eventType,
		AlertKey:   alertKey,
		NotifyDate: today,
		Severity:   "warning",
		Status:     status,
		Title:      title,
		Content:    content,
		Payload:    string(payloadBody),
	})
	return saveErr
}

func maybeNotifyChannelBillingRefreshFailure(channel *model.Channel, profile model.ChannelBillingProfile, snapshotID string, cause error) error {
	if channel == nil || cause == nil {
		return nil
	}
	nowTs := helper.GetTimestamp()
	streakStartAt, err := resolveChannelBillingRefreshFailureStreakStart(channel.Id, nowTs)
	if err != nil {
		return err
	}
	if streakStartAt <= 0 {
		return nil
	}
	today := time.Now().Format("2006-01-02")
	eventType, category, title, content := createBillingRefreshFailureContent(channel, profile, cause)
	if eventType == "" || title == "" || content == "" {
		return nil
	}
	alertKey := fmt.Sprintf("%s::%d", buildChannelBillingRefreshFailureAlertKey(channel, profile), streakStartAt)
	shouldSkip, err := shouldSkipExistingBillingAlert(channel.Id, eventType, alertKey, today)
	if err != nil {
		return err
	}
	if shouldSkip {
		return nil
	}
	status := model.ChannelBillingAlertStatusSent
	if err := monitor.NotifyRootUser(title, content); err != nil {
		status = model.ChannelBillingAlertStatusFailed
	}
	payload := map[string]any{
		"billing_mode":                  strings.TrimSpace(profile.BillingMode),
		"billing_api_base_url":          resolveChannelBillingAPIBaseURL(channel, profile),
		"category":                      category,
		"reason":                        sanitizeBillingAlertReason(cause),
		"failure_streak_started_at":     streakStartAt,
		"failure_streak_window_seconds": channelBillingRefreshFailureAlertThresholdSeconds,
	}
	if eventType == model.ChannelBillingAlertTypePlanExpired {
		payload["resource_type"] = model.ChannelBillingResourceTypePlan
		payload["quota_type"] = "custom"
		payload["quota_label"] = "套餐有效期"
		payload["status"] = model.ChannelBillingItemStatusExpired
	}
	payloadBody, _ := json.Marshal(payload)
	_, saveErr := model.SaveChannelBillingAlertEventWithDB(model.DB, model.ChannelBillingAlertEvent{
		ChannelId:  channel.Id,
		SnapshotId: snapshotID,
		EventType:  eventType,
		AlertKey:   alertKey,
		NotifyDate: today,
		Severity:   "warning",
		Status:     status,
		Title:      title,
		Content:    content,
		Payload:    string(payloadBody),
	})
	return saveErr
}

func evaluateAndNotifyChannelBillingAlerts(channel *model.Channel, profile model.ChannelBillingProfile, snapshot model.ChannelBillingSnapshot, items []model.ChannelBillingSnapshotItem) error {
	if channel == nil {
		return nil
	}
	cfg := profile.ParseNotifyConfig()
	now := time.Now().Unix()
	for _, item := range items {
		if item.ExpiresAt > 0 && item.ExpiresAt > now {
			remainingDays := (item.ExpiresAt - now) / 86400
			if remainingDays <= int64(cfg.ExpiryNoticeDays) {
				if err := maybeNotifyChannelBillingAlert(channel, snapshot.Id, item, model.ChannelBillingAlertTypeExpiringSoon); err != nil {
					return err
				}
			}
		}
		if item.LimitAmount > 0 && item.RemainingAmount > 0 && item.RemainingAmount/item.LimitAmount <= cfg.LowRemainingRatio {
			switch item.QuotaType {
			case "daily", "weekly", "monthly", "total":
				if err := maybeNotifyChannelBillingAlert(channel, snapshot.Id, item, model.ChannelBillingAlertTypeLowRemaining); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func persistCollectedChannelBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, collected collectedChannelBillingSnapshot) (model.ChannelBillingSnapshot, []model.ChannelBillingSnapshotItem, error) {
	if channel == nil {
		return model.ChannelBillingSnapshot{}, nil, fmt.Errorf("渠道不存在")
	}
	notifyCfg := profile.ParseNotifyConfig()
	finalItems := finalizeCollectedBillingItems(collected.Items, notifyCfg)
	now := helper.GetTimestamp()
	collected.Snapshot.CreatedAt = now
	savedSnapshot := model.ChannelBillingSnapshot{}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		row, err := model.CreateChannelBillingSnapshotWithDB(tx, collected.Snapshot)
		if err != nil {
			return err
		}
		savedSnapshot = row
		savedItems, err := model.CreateChannelBillingSnapshotItemsWithDB(tx, row.Id, channel.Id, finalItems)
		if err != nil {
			return err
		}
		finalItems = savedItems
		return nil
	})
	if err != nil {
		return model.ChannelBillingSnapshot{}, nil, err
	}
	if err := evaluateAndNotifyChannelBillingAlerts(channel, profile, savedSnapshot, finalItems); err != nil {
		return model.ChannelBillingSnapshot{}, nil, err
	}
	return savedSnapshot, finalItems, nil
}

func persistChannelBillingRefreshFailure(channel *model.Channel, profile model.ChannelBillingProfile, messageText string, cause error) error {
	if channel == nil || cause == nil {
		return nil
	}
	now := helper.GetTimestamp()
	previousSnapshot, err := model.GetLatestChannelBillingSnapshotByChannelIDWithDB(model.DB, channel.Id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	messageParts := []string{}
	if message := strings.TrimSpace(messageText); message != "" {
		messageParts = append(messageParts, message)
	}
	messageParts = append(messageParts, strings.TrimSpace(cause.Error()))
	failedSnapshot := model.ChannelBillingSnapshot{
		ChannelId:  strings.TrimSpace(channel.Id),
		SourceType: model.ChannelBillingSnapshotSourceAPI,
		Balance:    previousSnapshot.Balance,
		Currency:   previousSnapshot.Currency,
		RawStatus:  "failed",
		Message:    strings.Join(messageParts, " | "),
		RequestURL: strings.Join(resolveChannelBillingFailureRequestURLs(channel, profile), "\n"),
		CreatedAt:  now,
	}
	savedSnapshot := model.ChannelBillingSnapshot{}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		row, err := model.CreateChannelBillingSnapshotWithDB(tx, failedSnapshot)
		if err != nil {
			return err
		}
		savedSnapshot = row
		copiedItems := finalizeCollectedBillingItems(
			copyBillingSnapshotItemsForSnapshot(previousSnapshot.Items),
			profile.ParseNotifyConfig(),
		)
		_, err = model.CreateChannelBillingSnapshotItemsWithDB(
			tx,
			row.Id,
			channel.Id,
			copiedItems,
		)
		return err
	})
	if err != nil {
		return err
	}
	return maybeNotifyChannelBillingRefreshFailure(channel, profile, savedSnapshot.Id, cause)
}

func splitChannelBillingRequestURLs(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	urls := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

func refreshAndPersistChannelBillingEntitlements(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (float64, []string, error) {
	collected, err := collectChannelBillingSnapshot(channel, profile, messageText)
	if err != nil {
		if persistErr := persistChannelBillingRefreshFailure(channel, profile, messageText, err); persistErr != nil {
			return 0, nil, persistErr
		}
		return 0, nil, err
	}
	snapshot, items, err := persistCollectedChannelBillingSnapshot(channel, profile, collected)
	if err != nil {
		return 0, nil, err
	}
	requestURLs := splitChannelBillingRequestURLs(snapshot.RequestURL)
	now := time.Now().Unix()
	if shouldDisableChannelForBillingEntitlements(collected, items, now) {
		if err := monitor.DisableChannelForInsufficientBalance(channel.Id, channel.DisplayName(), collected.PrimaryAmount); err != nil {
			return collected.PrimaryAmount, requestURLs, err
		}
		return collected.PrimaryAmount, requestURLs, nil
	}
	return collected.PrimaryAmount, requestURLs, nil
}
