package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type BillingRatioBreakdown struct {
	GroupChannelRatio float64 `json:"group_channel_ratio"`
	ModelChannelRatio float64 `json:"model_channel_ratio"`
	EffectiveRatio    float64 `json:"effective_ratio"`
}

func groupModelChannelBillingRatioKey(groupID string, modelName string, channelID string) string {
	return strings.TrimSpace(groupID) + "::" + strings.TrimSpace(modelName) + "::" + strings.TrimSpace(channelID)
}

func defaultGroupModelChannelBillingRatio() float64 {
	return 1
}

func normalizeGroupModelChannelBillingRatio(value float64) float64 {
	return normalizeGroupBillingRatio(value)
}

func resolveGroupModelChannelBillingRatio(value *float64) float64 {
	if value == nil {
		return defaultGroupModelChannelBillingRatio()
	}
	return normalizeGroupModelChannelBillingRatio(*value)
}

func BuildGroupModelChannelBillingRatioOverrides(groupID string, items []GroupModelBindingItem) map[string]float64 {
	overrides := make(map[string]float64)
	for _, item := range items {
		if item.BillingRatio == nil {
			continue
		}
		modelName := strings.TrimSpace(item.Model)
		channelID := strings.TrimSpace(item.ChannelId)
		if strings.TrimSpace(groupID) == "" || modelName == "" || channelID == "" {
			continue
		}
		overrides[groupModelChannelBillingRatioKey(groupID, modelName, channelID)] = resolveGroupModelChannelBillingRatio(item.BillingRatio)
	}
	return overrides
}

func LoadGroupModelChannelBillingRatiosWithDB(db *gorm.DB, groupIDs []string, modelNames []string, channelIDs []string) (map[string]float64, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedGroups := normalizeTrimmedValuesPreserveOrder(groupIDs)
	normalizedModels := normalizeTrimmedValuesPreserveOrder(modelNames)
	normalizedChannels := normalizeChannelIDList(channelIDs)
	if len(normalizedGroups) == 0 {
		return map[string]float64{}, nil
	}
	rows := make([]GroupModelChannel, 0)
	groupCol := `"group"`
	query := db.
		Select(groupCol, "model", "channel_id", "billing_ratio").
		Where(groupCol+" IN ?", normalizedGroups)
	if len(normalizedModels) > 0 {
		query = query.Where("model IN ?", normalizedModels)
	}
	if len(normalizedChannels) > 0 {
		query = query.Where("channel_id IN ?", normalizedChannels)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	ratios := make(map[string]float64, len(rows))
	for _, row := range rows {
		groupID := strings.TrimSpace(row.Group)
		modelName := strings.TrimSpace(row.Model)
		channelID := strings.TrimSpace(row.ChannelId)
		if groupID == "" || modelName == "" || channelID == "" {
			continue
		}
		ratios[groupModelChannelBillingRatioKey(groupID, modelName, channelID)] = normalizeGroupModelChannelBillingRatio(row.BillingRatio)
	}
	return ratios, nil
}

func ApplyGroupModelChannelBillingRatios(rows []GroupModelChannel, overrides map[string]float64, existing map[string]float64) []GroupModelChannel {
	if len(rows) == 0 {
		return rows
	}
	for index := range rows {
		key := groupModelChannelBillingRatioKey(rows[index].Group, rows[index].Model, rows[index].ChannelId)
		if value, ok := overrides[key]; ok {
			rows[index].BillingRatio = normalizeGroupModelChannelBillingRatio(value)
			continue
		}
		if value, ok := existing[key]; ok {
			rows[index].BillingRatio = normalizeGroupModelChannelBillingRatio(value)
			continue
		}
		rows[index].BillingRatio = defaultGroupModelChannelBillingRatio()
	}
	return rows
}

func GetRouteBillingRatio(group string, modelName string, channelID string) BillingRatioBreakdown {
	groupChannelRatio := GetGroupChannelBillingRatio(group, channelID)
	modelChannelRatio := GetGroupModelChannelBillingRatio(group, modelName, channelID)
	return BillingRatioBreakdown{
		GroupChannelRatio: groupChannelRatio,
		ModelChannelRatio: modelChannelRatio,
		EffectiveRatio:    groupChannelRatio * modelChannelRatio,
	}
}
