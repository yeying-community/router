package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChannelModelEndpointsTableName = "channel_model_endpoints"
)

type ChannelModelEndpoint struct {
	ChannelId string `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model     string `json:"model" gorm:"primaryKey;type:varchar(255)"`
	Endpoint  string `json:"endpoint" gorm:"primaryKey;type:varchar(255)"`
	Enabled   bool   `json:"enabled" gorm:"not null;default:true;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (ChannelModelEndpoint) TableName() string {
	return ChannelModelEndpointsTableName
}

func NormalizeRequestedChannelModelEndpoint(path string) string {
	normalizedPath := relaymode.NormalizePath(strings.TrimSpace(path))
	switch {
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointChat):
		return ChannelModelEndpointChat
	case strings.HasPrefix(normalizedPath, "/v1/messages"):
		return ChannelModelEndpointMessages
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointResponses):
		return ChannelModelEndpointResponses
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointBatches):
		return ChannelModelEndpointBatches
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointImageEdit):
		return ChannelModelEndpointImageEdit
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointImages):
		return ChannelModelEndpointImages
	case strings.HasPrefix(normalizedPath, "/v1/audio/"):
		return ChannelModelEndpointAudio
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointVideos):
		return ChannelModelEndpointVideos
	default:
		return ""
	}
}

func ResolveChannelModelCapabilityEndpoints(row ChannelModel) []string {
	normalized := row
	normalizeChannelModelRow(&normalized)
	completeChannelModelRowDefaults(&normalized, 0)
	switch normalizeModelType(normalized.Type, normalized.Model) {
	case ProviderModelTypeImage:
		return NormalizeChannelModelDirectEndpoints(normalized.Type, normalized.Endpoints, normalized.Endpoint)
	case ProviderModelTypeAudio:
		return []string{ChannelModelEndpointAudio}
	case ProviderModelTypeVideo:
		return []string{ChannelModelEndpointVideos}
	default:
		directEndpoints := ResolveChannelModelDirectEndpoints(normalized)
		result := make([]string, 0, len(directEndpoints)*2)
		seen := make(map[string]struct{}, len(directEndpoints)*2)
		appendEndpoint := func(endpoint string) {
			normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
			if normalizedEndpoint == "" {
				return
			}
			if _, ok := seen[normalizedEndpoint]; ok {
				return
			}
			seen[normalizedEndpoint] = struct{}{}
			result = append(result, normalizedEndpoint)
		}
		for _, endpoint := range directEndpoints {
			switch NormalizeRequestedChannelModelEndpoint(endpoint) {
			case ChannelModelEndpointResponses:
				appendEndpoint(ChannelModelEndpointChat)
				appendEndpoint(ChannelModelEndpointResponses)
			case ChannelModelEndpointMessages:
				appendEndpoint(ChannelModelEndpointMessages)
				appendEndpoint(ChannelModelEndpointChat)
			default:
				appendEndpoint(endpoint)
			}
		}
		if len(result) == 0 {
			return []string{ChannelModelEndpointChat}
		}
		return result
	}
}

func ResolveChannelModelDirectEndpoints(row ChannelModel) []string {
	normalized := row
	normalizeChannelModelRow(&normalized)
	completeChannelModelRowDefaults(&normalized, 0)
	return NormalizeChannelModelDirectEndpoints(normalized.Type, normalized.Endpoints, normalized.Endpoint)
}

func BuildChannelModelEndpointRows(existing []ChannelModelEndpoint, rows []ChannelModel) []ChannelModelEndpoint {
	normalizedRows := NormalizeChannelModelConfigsPreserveOrder(rows)
	if len(normalizedRows) == 0 {
		return []ChannelModelEndpoint{}
	}
	existingByKey := make(map[string]ChannelModelEndpoint, len(existing))
	for _, row := range existing {
		normalized := ChannelModelEndpoint{
			ChannelId: strings.TrimSpace(row.ChannelId),
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			Enabled:   row.Enabled,
			UpdatedAt: row.UpdatedAt,
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		existingByKey[normalized.ChannelId+"::"+normalized.Model+"::"+normalized.Endpoint] = normalized
	}

	result := make([]ChannelModelEndpoint, 0, len(normalizedRows)*2)
	seen := make(map[string]struct{}, len(normalizedRows)*2)
	for _, row := range normalizedRows {
		channelID := strings.TrimSpace(row.ChannelId)
		modelID := strings.TrimSpace(row.Model)
		if channelID == "" || modelID == "" {
			continue
		}
		defaultEnabled := row.Selected && !row.Inactive
		for _, endpoint := range ResolveChannelModelDirectEndpoints(row) {
			normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
			if normalizedEndpoint == "" {
				continue
			}
			key := channelID + "::" + modelID + "::" + normalizedEndpoint
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			item := ChannelModelEndpoint{
				ChannelId: channelID,
				Model:     modelID,
				Endpoint:  normalizedEndpoint,
				Enabled:   defaultEnabled,
			}
			if existingRow, ok := existingByKey[key]; ok {
				item.Enabled = existingRow.Enabled && defaultEnabled
			}
			result = append(result, item)
		}
	}
	return result
}

func SyncChannelModelEndpointsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	existingRows, err := listChannelModelEndpointRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	nextRows := BuildChannelModelEndpointRows(existingRows, rows)
	return replaceChannelModelEndpointRowsWithDB(db, normalizedChannelID, nextRows)
}

func DisableChannelModelRequestEndpointCapability(channelID string, modelName string, requestPath string) (bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(requestPath)
	if normalizedChannelID == "" || normalizedModelName == "" || normalizedEndpoint == "" {
		return false, nil
	}

	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		rows, err := listChannelModelEndpointRowsByChannelIDWithDB(tx, normalizedChannelID)
		if err != nil {
			return err
		}
		nextRows, disabled := buildDisabledChannelModelEndpointRows(rows, normalizedChannelID, normalizedModelName, normalizedEndpoint)
		if !disabled {
			return nil
		}
		if err := replaceChannelModelEndpointRowsWithDB(tx, normalizedChannelID, nextRows); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil || !changed {
		return changed, err
	}
	if config.MemoryCacheEnabled {
		InitChannelCache()
	}
	return true, nil
}

func listChannelModelEndpointRowsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelModelEndpoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModelEndpoint{}, nil
	}
	rows := make([]ChannelModelEndpoint, 0)
	if err := db.
		Where("channel_id = ?", normalizedChannelID).
		Order("model asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ChannelId = strings.TrimSpace(rows[i].ChannelId)
		rows[i].Model = strings.TrimSpace(rows[i].Model)
		rows[i].Endpoint = NormalizeRequestedChannelModelEndpoint(rows[i].Endpoint)
	}
	return rows, nil
}

func listChannelModelEndpointSupportByChannelIDsWithDB(db *gorm.DB, channelIDs []string, modelName string) (map[string]map[string]bool, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	normalizedModelName := strings.TrimSpace(modelName)
	if len(normalizedChannelIDs) == 0 || normalizedModelName == "" {
		return map[string]map[string]bool{}, nil
	}
	rows := make([]ChannelModelEndpoint, 0)
	if err := db.
		Where("channel_id IN ? AND model = ?", normalizedChannelIDs, normalizedModelName).
		Order("channel_id asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]map[string]bool, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		endpoint := NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if channelID == "" || endpoint == "" {
			continue
		}
		if _, ok := result[channelID]; !ok {
			result[channelID] = make(map[string]bool)
		}
		result[channelID][endpoint] = row.Enabled
	}
	return result, nil
}

func replaceChannelModelEndpointRowsWithDB(db *gorm.DB, channelID string, rows []ChannelModelEndpoint) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	now := helper.GetTimestamp()
	normalizedRows := make([]ChannelModelEndpoint, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		normalized := ChannelModelEndpoint{
			ChannelId: normalizedChannelID,
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			Enabled:   row.Enabled,
			UpdatedAt: now,
		}
		if normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		key := normalized.Model + "::" + normalized.Endpoint
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalizedRows = append(normalizedRows, normalized)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockChannelRowForUpdateWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModelEndpoint{}).Error; err != nil {
			return err
		}
		if len(normalizedRows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "channel_id"},
				{Name: "model"},
				{Name: "endpoint"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
		}).Create(&normalizedRows).Error
	})
}

func buildDisabledChannelModelEndpointRows(rows []ChannelModelEndpoint, channelID string, modelName string, endpoint string) ([]ChannelModelEndpoint, bool) {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedChannelID == "" || normalizedModelName == "" || normalizedEndpoint == "" {
		return rows, false
	}
	result := make([]ChannelModelEndpoint, 0, len(rows)+1)
	changed := false
	found := false
	for _, row := range rows {
		normalized := ChannelModelEndpoint{
			ChannelId: strings.TrimSpace(row.ChannelId),
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			Enabled:   row.Enabled,
			UpdatedAt: row.UpdatedAt,
		}
		if normalized.ChannelId == normalizedChannelID && normalized.Model == normalizedModelName && normalized.Endpoint == normalizedEndpoint {
			found = true
			if normalized.Enabled {
				normalized.Enabled = false
				changed = true
			}
		}
		if normalized.ChannelId != "" && normalized.Model != "" && normalized.Endpoint != "" {
			result = append(result, normalized)
		}
	}
	if !found {
		result = append(result, ChannelModelEndpoint{
			ChannelId: normalizedChannelID,
			Model:     normalizedModelName,
			Endpoint:  normalizedEndpoint,
			Enabled:   false,
		})
		changed = true
	}
	return result, changed
}

func ResolveSelectedChannelModelConfig(rows []ChannelModel, modelName string) (ChannelModel, bool) {
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" {
		return ChannelModel{}, false
	}
	for _, row := range NormalizeChannelModelConfigsPreserveOrder(rows) {
		if row.Inactive || !row.Selected {
			continue
		}
		if normalizedModelName == strings.TrimSpace(row.Model) || normalizedModelName == strings.TrimSpace(row.UpstreamModel) {
			return row, true
		}
	}
	return ChannelModel{}, false
}

func IsChannelModelRequestEndpointSupportedByConfigs(rows []ChannelModel, modelName string, requestPath string) bool {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(requestPath)
	if normalizedEndpoint == "" {
		return true
	}
	row, ok := ResolveSelectedChannelModelConfig(rows, modelName)
	if !ok {
		return false
	}
	for _, endpoint := range ResolveChannelModelCapabilityEndpoints(row) {
		if NormalizeRequestedChannelModelEndpoint(endpoint) == normalizedEndpoint {
			return true
		}
	}
	return false
}

func IsChannelModelRequestEndpointSupportedByEndpointMap(endpointMap map[string]bool, requestPath string) (supported bool, explicit bool) {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(requestPath)
	if normalizedEndpoint == "" {
		return true, false
	}
	if len(endpointMap) == 0 {
		return false, false
	}
	hasTextEndpoint := false
	readEndpointEnabled := func(endpoint string) (bool, bool) {
		enabled, ok := endpointMap[endpoint]
		if ok {
			return enabled, true
		}
		return false, false
	}
	if _, ok := endpointMap[ChannelModelEndpointChat]; ok {
		hasTextEndpoint = true
	}
	if _, ok := endpointMap[ChannelModelEndpointResponses]; ok {
		hasTextEndpoint = true
	}
	if _, ok := endpointMap[ChannelModelEndpointMessages]; ok {
		hasTextEndpoint = true
	}
	if hasTextEndpoint {
		switch normalizedEndpoint {
		case ChannelModelEndpointChat, ChannelModelEndpointResponses, ChannelModelEndpointMessages:
			// Direct endpoint match first.
			if enabled, ok := readEndpointEnabled(normalizedEndpoint); ok {
				return enabled, true
			}
			// Compatibility bridge for text requests.
			if enabled, ok := readEndpointEnabled(ChannelModelEndpointChat); ok && enabled {
				return true, true
			}
			if enabled, ok := readEndpointEnabled(ChannelModelEndpointResponses); ok && enabled {
				return true, true
			}
			if enabled, ok := readEndpointEnabled(ChannelModelEndpointMessages); ok && enabled {
				return true, true
			}
			// Backward compatibility for historical rows where /v1/messages was normalized to chat.
			if normalizedEndpoint == ChannelModelEndpointMessages {
				if enabled, ok := readEndpointEnabled(ChannelModelEndpointChat); ok {
					return enabled, true
				}
			}
			return false, true
		}
	}
	if normalizedEndpoint == ChannelModelEndpointMessages {
		if enabled, ok := endpointMap[ChannelModelEndpointMessages]; ok {
			return enabled, true
		}
		// Backward compatibility for historical rows where /v1/messages was normalized to chat.
		if enabled, ok := endpointMap[ChannelModelEndpointChat]; ok {
			return enabled, true
		}
		return false, false
	}
	enabled, ok := endpointMap[normalizedEndpoint]
	if !ok {
		return false, false
	}
	return enabled, true
}
