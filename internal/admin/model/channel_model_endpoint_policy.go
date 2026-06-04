package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChannelModelEndpointPoliciesTableName = "channel_model_endpoint_policies"

	ChannelEndpointPolicyActionImageURLToBase64 = "image_url_to_base64"
)

type ChannelModelEndpointPolicy struct {
	ID             string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	ChannelId      string `json:"channel_id" gorm:"type:varchar(64);uniqueIndex:uniq_channel_model_endpoint_policy,priority:1"`
	Model          string `json:"model" gorm:"type:varchar(255);uniqueIndex:uniq_channel_model_endpoint_policy,priority:2"`
	Endpoint       string `json:"endpoint" gorm:"type:varchar(255);uniqueIndex:uniq_channel_model_endpoint_policy,priority:3"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true;index"`
	TemplateKey    string `json:"template_key,omitempty" gorm:"type:varchar(128);default:''"`
	Capabilities   string `json:"capabilities,omitempty" gorm:"type:text;default:''"`
	RequestPolicy  string `json:"request_policy,omitempty" gorm:"type:text;default:''"`
	ResponsePolicy string `json:"response_policy,omitempty" gorm:"type:text;default:''"`
	Reason         string `json:"reason,omitempty" gorm:"type:text;default:''"`
	Source         string `json:"source,omitempty" gorm:"type:varchar(32);default:'manual'"`
	LastVerifiedAt int64  `json:"last_verified_at,omitempty" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func (ChannelModelEndpointPolicy) TableName() string {
	return ChannelModelEndpointPoliciesTableName
}

type ChannelModelEndpointCapabilities struct {
	InputText        bool `json:"input_text,omitempty"`
	InputImageURL    bool `json:"input_image_url,omitempty"`
	InputImageBase64 bool `json:"input_image_base64,omitempty"`
	InputPDFURL      bool `json:"input_pdf_url,omitempty"`
	InputPDFFile     bool `json:"input_pdf_file,omitempty"`
	Tools            bool `json:"tools,omitempty"`
	Stream           bool `json:"stream,omitempty"`
	NonStream        bool `json:"non_stream,omitempty"`
}

type ChannelModelEndpointRequestPolicy struct {
	Actions []ChannelModelEndpointPolicyAction `json:"actions,omitempty"`
}

type ChannelModelEndpointPolicyAction struct {
	Type       string                                 `json:"type"`
	InputTypes []string                               `json:"input_types,omitempty"`
	Limits     *ChannelModelEndpointPolicyActionLimit `json:"limits,omitempty"`
	Reason     string                                 `json:"reason,omitempty"`
}

type ChannelModelEndpointPolicyActionLimit struct {
	MaxBytes            int64    `json:"max_bytes,omitempty"`
	TimeoutMs           int      `json:"timeout_ms,omitempty"`
	AllowedContentTypes []string `json:"allowed_content_types,omitempty"`
}

func NormalizeChannelModelEndpointPolicyRow(row *ChannelModelEndpointPolicy) {
	if row == nil {
		return
	}
	row.ID = strings.TrimSpace(row.ID)
	row.ChannelId = strings.TrimSpace(row.ChannelId)
	row.Model = strings.TrimSpace(row.Model)
	row.Endpoint = NormalizeRequestedChannelModelEndpoint(row.Endpoint)
	row.TemplateKey = NormalizeChannelEndpointPolicyTemplateKey(row.TemplateKey)
	row.Capabilities = strings.TrimSpace(row.Capabilities)
	row.RequestPolicy = strings.TrimSpace(row.RequestPolicy)
	row.ResponsePolicy = strings.TrimSpace(row.ResponsePolicy)
	row.Reason = strings.TrimSpace(row.Reason)
	row.Source = strings.TrimSpace(row.Source)
}

func NormalizeChannelEndpointPolicyTemplateKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToUpper(trimmed) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteRune('_')
			lastUnderscore = true
		}
	}
	normalized := strings.Trim(builder.String(), "_")
	if normalized == "" {
		return ""
	}
	switch normalized {
	case "ANTHROPIC_IMAGE_URL_TO_BASE64":
		return "IMAGE_URL_TO_BASE64"
	}
	return normalized
}

func (row ChannelModelEndpointPolicy) ParseRequestPolicy() (ChannelModelEndpointRequestPolicy, error) {
	policy := ChannelModelEndpointRequestPolicy{}
	if strings.TrimSpace(row.RequestPolicy) == "" {
		return policy, nil
	}
	if err := json.Unmarshal([]byte(row.RequestPolicy), &policy); err != nil {
		return policy, fmt.Errorf("parse request policy: %w", err)
	}
	for i := range policy.Actions {
		policy.Actions[i].Type = strings.TrimSpace(policy.Actions[i].Type)
		policy.Actions[i].InputTypes = normalizeTrimmedValuesPreserveOrder(policy.Actions[i].InputTypes)
		policy.Actions[i].Reason = strings.TrimSpace(policy.Actions[i].Reason)
		if policy.Actions[i].Limits != nil {
			policy.Actions[i].Limits.AllowedContentTypes = normalizeTrimmedValuesPreserveOrder(policy.Actions[i].Limits.AllowedContentTypes)
		}
	}
	return policy, nil
}

func (row ChannelModelEndpointPolicy) ParseCapabilities() (ChannelModelEndpointCapabilities, error) {
	capabilities := ChannelModelEndpointCapabilities{}
	if strings.TrimSpace(row.Capabilities) == "" {
		return capabilities, nil
	}
	if err := json.Unmarshal([]byte(row.Capabilities), &capabilities); err != nil {
		return capabilities, fmt.Errorf("parse capabilities: %w", err)
	}
	return capabilities, nil
}

func ParseEndpointPolicyJSON(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var payload any
	return json.Unmarshal([]byte(trimmed), &payload)
}

func listChannelModelEndpointPoliciesByChannelIDWithDB(dbHandle *gorm.DB, channelID string) ([]ChannelModelEndpointPolicy, error) {
	if dbHandle == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModelEndpointPolicy{}, nil
	}
	rows := make([]ChannelModelEndpointPolicy, 0)
	if err := dbHandle.
		Where("channel_id = ?", normalizedChannelID).
		Order("model asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		NormalizeChannelModelEndpointPolicyRow(&rows[i])
	}
	return rows, nil
}

func listChannelModelEndpointPoliciesByCandidatesWithDB(dbHandle *gorm.DB, channelID string, endpoint string, modelCandidates []string) ([]ChannelModelEndpointPolicy, error) {
	if dbHandle == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	normalizedCandidates := normalizeTrimmedValuesPreserveOrder(modelCandidates)
	if normalizedChannelID == "" || normalizedEndpoint == "" || len(normalizedCandidates) == 0 {
		return []ChannelModelEndpointPolicy{}, nil
	}
	rows := make([]ChannelModelEndpointPolicy, 0)
	if err := dbHandle.
		Where("channel_id = ? AND endpoint = ? AND model IN ?", normalizedChannelID, normalizedEndpoint, normalizedCandidates).
		Order("model asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		NormalizeChannelModelEndpointPolicyRow(&rows[i])
	}
	return rows, nil
}

func ListChannelModelEndpointPoliciesByChannelIDWithDB(dbHandle *gorm.DB, channelID string, modelName string, endpoint string) ([]ChannelModelEndpointPolicy, error) {
	rows, err := listChannelModelEndpointPoliciesByChannelIDWithDB(dbHandle, channelID)
	if err != nil {
		return nil, err
	}
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedModelName == "" && normalizedEndpoint == "" {
		return rows, nil
	}
	result := make([]ChannelModelEndpointPolicy, 0, len(rows))
	for _, row := range rows {
		if normalizedModelName != "" && normalizedModelName != row.Model {
			continue
		}
		if normalizedEndpoint != "" && normalizedEndpoint != row.Endpoint {
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

func UpsertChannelModelEndpointPolicyWithDB(dbHandle *gorm.DB, row ChannelModelEndpointPolicy) (ChannelModelEndpointPolicy, error) {
	if dbHandle == nil {
		return ChannelModelEndpointPolicy{}, fmt.Errorf("database handle is nil")
	}
	normalized := row
	NormalizeChannelModelEndpointPolicyRow(&normalized)
	if normalized.ChannelId == "" {
		return ChannelModelEndpointPolicy{}, fmt.Errorf("channel_id 不能为空")
	}
	if normalized.Model == "" {
		return ChannelModelEndpointPolicy{}, fmt.Errorf("model 不能为空")
	}
	if normalized.Endpoint == "" {
		return ChannelModelEndpointPolicy{}, fmt.Errorf("endpoint 无效")
	}
	if _, err := normalized.ParseCapabilities(); err != nil {
		return ChannelModelEndpointPolicy{}, err
	}
	if _, err := normalized.ParseRequestPolicy(); err != nil {
		return ChannelModelEndpointPolicy{}, err
	}
	if err := ParseEndpointPolicyJSON(normalized.ResponsePolicy); err != nil {
		return ChannelModelEndpointPolicy{}, fmt.Errorf("parse response policy: %w", err)
	}
	if normalized.ID == "" {
		normalized.ID = strings.ReplaceAll(random.GetUUID(), "-", "")
	}
	if strings.TrimSpace(normalized.Source) == "" {
		normalized.Source = "manual"
	}
	normalized.UpdatedAt = helper.GetTimestamp()
	if err := dbHandle.Transaction(func(tx *gorm.DB) error {
		if err := lockChannelRowForUpdateWithDB(tx, normalized.ChannelId); err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "channel_id"},
				{Name: "model"},
				{Name: "endpoint"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"id",
				"enabled",
				"template_key",
				"capabilities",
				"request_policy",
				"response_policy",
				"reason",
				"source",
				"last_verified_at",
				"updated_at",
			}),
		}).Create(&normalized).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return ChannelModelEndpointPolicy{}, err
	}
	if config.MemoryCacheEnabled {
		InitChannelCache()
	}
	return normalized, nil
}
