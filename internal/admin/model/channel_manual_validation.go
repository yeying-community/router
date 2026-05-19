package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type providerModelValidationRow struct {
	Provider           string `gorm:"column:provider"`
	Model              string `gorm:"column:model"`
	Type               string `gorm:"column:type"`
	Status             string `gorm:"column:status"`
	SupportedEndpoints string `gorm:"column:supported_endpoints"`
}

func ValidateManualChannelModelsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	if normalizedChannelID == "" || len(normalizedRows) == 0 {
		return nil
	}
	for _, row := range normalizedRows {
		if row.Inactive || !row.Selected {
			continue
		}
		reason, err := ExplainManualChannelModelEnableBlockWithDB(db, normalizedChannelID, row)
		if err != nil {
			return err
		}
		if strings.TrimSpace(reason) != "" {
			return fmt.Errorf("%s", reason)
		}
	}
	return nil
}

func ValidateManualChannelEndpointEnableWithDB(db *gorm.DB, channelID string, row ChannelModel, endpoint string) error {
	reason, err := ExplainManualChannelEndpointEnableBlockWithDB(db, channelID, row, endpoint)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reason) != "" {
		return fmt.Errorf("%s", reason)
	}
	return nil
}

func ExplainManualChannelEndpointEnableBlockWithDB(db *gorm.DB, channelID string, row ChannelModel, endpoint string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedChannelID == "" || normalizedEndpoint == "" {
		return "", nil
	}
	official, err := loadProviderModelValidationRowWithDB(db, row.Provider, row.Model, row.UpstreamModel)
	if err != nil {
		return "", err
	}
	if official == nil {
		return fmt.Sprintf("模型 %s 缺少供应商官方信息，不能启用端点 %s", displayChannelModelName(row), normalizedEndpoint), nil
	}
	if normalizeManualValidationProviderModelStatus(official.Status) != ProviderModelStatusActive {
		return fmt.Sprintf("模型 %s 当前官方状态不是 active，不能启用端点 %s", displayOfficialModelName(row, official.Model), normalizedEndpoint), nil
	}
	officialEndpoints := NormalizeProviderModelSupportedEndpoints(
		normalizeModelType(official.Type, official.Model),
		splitProviderModelSupportedEndpoints(official.SupportedEndpoints),
	)
	if len(officialEndpoints) == 0 {
		officialEndpoints = DefaultProviderModelSupportedEndpoints(
			official.Provider,
			normalizeModelType(official.Type, official.Model),
			official.Model,
		)
	}
	if !containsNormalizedEndpoint(officialEndpoints, normalizedEndpoint) {
		return fmt.Sprintf("模型 %s 的供应商官方端点范围不包含 %s", official.Model, normalizedEndpoint), nil
	}
	ok, err := HasSuccessfulChannelEndpointTestResultWithDB(db, normalizedChannelID, normalizedEndpoint, row.Model, row.UpstreamModel)
	if err != nil {
		return "", err
	}
	if !ok {
		return fmt.Sprintf("模型 %s 的端点 %s 缺少最近一次成功测试结果，不能启用", displayChannelModelName(row), normalizedEndpoint), nil
	}
	return "", nil
}

func ExplainManualChannelModelEnableBlockWithDB(db *gorm.DB, channelID string, row ChannelModel) (string, error) {
	official, err := loadProviderModelValidationRowWithDB(db, row.Provider, row.Model, row.UpstreamModel)
	if err != nil {
		return "", err
	}
	if official == nil {
		return fmt.Sprintf("模型 %s 缺少供应商官方信息，不能启用", displayChannelModelName(row)), nil
	}
	if normalizeManualValidationProviderModelStatus(official.Status) != ProviderModelStatusActive {
		return fmt.Sprintf("模型 %s 当前官方状态不是 active，不能启用", displayOfficialModelName(row, official.Model)), nil
	}
	ok, err := HasReturnedChannelModelSyncResultWithDB(db, channelID, row.Model, row.UpstreamModel)
	if err != nil {
		return "", err
	}
	if !ok {
		return fmt.Sprintf("模型 %s 最近一次上游返回未包含，不能启用", displayChannelModelName(row)), nil
	}
	return "", nil
}

func loadProviderModelValidationRowWithDB(db *gorm.DB, provider string, candidates ...string) (*providerModelValidationRow, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	modelCandidates := NormalizeProviderLookupCandidates(candidates...)
	if len(modelCandidates) == 0 {
		return nil, nil
	}
	query := db.Model(&ProviderModel{}).
		Select("provider", "model", "type", "status", "supported_endpoints").
		Where("is_deleted = ?", false)
	if normalizedProvider := NormalizeGroupModelProviderValue(provider); normalizedProvider != "" {
		query = query.Where("provider = ?", normalizedProvider)
	}
	rows := make([]providerModelValidationRow, 0)
	if err := query.Where("model IN ?", modelCandidates).Find(&rows).Error; err != nil {
		return nil, err
	}
	if NormalizeGroupModelProviderValue(provider) == "" {
		for _, candidate := range modelCandidates {
			matchCount := 0
			var matched providerModelValidationRow
			for _, row := range rows {
				if strings.TrimSpace(row.Model) != candidate {
					continue
				}
				matchCount++
				matched = row
			}
			if matchCount == 1 {
				item := matched
				return &item, nil
			}
			if matchCount > 1 {
				return nil, nil
			}
		}
		return nil, nil
	}
	for _, candidate := range modelCandidates {
		for _, row := range rows {
			if strings.TrimSpace(row.Model) == candidate {
				item := row
				return &item, nil
			}
		}
	}
	return nil, nil
}

func normalizeManualValidationProviderModelStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case ProviderModelStatusDeprecated:
		return ProviderModelStatusDeprecated
	default:
		return ProviderModelStatusActive
	}
}

func containsNormalizedEndpoint(values []string, endpoint string) bool {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return false
	}
	for _, item := range values {
		if NormalizeRequestedChannelModelEndpoint(item) == normalizedEndpoint {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func displayChannelModelName(row ChannelModel) string {
	return firstNonEmpty(row.UpstreamModel, row.Model)
}

func displayOfficialModelName(row ChannelModel, officialModel string) string {
	return firstNonEmpty(row.UpstreamModel, officialModel, row.Model)
}
