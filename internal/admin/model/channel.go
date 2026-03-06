package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/logger"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
	ChannelStatusCreating         = 4
)

type Channel struct {
	Id                         string                         `json:"id" gorm:"type:char(36);primaryKey"`
	Protocol                   string                         `json:"protocol" gorm:"type:varchar(64);default:'openai';index"`
	Key                        string                         `json:"key" gorm:"type:text"`
	Status                     int                            `json:"status" gorm:"default:1"`
	Name                       string                         `json:"name" gorm:"index"`
	Weight                     *uint                          `json:"weight" gorm:"default:0"`
	CreatedTime                int64                          `json:"created_time" gorm:"bigint"`
	TestTime                   int64                          `json:"test_time" gorm:"bigint"`
	ResponseTime               int                            `json:"response_time"`
	BaseURL                    *string                        `json:"base_url" gorm:"column:base_url;default:''"`
	Other                      *string                        `json:"other"`
	Balance                    float64                        `json:"balance"`
	BalanceUpdatedTime         int64                          `json:"balance_updated_time" gorm:"bigint"`
	Models                     string                         `json:"models" gorm:"-"`
	AvailableModels            []string                       `json:"available_models,omitempty" gorm:"-"`
	UsedQuota                  int64                          `json:"used_quota" gorm:"bigint;default:0"`
	ModelMapping               *string                        `json:"model_mapping" gorm:"type:varchar(1024);default:''"`
	Priority                   *int64                         `json:"priority" gorm:"bigint;default:0"`
	Config                     string                         `json:"config"`
	SystemPrompt               *string                        `json:"system_prompt" gorm:"type:text"`
	ModelRatio                 *string                        `json:"model_ratio" gorm:"type:text;default:''"`
	CompletionRatio            *string                        `json:"completion_ratio" gorm:"type:text;default:''"`
	TestModel                  string                         `json:"test_model" gorm:"type:varchar(255);default:''"`
	CapabilityProfiles         []ChannelCapabilityProfileRule `json:"capability_profiles,omitempty" gorm:"-"`
	CapabilityResults          []ChannelCapabilityResult      `json:"capability_results,omitempty" gorm:"-"`
	CapabilityLastTestedAt     int64                          `json:"capability_last_tested_at,omitempty" gorm:"-"`
	KeySet                     bool                           `json:"key_set" gorm:"-"`
	ModelsProvided             bool                           `json:"-" gorm:"-"`
	CapabilityProfilesProvided bool                           `json:"-" gorm:"-"`
	CapabilityResultsStale     bool                           `json:"-" gorm:"-"`
}

type ChannelConfig struct {
	Region            string `json:"region,omitempty"`
	SK                string `json:"sk,omitempty"`
	AK                string `json:"ak,omitempty"`
	UserID            string `json:"user_id,omitempty"`
	APIVersion        string `json:"api_version,omitempty"`
	LibraryID         string `json:"library_id,omitempty"`
	Plugin            string `json:"plugin,omitempty"`
	VertexAIProjectID string `json:"vertex_ai_project_id,omitempty"`
	VertexAIADC       string `json:"vertex_ai_adc,omitempty"`
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

func GetAllChannels(startIdx int, num int, scope string) ([]*Channel, error) {
	return mustChannelRepo().GetAllChannels(startIdx, num, scope)
}

func SearchChannels(keyword string) ([]*Channel, error) {
	return mustChannelRepo().SearchChannels(keyword)
}

func GetChannelById(id string, selectAll bool) (*Channel, error) {
	return mustChannelRepo().GetChannelById(id, selectAll)
}

func BatchInsertChannels(channels []Channel) error {
	return mustChannelRepo().BatchInsertChannels(channels)
}

func (channel *Channel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

func (channel *Channel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	return strings.TrimSpace(*channel.BaseURL)
}

func (channel *Channel) GetModelMapping() map[string]string {
	if channel.ModelMapping == nil || *channel.ModelMapping == "" || *channel.ModelMapping == "{}" {
		return nil
	}
	modelMapping := make(map[string]string)
	err := json.Unmarshal([]byte(*channel.ModelMapping), &modelMapping)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to unmarshal model mapping for channel %s, error: %s", channel.Id, err.Error()))
		return nil
	}
	return modelMapping
}

func (channel *Channel) SelectedModelIDs() []string {
	if channel == nil {
		return nil
	}
	return ParseChannelModelCSV(channel.Models)
}

func (channel *Channel) SetSelectedModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	channel.Models = JoinChannelModelCSV(modelIDs)
}

func (channel *Channel) SetAvailableModelIDs(modelIDs []string) {
	if channel == nil {
		return
	}
	channel.AvailableModels = NormalizeChannelModelIDsPreserveOrder(modelIDs)
}

func (channel *Channel) SetCapabilityProfiles(rules []ChannelCapabilityProfileRule) {
	if channel == nil {
		return
	}
	channel.CapabilityProfiles = NormalizeChannelCapabilityProfileRules(rules)
}

func (channel *Channel) SetCapabilityResults(results []ChannelCapabilityResult) {
	if channel == nil {
		return
	}
	channel.CapabilityResults = NormalizeChannelCapabilityResultRows(results)
	channel.CapabilityLastTestedAt = calcChannelCapabilityLastTestedAt(channel.CapabilityResults)
}

func (channel *Channel) CapabilityProfilesByCapability(capability string) []ChannelCapabilityProfileRule {
	if channel == nil {
		return nil
	}
	normalizedCapability := NormalizeChannelCapabilityName(capability)
	if normalizedCapability == "" {
		return nil
	}
	result := make([]ChannelCapabilityProfileRule, 0)
	for _, rule := range channel.CapabilityProfiles {
		if NormalizeChannelCapabilityName(rule.Capability) != normalizedCapability {
			continue
		}
		result = append(result, rule)
	}
	return NormalizeChannelCapabilityProfileRules(result)
}

func (channel *Channel) ResolveCapabilityUpstreamUserAgent(capability string, clientProfile string, profiles []ClientProfile, inboundUserAgent string) string {
	if channel == nil {
		return strings.TrimSpace(inboundUserAgent)
	}
	normalizedCapability := NormalizeChannelCapabilityName(capability)
	normalizedClientProfile := NormalizeClientProfileName(clientProfile)
	for _, rule := range channel.CapabilityProfilesByCapability(normalizedCapability) {
		if NormalizeClientProfileName(rule.ClientProfile) != normalizedClientProfile {
			continue
		}
		if strings.TrimSpace(rule.UpstreamUserAgent) != "" {
			return strings.TrimSpace(rule.UpstreamUserAgent)
		}
		if defaultUA := ResolveClientProfileDefaultUserAgent(normalizedClientProfile, profiles); defaultUA != "" {
			return defaultUA
		}
		return strings.TrimSpace(inboundUserAgent)
	}
	for _, rule := range channel.CapabilityProfilesByCapability(normalizedCapability) {
		if NormalizeClientProfileName(rule.ClientProfile) != ClientProfileAny {
			continue
		}
		if strings.TrimSpace(rule.UpstreamUserAgent) != "" {
			return strings.TrimSpace(rule.UpstreamUserAgent)
		}
		if defaultUA := ResolveClientProfileDefaultUserAgent(ClientProfileAny, profiles); defaultUA != "" {
			return defaultUA
		}
		return strings.TrimSpace(inboundUserAgent)
	}
	return strings.TrimSpace(inboundUserAgent)
}

func (channel *Channel) SupportsCapabilityClientProfile(capability string, clientProfile string) bool {
	if channel == nil {
		return false
	}
	normalizedCapability := NormalizeChannelCapabilityName(capability)
	if normalizedCapability == "" {
		return true
	}
	rules := channel.CapabilityProfilesByCapability(normalizedCapability)
	if len(rules) == 0 {
		return false
	}
	normalizedClientProfile := NormalizeClientProfileName(clientProfile)
	for _, rule := range rules {
		ruleProfile := NormalizeClientProfileName(rule.ClientProfile)
		if ruleProfile == ClientProfileAny || ruleProfile == normalizedClientProfile {
			return true
		}
	}
	return false
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

func (channel *Channel) UpdateBalance(balance float64) {
	mustChannelRepo().UpdateBalance(channel, balance)
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

func UpdateChannelTestModel(id string, testModel string) error {
	return mustChannelRepo().UpdateChannelTestModelByID(id, testModel)
}
