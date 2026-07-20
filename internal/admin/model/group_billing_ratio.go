package model

import (
	"strings"
	"sync"

	"gorm.io/gorm"
)

var (
	groupBillingRatioLock       sync.RWMutex
	groupChannelBillingRatioMap = map[string]map[string]float64{}
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

func syncGroupBillingRatiosRuntimeWithDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	channelRows := make([]GroupChannel, 0)
	if err := db.Find(&channelRows).Error; err != nil {
		return err
	}
	setGroupChannelBillingRatioRuntime(buildGroupChannelBillingRatioMap(channelRows))
	return nil
}
