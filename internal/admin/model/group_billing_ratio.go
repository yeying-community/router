package model

import (
	"strings"
	"sync"

	"gorm.io/gorm"
)

var (
	groupBillingRatioLock            sync.RWMutex
	groupChannelBillingRatioMap      = map[string]map[string]float64{}
	groupModelChannelBillingRatioMap = map[string]map[string]map[string]float64{}
)

func normalizeGroupBillingRatio(value float64) float64 {
	if value < 0 {
		return 1
	}
	return value
}

func setGroupChannelBillingRatioRuntime(ratios map[string]map[string]float64) {
	groupBillingRatioLock.Lock()
	groupChannelBillingRatioMap = ratios
	groupBillingRatioLock.Unlock()
}

func setGroupBillingRatiosRuntime(channelRatios map[string]map[string]float64, modelChannelRatios map[string]map[string]map[string]float64) {
	groupBillingRatioLock.Lock()
	groupChannelBillingRatioMap = channelRatios
	groupModelChannelBillingRatioMap = modelChannelRatios
	groupBillingRatioLock.Unlock()
}

func GetGroupChannelBillingRatio(group string, channelID string) float64 {
	groupID := strings.TrimSpace(group)
	normalizedChannelID := strings.TrimSpace(channelID)
	if groupID == "" || normalizedChannelID == "" {
		return 1
	}
	groupBillingRatioLock.RLock()
	ratio, ok := groupChannelBillingRatioMap[groupID][normalizedChannelID]
	groupBillingRatioLock.RUnlock()
	if !ok {
		return 1
	}
	return normalizeGroupBillingRatio(ratio)
}

func GetGroupModelChannelBillingRatio(group string, modelName string, channelID string) float64 {
	groupID := strings.TrimSpace(group)
	normalizedModel := strings.TrimSpace(modelName)
	normalizedChannelID := strings.TrimSpace(channelID)
	if groupID == "" || normalizedModel == "" || normalizedChannelID == "" {
		return 1
	}
	groupBillingRatioLock.RLock()
	ratio, ok := groupModelChannelBillingRatioMap[groupID][normalizedModel][normalizedChannelID]
	groupBillingRatioLock.RUnlock()
	if !ok {
		return 1
	}
	return normalizeGroupModelChannelBillingRatio(ratio)
}

func buildGroupChannelBillingRatioMap(rows []GroupChannel) map[string]map[string]float64 {
	ratios := make(map[string]map[string]float64)
	for _, row := range rows {
		groupID := strings.TrimSpace(row.Group)
		channelID := strings.TrimSpace(row.ChannelId)
		if groupID == "" || channelID == "" || !row.Enabled {
			continue
		}
		if _, ok := ratios[groupID]; !ok {
			ratios[groupID] = make(map[string]float64)
		}
		ratios[groupID][channelID] = normalizeGroupBillingRatio(row.BillingRatio)
	}
	return ratios
}

func buildGroupModelChannelBillingRatioMap(rows []GroupModelChannel) map[string]map[string]map[string]float64 {
	ratios := make(map[string]map[string]map[string]float64)
	for _, row := range rows {
		groupID := strings.TrimSpace(row.Group)
		modelName := strings.TrimSpace(row.Model)
		channelID := strings.TrimSpace(row.ChannelId)
		if groupID == "" || modelName == "" || channelID == "" {
			continue
		}
		if _, ok := ratios[groupID]; !ok {
			ratios[groupID] = make(map[string]map[string]float64)
		}
		if _, ok := ratios[groupID][modelName]; !ok {
			ratios[groupID][modelName] = make(map[string]float64)
		}
		ratios[groupID][modelName][channelID] = normalizeGroupModelChannelBillingRatio(row.BillingRatio)
	}
	return ratios
}

func syncGroupBillingRatiosRuntimeWithDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	channelRows := make([]GroupChannel, 0)
	if err := db.Find(&channelRows).Error; err != nil {
		return err
	}
	modelChannelRows := make([]GroupModelChannel, 0)
	if err := db.Find(&modelChannelRows).Error; err != nil {
		return err
	}
	setGroupBillingRatiosRuntime(
		buildGroupChannelBillingRatioMap(channelRows),
		buildGroupModelChannelBillingRatioMap(modelChannelRows),
	)
	return nil
}
