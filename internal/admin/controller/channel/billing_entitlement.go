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

func collectOpenAIBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	baseURL := resolveChannelBillingAPIBaseURL(channel, profile)
	if baseURL == "" {
		return collectedChannelBillingSnapshot{}, fmt.Errorf("渠道账务未配置账务 API 地址")
	}
	subscriptionURL := fmt.Sprintf("%s/v1/dashboard/billing/subscription", baseURL)
	body, err := fetchChannelBillingResponseBody("GET", subscriptionURL, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	subscription := OpenAISubscriptionResponse{}
	if err := json.Unmarshal(body, &subscription); err != nil {
		return collectedChannelBillingSnapshot{}, err
	}

	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	usageURL := fmt.Sprintf("%s/v1/dashboard/billing/usage?start_date=%s&end_date=%s", baseURL, startDate, endDate)
	body, err = fetchChannelBillingResponseBody("GET", usageURL, channel, buildBearerAuthHeader(channel.Key))
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	usage := OpenAIUsageResponse{}
	if err := json.Unmarshal(body, &usage); err != nil {
		return collectedChannelBillingSnapshot{}, err
	}

	usedAmount := usage.TotalUsage / 100
	limitAmount := subscription.HardLimitUSD
	remainingAmount := limitAmount - usedAmount
	if remainingAmount < 0 {
		remainingAmount = 0
	}
	item := model.ChannelBillingSnapshotItem{
		ResourceType:    model.ChannelBillingResourceTypeCredit,
		QuotaType:       "total",
		QuotaLabel:      "总额度",
		Amount:          remainingAmount,
		LimitAmount:     limitAmount,
		UsedAmount:      usedAmount,
		RemainingAmount: remainingAmount,
		Currency:        "USD",
		ExpiresAt:       subscription.AccessUntil,
		SourceRef:       "openai_subscription",
		SortOrder:       1,
	}
	return collectedChannelBillingSnapshot{
		Snapshot: model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			Balance:    remainingAmount,
			Currency:   "USD",
			RawStatus:  "ok",
			Message:    strings.TrimSpace(messageText),
			RequestURL: strings.Join([]string{subscriptionURL, usageURL}, "\n"),
		},
		Items:         []model.ChannelBillingSnapshotItem{item},
		PrimaryAmount: remainingAmount,
	}, nil
}

func collectCDKBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	stats, err := fetchChannelCDKBillingStats(channel, profile)
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	cardInfo, err := fetchChannelCDKCardInfo(channel, profile)
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	currency := resolveChannelCDKBillingCurrency(profile)
	data := stats.Data
	cardInfoData := cardInfo.Data
	dailyResetAt := int64(0)
	if t, err := time.Parse(time.RFC3339Nano, data.ResetAt); err == nil {
		dailyResetAt = t.Unix()
	}
	weeklyResetAt := int64(0)
	if t, err := time.Parse(time.RFC3339Nano, data.WeeklyResetAt); err == nil {
		weeklyResetAt = t.Unix()
	}
	expiresAt := int64(0)
	for _, rawValue := range []string{cardInfoData.ExpiresAt, cardInfoData.ExpireTime} {
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(rawValue)); err == nil {
			expiresAt = t.Unix()
			break
		}
	}
	dailyLimit := data.Consumed + data.Remaining
	if cardInfoData.DailyQuota > 0 {
		dailyLimit = cardInfoData.DailyQuota
	}
	planMetadataBody, _ := json.Marshal(map[string]any{
		"status":                    strings.TrimSpace(cardInfoData.Status),
		"masked_cdk":                strings.TrimSpace(cardInfoData.MaskedCDK),
		"product_name":              strings.TrimSpace(cardInfoData.ProductName),
		"category_name":             strings.TrimSpace(cardInfoData.CategoryName),
		"category_pool":             strings.TrimSpace(cardInfoData.CategoryPool),
		"billing_mode":              strings.TrimSpace(cardInfoData.BillingMode),
		"can_renew":                 cardInfoData.CanRenew,
		"allow_refund_request":      cardInfoData.AllowRefundRequest,
		"allow_daily_limit_reset":   cardInfoData.AllowDailyLimitReset,
		"can_reset_daily_limit":     cardInfoData.CanResetDailyLimit,
		"allow_weekly_limit_reset":  cardInfoData.AllowWeeklyLimitReset,
		"can_reset_weekly_limit":    cardInfoData.CanResetWeeklyLimit,
		"limit_concurrent_sessions": cardInfoData.LimitConcurrentSessions,
		"nickname":                  strings.TrimSpace(cardInfoData.Nickname),
		"upstream_user_name":        strings.TrimSpace(cardInfoData.UpstreamUserName),
	})
	items := []model.ChannelBillingSnapshotItem{
		{
			ResourceType:    model.ChannelBillingResourceTypeQuota,
			QuotaType:       "daily",
			QuotaLabel:      "日额度",
			Amount:          data.Remaining,
			LimitAmount:     dailyLimit,
			UsedAmount:      data.Consumed,
			RemainingAmount: data.Remaining,
			Currency:        currency,
			ResetAt:         dailyResetAt,
			SourceRef:       "cdk_daily",
			SortOrder:       1,
		},
		{
			ResourceType:    model.ChannelBillingResourceTypeQuota,
			QuotaType:       "weekly",
			QuotaLabel:      "周额度",
			Amount:          data.WeeklyRemaining,
			LimitAmount:     data.WeeklyLimit,
			UsedAmount:      data.WeeklyConsumed,
			RemainingAmount: data.WeeklyRemaining,
			Currency:        currency,
			ResetAt:         weeklyResetAt,
			SourceRef:       "cdk_weekly",
			SortOrder:       2,
		},
		{
			ResourceType:    model.ChannelBillingResourceTypeCredit,
			QuotaType:       "total",
			QuotaLabel:      "总额度",
			Amount:          data.TotalRemaining,
			LimitAmount:     data.TotalLimit,
			UsedAmount:      data.TotalConsumed,
			RemainingAmount: data.TotalRemaining,
			Currency:        currency,
			SourceRef:       "cdk_total",
			SortOrder:       3,
		},
		{
			ResourceType:    model.ChannelBillingResourceTypePlan,
			QuotaType:       "custom",
			QuotaLabel:      "套餐有效期",
			Amount:          1,
			LimitAmount:     1,
			UsedAmount:      0,
			RemainingAmount: 1,
			Currency:        currency,
			ExpiresAt:       expiresAt,
			SourceRef:       "cdk_plan",
			Metadata:        string(planMetadataBody),
			SortOrder:       4,
		},
	}
	snapshotMessageParts := make([]string, 0, 3)
	if message := strings.TrimSpace(messageText); message != "" {
		snapshotMessageParts = append(snapshotMessageParts, message)
	}
	if productName := strings.TrimSpace(cardInfoData.ProductName); productName != "" {
		snapshotMessageParts = append(snapshotMessageParts, fmt.Sprintf("套餐=%s", productName))
	}
	if categoryName := strings.TrimSpace(cardInfoData.CategoryName); categoryName != "" {
		snapshotMessageParts = append(snapshotMessageParts, fmt.Sprintf("分类=%s", categoryName))
	}
	return collectedChannelBillingSnapshot{
		Snapshot: model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			Balance:    data.TotalRemaining,
			Currency:   currency,
			RawStatus:  "ok",
			Message:    strings.Join(snapshotMessageParts, " | "),
			RequestURL: strings.Join([]string{
				resolveChannelCDKBillingRequestURL(channel, profile),
				resolveChannelCDKCardInfoRequestURL(channel, profile),
			}, "\n"),
		},
		Items:         items,
		PrimaryAmount: data.TotalRemaining,
	}, nil
}

func collectLegacyTotalBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	amount, err := refreshChannelBillingAmount(channel)
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	currency := resolveChannelBillingSnapshotCurrency(channel)
	item := model.ChannelBillingSnapshotItem{
		ResourceType:    model.ChannelBillingResourceTypeCredit,
		QuotaType:       "total",
		QuotaLabel:      "总额度",
		Amount:          amount,
		RemainingAmount: amount,
		Currency:        currency,
		SourceRef:       "legacy_total",
		SortOrder:       1,
	}
	return collectedChannelBillingSnapshot{
		Snapshot: model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			Balance:    amount,
			Currency:   currency,
			RawStatus:  "ok",
			Message:    strings.TrimSpace(messageText),
			RequestURL: strings.Join(resolveChannelBillingRequestURLs(channel), "\n"),
		},
		Items:          []model.ChannelBillingSnapshotItem{item},
		PrimaryAmount:  amount,
		ShouldHardStop: amount <= 0,
	}, nil
}

func collectChannelBillingSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	switch strings.TrimSpace(profile.BillingMode) {
	case model.ChannelBillingModeBuiltinCDK:
		return collectCDKBillingSnapshot(channel, profile, messageText)
	case model.ChannelBillingModeBuiltinOpenAI:
		return collectOpenAIBillingSnapshot(channel, profile, messageText)
	case model.ChannelBillingModeBuiltinCloseAI,
		model.ChannelBillingModeBuiltinOpenAISB,
		model.ChannelBillingModeBuiltinAIProxy,
		model.ChannelBillingModeBuiltinAPI2GPT,
		model.ChannelBillingModeBuiltinAIGC2D,
		model.ChannelBillingModeBuiltinSiliconFlow,
		model.ChannelBillingModeBuiltinDeepSeek,
		model.ChannelBillingModeBuiltinOpenRouter:
		return collectLegacyTotalBillingSnapshot(channel, profile, messageText)
	default:
		return collectedChannelBillingSnapshot{}, fmt.Errorf("当前渠道不支持自动刷新账务")
	}
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
	if strings.TrimSpace(profile.BillingMode) == model.ChannelBillingModeBuiltinCDK {
		return []string{
			resolveChannelCDKBillingRequestURL(channel, profile),
			resolveChannelCDKCardInfoRequestURL(channel, profile),
		}
	}
	return resolveChannelBillingRequestURLs(channel)
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
	if strings.TrimSpace(profile.BillingMode) != model.ChannelBillingModeBuiltinCDK {
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

func createBillingRefreshFailureContent(channel *model.Channel, profile model.ChannelBillingProfile, err error) (string, string, string, string) {
	if channel == nil || err == nil {
		return "", "", "", ""
	}
	channelName := strings.TrimSpace(channel.DisplayName())
	channelID := strings.TrimSpace(channel.Id)
	reason := strings.TrimSpace(err.Error())
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
	channelText := channelName
	if channelID != "" {
		channelText = fmt.Sprintf("%s (#%s)", channelName, channelID)
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
				<p>权益：%s</p>
				<p>处理：续费、切换备用渠道或主动下线，避免到期后继续路由。</p>
			`, formatBillingAlertTime(item.ExpiresAt), channelText, label)
			return subject, content
		} else {
			subject := "渠道额度即将到期"
			content = fmt.Sprintf(`
				<p><strong>额度即将到期</strong>：%s</p>
				<p>类别：额度到期</p>
				<p>渠道：%s</p>
				<p>额度：%s</p>
				<p>剩余：%s</p>
				<p>处理：续费、升级或充值，避免额度到期后不可用。</p>
			`, formatBillingAlertTime(item.ExpiresAt), channelText, label, formatBillingAlertAmount(item.RemainingAmount, item.Currency))
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
			<p>剩余：%s / %s（%s）</p>
			<p>处理：充值、升级套餐或切换备用渠道。</p>
		`, label, channelText, formatBillingAlertAmount(item.RemainingAmount, item.Currency), formatBillingAlertAmount(item.LimitAmount, item.Currency), ratioText)
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
	today := time.Now().Format("2006-01-02")
	eventType, category, title, content := createBillingRefreshFailureContent(channel, profile, cause)
	if eventType == "" || title == "" || content == "" {
		return nil
	}
	alertKey := buildChannelBillingRefreshFailureAlertKey(channel, profile)
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
		"billing_mode":         strings.TrimSpace(profile.BillingMode),
		"billing_api_base_url": resolveChannelBillingAPIBaseURL(channel, profile),
		"category":             category,
		"reason":               strings.TrimSpace(cause.Error()),
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

func refreshAndPersistChannelBillingEntitlements(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (float64, error) {
	collected, err := collectChannelBillingSnapshot(channel, profile, messageText)
	if err != nil {
		if persistErr := persistChannelBillingRefreshFailure(channel, profile, messageText, err); persistErr != nil {
			return 0, persistErr
		}
		return 0, err
	}
	_, items, err := persistCollectedChannelBillingSnapshot(channel, profile, collected)
	if err != nil {
		return 0, err
	}
	if shouldDisableChannelForBillingEntitlements(collected, items, time.Now().Unix()) {
		monitor.DisableChannelForInsufficientBalance(channel.Id, channel.DisplayName(), collected.PrimaryAmount)
		return collected.PrimaryAmount, nil
	}
	return collected.PrimaryAmount, nil
}
