package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/relay/relaymode"
	"gorm.io/gorm"
)

const (
	ChannelModelEndpointsTableName = "channel_model_endpoints"
)

type ChannelModelEndpoint struct {
	ChannelId      string `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model          string `json:"model" gorm:"primaryKey;type:varchar(255)"`
	Endpoint       string `json:"endpoint" gorm:"primaryKey;type:varchar(255)"`
	BaseURL        string `json:"base_url,omitempty" gorm:"column:base_url;type:text"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true;index"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
	DisabledReason string `json:"disabled_reason,omitempty" gorm:"type:text"`
	DisabledAt     int64  `json:"disabled_at,omitempty" gorm:"bigint;index"`
	DisabledBy     string `json:"disabled_by,omitempty" gorm:"type:varchar(64);default:'';index"`
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
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointRealtime):
		return ChannelModelEndpointRealtime
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointBatches):
		return ChannelModelEndpointBatches
	case strings.HasPrefix(normalizedPath, ChannelModelEndpointEmbeddings):
		return ChannelModelEndpointEmbeddings
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

func BuildChannelModelEndpointRowsWithProviderEndpoints(existing []ChannelModelEndpoint, rows []ChannelModel, providerEndpoints map[string][]string) []ChannelModelEndpoint {
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	if len(normalizedRows) == 0 {
		return []ChannelModelEndpoint{}
	}
	existingByKey := make(map[string]ChannelModelEndpoint, len(existing))
	for _, row := range existing {
		normalized := ChannelModelEndpoint{
			ChannelId:      strings.TrimSpace(row.ChannelId),
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
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
		eligibleForEnable := row.Selected && !row.Inactive
		if !eligibleForEnable {
			continue
		}
		for _, endpoint := range resolveProviderEndpointCandidatesForChannelModel(row, providerEndpoints) {
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
				BaseURL:   "",
				Enabled:   false,
			}
			if existingRow, ok := existingByKey[key]; ok {
				item.BaseURL = existingRow.BaseURL
				item.Enabled = existingRow.Enabled && eligibleForEnable
				item.UpdatedAt = existingRow.UpdatedAt
				item.DisabledReason = existingRow.DisabledReason
				item.DisabledAt = existingRow.DisabledAt
				item.DisabledBy = existingRow.DisabledBy
			}
			if item.Enabled {
				item.DisabledReason = ""
				item.DisabledAt = 0
				item.DisabledBy = ""
			}
			result = append(result, item)
		}
	}
	return result
}

func MergeChannelModelEndpointListRows(snapshotRows []ChannelModelEndpoint, explicitRows []ChannelModelEndpoint) []ChannelModelEndpoint {
	explicitByKey := make(map[string]ChannelModelEndpoint, len(explicitRows))
	for _, row := range explicitRows {
		normalized := ChannelModelEndpoint{
			ChannelId:      strings.TrimSpace(row.ChannelId),
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		explicitByKey[normalized.ChannelId+"::"+normalized.Model+"::"+normalized.Endpoint] = normalized
	}
	items := make([]ChannelModelEndpoint, 0, len(snapshotRows)+len(explicitRows))
	seen := make(map[string]struct{}, len(snapshotRows)+len(explicitRows))
	for _, row := range snapshotRows {
		normalized := ChannelModelEndpoint{
			ChannelId: strings.TrimSpace(row.ChannelId),
			Model:     strings.TrimSpace(row.Model),
			Endpoint:  NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:   normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:   row.Enabled,
			UpdatedAt: row.UpdatedAt,
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		key := normalized.ChannelId + "::" + normalized.Model + "::" + normalized.Endpoint
		if explicitRow, ok := explicitByKey[key]; ok {
			normalized.BaseURL = explicitRow.BaseURL
			normalized.Enabled = explicitRow.Enabled
			normalized.UpdatedAt = explicitRow.UpdatedAt
			normalized.DisabledReason = explicitRow.DisabledReason
			normalized.DisabledAt = explicitRow.DisabledAt
			normalized.DisabledBy = explicitRow.DisabledBy
		}
		if normalized.Enabled {
			normalized.DisabledReason = ""
			normalized.DisabledAt = 0
			normalized.DisabledBy = ""
		}
		seen[key] = struct{}{}
		items = append(items, normalized)
	}
	for _, row := range explicitRows {
		normalized := ChannelModelEndpoint{
			ChannelId:      strings.TrimSpace(row.ChannelId),
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		key := normalized.ChannelId + "::" + normalized.Model + "::" + normalized.Endpoint
		if _, ok := seen[key]; ok {
			continue
		}
		if normalized.Enabled {
			normalized.DisabledReason = ""
			normalized.DisabledAt = 0
			normalized.DisabledBy = ""
		}
		seen[key] = struct{}{}
		items = append(items, normalized)
	}
	return items
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
	providerEndpoints, err := loadProviderEndpointCandidatesForChannelModelsWithDB(db, rows)
	if err != nil {
		return err
	}
	nextRows := BuildChannelModelEndpointRowsWithProviderEndpoints(existingRows, rows, providerEndpoints)
	return replaceChannelModelEndpointRowsWithDB(db, normalizedChannelID, nextRows)
}

func ListChannelModelEndpointsByChannelIDWithDB(db *gorm.DB, channelID string, modelName string, endpoint string) ([]ChannelModelEndpoint, error) {
	rows, err := listChannelModelEndpointRowsByChannelIDWithDB(db, channelID)
	if err != nil {
		return nil, err
	}
	return filterChannelModelEndpointRows(rows, modelName, endpoint), nil
}

func ListEnabledChannelModelEndpointsByCandidatesWithDB(db *gorm.DB, channelID string, candidates ...string) ([]ChannelModelEndpoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	modelCandidates := NormalizeProviderLookupCandidates(candidates...)
	if normalizedChannelID == "" || len(modelCandidates) == 0 {
		return []ChannelModelEndpoint{}, nil
	}
	rows := make([]ChannelModelEndpoint, 0)
	if err := db.
		Where("channel_id = ? AND enabled = ? AND model IN ?", normalizedChannelID, true, modelCandidates).
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

func ListRecentDisabledChannelModelEndpointsWithDB(db *gorm.DB, limit int) ([]ChannelModelEndpoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows := make([]ChannelModelEndpoint, 0, limit)
	if err := db.
		Where("enabled = ? AND disabled_at > 0", false).
		Order("disabled_at desc, updated_at desc, channel_id asc, model asc, endpoint asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ChannelId = strings.TrimSpace(rows[i].ChannelId)
		rows[i].Model = strings.TrimSpace(rows[i].Model)
		rows[i].Endpoint = NormalizeRequestedChannelModelEndpoint(rows[i].Endpoint)
		rows[i].DisabledReason = strings.TrimSpace(rows[i].DisabledReason)
		rows[i].DisabledBy = strings.TrimSpace(rows[i].DisabledBy)
	}
	return rows, nil
}

func ListChannelModelEndpointCandidatesByChannelIDWithDB(db *gorm.DB, channelID string, modelName string, endpoint string) ([]ChannelModelEndpoint, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModelEndpoint{}, nil
	}
	explicitRows, err := listChannelModelEndpointRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return nil, err
	}
	configRows, err := listChannelModelRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return nil, err
	}
	providerEndpoints, err := loadProviderEndpointCandidatesForChannelModelsWithDB(db, configRows)
	if err != nil {
		return nil, err
	}
	candidateRows := BuildChannelModelEndpointRowsWithProviderEndpoints(explicitRows, configRows, providerEndpoints)
	return filterChannelModelEndpointRows(candidateRows, modelName, endpoint), nil
}

func ReplaceChannelModelEndpointsWithDB(db *gorm.DB, channelID string, rows []ChannelModelEndpoint) error {
	if err := replaceChannelModelEndpointRowsWithDB(db, channelID, rows); err != nil {
		return err
	}
	if config.MemoryCacheEnabled {
		InitChannelCache()
	}
	return nil
}

func SetChannelModelEndpointCapabilityWithDB(db *gorm.DB, channelID string, modelName string, endpoint string, enabled bool) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if normalizedChannelID == "" || normalizedModelName == "" || normalizedEndpoint == "" {
		return nil
	}
	rows, err := listChannelModelEndpointRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	nextRows := make([]ChannelModelEndpoint, 0, len(rows)+1)
	replaced := false
	for _, row := range rows {
		normalized := ChannelModelEndpoint{
			ChannelId:      strings.TrimSpace(row.ChannelId),
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		if normalized.ChannelId == normalizedChannelID &&
			normalized.Model == normalizedModelName &&
			normalized.Endpoint == normalizedEndpoint {
			normalized.Enabled = enabled
			replaced = true
		}
		nextRows = append(nextRows, normalized)
	}
	if !replaced {
		nextRows = append(nextRows, ChannelModelEndpoint{
			ChannelId: normalizedChannelID,
			Model:     normalizedModelName,
			Endpoint:  normalizedEndpoint,
			Enabled:   enabled,
		})
	}
	return replaceChannelModelEndpointRowsWithDB(db, normalizedChannelID, nextRows)
}

func buildProviderModelEndpointKey(provider string, modelName string) string {
	normalizedProvider := NormalizeGroupModelProviderValue(provider)
	normalizedModel := strings.TrimSpace(modelName)
	if normalizedProvider == "" || normalizedModel == "" {
		return ""
	}
	return normalizedProvider + "\x00" + normalizedModel
}

func resolveProviderEndpointCandidatesForChannelModel(row ChannelModel, providerEndpoints map[string][]string) []string {
	normalized := row
	normalizeChannelModelRow(&normalized)
	provider := NormalizeGroupModelProviderValue(normalized.Provider)
	if provider == "" || len(providerEndpoints) == 0 {
		return []string{}
	}
	for _, modelName := range NormalizeProviderLookupCandidates(normalized.Model, normalized.UpstreamModel) {
		key := buildProviderModelEndpointKey(provider, modelName)
		if endpoints := NormalizeProviderModelSupportedEndpointsForModel(normalized.Type, modelName, providerEndpoints[key]); len(endpoints) > 0 {
			return endpoints
		}
	}
	return []string{}
}

func loadProviderEndpointCandidatesForChannelModelsWithDB(db *gorm.DB, rows []ChannelModel) (map[string][]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	byProvider := make(map[string][]string)
	for _, row := range NormalizeChannelModelsPreserveOrder(rows) {
		provider := NormalizeGroupModelProviderValue(row.Provider)
		if provider == "" {
			continue
		}
		byProvider[provider] = append(byProvider[provider], row.Model, row.UpstreamModel)
	}
	result := make(map[string][]string)
	for provider, modelNames := range byProvider {
		endpointsByModel, err := LoadProviderModelEndpointMapByModelsWithDB(db, provider, modelNames)
		if err != nil {
			return nil, err
		}
		for modelName, endpoints := range endpointsByModel {
			key := buildProviderModelEndpointKey(provider, modelName)
			if key == "" {
				continue
			}
			result[key] = endpoints
		}
	}
	return result, nil
}

func DisableChannelModelRequestEndpointCapability(channelID string, modelName string, requestPath string) (bool, error) {
	return DisableChannelModelRequestEndpointCapabilityWithReason(channelID, modelName, requestPath, "", "")
}

func DisableChannelModelRequestEndpointCapabilityWithReason(channelID string, modelName string, requestPath string, reason string, disabledBy string) (bool, error) {
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
		nextRows, disabled := buildDisabledChannelModelEndpointRows(rows, normalizedChannelID, normalizedModelName, normalizedEndpoint, reason, disabledBy)
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

func HasChannelModelEndpoint(rows []ChannelModelEndpoint, modelName string, endpoint string) bool {
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedModelName == "" || normalizedEndpoint == "" {
		return false
	}
	for _, row := range rows {
		if normalizedModelName != strings.TrimSpace(row.Model) {
			continue
		}
		if normalizedEndpoint != NormalizeRequestedChannelModelEndpoint(row.Endpoint) {
			continue
		}
		return true
	}
	return false
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

func filterChannelModelEndpointRows(rows []ChannelModelEndpoint, modelName string, endpoint string) []ChannelModelEndpoint {
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedModelName == "" && normalizedEndpoint == "" {
		return rows
	}
	result := make([]ChannelModelEndpoint, 0, len(rows))
	for _, row := range rows {
		if normalizedModelName != "" && normalizedModelName != strings.TrimSpace(row.Model) {
			continue
		}
		if normalizedEndpoint != "" && normalizedEndpoint != NormalizeRequestedChannelModelEndpoint(row.Endpoint) {
			continue
		}
		result = append(result, row)
	}
	return result
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
			ChannelId:      normalizedChannelID,
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      now,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
		}
		if normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		if normalized.Enabled {
			normalized.DisabledReason = ""
			normalized.DisabledAt = 0
			normalized.DisabledBy = ""
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
		payloads := make([]map[string]any, 0, len(normalizedRows))
		for _, row := range normalizedRows {
			payloads = append(payloads, map[string]any{
				"channel_id":      row.ChannelId,
				"model":           row.Model,
				"endpoint":        row.Endpoint,
				"base_url":        row.BaseURL,
				"enabled":         row.Enabled,
				"updated_at":      row.UpdatedAt,
				"disabled_reason": row.DisabledReason,
				"disabled_at":     row.DisabledAt,
				"disabled_by":     row.DisabledBy,
			})
		}
		return tx.Table(ChannelModelEndpointsTableName).Create(&payloads).Error
	})
}

func buildDisabledChannelModelEndpointRows(rows []ChannelModelEndpoint, channelID string, modelName string, endpoint string, reason string, disabledBy string) ([]ChannelModelEndpoint, bool) {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedChannelID == "" || normalizedModelName == "" || normalizedEndpoint == "" {
		return rows, false
	}
	now := helper.GetTimestamp()
	normalizedReason := strings.TrimSpace(reason)
	normalizedDisabledBy := strings.TrimSpace(disabledBy)
	if normalizedDisabledBy == "" {
		normalizedDisabledBy = "runtime"
	}
	result := make([]ChannelModelEndpoint, 0, len(rows)+1)
	changed := false
	found := false
	for _, row := range rows {
		normalized := ChannelModelEndpoint{
			ChannelId:      strings.TrimSpace(row.ChannelId),
			Model:          strings.TrimSpace(row.Model),
			Endpoint:       NormalizeRequestedChannelModelEndpoint(row.Endpoint),
			BaseURL:        normalizeConfiguredBaseURL(row.BaseURL),
			Enabled:        row.Enabled,
			UpdatedAt:      row.UpdatedAt,
			DisabledReason: strings.TrimSpace(row.DisabledReason),
			DisabledAt:     row.DisabledAt,
			DisabledBy:     strings.TrimSpace(row.DisabledBy),
		}
		if normalized.ChannelId == normalizedChannelID && normalized.Model == normalizedModelName && normalized.Endpoint == normalizedEndpoint {
			found = true
			if normalized.Enabled ||
				strings.TrimSpace(normalized.DisabledReason) != normalizedReason ||
				strings.TrimSpace(normalized.DisabledBy) != normalizedDisabledBy ||
				normalized.DisabledAt == 0 {
				changed = true
			}
			normalized.Enabled = false
			normalized.DisabledReason = normalizedReason
			normalized.DisabledAt = now
			normalized.DisabledBy = normalizedDisabledBy
		}
		if normalized.ChannelId != "" && normalized.Model != "" && normalized.Endpoint != "" {
			result = append(result, normalized)
		}
	}
	if !found {
		result = append(result, ChannelModelEndpoint{
			ChannelId:      normalizedChannelID,
			Model:          normalizedModelName,
			Endpoint:       normalizedEndpoint,
			BaseURL:        "",
			Enabled:        false,
			DisabledReason: normalizedReason,
			DisabledAt:     now,
			DisabledBy:     normalizedDisabledBy,
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
	for _, row := range NormalizeChannelModelsPreserveOrder(rows) {
		if row.Inactive || !row.Selected {
			continue
		}
		if normalizedModelName == strings.TrimSpace(row.Model) || normalizedModelName == strings.TrimSpace(row.UpstreamModel) {
			return row, true
		}
	}
	return ChannelModel{}, false
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
	for _, endpoint := range []string{
		ChannelModelEndpointChat,
		ChannelModelEndpointResponses,
		ChannelModelEndpointMessages,
	} {
		if _, ok := endpointMap[endpoint]; ok {
			hasTextEndpoint = true
			break
		}
	}
	if normalizedEndpoint == ChannelModelEndpointChat ||
		normalizedEndpoint == ChannelModelEndpointResponses ||
		normalizedEndpoint == ChannelModelEndpointMessages {
		enabled, ok := endpointMap[normalizedEndpoint]
		if ok {
			return enabled, true
		}
		if hasTextEndpoint {
			return false, true
		}
		return false, false
	}
	enabled, ok := endpointMap[normalizedEndpoint]
	if !ok {
		return false, false
	}
	return enabled, true
}
