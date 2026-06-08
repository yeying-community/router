package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

const (
	ChannelModelsTableName = "channel_models"
)

type ChannelModel struct {
	ChannelId       string                              `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model           string                              `json:"model" gorm:"primaryKey;type:varchar(255)"`
	UpstreamModel   string                              `json:"upstream_model" gorm:"type:varchar(255);default:'';index"`
	Provider        string                              `json:"provider,omitempty" gorm:"type:varchar(128);default:'';index"`
	Type            string                              `json:"type" gorm:"type:varchar(32);default:'text'"`
	Endpoint        string                              `json:"endpoint" gorm:"type:varchar(255);default:''"`
	Endpoints       []string                            `json:"endpoints,omitempty" gorm:"-"`
	Inactive        bool                                `json:"inactive,omitempty" gorm:"not null;default:false;index"`
	Selected        bool                                `json:"selected" gorm:"default:false;index"`
	InputPrice      *float64                            `json:"input_price,omitempty" gorm:"type:double precision"`
	OutputPrice     *float64                            `json:"output_price,omitempty" gorm:"type:double precision"`
	PriceUnit       string                              `json:"price_unit,omitempty" gorm:"type:varchar(64);default:''"`
	Currency        string                              `json:"currency,omitempty" gorm:"type:varchar(16);default:''"`
	PriceComponents []ProviderModelPriceComponentDetail `json:"price_components,omitempty" gorm:"-"`
	SortOrder       int                                 `json:"sort_order" gorm:"default:0"`
	UpdatedAt       int64                               `json:"updated_at" gorm:"bigint"`
	DisabledReason  string                              `json:"disabled_reason,omitempty" gorm:"type:text"`
	DisabledAt      int64                               `json:"disabled_at,omitempty" gorm:"bigint;index"`
	DisabledBy      string                              `json:"disabled_by,omitempty" gorm:"type:varchar(64);default:'';index"`
}

func (ChannelModel) TableName() string {
	return ChannelModelsTableName
}

func NormalizeChannelModelIDsPreserveOrder(modelIDs []string) []string {
	return normalizeTrimmedValuesPreserveOrder(modelIDs)
}

func NormalizeChannelModelsPreserveOrder(rows []ChannelModel) []ChannelModel {
	if len(rows) == 0 {
		return []ChannelModel{}
	}
	seen := make(map[string]struct{}, len(rows))
	result := make([]ChannelModel, 0, len(rows))
	for _, row := range rows {
		normalized := row
		normalizeChannelModelRow(&normalized)
		if normalized.Model == "" {
			continue
		}
		if _, ok := seen[normalized.Model]; ok {
			continue
		}
		seen[normalized.Model] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func BuildDefaultChannelModels(modelIDs []string) []ChannelModel {
	return BuildDefaultChannelModelsWithProtocol(modelIDs, 0)
}

func BuildDefaultChannelModelsWithProtocol(modelIDs []string, channelProtocol int) []ChannelModel {
	normalized := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	rows := make([]ChannelModel, 0, len(normalized))
	for idx, modelID := range normalized {
		row := ChannelModel{
			Model:         modelID,
			UpstreamModel: modelID,
			Selected:      true,
			SortOrder:     idx + 1,
		}
		completeChannelModelRowDefaults(&row, channelProtocol)
		rows = append(rows, row)
	}
	return rows
}

type ChannelModelUsageReference struct {
	Group string `json:"group"`
	Model string `json:"model"`
}

func ListChannelModelUsageReferencesWithDB(db *gorm.DB, channelID string, modelName string, upstreamModel string) ([]ChannelModelUsageReference, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	modelCandidates := NormalizeProviderLookupCandidates(modelName, upstreamModel)
	if normalizedChannelID == "" || len(modelCandidates) == 0 {
		return []ChannelModelUsageReference{}, nil
	}
	groupCol := `"group"`
	rows := make([]GroupModelChannel, 0)
	if err := db.
		Where("channel_id = ? AND (model IN ? OR upstream_model IN ?)", normalizedChannelID, modelCandidates, modelCandidates).
		Order(groupCol + " asc, model asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ChannelModelUsageReference, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		groupID := strings.TrimSpace(row.Group)
		groupModel := strings.TrimSpace(row.Model)
		if groupID == "" || groupModel == "" {
			continue
		}
		key := groupID + "::" + groupModel
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, ChannelModelUsageReference{
			Group: groupID,
			Model: groupModel,
		})
	}
	return result, nil
}

func ListRecentDisabledChannelModelsWithDB(db *gorm.DB, limit int) ([]ChannelModel, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows := make([]ChannelModel, 0, limit)
	if err := db.
		Where("inactive = ? AND disabled_at > 0", true).
		Order("disabled_at desc, updated_at desc, channel_id asc, sort_order asc, model asc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return NormalizeChannelModelsPreserveOrder(rows), nil
}

func FormatChannelModelUsageReferences(usages []ChannelModelUsageReference) string {
	if len(usages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(usages))
	for _, usage := range usages {
		groupID := strings.TrimSpace(usage.Group)
		modelName := strings.TrimSpace(usage.Model)
		if groupID == "" || modelName == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s/%s", groupID, modelName))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func formatChannelModelEnabledEndpoints(rows []ChannelModelEndpoint) string {
	if len(rows) == 0 {
		return ""
	}
	endpoints := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		endpoint := NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if endpoint == "" {
			continue
		}
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}
		endpoints = append(endpoints, endpoint)
	}
	sort.Strings(endpoints)
	return strings.Join(endpoints, ", ")
}

func ValidateChannelModelDisableTransitionsWithDB(db *gorm.DB, channelID string, existingRows []ChannelModel, nextRows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	existingByModel := make(map[string]ChannelModel)
	for _, row := range NormalizeChannelModelsPreserveOrder(existingRows) {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		existingByModel[modelName] = row
	}
	nextByModel := make(map[string]ChannelModel)
	for _, row := range NormalizeChannelModelsPreserveOrder(nextRows) {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		nextByModel[modelName] = row
	}
	for modelName, existingRow := range existingByModel {
		if existingRow.Inactive || !existingRow.Selected {
			continue
		}
		nextRow, ok := nextByModel[modelName]
		if !ok {
			continue
		}
		if !nextRow.Inactive && nextRow.Selected {
			continue
		}
		enabledEndpointRows, err := ListEnabledChannelModelEndpointsByCandidatesWithDB(db, normalizedChannelID, existingRow.Model, existingRow.UpstreamModel)
		if err != nil {
			return err
		}
		if len(enabledEndpointRows) == 0 {
			continue
		}
		return fmt.Errorf("模型 %s 仍有已启用端点，无法关闭：%s", displayChannelModelName(existingRow), formatChannelModelEnabledEndpoints(enabledEndpointRows))
	}
	return nil
}

func DeleteChannelModelWithDB(db *gorm.DB, channelID string, modelName string, upstreamModel string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	modelCandidates := NormalizeProviderLookupCandidates(modelName, upstreamModel)
	if normalizedChannelID == "" || len(modelCandidates) == 0 {
		return fmt.Errorf("渠道模型无效")
	}
	targetRow := ChannelModel{}
	if err := db.
		Where("channel_id = ? AND (model IN ? OR upstream_model IN ?)", normalizedChannelID, modelCandidates, modelCandidates).
		Order("sort_order asc, model asc").
		First(&targetRow).Error; err != nil {
		return err
	}
	returned, err := HasReturnedChannelModelSyncResultWithDB(db, normalizedChannelID, targetRow.Model, targetRow.UpstreamModel)
	if err != nil {
		return err
	}
	if returned {
		return fmt.Errorf("模型 %s 最近一次上游返回仍包含，无法删除", displayChannelModelName(targetRow))
	}
	enabledEndpointRows, err := ListEnabledChannelModelEndpointsByCandidatesWithDB(db, normalizedChannelID, targetRow.Model, targetRow.UpstreamModel)
	if err != nil {
		return err
	}
	if len(enabledEndpointRows) > 0 {
		return fmt.Errorf("模型 %s 仍有已启用端点，无法删除：%s", displayChannelModelName(targetRow), formatChannelModelEnabledEndpoints(enabledEndpointRows))
	}
	usages, err := ListChannelModelUsageReferencesWithDB(db, normalizedChannelID, targetRow.Model, targetRow.UpstreamModel)
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		return fmt.Errorf("该模型仍被分组使用，无法删除：%s", FormatChannelModelUsageReferences(usages))
	}
	deleteCandidates := NormalizeProviderLookupCandidates(targetRow.Model, targetRow.UpstreamModel)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockChannelRowForUpdateWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		if err := tx.Where("channel_id = ? AND model = ?", normalizedChannelID, strings.TrimSpace(targetRow.Model)).Delete(&ChannelModelEndpointPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ? AND model = ?", normalizedChannelID, strings.TrimSpace(targetRow.Model)).Delete(&ChannelModelEndpoint{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ? AND (model IN ? OR upstream_model IN ?)", normalizedChannelID, deleteCandidates, deleteCandidates).Delete(&ChannelTest{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ? AND (model IN ? OR upstream_model IN ?)", normalizedChannelID, deleteCandidates, deleteCandidates).Delete(&ChannelModelSyncResult{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ? AND model = ?", normalizedChannelID, strings.TrimSpace(targetRow.Model)).Delete(&ChannelModelPriceComponent{}).Error; err != nil {
			return err
		}
		return tx.Where("channel_id = ? AND model = ?", normalizedChannelID, strings.TrimSpace(targetRow.Model)).Delete(&ChannelModel{}).Error
	})
}

func ParseChannelModelCSV(models string) []string {
	if strings.TrimSpace(models) == "" {
		return []string{}
	}
	return NormalizeChannelModelIDsPreserveOrder(strings.FieldsFunc(models, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	}))
}

func JoinChannelModelCSV(modelIDs []string) string {
	return strings.Join(NormalizeChannelModelIDsPreserveOrder(modelIDs), ",")
}

func HydrateChannelWithModels(db *gorm.DB, channel *Channel) error {
	if channel == nil {
		return nil
	}
	return HydrateChannelsWithModels(db, []*Channel{channel})
}

func HydrateChannelsWithModels(db *gorm.DB, channels []*Channel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	channelIDs := make([]string, 0, len(channels))
	normalizedChannels := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channel.Id = strings.TrimSpace(channel.Id)
		if channel.Id == "" {
			channel.SetSelectedModelIDs(nil)
			channel.SetAvailableModelIDs(nil)
			channel.SetChannelModels(nil)
			continue
		}
		channelIDs = append(channelIDs, channel.Id)
		normalizedChannels = append(normalizedChannels, channel)
	}
	if len(normalizedChannels) == 0 {
		return nil
	}

	rowsByChannelID, err := loadChannelModelRowsByChannelIDs(db, channelIDs)
	if err != nil {
		return err
	}
	if err := attachChannelModelPriceComponentsWithDB(db, rowsByChannelID); err != nil {
		return err
	}
	for _, channel := range normalizedChannels {
		applyChannelModelRows(channel, rowsByChannelID[channel.Id])
	}
	return nil
}

func ListSelectedChannelModelIDsByChannelIDWithDB(db *gorm.DB, channelID string) ([]string, error) {
	rows, err := listChannelModelRowsByChannelIDWithDB(db, channelID)
	if err != nil {
		return nil, err
	}
	modelIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Inactive || !row.Selected {
			continue
		}
		modelIDs = append(modelIDs, row.Model)
	}
	return NormalizeChannelModelIDsPreserveOrder(modelIDs), nil
}

func ListAvailableChannelModelIDsByChannelIDWithDB(db *gorm.DB, channelID string) ([]string, error) {
	rows, err := listChannelModelRowsByChannelIDWithDB(db, channelID)
	if err != nil {
		return nil, err
	}
	modelIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Inactive {
			continue
		}
		modelIDs = append(modelIDs, row.Model)
	}
	return NormalizeChannelModelIDsPreserveOrder(modelIDs), nil
}

func SyncFetchedChannelModelIDsWithDB(db *gorm.DB, channelID string, modelIDs []string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedModelIDs := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	existingRows, err := listChannelModelRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	rows := BuildFetchedChannelModels(existingRows, BuildDefaultChannelModelsWithProtocol(normalizedModelIDs, channelProtocol), channelProtocol, true)
	return ReplaceChannelModelsWithDB(db, normalizedChannelID, rows)
}

func SyncFetchedChannelModelsWithDB(db *gorm.DB, channelID string, fetchedRows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	existingRows, err := listChannelModelRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	rows := BuildFetchedChannelModels(existingRows, fetchedRows, channelProtocol, false)
	return ReplaceChannelModelsWithDB(db, normalizedChannelID, rows)
}

func AppendMissingFetchedChannelModelsWithDB(db *gorm.DB, channelID string, fetchedRows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	existingRows, err := listChannelModelRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	normalizedExisting := NormalizeChannelModelsPreserveOrder(existingRows)
	normalizedFetched := NormalizeChannelModelsPreserveOrder(fetchedRows)
	existingKeys := make(map[string]struct{}, len(normalizedExisting)*2)
	for _, row := range normalizedExisting {
		modelName := strings.TrimSpace(row.Model)
		upstreamModel := strings.TrimSpace(row.UpstreamModel)
		if modelName != "" {
			existingKeys["model:"+modelName] = struct{}{}
		}
		if upstreamModel != "" {
			existingKeys["upstream:"+upstreamModel] = struct{}{}
		}
	}
	nextRows := make([]ChannelModel, 0, len(normalizedExisting)+len(normalizedFetched))
	nextRows = append(nextRows, normalizedExisting...)
	for _, row := range normalizedFetched {
		modelName := strings.TrimSpace(row.Model)
		upstreamModel := strings.TrimSpace(row.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = modelName
		}
		if modelName == "" {
			modelName = upstreamModel
		}
		if modelName == "" {
			continue
		}
		if _, ok := existingKeys["model:"+modelName]; ok {
			continue
		}
		if upstreamModel != "" {
			if _, ok := existingKeys["upstream:"+upstreamModel]; ok {
				continue
			}
		}
		appended := row
		appended.Model = modelName
		appended.UpstreamModel = upstreamModel
		appended.Selected = false
		appended.Inactive = false
		completeChannelModelRowDefaults(&appended, channelProtocol)
		nextRows = append(nextRows, appended)
		existingKeys["model:"+modelName] = struct{}{}
		if upstreamModel != "" {
			existingKeys["upstream:"+upstreamModel] = struct{}{}
		}
	}
	return ReplaceChannelModelsWithDB(db, normalizedChannelID, nextRows)
}

func SyncFetchedChannelModelsFromBaseWithDB(db *gorm.DB, channelID string, baseRows []ChannelModel, fetchedRows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	rows := BuildFetchedChannelModels(baseRows, fetchedRows, channelProtocol, false)
	return ReplaceChannelModelsWithDB(db, normalizedChannelID, rows)
}

func ReplaceChannelSelectedModelsWithDB(db *gorm.DB, channelID string, selected []string) error {
	existingRows, err := listChannelModelRowsByChannelIDWithDB(db, channelID)
	if err != nil {
		return err
	}
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, channelID)
	if err != nil {
		return err
	}
	selectedSet := buildChannelModelSelectionSet(selected)
	seen := make(map[string]struct{}, len(existingRows)+len(selected))
	rows := make([]ChannelModel, 0, len(existingRows)+len(selected))
	for _, row := range existingRows {
		if _, ok := seen[row.Model]; ok {
			continue
		}
		seen[row.Model] = struct{}{}
		row.Selected = false
		if !row.Inactive {
			if _, ok := selectedSet[row.Model]; ok {
				row.Selected = true
			}
		}
		rows = append(rows, row)
	}
	for _, modelID := range NormalizeChannelModelIDsPreserveOrder(selected) {
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		row := ChannelModel{
			Model:         modelID,
			UpstreamModel: modelID,
			Selected:      true,
		}
		completeChannelModelRowDefaults(&row, channelProtocol)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return ReplaceChannelModelsWithDB(db, channelID, nil)
	}
	for idx := range rows {
		rows[idx].SortOrder = idx + 1
	}
	return ReplaceChannelModelsWithDB(db, channelID, rows)
}

func ReplaceChannelModelsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if err := replaceChannelModelRowsWithDB(db, normalizedChannelID, rows); err != nil {
		return err
	}
	storedRows, err := listChannelModelRowsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	return SyncChannelModelEndpointsWithDB(db, normalizedChannelID, storedRows)
}

func DisableChannelModelCapability(channelID string, modelName string) (bool, error) {
	return DisableChannelModelCapabilityWithReason(channelID, modelName, "", "")
}

func DisableChannelModelCapabilityWithReason(channelID string, modelName string, reason string, disabledBy string) (bool, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedChannelID == "" || normalizedModelName == "" {
		return false, nil
	}

	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		rows, err := listChannelModelRowsByChannelIDWithDB(tx, normalizedChannelID)
		if err != nil {
			return err
		}
		nextRows, disabled := buildDisabledChannelModels(rows, normalizedModelName, reason, disabledBy)
		if !disabled {
			return nil
		}
		if err := ReplaceChannelModelsWithDB(tx, normalizedChannelID, nextRows); err != nil {
			return err
		}
		if err := EnsureChannelTestModelWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil || !changed {
		return changed, err
	}

	channel, err := GetChannelById(normalizedChannelID)
	if err != nil {
		return true, err
	}
	return true, channel.UpdateGroupModelChannels()
}

func DeleteChannelModelsByChannelIDWithDB(db *gorm.DB, channelID string) error {
	return DeleteChannelModelsByChannelIDsWithDB(db, []string{channelID})
}

func DeleteChannelModelsByChannelIDsWithDB(db *gorm.DB, channelIDs []string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id IN ?", normalizedIDs).Delete(&ChannelModelPriceComponent{}).Error; err != nil {
			return err
		}
		return tx.Where("channel_id IN ?", normalizedIDs).Delete(&ChannelModel{}).Error
	})
}

func EnsureChannelTestModelWithDB(db *gorm.DB, channelID string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}

	type channelTestModelRecord struct {
		TestModel string `gorm:"column:test_model"`
	}

	record := channelTestModelRecord{}
	if err := db.Model(&Channel{}).
		Select("test_model").
		Where("id = ?", normalizedChannelID).
		Take(&record).Error; err != nil {
		return err
	}

	selectedModelIDs, err := ListSelectedChannelModelIDsByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	current := strings.TrimSpace(record.TestModel)
	for _, modelID := range selectedModelIDs {
		if modelID == current {
			return nil
		}
	}

	next := ""
	if len(selectedModelIDs) > 0 {
		next = selectedModelIDs[0]
	}
	if current == next {
		return nil
	}
	return db.Model(&Channel{}).
		Where("id = ?", normalizedChannelID).
		Update("test_model", next).Error
}

func loadChannelModelRowsByChannelIDs(db *gorm.DB, channelIDs []string) (map[string][]ChannelModel, error) {
	rowsByChannelID := make(map[string][]ChannelModel)
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return rowsByChannelID, nil
	}
	endpointStateByChannelID, err := loadChannelModelEndpointStateByChannelIDsWithDB(db, normalizedIDs)
	if err != nil {
		return nil, err
	}
	rows := make([]ChannelModel, 0)
	if err := db.
		Where("channel_id IN ?", normalizedIDs).
		Order("channel_id asc, sort_order asc, model asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		normalizeChannelModelRow(&row)
		if row.ChannelId == "" || row.Model == "" {
			continue
		}
		applyChannelModelEndpointState(&row, endpointStateByChannelID[row.ChannelId][row.Model])
		rowsByChannelID[row.ChannelId] = append(rowsByChannelID[row.ChannelId], row)
	}
	return rowsByChannelID, nil
}

func attachChannelModelPriceComponentsWithDB(db *gorm.DB, rowsByChannelID map[string][]ChannelModel) error {
	if db == nil || len(rowsByChannelID) == 0 {
		return nil
	}
	channelIDs := make([]string, 0, len(rowsByChannelID))
	for channelID := range rowsByChannelID {
		if strings.TrimSpace(channelID) != "" {
			channelIDs = append(channelIDs, strings.TrimSpace(channelID))
		}
	}
	channelIDs = normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(channelIDs) == 0 {
		return nil
	}
	componentRows := make([]ChannelModelPriceComponent, 0)
	if err := db.
		Where("channel_id IN ?", channelIDs).
		Order("channel_id asc, model asc, sort_order asc, component asc, condition asc").
		Find(&componentRows).Error; err != nil {
		return err
	}
	componentsByKey := make(map[string][]ProviderModelPriceComponentDetail, len(componentRows))
	for _, row := range componentRows {
		channelID := strings.TrimSpace(row.ChannelId)
		modelName := strings.TrimSpace(row.Model)
		component := strings.TrimSpace(strings.ToLower(row.Component))
		if channelID == "" || modelName == "" || component == "" {
			continue
		}
		componentsByKey[channelID+"\x00"+modelName] = append(componentsByKey[channelID+"\x00"+modelName], ProviderModelPriceComponentDetail{
			Component:   component,
			Condition:   strings.TrimSpace(row.Condition),
			InputPrice:  row.InputPrice,
			OutputPrice: row.OutputPrice,
			PriceUnit:   strings.TrimSpace(strings.ToLower(row.PriceUnit)),
			Currency:    strings.TrimSpace(strings.ToUpper(row.Currency)),
			Source:      strings.TrimSpace(strings.ToLower(row.Source)),
			SourceURL:   strings.TrimSpace(row.SourceURL),
			SortOrder:   row.SortOrder,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	for channelID, rows := range rowsByChannelID {
		for i := range rows {
			rows[i].PriceComponents = NormalizeProviderModelPriceComponents(componentsByKey[channelID+"\x00"+rows[i].Model])
		}
		rowsByChannelID[channelID] = rows
	}
	return nil
}

func normalizeTrimmedValuesPreserveOrder(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, item := range values {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func listChannelModelRowsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelModel, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModel{}, nil
	}
	rows := make([]ChannelModel, 0)
	if err := db.
		Where("channel_id = ?", normalizedChannelID).
		Order("sort_order asc, model asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	rowsByChannelID := map[string][]ChannelModel{
		normalizedChannelID: rows,
	}
	if err := attachChannelModelPriceComponentsWithDB(db, rowsByChannelID); err != nil {
		return nil, err
	}
	rows = rowsByChannelID[normalizedChannelID]
	endpointStateByChannelID, err := loadChannelModelEndpointStateByChannelIDsWithDB(db, []string{normalizedChannelID})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		normalizeChannelModelRow(&rows[i])
		applyChannelModelEndpointState(&rows[i], endpointStateByChannelID[normalizedChannelID][rows[i].Model])
	}
	return rows, nil
}

func ListChannelModelRowsPageWithDB(db *gorm.DB, channelID string, page int, pageSize int, keyword string) ([]ChannelModel, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModel{}, 0, nil
	}
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	query := buildChannelModelListQueryWithDB(db, normalizedChannelID, keyword)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]ChannelModel, 0, pageSize)
	if err := query.
		Order("inactive asc, sort_order asc, model asc").
		Limit(pageSize).
		Offset(page * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	rowsByChannelID := map[string][]ChannelModel{
		normalizedChannelID: rows,
	}
	if err := attachChannelModelPriceComponentsWithDB(db, rowsByChannelID); err != nil {
		return nil, 0, err
	}
	rows = rowsByChannelID[normalizedChannelID]
	endpointStateByChannelID, err := loadChannelModelEndpointStateByChannelIDsWithDB(db, []string{normalizedChannelID})
	if err != nil {
		return nil, 0, err
	}
	for i := range rows {
		normalizeChannelModelRow(&rows[i])
		applyChannelModelEndpointState(&rows[i], endpointStateByChannelID[normalizedChannelID][rows[i].Model])
	}
	return rows, total, nil
}

func ListChannelModelRowsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelModel, error) {
	return listChannelModelRowsByChannelIDWithDB(db, channelID)
}

func buildChannelModelListQueryWithDB(db *gorm.DB, channelID string, keyword string) *gorm.DB {
	query := db.Model(&ChannelModel{}).Where("channel_id = ?", strings.TrimSpace(channelID))
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		"LOWER(model) LIKE ? OR LOWER(COALESCE(upstream_model, '')) LIKE ? OR LOWER(COALESCE(type, '')) LIKE ? OR LOWER(COALESCE(endpoint, '')) LIKE ?",
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func normalizeChannelModelRow(row *ChannelModel) {
	if row == nil {
		return
	}
	row.ChannelId = strings.TrimSpace(row.ChannelId)
	row.Model = strings.TrimSpace(row.Model)
	row.UpstreamModel = strings.TrimSpace(row.UpstreamModel)
	row.Provider = strings.TrimSpace(strings.ToLower(row.Provider))
	row.DisabledReason = strings.TrimSpace(row.DisabledReason)
	row.DisabledBy = strings.TrimSpace(row.DisabledBy)
	if row.Model == "" && row.UpstreamModel != "" {
		row.Model = row.UpstreamModel
	}
	if row.UpstreamModel == "" {
		row.UpstreamModel = row.Model
	}
	row.Type = normalizeExplicitChannelModelType(row.Type)
	rawEndpoint := strings.TrimSpace(row.Endpoint)
	explicitEndpoint := ""
	if rawEndpoint != "" {
		explicitEndpoint = NormalizeChannelModelEndpoint(row.Type, rawEndpoint)
	}
	row.Endpoints = NormalizeChannelModelDirectEndpoints(row.Type, row.Endpoints, row.Endpoint)
	if len(row.Endpoints) > 0 {
		if explicitEndpoint != "" {
			for _, endpoint := range row.Endpoints {
				if endpoint == explicitEndpoint {
					row.Endpoint = explicitEndpoint
					goto endpointResolved
				}
			}
		}
		row.Endpoint = row.Endpoints[0]
	} else {
		row.Endpoint = NormalizeChannelModelEndpoint(row.Type, row.Endpoint)
	}
endpointResolved:
	row.PriceUnit = strings.TrimSpace(strings.ToLower(row.PriceUnit))
	row.Currency = normalizeChannelModelCurrency(row.Currency)
	row.InputPrice = cloneNormalizedChannelModelPrice(row.InputPrice)
	row.OutputPrice = cloneNormalizedChannelModelPrice(row.OutputPrice)
	row.PriceComponents = NormalizeProviderModelPriceComponents(row.PriceComponents)
}

func applyChannelModelRows(channel *Channel, rows []ChannelModel) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelsPreserveOrder(rows)
	for i := range normalized {
		completeChannelModelRowDefaults(&normalized[i], channel.GetChannelProtocol())
	}
	channel.SetChannelModels(normalized)
}

func buildChannelModelSelectionSet(modelIDs []string) map[string]struct{} {
	normalized := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	set := make(map[string]struct{}, len(normalized))
	for _, modelID := range normalized {
		set[modelID] = struct{}{}
	}
	return set
}

func buildDisabledChannelModels(rows []ChannelModel, modelName string, reason string, disabledBy string) ([]ChannelModel, bool) {
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" || len(normalizedRows) == 0 {
		return normalizedRows, false
	}
	now := helper.GetTimestamp()
	normalizedReason := strings.TrimSpace(reason)
	normalizedDisabledBy := strings.TrimSpace(disabledBy)
	if normalizedDisabledBy == "" {
		normalizedDisabledBy = "runtime"
	}
	changed := false
	for idx := range normalizedRows {
		if strings.TrimSpace(normalizedRows[idx].Model) != normalizedModelName {
			continue
		}
		if normalizedRows[idx].Inactive && !normalizedRows[idx].Selected &&
			strings.TrimSpace(normalizedRows[idx].DisabledReason) == normalizedReason &&
			strings.TrimSpace(normalizedRows[idx].DisabledBy) == normalizedDisabledBy &&
			normalizedRows[idx].DisabledAt > 0 {
			return normalizedRows, changed
		}
		normalizedRows[idx].Inactive = true
		normalizedRows[idx].Selected = false
		normalizedRows[idx].DisabledReason = normalizedReason
		normalizedRows[idx].DisabledAt = now
		normalizedRows[idx].DisabledBy = normalizedDisabledBy
		changed = true
	}
	return normalizedRows, changed
}

func replaceChannelModelRowsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	providerByModel, err := LoadUniqueProviderMapByModelsWithDB(db, channelModelProviderLookupCandidates(normalizedRows))
	if err != nil {
		return err
	}
	now := helper.GetTimestamp()
	dbRows := make([]ChannelModel, 0, len(normalizedRows))
	componentRows := make([]ChannelModelPriceComponent, 0)
	for idx, row := range normalizedRows {
		row.ChannelId = normalizedChannelID
		row.SortOrder = idx + 1
		row.UpdatedAt = now
		normalizeChannelModelRow(&row)
		completeChannelModelRowDefaults(&row, channelProtocol)
		if row.Selected && !row.Inactive {
			row.DisabledReason = ""
			row.DisabledAt = 0
			row.DisabledBy = ""
		}
		if strings.TrimSpace(row.Provider) == "" {
			row.Provider = ResolveProviderFromModelMap(providerByModel, row.UpstreamModel, row.Model)
		}
		for componentIdx, component := range NormalizeProviderModelPriceComponents(row.PriceComponents) {
			componentUpdatedAt := component.UpdatedAt
			if componentUpdatedAt == 0 {
				componentUpdatedAt = now
			}
			componentSortOrder := component.SortOrder
			if componentSortOrder == 0 {
				componentSortOrder = componentIdx + 1
			}
			componentRows = append(componentRows, ChannelModelPriceComponent{
				ChannelId:   normalizedChannelID,
				Model:       row.Model,
				Component:   component.Component,
				Condition:   component.Condition,
				InputPrice:  component.InputPrice,
				OutputPrice: component.OutputPrice,
				PriceUnit:   component.PriceUnit,
				Currency:    component.Currency,
				Source:      component.Source,
				SourceURL:   component.SourceURL,
				SortOrder:   componentSortOrder,
				UpdatedAt:   componentUpdatedAt,
			})
		}
		row.PriceComponents = nil
		dbRows = append(dbRows, row)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockChannelRowForUpdateWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModelPriceComponent{}).Error; err != nil {
			return err
		}
		if len(dbRows) == 0 {
			return nil
		}
		if err := tx.Select("*").Create(&dbRows).Error; err != nil {
			return err
		}
		if len(componentRows) == 0 {
			return nil
		}
		return tx.Select("*").Create(&componentRows).Error
	})
}

func channelModelProviderLookupCandidates(rows []ChannelModel) []string {
	candidates := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		candidates = append(candidates, row.Model, row.UpstreamModel)
	}
	return NormalizeProviderLookupCandidates(candidates...)
}

func lockChannelRowForUpdateWithDB(db *gorm.DB, channelID string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	type channelIDRow struct {
		ID string `gorm:"column:id"`
	}
	row := channelIDRow{}
	return db.
		Set("gorm:query_option", "FOR UPDATE").
		Model(&Channel{}).
		Select("id").
		Where("id = ?", normalizedChannelID).
		Take(&row).Error
}

func BuildFetchedChannelModels(existingRows []ChannelModel, fetchedRows []ChannelModel, channelProtocol int, selectAll bool) []ChannelModel {
	normalizedFetchedRows := NormalizeChannelModelsPreserveOrder(fetchedRows)
	normalizedExisting := NormalizeChannelModelsPreserveOrder(existingRows)
	existingByUpstream := make(map[string]ChannelModel, len(normalizedExisting))
	for _, row := range normalizedExisting {
		upstream := strings.TrimSpace(row.UpstreamModel)
		if upstream == "" {
			upstream = strings.TrimSpace(row.Model)
		}
		if upstream == "" {
			continue
		}
		if _, ok := existingByUpstream[upstream]; ok {
			continue
		}
		completeChannelModelRowDefaults(&row, channelProtocol)
		existingByUpstream[upstream] = row
	}
	rows := make([]ChannelModel, 0, len(normalizedFetchedRows))
	seenUpstream := make(map[string]struct{}, len(normalizedFetchedRows))
	for idx, fetchedRow := range normalizedFetchedRows {
		upstreamModel := strings.TrimSpace(fetchedRow.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = strings.TrimSpace(fetchedRow.Model)
		}
		if upstreamModel == "" {
			continue
		}
		row, ok := existingByUpstream[upstreamModel]
		if !ok {
			row = fetchedRow
			if !selectAll {
				row.Selected = false
			}
		} else {
			if strings.TrimSpace(row.Model) == "" && strings.TrimSpace(fetchedRow.Model) != "" {
				row.Model = strings.TrimSpace(fetchedRow.Model)
			}
			if strings.TrimSpace(fetchedRow.Type) != "" {
				row.Type = strings.TrimSpace(fetchedRow.Type)
			}
			if strings.TrimSpace(fetchedRow.PriceUnit) != "" {
				row.PriceUnit = strings.TrimSpace(fetchedRow.PriceUnit)
			}
			if strings.TrimSpace(fetchedRow.Currency) != "" {
				row.Currency = strings.TrimSpace(fetchedRow.Currency)
			}
			row.Provider = strings.TrimSpace(fetchedRow.Provider)
		}
		row.UpstreamModel = upstreamModel
		row.Inactive = false
		if selectAll {
			row.Selected = true
		}
		row.SortOrder = idx + 1
		completeChannelModelRowDefaults(&row, channelProtocol)
		rows = append(rows, row)
		seenUpstream[upstreamModel] = struct{}{}
	}
	for _, row := range normalizedExisting {
		upstreamModel := strings.TrimSpace(row.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = strings.TrimSpace(row.Model)
		}
		if upstreamModel == "" {
			continue
		}
		if _, ok := seenUpstream[upstreamModel]; ok {
			continue
		}
		row.Selected = false
		row.Inactive = true
		row.SortOrder = len(rows) + 1
		completeChannelModelRowDefaults(&row, channelProtocol)
		rows = append(rows, row)
	}
	return NormalizeChannelModelsPreserveOrder(rows)
}

func FindSelectedChannelModelConfig(rows []ChannelModel, candidates ...string) (ChannelModel, bool) {
	normalizedRows := NormalizeChannelModelsPreserveOrder(rows)
	if len(normalizedRows) == 0 {
		return ChannelModel{}, false
	}
	normalizedCandidates := normalizeTrimmedValuesPreserveOrder(candidates)
	if len(normalizedCandidates) == 0 {
		return ChannelModel{}, false
	}
	for _, row := range normalizedRows {
		if !row.Selected {
			continue
		}
		for _, candidate := range normalizedCandidates {
			if candidate == strings.TrimSpace(row.Model) || candidate == strings.TrimSpace(row.UpstreamModel) {
				return row, true
			}
		}
	}
	return ChannelModel{}, false
}

func loadChannelProtocolByChannelIDWithDB(db *gorm.DB, channelID string) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return 0, nil
	}
	type channelProtocolRecord struct {
		Protocol string `gorm:"column:protocol"`
	}
	record := channelProtocolRecord{}
	if err := db.Model(&Channel{}).
		Select("protocol").
		Where("id = ?", normalizedChannelID).
		Take(&record).Error; err != nil {
		return 0, err
	}
	channel := Channel{Protocol: record.Protocol}
	return channel.GetChannelProtocol(), nil
}

func completeChannelModelRowDefaults(row *ChannelModel, channelProtocol int) {
	if row == nil {
		return
	}
	rawEndpoint := strings.TrimSpace(row.Endpoint)
	hasExplicitEndpointList := false
	for _, endpoint := range row.Endpoints {
		if strings.TrimSpace(endpoint) != "" {
			hasExplicitEndpointList = true
			break
		}
	}
	normalizeChannelModelRow(row)
	row.Type = resolveChannelModelType(row.Type, channelProtocol, row.UpstreamModel, row.Model)
	endpointFallback := rawEndpoint
	if endpointFallback == "" && !hasExplicitEndpointList {
		// Use protocol-priority endpoint only for first-time defaults.
		endpointFallback = DefaultChannelModelEndpointWithProtocol(row.Type, channelProtocol)
		// Drop generic defaults injected before protocol is resolved.
		row.Endpoints = nil
	}
	row.Endpoints = NormalizeChannelModelDirectEndpoints(row.Type, row.Endpoints, endpointFallback)
	explicitEndpoint := ""
	if strings.TrimSpace(endpointFallback) != "" {
		explicitEndpoint = NormalizeChannelModelEndpoint(row.Type, endpointFallback)
	}
	if len(row.Endpoints) > 0 {
		if explicitEndpoint != "" {
			for _, endpoint := range row.Endpoints {
				if endpoint == explicitEndpoint {
					row.Endpoint = explicitEndpoint
					goto endpointDefaultResolved
				}
			}
		}
		row.Endpoint = row.Endpoints[0]
	} else {
		row.Endpoint = NormalizeChannelModelEndpoint(row.Type, endpointFallback)
	}
endpointDefaultResolved:
	row.PriceUnit = normalizeChannelModelPriceUnit(row.PriceUnit, row.Type, channelProtocol, row.UpstreamModel, row.Model)
	row.Currency = normalizeChannelModelCurrency(row.Currency)
	row.InputPrice = cloneNormalizedChannelModelPrice(row.InputPrice)
	row.OutputPrice = cloneNormalizedChannelModelPrice(row.OutputPrice)
}

func NormalizeChannelModelDirectEndpoints(modelType string, endpoints []string, fallback string) []string {
	candidates := make([]string, 0, len(endpoints)+1)
	candidates = append(candidates, endpoints...)
	if strings.TrimSpace(fallback) != "" {
		candidates = append(candidates, fallback)
	}
	if len(candidates) == 0 {
		candidates = append(candidates, DefaultChannelModelEndpoint(modelType))
	}
	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := NormalizeChannelModelEndpoint(modelType, candidate)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		normalized := NormalizeChannelModelEndpoint(modelType, fallback)
		if normalized == "" {
			normalized = DefaultChannelModelEndpoint(modelType)
		}
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

type channelModelEndpointState struct {
	Endpoints []string
	Enabled   map[string]bool
}

func loadChannelModelEndpointStateByChannelIDsWithDB(db *gorm.DB, channelIDs []string) (map[string]map[string]channelModelEndpointState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		return map[string]map[string]channelModelEndpointState{}, nil
	}
	rows := make([]ChannelModelEndpoint, 0)
	if err := db.
		Where("channel_id IN ?", normalizedChannelIDs).
		Order("channel_id asc, model asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]map[string]channelModelEndpointState)
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		modelName := strings.TrimSpace(row.Model)
		endpoint := NormalizeRequestedChannelModelEndpoint(row.Endpoint)
		if channelID == "" || modelName == "" || endpoint == "" {
			continue
		}
		if _, ok := result[channelID]; !ok {
			result[channelID] = make(map[string]channelModelEndpointState)
		}
		state, ok := result[channelID][modelName]
		if !ok {
			state = channelModelEndpointState{
				Endpoints: make([]string, 0, 3),
				Enabled:   make(map[string]bool),
			}
		}
		if _, exists := state.Enabled[endpoint]; !exists {
			state.Endpoints = append(state.Endpoints, endpoint)
		}
		state.Enabled[endpoint] = row.Enabled
		result[channelID][modelName] = state
	}
	return result, nil
}

func applyChannelModelEndpointState(row *ChannelModel, state channelModelEndpointState) {
	if row == nil {
		return
	}
	rawEndpoint := strings.TrimSpace(row.Endpoint)
	explicitEndpoint := ""
	if rawEndpoint != "" {
		explicitEndpoint = NormalizeChannelModelEndpoint(row.Type, rawEndpoint)
	}
	candidates := make([]string, 0, len(row.Endpoints)+len(state.Endpoints)+1)
	candidates = append(candidates, row.Endpoints...)
	candidates = append(candidates, state.Endpoints...)
	candidates = append(candidates, row.Endpoint)
	row.Endpoints = NormalizeChannelModelDirectEndpoints(row.Type, candidates, row.Endpoint)
	if len(row.Endpoints) == 0 {
		row.Endpoint = NormalizeChannelModelEndpoint(row.Type, row.Endpoint)
		return
	}
	if explicitEndpoint != "" {
		for _, endpoint := range row.Endpoints {
			if endpoint == explicitEndpoint {
				row.Endpoint = explicitEndpoint
				return
			}
		}
	}
	if len(state.Enabled) > 0 {
		for _, endpoint := range row.Endpoints {
			if enabled, ok := state.Enabled[endpoint]; ok && enabled {
				row.Endpoint = endpoint
				return
			}
		}
	}
	row.Endpoint = row.Endpoints[0]
}

func normalizeExplicitChannelModelType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case ProviderModelTypeImage:
		return ProviderModelTypeImage
	case ProviderModelTypeAudio:
		return ProviderModelTypeAudio
	case ProviderModelTypeVideo:
		return ProviderModelTypeVideo
	default:
		return ""
	}
}

func resolveChannelProviderDetail(channelProtocol int, upstreamModel string, model string) (ResolvedModelPricing, bool) {
	candidates := normalizeTrimmedValuesPreserveOrder([]string{upstreamModel, model})
	for _, candidate := range candidates {
		if detail, ok := lookupProviderDefaultModelPricing(candidate, channelProtocol); ok {
			return detail, true
		}
	}
	return ResolvedModelPricing{}, false
}

func resolveChannelModelType(raw string, channelProtocol int, upstreamModel string, model string) string {
	explicit := normalizeExplicitChannelModelType(raw)
	if explicit != "" {
		return explicit
	}
	if detail, ok := resolveChannelProviderDetail(channelProtocol, upstreamModel, model); ok {
		resolved := normalizeModelType(detail.Type, detail.Model)
		if resolved != "" {
			return resolved
		}
	}
	referenceModel := strings.TrimSpace(upstreamModel)
	if referenceModel == "" {
		referenceModel = strings.TrimSpace(model)
	}
	return normalizeModelType("", referenceModel)
}

func normalizeChannelModelPriceUnit(raw string, modelType string, channelProtocol int, upstreamModel string, model string) string {
	priceUnit := strings.TrimSpace(strings.ToLower(raw))
	if detail, ok := resolveChannelProviderDetail(channelProtocol, upstreamModel, model); ok {
		providerPriceUnit := strings.TrimSpace(strings.ToLower(detail.PriceUnit))
		if providerPriceUnit == "" {
			providerPriceUnit = defaultPriceUnitByType(detail.Type, detail.Model)
		}
		if providerPriceUnit != "" && (priceUnit == "" || priceUnit == ProviderPriceUnitPer1KTokens) {
			return providerPriceUnit
		}
	}
	if priceUnit != "" {
		return priceUnit
	}
	referenceModel := strings.TrimSpace(upstreamModel)
	if referenceModel == "" {
		referenceModel = strings.TrimSpace(model)
	}
	return defaultPriceUnitByType(modelType, referenceModel)
}

func normalizeChannelModelCurrency(raw string) string {
	currency := strings.TrimSpace(strings.ToUpper(raw))
	if currency != "" {
		return currency
	}
	return ProviderPriceCurrencyUSD
}

func cloneNormalizedChannelModelPrice(value *float64) *float64 {
	if value == nil {
		return nil
	}
	normalized := *value
	if normalized < 0 {
		normalized = 0
	}
	return &normalized
}

const (
	ChannelModelEndpointChat       = "/v1/chat/completions"
	ChannelModelEndpointMessages   = "/v1/messages"
	ChannelModelEndpointResponses  = "/v1/responses"
	ChannelModelEndpointRealtime   = "/v1/realtime"
	ChannelModelEndpointBatches    = "/v1/batches"
	ChannelModelEndpointEmbeddings = "/v1/embeddings"
	ChannelModelEndpointImages     = "/v1/images/generations"
	ChannelModelEndpointImageEdit  = "/v1/images/edits"
	ChannelModelEndpointAudio      = "/v1/audio/speech"
	ChannelModelEndpointVideos     = "/v1/videos"
)

func channelModelEndpointSortRank(endpoint string) int {
	switch NormalizeRequestedChannelModelEndpoint(endpoint) {
	case ChannelModelEndpointChat:
		return 10
	case ChannelModelEndpointResponses:
		return 20
	case ChannelModelEndpointMessages:
		return 30
	case ChannelModelEndpointRealtime:
		return 35
	case ChannelModelEndpointImages:
		return 40
	case ChannelModelEndpointImageEdit:
		return 50
	case ChannelModelEndpointBatches:
		return 60
	case ChannelModelEndpointEmbeddings:
		return 65
	case ChannelModelEndpointAudio:
		return 70
	case ChannelModelEndpointVideos:
		return 80
	default:
		return 1000
	}
}

func DefaultChannelModelEndpoint(modelType string) string {
	switch normalizeModelType(modelType, "") {
	case ProviderModelTypeImage:
		return ChannelModelEndpointImages
	case ProviderModelTypeAudio:
		return ChannelModelEndpointAudio
	case ProviderModelTypeVideo:
		return ChannelModelEndpointVideos
	case ProviderModelTypeEmbedding:
		return ChannelModelEndpointEmbeddings
	default:
		return ChannelModelEndpointResponses
	}
}

func DefaultChannelModelEndpointWithProtocol(modelType string, channelProtocol int) string {
	switch normalizeModelType(modelType, "") {
	case ProviderModelTypeText:
		if channelProtocol == relaychannel.Anthropic {
			return ChannelModelEndpointMessages
		}
	}
	return DefaultChannelModelEndpoint(modelType)
}

func NormalizeChannelModelEndpoint(modelType string, endpoint string) string {
	normalizedType := normalizeModelType(modelType, "")
	normalizedEndpoint := strings.TrimSpace(strings.ToLower(endpoint))
	switch normalizedType {
	case ProviderModelTypeImage:
		switch normalizedEndpoint {
		case ChannelModelEndpointResponses:
			return ChannelModelEndpointResponses
		case ChannelModelEndpointBatches:
			return ChannelModelEndpointBatches
		case ChannelModelEndpointImageEdit:
			return ChannelModelEndpointImageEdit
		case ChannelModelEndpointImages:
			return ChannelModelEndpointImages
		default:
			return ChannelModelEndpointImages
		}
	case ProviderModelTypeAudio:
		if normalizedEndpoint == ChannelModelEndpointRealtime {
			return ChannelModelEndpointRealtime
		}
		if normalizedEndpoint == ChannelModelEndpointAudio {
			return ChannelModelEndpointAudio
		}
		return ChannelModelEndpointAudio
	case ProviderModelTypeVideo:
		if normalizedEndpoint == ChannelModelEndpointVideos {
			return ChannelModelEndpointVideos
		}
		return ChannelModelEndpointVideos
	case ProviderModelTypeEmbedding:
		if normalizedEndpoint == ChannelModelEndpointEmbeddings {
			return ChannelModelEndpointEmbeddings
		}
		return ChannelModelEndpointEmbeddings
	default:
		switch normalizedEndpoint {
		case ChannelModelEndpointChat:
			return ChannelModelEndpointChat
		case ChannelModelEndpointMessages:
			return ChannelModelEndpointMessages
		case ChannelModelEndpointResponses:
			return ChannelModelEndpointResponses
		default:
			return ChannelModelEndpointResponses
		}
	}
}
