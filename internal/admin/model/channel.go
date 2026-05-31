package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/yeying-community/router/common/random"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
	ChannelStatusCreating         = 4
	ChannelStatusHalfOpen         = 5

	ChannelIdentifierMaxLength = 64
	ChannelHalfOpenPriority    = -1 << 60
)

var channelIdentifierPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type Channel struct {
	Id                    string         `json:"id" gorm:"type:char(36);primaryKey"`
	Protocol              string         `json:"protocol" gorm:"type:varchar(64);default:'openai';index"`
	Key                   string         `json:"key" gorm:"type:text"`
	Status                int            `json:"status" gorm:"default:1"`
	Name                  string         `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Weight                *uint          `json:"weight" gorm:"default:0"`
	CreatedTime           int64          `json:"created_time" gorm:"bigint"`
	UpdatedAt             int64          `json:"updated_at" gorm:"bigint;index"`
	TestTime              int64          `json:"test_time" gorm:"bigint"`
	ResponseTime          int            `json:"response_time"`
	BaseURL               *string        `json:"base_url" gorm:"column:base_url;default:''"`
	Other                 *string        `json:"other"`
	Models                string         `json:"models" gorm:"-"`
	AvailableModels       []string       `json:"available_models,omitempty" gorm:"-"`
	ChannelModels         []ChannelModel `json:"channel_models,omitempty" gorm:"-"`
	Tests                 []ChannelTest  `json:"channel_tests,omitempty" gorm:"-"`
	TestsLastTestedAt     int64          `json:"channel_tests_last_tested_at,omitempty" gorm:"-"`
	UsedQuota             int64          `json:"used_quota" gorm:"bigint;default:0"`
	Priority              *int64         `json:"priority" gorm:"bigint;default:0"`
	Config                string         `json:"config"`
	SystemPrompt          *string        `json:"system_prompt" gorm:"type:text"`
	TestModel             string         `json:"test_model" gorm:"type:varchar(255);default:''"`
	KeySet                bool           `json:"key_set" gorm:"-"`
	ModelsProvided        bool           `json:"-" gorm:"-"`
	ChannelModelsProvided bool           `json:"-" gorm:"-"`
	NameProvided          bool           `json:"-" gorm:"-"`
}

type ChannelConfig struct {
	Region            string `json:"region,omitempty"`
	SK                string `json:"sk,omitempty"`
	AK                string `json:"ak,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	APIVersion        string `json:"api_version,omitempty"`
	LibraryID         string `json:"library_id,omitempty"`
	Plugin            string `json:"plugin,omitempty"`
	APIBaseURL        string `json:"api_base_url,omitempty"`
	AccountBaseURL    string `json:"account_base_url,omitempty"`
	VertexAIProjectID string `json:"vertex_ai_project_id,omitempty"`
	VertexAIADC       string `json:"vertex_ai_adc,omitempty"`
}

func normalizeConfiguredBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func baseURLHasVersionSuffix(raw string, suffix string) bool {
	normalized := strings.ToLower(normalizeConfiguredBaseURL(raw))
	return normalized != "" && strings.HasSuffix(normalized, strings.ToLower(strings.TrimSpace(suffix)))
}

func (config ChannelConfig) GetAPIBaseURL() string {
	return normalizeConfiguredBaseURL(config.APIBaseURL)
}

func (config ChannelConfig) GetAccountBaseURL() string {
	return normalizeConfiguredBaseURL(config.AccountBaseURL)
}

func (channel *Channel) NormalizeProtocol() {
	if channel == nil {
		return
	}
	protocol := relaychannel.NormalizeProtocolName(channel.Protocol)
	if protocol == "" {
		protocol = "openai"
	}
	channel.Protocol = protocol
}

func NormalizeChannelIdentifier(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func ValidateChannelIdentifier(id string) error {
	normalized := NormalizeChannelIdentifier(id)
	switch {
	case normalized == "":
		return fmt.Errorf("渠道标识不能为空")
	case len(normalized) > ChannelIdentifierMaxLength:
		return fmt.Errorf("渠道标识长度不能超过 %d 个字符", ChannelIdentifierMaxLength)
	case !channelIdentifierPattern.MatchString(normalized):
		return fmt.Errorf("渠道标识只支持小写字母、数字和 -")
	default:
		return nil
	}
}

func (channel *Channel) NormalizeIdentity() {
	if channel == nil {
		return
	}
	channel.Id = strings.TrimSpace(channel.Id)
	channel.Name = NormalizeChannelIdentifier(channel.Name)
}

func (channel *Channel) AfterFind(tx *gorm.DB) error {
	channel.NormalizeIdentity()
	channel.NormalizeProtocol()
	return nil
}

func (channel *Channel) ValidateIdentifier() error {
	if channel == nil {
		return fmt.Errorf("渠道不能为空")
	}
	return ValidateChannelIdentifier(channel.Name)
}

func (channel *Channel) ValidateProtocolConfiguration() error {
	if channel == nil {
		return fmt.Errorf("渠道不能为空")
	}
	switch channel.GetChannelProtocol() {
	case relaychannel.DeepSeek:
		if baseURLHasVersionSuffix(channel.GetBaseURL(), "/v1") {
			return fmt.Errorf("DeepSeek 渠道 base_url 不能追加 /v1，请使用 https://api.deepseek.com 或 https://api.deepseek.com/beta")
		}
		cfg, err := channel.LoadConfig()
		if err != nil {
			return nil
		}
		if baseURLHasVersionSuffix(cfg.GetAPIBaseURL(), "/v1") {
			return fmt.Errorf("DeepSeek 渠道 config.api_base_url 不能追加 /v1，请使用 https://api.deepseek.com 或 https://api.deepseek.com/beta")
		}
	}
	return nil
}

func (channel *Channel) DisplayName() string {
	if channel == nil {
		return ""
	}
	if name := NormalizeChannelIdentifier(channel.Name); name != "" {
		return name
	}
	return strings.TrimSpace(channel.Id)
}

func (channel *Channel) EnsureID() {
	if channel == nil {
		return
	}
	if strings.TrimSpace(channel.Id) == "" {
		channel.Id = random.GetUUID()
	}
}

func (channel *Channel) GetProtocol() string {
	if channel == nil {
		return "openai"
	}
	protocol := relaychannel.NormalizeProtocolName(channel.Protocol)
	if protocol != "" {
		return protocol
	}
	return "openai"
}

func (channel *Channel) GetChannelProtocol() int {
	if channel == nil {
		return relaychannel.OpenAI
	}
	return relaychannel.TypeByProtocol(channel.GetProtocol())
}

func GetChannelById(id string) (*Channel, error) {
	return mustChannelRepo().GetChannelById(id)
}

func (channel *Channel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

func CloneChannelWithPriority(channel *Channel, priority int64) *Channel {
	if channel == nil {
		return nil
	}
	cloned := *channel
	clonedPriority := priority
	cloned.Priority = &clonedPriority
	return &cloned
}

func (channel *Channel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	return strings.TrimSpace(*channel.BaseURL)
}

func (channel *Channel) ResolveAPIBaseURL(requestPath string) string {
	return channel.ResolveAPIBaseURLForModel(requestPath)
}

func (channel *Channel) ResolveAPIBaseURLForModel(requestPath string, modelCandidates ...string) string {
	if channel == nil {
		return ""
	}
	if endpointBaseURL := CacheGetChannelModelEndpointBaseURL(strings.TrimSpace(channel.Id), requestPath, modelCandidates...); endpointBaseURL != "" {
		return endpointBaseURL
	}
	if cfg, err := channel.LoadConfig(); err == nil {
		if apiBaseURL := cfg.GetAPIBaseURL(); apiBaseURL != "" {
			return apiBaseURL
		}
	}
	if baseURL := normalizeConfiguredBaseURL(channel.GetBaseURL()); baseURL != "" {
		return baseURL
	}
	return normalizeConfiguredBaseURL(relaychannel.BaseURLByProtocol(channel.GetProtocol()))
}

func (channel *Channel) ResolveAccountBaseURL() string {
	if channel == nil {
		return ""
	}
	if cfg, err := channel.LoadConfig(); err == nil {
		if accountBaseURL := cfg.GetAccountBaseURL(); accountBaseURL != "" {
			return accountBaseURL
		}
	}
	if baseURL := normalizeConfiguredBaseURL(channel.GetBaseURL()); baseURL != "" {
		return baseURL
	}
	return normalizeConfiguredBaseURL(relaychannel.BaseURLByProtocol(channel.GetProtocol()))
}

func (channel *Channel) GetModelMapping() map[string]string {
	selected := channel.selectedChannelModels()
	if len(selected) == 0 {
		return nil
	}
	modelMapping := make(map[string]string, len(selected))
	for _, row := range selected {
		if row.Model == "" || row.UpstreamModel == "" || row.UpstreamModel == row.Model {
			continue
		}
		modelMapping[row.Model] = row.UpstreamModel
	}
	if len(modelMapping) == 0 {
		return nil
	}
	return modelMapping
}

func (channel *Channel) GetChannelModels() []ChannelModel {
	if channel == nil {
		return nil
	}
	if len(channel.ChannelModels) > 0 {
		rows := NormalizeChannelModelsPreserveOrder(channel.ChannelModels)
		for i := range rows {
			completeChannelModelRowDefaults(&rows[i], channel.GetChannelProtocol())
		}
		return rows
	}
	selected := ParseChannelModelCSV(channel.Models)
	if len(selected) == 0 {
		return []ChannelModel{}
	}
	return BuildDefaultChannelModelsWithProtocol(selected, channel.GetChannelProtocol())
}

func (channel *Channel) SelectedModelIDs() []string {
	if channel == nil {
		return nil
	}
	if len(channel.ChannelModels) > 0 {
		modelIDs := make([]string, 0, len(channel.ChannelModels))
		for _, row := range channel.GetChannelModels() {
			if row.Inactive || !row.Selected {
				continue
			}
			modelIDs = append(modelIDs, row.Model)
		}
		return NormalizeChannelModelIDsPreserveOrder(modelIDs)
	}
	return ParseChannelModelCSV(channel.Models)
}

func (channel *Channel) SetSelectedModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	channel.Models = JoinChannelModelCSV(normalized)
	if len(channel.ChannelModels) == 0 {
		return
	}

	selectedSet := buildChannelModelSelectionSet(normalized)
	existing := NormalizeChannelModelsPreserveOrder(channel.ChannelModels)
	next := make([]ChannelModel, 0, len(existing)+len(normalized))
	seen := make(map[string]struct{}, len(existing)+len(normalized))
	for _, row := range existing {
		if row.Model == "" {
			continue
		}
		row.Selected = false
		if !row.Inactive {
			if _, ok := selectedSet[row.Model]; ok {
				row.Selected = true
			}
		}
		completeChannelModelRowDefaults(&row, channel.GetChannelProtocol())
		next = append(next, row)
		seen[row.Model] = struct{}{}
	}
	for _, modelID := range normalized {
		if _, ok := seen[modelID]; ok {
			continue
		}
		rows := BuildDefaultChannelModelsWithProtocol([]string{modelID}, channel.GetChannelProtocol())
		if len(rows) == 0 {
			continue
		}
		next = append(next, rows[0])
	}
	channel.SetChannelModels(next)
}

func (channel *Channel) SetAvailableModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	channel.AvailableModels = NormalizeChannelModelIDsPreserveOrder(modelIDs)
}

func (channel *Channel) SetChannelModels(configs []ChannelModel) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelsPreserveOrder(configs)
	for i := range normalized {
		completeChannelModelRowDefaults(&normalized[i], channel.GetChannelProtocol())
	}
	channel.ChannelModels = normalized

	available := make([]string, 0, len(normalized))
	selected := make([]string, 0, len(normalized))
	for _, row := range normalized {
		if !row.Inactive {
			available = append(available, row.Model)
		}
		if row.Inactive || !row.Selected {
			continue
		}
		selected = append(selected, row.Model)
	}
	channel.SetAvailableModelIDs(available)
	channel.Models = JoinChannelModelCSV(selected)
}

func (channel *Channel) NormalizeChannelModelState() {
	if channel == nil {
		return
	}
	if channel.ChannelModelsProvided {
		channel.SetChannelModels(channel.ChannelModels)
		return
	}
	if len(channel.ChannelModels) > 0 {
		channel.SetChannelModels(channel.ChannelModels)
		return
	}
	if channel.ModelsProvided {
		channel.Models = JoinChannelModelCSV(ParseChannelModelCSV(channel.Models))
	}
}

func (channel *Channel) SetChannelTests(results []ChannelTest) {
	if channel == nil {
		return
	}
	channel.Tests = NormalizeChannelTestRows(results)
	channel.TestsLastTestedAt = CalcChannelTestsLastTestedAt(channel.Tests)
}

func (channel *Channel) Insert() error {
	return mustChannelRepo().Insert(channel)
}

func (channel *Channel) Update() error {
	return mustChannelRepo().Update(channel)
}

func (channel *Channel) UpdateResponseTime(responseTime int64) {
	mustChannelRepo().UpdateResponseTime(channel, responseTime)
}

func (channel *Channel) Delete() error {
	return mustChannelRepo().Delete(channel)
}

func (channel *Channel) LoadConfig() (ChannelConfig, error) {
	var cfg ChannelConfig
	if channel.Config == "" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(channel.Config), &cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func UpdateChannelStatusById(id string, status int) {
	mustChannelRepo().UpdateChannelStatusById(id, status)
}

func UpdateChannelUsedQuota(id string, quota int64) {
	mustChannelRepo().UpdateChannelUsedQuota(id, quota)
}

func updateChannelUsedQuota(id string, quota int64) {
	mustChannelRepo().UpdateChannelUsedQuotaDirect(id, quota)
}

func DeleteChannelByStatus(status int64) (int64, error) {
	return mustChannelRepo().DeleteChannelByStatus(status)
}

func DeleteDisabledChannel() (int64, error) {
	return mustChannelRepo().DeleteDisabledChannel()
}

func (channel *Channel) selectedChannelModels() []ChannelModel {
	configs := channel.GetChannelModels()
	if len(configs) == 0 {
		return nil
	}
	selected := make([]ChannelModel, 0, len(configs))
	for _, row := range configs {
		if !row.Selected {
			continue
		}
		selected = append(selected, row)
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func (channel *Channel) GetSelectedChannelModels() []ChannelModel {
	if channel == nil {
		return nil
	}
	return channel.selectedChannelModels()
}
