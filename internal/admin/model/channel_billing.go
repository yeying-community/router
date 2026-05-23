package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

const (
	ChannelBillingProfilesTableName      = "channel_billing_profiles"
	ChannelBillingSnapshotsTableName     = "channel_billing_snapshots"
	ChannelBillingSnapshotItemsTableName = "channel_billing_snapshot_items"
	ChannelBillingActionsTableName       = "channel_billing_actions"

	ChannelBillingModeUnsupported        = "unsupported"
	ChannelBillingModeManual             = "manual"
	ChannelBillingModeBuiltinOpenAI      = "builtin_openai"
	ChannelBillingModeBuiltinCloseAI     = "builtin_closeai"
	ChannelBillingModeBuiltinOpenAISB    = "builtin_openai_sb"
	ChannelBillingModeBuiltinAIProxy     = "builtin_aiproxy"
	ChannelBillingModeBuiltinAPI2GPT     = "builtin_api2gpt"
	ChannelBillingModeBuiltinAIGC2D      = "builtin_aigc2d"
	ChannelBillingModeBuiltinSiliconFlow = "builtin_siliconflow"
	ChannelBillingModeBuiltinDeepSeek    = "builtin_deepseek"
	ChannelBillingModeBuiltinOpenRouter  = "builtin_openrouter"
	ChannelBillingModeBuiltinCDK         = "builtin_cdk"

	ChannelBillingCapabilityRefreshBilling       = "refresh_billing"
	ChannelBillingCapabilityOpenActivatePage     = "open_activate_page"
	ChannelBillingCapabilityManualUpdateSnapshot = "manual_update_snapshot"
	ChannelBillingCapabilityOpenBillingPortal    = "open_billing_portal"

	ChannelBillingSnapshotSourceAPI    = "api"
	ChannelBillingSnapshotSourceManual = "manual"

	ChannelBillingActionTypeOpenActivatePage     = "open_activate_page"
	ChannelBillingActionTypeManualUpdateSnapshot = "manual_update_snapshot"

	ChannelBillingActionStatusPending = "pending"
	ChannelBillingActionStatusDone    = "done"
	ChannelBillingActionStatusFailed  = "failed"
)

type ChannelBillingProfile struct {
	ChannelId          string `json:"channel_id" gorm:"type:char(36);primaryKey"`
	Enabled            bool   `json:"enabled" gorm:"not null;default:true"`
	BillingMode        string `json:"billing_mode" gorm:"column:billing_mode;type:varchar(64);not null;default:'unsupported'"`
	BillingConfig      string `json:"billing_config" gorm:"column:billing_config;type:text"`
	ActionCapabilities string `json:"action_capabilities" gorm:"type:text"`
	ActionConfig       string `json:"action_config" gorm:"type:text"`
	NotifyConfig       string `json:"notify_config" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (ChannelBillingProfile) TableName() string {
	return ChannelBillingProfilesTableName
}

type ChannelBillingSnapshot struct {
	Id              string                       `json:"id" gorm:"type:char(36);primaryKey"`
	ChannelId       string                       `json:"channel_id" gorm:"type:char(36);not null;index"`
	SourceType      string                       `json:"source_type" gorm:"type:varchar(32);not null;default:'manual'"`
	Balance         float64                      `json:"balance" gorm:"not null;default:0"`
	Currency        string                       `json:"currency" gorm:"type:varchar(16);default:''"`
	RawStatus       string                       `json:"raw_status" gorm:"type:varchar(64);default:''"`
	Message         string                       `json:"message" gorm:"type:text"`
	RequestURL      string                       `json:"request_url" gorm:"type:text"`
	ResponseExcerpt string                       `json:"response_excerpt" gorm:"type:text"`
	OperatorUserId  string                       `json:"operator_user_id" gorm:"type:char(36);default:'';index"`
	TaskId          string                       `json:"task_id" gorm:"type:char(36);default:'';index"`
	CreatedAt       int64                        `json:"created_at" gorm:"bigint;index"`
	Items           []ChannelBillingSnapshotItem `json:"items,omitempty" gorm:"-"`
}

func (ChannelBillingSnapshot) TableName() string {
	return ChannelBillingSnapshotsTableName
}

type ChannelBillingSnapshotItem struct {
	Id         string  `json:"id" gorm:"type:char(36);primaryKey"`
	SnapshotId string  `json:"snapshot_id" gorm:"type:char(36);not null;index"`
	ChannelId  string  `json:"channel_id" gorm:"type:char(36);not null;index"`
	QuotaType  string  `json:"quota_type" gorm:"type:varchar(32);not null;default:'custom'"`
	QuotaLabel string  `json:"quota_label" gorm:"type:varchar(64);not null;default:''"`
	Amount     float64 `json:"amount" gorm:"not null;default:0"`
	Currency   string  `json:"currency" gorm:"type:varchar(16);not null;default:''"`
	ExpiresAt  int64   `json:"expires_at" gorm:"bigint;not null;default:0;index"`
	SortOrder  int     `json:"sort_order" gorm:"type:int;not null;default:0"`
	CreatedAt  int64   `json:"created_at" gorm:"bigint;index"`
}

func (ChannelBillingSnapshotItem) TableName() string {
	return ChannelBillingSnapshotItemsTableName
}

type ChannelBillingAction struct {
	Id             string `json:"id" gorm:"type:char(36);primaryKey"`
	ChannelId      string `json:"channel_id" gorm:"type:char(36);not null;index"`
	ActionType     string `json:"action_type" gorm:"type:varchar(64);not null;index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	RequestPayload string `json:"request_payload" gorm:"type:text"`
	ResultPayload  string `json:"result_payload" gorm:"type:text"`
	Message        string `json:"message" gorm:"type:text"`
	OperatorUserId string `json:"operator_user_id" gorm:"type:char(36);default:'';index"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint;index"`
}

func (ChannelBillingAction) TableName() string {
	return ChannelBillingActionsTableName
}

func normalizeChannelBillingCapabilities(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, item := range values {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func marshalJSONString(value any) string {
	if value == nil {
		return ""
	}
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(body)
}

func parseJSONArrayString(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}
	items := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return []string{}
	}
	return normalizeChannelBillingCapabilities(items)
}

func parseJSONObjectString(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func (row ChannelBillingProfile) ParseActionCapabilities() []string {
	return parseJSONArrayString(row.ActionCapabilities)
}

func (row ChannelBillingProfile) ParseActionConfig() map[string]any {
	return parseJSONObjectString(row.ActionConfig)
}

func (row ChannelBillingProfile) HasCapability(capability string) bool {
	normalized := strings.TrimSpace(capability)
	if normalized == "" {
		return false
	}
	for _, item := range row.ParseActionCapabilities() {
		if item == normalized {
			return true
		}
	}
	return false
}

func ListChannelBillingSnapshotsByChannelIDWithDB(db *gorm.DB, channelID string, limit int) ([]ChannelBillingSnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelBillingSnapshot{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows := make([]ChannelBillingSnapshot, 0, limit)
	if err := db.Where("channel_id = ?", normalizedChannelID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := HydrateChannelBillingSnapshotsWithItemsWithDB(db, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func NormalizeChannelBillingSnapshotItems(items []ChannelBillingSnapshotItem) []ChannelBillingSnapshotItem {
	if len(items) == 0 {
		return []ChannelBillingSnapshotItem{}
	}
	normalized := make([]ChannelBillingSnapshotItem, 0, len(items))
	for index, item := range items {
		nextItem := item
		nextItem.Id = strings.TrimSpace(nextItem.Id)
		nextItem.SnapshotId = strings.TrimSpace(nextItem.SnapshotId)
		nextItem.ChannelId = strings.TrimSpace(nextItem.ChannelId)
		nextItem.QuotaType = strings.TrimSpace(strings.ToLower(nextItem.QuotaType))
		if nextItem.QuotaType == "" {
			nextItem.QuotaType = "custom"
		}
		nextItem.QuotaLabel = strings.TrimSpace(nextItem.QuotaLabel)
		nextItem.Currency = strings.TrimSpace(strings.ToUpper(nextItem.Currency))
		if nextItem.SortOrder == 0 && len(items) > 1 {
			nextItem.SortOrder = index + 1
		}
		if nextItem.QuotaLabel == "" {
			switch nextItem.QuotaType {
			case "daily":
				nextItem.QuotaLabel = "日额度"
			case "weekly":
				nextItem.QuotaLabel = "周额度"
			case "monthly":
				nextItem.QuotaLabel = "月额度"
			case "total":
				nextItem.QuotaLabel = "总额度"
			}
		}
		if nextItem.Currency == "" {
			nextItem.Currency = "USD"
		}
		normalized = append(normalized, nextItem)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder != normalized[j].SortOrder {
			return normalized[i].SortOrder < normalized[j].SortOrder
		}
		if normalized[i].ExpiresAt != normalized[j].ExpiresAt {
			return normalized[i].ExpiresAt < normalized[j].ExpiresAt
		}
		return normalized[i].QuotaLabel < normalized[j].QuotaLabel
	})
	return normalized
}

func ListChannelBillingSnapshotItemsBySnapshotIDsWithDB(db *gorm.DB, snapshotIDs []string) ([]ChannelBillingSnapshotItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedSnapshotIDs := normalizeTrimmedValuesPreserveOrder(snapshotIDs)
	if len(normalizedSnapshotIDs) == 0 {
		return []ChannelBillingSnapshotItem{}, nil
	}
	rows := make([]ChannelBillingSnapshotItem, 0)
	if err := db.
		Where("snapshot_id IN ?", normalizedSnapshotIDs).
		Order("sort_order asc, created_at asc, id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return NormalizeChannelBillingSnapshotItems(rows), nil
}

func HydrateChannelBillingSnapshotsWithItemsWithDB(db *gorm.DB, rows []ChannelBillingSnapshot) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if len(rows) == 0 {
		return nil
	}
	snapshotIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Id) != "" {
			snapshotIDs = append(snapshotIDs, row.Id)
		}
	}
	items, err := ListChannelBillingSnapshotItemsBySnapshotIDsWithDB(db, snapshotIDs)
	if err != nil {
		return err
	}
	itemMap := make(map[string][]ChannelBillingSnapshotItem, len(snapshotIDs))
	for _, item := range items {
		itemMap[item.SnapshotId] = append(itemMap[item.SnapshotId], item)
	}
	for index := range rows {
		rows[index].Items = NormalizeChannelBillingSnapshotItems(itemMap[strings.TrimSpace(rows[index].Id)])
	}
	return nil
}

func GetLatestChannelBillingSnapshotByChannelIDWithDB(db *gorm.DB, channelID string) (ChannelBillingSnapshot, error) {
	if db == nil {
		return ChannelBillingSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return ChannelBillingSnapshot{}, gorm.ErrRecordNotFound
	}
	row := ChannelBillingSnapshot{}
	if err := db.
		Where("channel_id = ?", normalizedChannelID).
		Order("created_at desc, id desc").
		Take(&row).Error; err != nil {
		return ChannelBillingSnapshot{}, err
	}
	rows := []ChannelBillingSnapshot{row}
	if err := HydrateChannelBillingSnapshotsWithItemsWithDB(db, rows); err != nil {
		return ChannelBillingSnapshot{}, err
	}
	return rows[0], nil
}

func ListLatestChannelBillingSnapshotsByChannelIDsWithDB(db *gorm.DB, channelIDs []string) ([]ChannelBillingSnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		return []ChannelBillingSnapshot{}, nil
	}
	rows := make([]ChannelBillingSnapshot, 0)
	if err := db.
		Where("channel_id IN ?", normalizedChannelIDs).
		Order("channel_id asc, created_at desc, id desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	latestRows := make([]ChannelBillingSnapshot, 0, len(normalizedChannelIDs))
	seen := make(map[string]struct{}, len(normalizedChannelIDs))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		latestRows = append(latestRows, row)
	}
	if err := HydrateChannelBillingSnapshotsWithItemsWithDB(db, latestRows); err != nil {
		return nil, err
	}
	return latestRows, nil
}

func ListChannelBillingActionsByChannelIDWithDB(db *gorm.DB, channelID string, limit int) ([]ChannelBillingAction, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelBillingAction{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows := make([]ChannelBillingAction, 0, limit)
	if err := db.Where("channel_id = ?", normalizedChannelID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetChannelBillingProfileByChannelIDWithDB(db *gorm.DB, channelID string) (ChannelBillingProfile, error) {
	if db == nil {
		return ChannelBillingProfile{}, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return ChannelBillingProfile{}, gorm.ErrRecordNotFound
	}
	row := ChannelBillingProfile{}
	err := db.Where("channel_id = ?", normalizedChannelID).Take(&row).Error
	return row, err
}

func SaveChannelBillingProfileWithDB(db *gorm.DB, row ChannelBillingProfile) (ChannelBillingProfile, error) {
	if db == nil {
		return ChannelBillingProfile{}, fmt.Errorf("database handle is nil")
	}
	normalized := row
	normalized.ChannelId = strings.TrimSpace(normalized.ChannelId)
	normalized.BillingMode = strings.TrimSpace(normalized.BillingMode)
	if normalized.ChannelId == "" {
		return ChannelBillingProfile{}, fmt.Errorf("channel_id 不能为空")
	}
	if normalized.BillingMode == "" {
		normalized.BillingMode = ChannelBillingModeUnsupported
	}
	now := helper.GetTimestamp()
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now
	if err := db.Where("channel_id = ?", normalized.ChannelId).
		Assign(map[string]any{
			"enabled":             normalized.Enabled,
			"billing_mode":        normalized.BillingMode,
			"billing_config":      normalized.BillingConfig,
			"action_capabilities": normalized.ActionCapabilities,
			"action_config":       normalized.ActionConfig,
			"notify_config":       normalized.NotifyConfig,
			"updated_at":          normalized.UpdatedAt,
		}).
		FirstOrCreate(&normalized).Error; err != nil {
		return ChannelBillingProfile{}, err
	}
	return normalized, nil
}

func CreateChannelBillingActionWithDB(db *gorm.DB, row ChannelBillingAction) (ChannelBillingAction, error) {
	if db == nil {
		return ChannelBillingAction{}, fmt.Errorf("database handle is nil")
	}
	normalized := row
	normalized.ChannelId = strings.TrimSpace(normalized.ChannelId)
	normalized.ActionType = strings.TrimSpace(normalized.ActionType)
	normalized.Status = strings.TrimSpace(normalized.Status)
	normalized.OperatorUserId = strings.TrimSpace(normalized.OperatorUserId)
	if normalized.ChannelId == "" || normalized.ActionType == "" {
		return ChannelBillingAction{}, fmt.Errorf("渠道账务动作无效")
	}
	if normalized.Status == "" {
		normalized.Status = ChannelBillingActionStatusPending
	}
	now := helper.GetTimestamp()
	if normalized.Id == "" {
		normalized.Id = random.GetUUID()
	}
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now
	return normalized, db.Create(&normalized).Error
}

func CreateChannelBillingSnapshotWithDB(db *gorm.DB, row ChannelBillingSnapshot) (ChannelBillingSnapshot, error) {
	if db == nil {
		return ChannelBillingSnapshot{}, fmt.Errorf("database handle is nil")
	}
	normalized := row
	normalized.ChannelId = strings.TrimSpace(normalized.ChannelId)
	normalized.SourceType = strings.TrimSpace(normalized.SourceType)
	normalized.Currency = strings.TrimSpace(strings.ToUpper(normalized.Currency))
	normalized.OperatorUserId = strings.TrimSpace(normalized.OperatorUserId)
	if normalized.ChannelId == "" {
		return ChannelBillingSnapshot{}, fmt.Errorf("渠道账务快照无效")
	}
	if normalized.SourceType == "" {
		normalized.SourceType = ChannelBillingSnapshotSourceManual
	}
	now := helper.GetTimestamp()
	if normalized.Id == "" {
		normalized.Id = random.GetUUID()
	}
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = now
	}
	return normalized, db.Create(&normalized).Error
}

func CreateChannelBillingSnapshotItemsWithDB(db *gorm.DB, snapshotID string, channelID string, items []ChannelBillingSnapshotItem) ([]ChannelBillingSnapshotItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedSnapshotID := strings.TrimSpace(snapshotID)
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedSnapshotID == "" || normalizedChannelID == "" {
		return nil, fmt.Errorf("渠道账务额度项无效")
	}
	normalizedItems := NormalizeChannelBillingSnapshotItems(items)
	if len(normalizedItems) == 0 {
		return []ChannelBillingSnapshotItem{}, nil
	}
	now := helper.GetTimestamp()
	for index := range normalizedItems {
		if strings.TrimSpace(normalizedItems[index].Id) == "" {
			normalizedItems[index].Id = random.GetUUID()
		}
		normalizedItems[index].SnapshotId = normalizedSnapshotID
		normalizedItems[index].ChannelId = normalizedChannelID
		if normalizedItems[index].CreatedAt == 0 {
			normalizedItems[index].CreatedAt = now
		}
		if normalizedItems[index].SortOrder == 0 && len(normalizedItems) > 1 {
			normalizedItems[index].SortOrder = index + 1
		}
	}
	return normalizedItems, db.Create(&normalizedItems).Error
}

type channelBillingActivateActionConfig struct {
	URLTemplate string `json:"url_template,omitempty"`
}

type channelBillingPortalActionConfig struct {
	URL string `json:"url,omitempty"`
}

type channelBillingActionConfig struct {
	Activate      *channelBillingActivateActionConfig `json:"activate,omitempty"`
	BillingPortal *channelBillingPortalActionConfig   `json:"billing_portal,omitempty"`
}

type channelBillingConfig struct {
	APIBaseURL string `json:"api_base_url,omitempty"`
	CDK        string `json:"cdk,omitempty"`
	Currency   string `json:"currency,omitempty"`
}

func inferBuiltinChannelBillingMode(channel *Channel) string {
	if channel == nil {
		return ChannelBillingModeUnsupported
	}
	switch channel.GetChannelProtocol() {
	case relaychannel.OpenAI:
		return ChannelBillingModeBuiltinOpenAI
	case relaychannel.CloseAI:
		return ChannelBillingModeBuiltinCloseAI
	case relaychannel.OpenAISB:
		return ChannelBillingModeBuiltinOpenAISB
	case relaychannel.AIProxy:
		return ChannelBillingModeBuiltinAIProxy
	case relaychannel.API2GPT:
		return ChannelBillingModeBuiltinAPI2GPT
	case relaychannel.AIGC2D:
		return ChannelBillingModeBuiltinAIGC2D
	case relaychannel.SiliconFlow:
		return ChannelBillingModeBuiltinSiliconFlow
	case relaychannel.DeepSeek:
		return ChannelBillingModeBuiltinDeepSeek
	case relaychannel.OpenRouter:
		return ChannelBillingModeBuiltinOpenRouter
	default:
		return ChannelBillingModeUnsupported
	}
}

func (row ChannelBillingProfile) ParseBillingConfig() channelBillingConfig {
	configMap := parseJSONObjectString(row.BillingConfig)
	return channelBillingConfig{
		APIBaseURL: strings.TrimSpace(fmt.Sprintf("%v", configMap["api_base_url"])),
		CDK:        strings.TrimSpace(fmt.Sprintf("%v", configMap["cdk"])),
		Currency:   strings.TrimSpace(strings.ToUpper(fmt.Sprintf("%v", configMap["currency"]))),
	}
}

var activateCDKPattern = regexp.MustCompile(`cdk=[^&]*`)

func deriveChannelActivateURLTemplate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsedURL, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsedURL.Path), "activate") {
		return ""
	}
	if !strings.Contains(strings.ToLower(parsedURL.RawQuery), "cdk=") {
		return ""
	}
	return activateCDKPattern.ReplaceAllString(trimmed, "cdk={{cdk}}")
}

func ExtractCDKFromAccountURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("cdk"))
}

func DeriveCDKAPIBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

func BuildChannelBillingProfileFromChannelConfig(channel *Channel) (ChannelBillingProfile, bool) {
	if channel == nil {
		return ChannelBillingProfile{}, false
	}
	cfg, _ := channel.LoadConfig()
	accountBaseURL := cfg.GetAccountBaseURL()
	apiBaseURL := cfg.GetAPIBaseURL()
	if apiBaseURL == "" {
		apiBaseURL = normalizeConfiguredBaseURL(channel.GetBaseURL())
	}
	activateURLTemplate := deriveChannelActivateURLTemplate(accountBaseURL)
	if activateURLTemplate != "" {
		cdkKey := ExtractCDKFromAccountURL(accountBaseURL)
		cdkAPIBase := DeriveCDKAPIBaseURL(accountBaseURL)
		if cdkKey != "" && cdkAPIBase != "" {
			billingConfig := channelBillingConfig{
				APIBaseURL: cdkAPIBase,
				CDK:        cdkKey,
			}
			capabilities := []string{
				ChannelBillingCapabilityRefreshBilling,
				ChannelBillingCapabilityOpenActivatePage,
				ChannelBillingCapabilityManualUpdateSnapshot,
			}
			actionConfig := channelBillingActionConfig{
				Activate: &channelBillingActivateActionConfig{
					URLTemplate: activateURLTemplate,
				},
				BillingPortal: &channelBillingPortalActionConfig{
					URL: strings.TrimSpace(accountBaseURL),
				},
			}
			return ChannelBillingProfile{
				ChannelId:          strings.TrimSpace(channel.Id),
				Enabled:            true,
				BillingMode:        ChannelBillingModeBuiltinCDK,
				BillingConfig:      marshalJSONString(billingConfig),
				ActionCapabilities: marshalJSONString(capabilities),
				ActionConfig:       marshalJSONString(actionConfig),
			}, true
		}
		capabilities := []string{
			ChannelBillingCapabilityOpenActivatePage,
			ChannelBillingCapabilityManualUpdateSnapshot,
		}
		actionConfig := channelBillingActionConfig{
			Activate: &channelBillingActivateActionConfig{
				URLTemplate: activateURLTemplate,
			},
			BillingPortal: &channelBillingPortalActionConfig{
				URL: strings.TrimSpace(accountBaseURL),
			},
		}
		return ChannelBillingProfile{
			ChannelId:          strings.TrimSpace(channel.Id),
			Enabled:            true,
			BillingMode:        ChannelBillingModeManual,
			ActionCapabilities: marshalJSONString(capabilities),
			ActionConfig:       marshalJSONString(actionConfig),
		}, true
	}
	mode := inferBuiltinChannelBillingMode(channel)
	if mode == ChannelBillingModeUnsupported {
		return ChannelBillingProfile{
			ChannelId:          strings.TrimSpace(channel.Id),
			Enabled:            true,
			BillingMode:        ChannelBillingModeUnsupported,
			ActionCapabilities: marshalJSONString([]string{ChannelBillingCapabilityManualUpdateSnapshot}),
		}, true
	}
	fetchConfig := channelBillingConfig{}
	if apiBaseURL != "" {
		fetchConfig.APIBaseURL = apiBaseURL
	}
	return ChannelBillingProfile{
		ChannelId:     strings.TrimSpace(channel.Id),
		Enabled:       true,
		BillingMode:   mode,
		BillingConfig: marshalJSONString(fetchConfig),
		ActionCapabilities: marshalJSONString([]string{
			ChannelBillingCapabilityRefreshBilling,
			ChannelBillingCapabilityManualUpdateSnapshot,
		}),
	}, true
}

func GetEffectiveChannelBillingProfileWithDB(db *gorm.DB, channel *Channel) (ChannelBillingProfile, bool, error) {
	if channel == nil {
		return ChannelBillingProfile{}, false, fmt.Errorf("渠道不存在")
	}
	row, err := GetChannelBillingProfileByChannelIDWithDB(db, channel.Id)
	if err == nil {
		return row, false, nil
	}
	if errorsIsRecordNotFound(err) {
		return ChannelBillingProfile{}, false, nil
	}
	return ChannelBillingProfile{}, false, err
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func RenderChannelBillingActivateOpenURL(profile ChannelBillingProfile, cdk string) (string, error) {
	actionConfig := profile.ParseActionConfig()
	activateValue, ok := actionConfig["activate"]
	if !ok {
		return "", fmt.Errorf("当前渠道未配置激活入口")
	}
	activateConfig, ok := activateValue.(map[string]any)
	if !ok {
		return "", fmt.Errorf("当前渠道激活入口配置无效")
	}
	template := strings.TrimSpace(fmt.Sprintf("%v", activateConfig["url_template"]))
	if template == "" {
		return "", fmt.Errorf("当前渠道激活入口配置无效")
	}
	normalizedCDK := strings.TrimSpace(cdk)
	if normalizedCDK == "" {
		return "", fmt.Errorf("cdk 不能为空")
	}
	encodedCDK := url.QueryEscape(normalizedCDK)
	openURL := strings.ReplaceAll(template, "{{cdk}}", encodedCDK)
	openURL = strings.ReplaceAll(openURL, "%7B%7Bcdk%7D%7D", encodedCDK)
	openURL = strings.ReplaceAll(openURL, "%7b%7bcdk%7d%7d", encodedCDK)
	return openURL, nil
}
