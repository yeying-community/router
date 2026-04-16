package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

const (
	GroupModelProvidersTableName = "group_model_providers"
)

type GroupModelProvider struct {
	Group     string `json:"group" gorm:"column:group;primaryKey;type:varchar(32);autoIncrement:false"`
	Model     string `json:"model" gorm:"primaryKey;type:varchar(255);autoIncrement:false"`
	Provider  string `json:"provider" gorm:"type:varchar(128);not null;default:'';index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint;index"`
}

func (GroupModelProvider) TableName() string {
	return GroupModelProvidersTableName
}

type groupModelProviderSourceRow struct {
	Model    string `gorm:"column:model"`
	Provider string `gorm:"column:provider"`
}

func NormalizeGroupModelProviderValue(provider string) string {
	normalized := commonutils.NormalizeProvider(provider)
	// "custom" is a legacy placeholder and must not leak as canonical provider.
	if normalized == "custom" {
		return ""
	}
	return normalized
}

func ListGroupModelProviderMapByModels(groupID string, modelNames []string) (map[string]string, error) {
	return listGroupModelProviderMapByModelsWithDB(DB, groupID, modelNames)
}

func SyncGroupModelProvidersForGroups(groupIDs ...string) error {
	return SyncGroupModelProvidersForGroupsWithDB(DB, groupIDs...)
}

func SyncGroupModelProvidersForGroupsWithDB(db *gorm.DB, groupIDs ...string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedGroupIDs := normalizeTrimmedValuesPreserveOrder(groupIDs)
	if len(normalizedGroupIDs) == 0 {
		return nil
	}
	for _, groupID := range normalizedGroupIDs {
		rows := make([]groupModelProviderSourceRow, 0)
		if err := db.
			Table(AbilityTableName+" AS a").
			Select(`a.model AS model, LOWER(TRIM(cm.provider)) AS provider`).
			Joins("LEFT JOIN "+ChannelModelsTableName+" AS cm ON cm.channel_id = a.channel_id AND cm.model = a.model").
			Where(`a."group" = ?`, groupID).
			Where("a.channel_id <> ''").
			Order("a.model asc, a.channel_id asc").
			Scan(&rows).Error; err != nil {
			return err
		}
		providerByModel, err := buildGroupModelProviderMapFromSourceRows(rows)
		if err != nil {
			return fmt.Errorf("sync group_model_providers failed for group %s: %w", groupID, err)
		}
		if err := replaceGroupModelProvidersWithDB(db, groupID, providerByModel); err != nil {
			return err
		}
	}
	return nil
}

func listGroupModelProviderMapByModelsWithDB(db *gorm.DB, groupID string, modelNames []string) (map[string]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	normalizedModelNames := NormalizeChannelModelIDsPreserveOrder(modelNames)
	result := make(map[string]string, len(normalizedModelNames))
	if normalizedGroupID == "" || len(normalizedModelNames) == 0 {
		return result, nil
	}
	rows := make([]GroupModelProvider, 0, len(normalizedModelNames))
	groupCol := `"group"`
	if err := db.
		Select(groupCol, "model", "provider").
		Where(groupCol+" = ?", normalizedGroupID).
		Where("model IN ?", normalizedModelNames).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		result[modelName] = NormalizeGroupModelProviderValue(row.Provider)
	}
	return result, nil
}

func buildGroupModelProviderMapFromSourceRows(rows []groupModelProviderSourceRow) (map[string]string, error) {
	if len(rows) == 0 {
		return map[string]string{}, nil
	}
	modelOrder := make([]string, 0, len(rows))
	seenModel := make(map[string]struct{}, len(rows))
	candidatesByModel := make(map[string]map[string]struct{}, len(rows))
	for _, row := range rows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		if _, ok := seenModel[modelName]; !ok {
			seenModel[modelName] = struct{}{}
			modelOrder = append(modelOrder, modelName)
		}
		provider := commonutils.NormalizeProvider(row.Provider)
		if provider == "" {
			continue
		}
		if _, ok := candidatesByModel[modelName]; !ok {
			candidatesByModel[modelName] = make(map[string]struct{}, 1)
		}
		candidatesByModel[modelName][provider] = struct{}{}
	}
	providerByModel := make(map[string]string, len(modelOrder))
	for _, modelName := range modelOrder {
		candidateSet := candidatesByModel[modelName]
		if len(candidateSet) == 0 {
			providerByModel[modelName] = ""
			continue
		}
		providers := make([]string, 0, len(candidateSet))
		for provider := range candidateSet {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		if len(providers) > 1 {
			return nil, fmt.Errorf("同一分组模型仅允许一个供应商: %s (%s)", modelName, strings.Join(providers, " / "))
		}
		providerByModel[modelName] = providers[0]
	}
	return providerByModel, nil
}

func replaceGroupModelProvidersWithDB(db *gorm.DB, groupID string, providerByModel map[string]string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedGroupID := strings.TrimSpace(groupID)
	if normalizedGroupID == "" {
		return fmt.Errorf("分组 ID 不能为空")
	}
	groupCol := `"group"`
	if err := db.Where(groupCol+" = ?", normalizedGroupID).Delete(&GroupModelProvider{}).Error; err != nil {
		return err
	}
	if len(providerByModel) == 0 {
		return nil
	}
	modelNames := make([]string, 0, len(providerByModel))
	for modelName := range providerByModel {
		normalizedModelName := strings.TrimSpace(modelName)
		if normalizedModelName == "" {
			continue
		}
		modelNames = append(modelNames, normalizedModelName)
	}
	sort.Strings(modelNames)
	if len(modelNames) == 0 {
		return nil
	}
	now := helper.GetTimestamp()
	rows := make([]GroupModelProvider, 0, len(modelNames))
	for _, modelName := range modelNames {
		rows = append(rows, GroupModelProvider{
			Group:     normalizedGroupID,
			Model:     modelName,
			Provider:  NormalizeGroupModelProviderValue(providerByModel[modelName]),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return db.Create(&rows).Error
}
