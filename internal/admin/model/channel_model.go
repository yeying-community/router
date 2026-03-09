package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelModelsTableName = "channel_models"
)

type ChannelModel struct {
	ChannelId     string   `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model         string   `json:"model" gorm:"primaryKey;type:varchar(255)"`
	UpstreamModel string   `json:"upstream_model" gorm:"type:varchar(255);default:'';index"`
	Type          string   `json:"type" gorm:"type:varchar(32);default:'text'"`
	Selected      bool     `json:"selected" gorm:"default:true;index"`
	InputPrice    *float64 `json:"input_price,omitempty" gorm:"type:double precision"`
	OutputPrice   *float64 `json:"output_price,omitempty" gorm:"type:double precision"`
	PriceUnit     string   `json:"price_unit,omitempty" gorm:"type:varchar(64);default:''"`
	Currency      string   `json:"currency,omitempty" gorm:"type:varchar(16);default:''"`
	SortOrder     int      `json:"sort_order" gorm:"default:0"`
	UpdatedAt     int64    `json:"updated_at" gorm:"bigint"`
}

func (ChannelModel) TableName() string {
	return ChannelModelsTableName
}

func NormalizeChannelModelIDsPreserveOrder(modelIDs []string) []string {
	return normalizeTrimmedValuesPreserveOrder(modelIDs)
}

func NormalizeChannelModelConfigsPreserveOrder(rows []ChannelModel) []ChannelModel {
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

func BuildDefaultChannelModelConfigs(modelIDs []string) []ChannelModel {
	return BuildDefaultChannelModelConfigsWithProtocol(modelIDs, 0)
}

func BuildDefaultChannelModelConfigsWithProtocol(modelIDs []string, channelProtocol int) []ChannelModel {
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
			channel.SetModelConfigs(nil)
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
		if !row.Selected {
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
		modelIDs = append(modelIDs, row.Model)
	}
	return NormalizeChannelModelIDsPreserveOrder(modelIDs), nil
}

func SyncFetchedChannelModelsWithDB(db *gorm.DB, channelID string, modelIDs []string) error {
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
	rows := BuildFetchedChannelModelConfigs(existingRows, BuildDefaultChannelModelConfigsWithProtocol(normalizedModelIDs, channelProtocol), channelProtocol, true)
	return ReplaceChannelModelConfigsWithDB(db, normalizedChannelID, rows)
}

func SyncFetchedChannelModelConfigsWithDB(db *gorm.DB, channelID string, fetchedRows []ChannelModel) error {
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
	rows := BuildFetchedChannelModelConfigs(existingRows, fetchedRows, channelProtocol, true)
	return ReplaceChannelModelConfigsWithDB(db, normalizedChannelID, rows)
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
		if _, ok := selectedSet[row.Model]; ok {
			row.Selected = true
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
		return ReplaceChannelModelConfigsWithDB(db, channelID, nil)
	}
	for idx := range rows {
		rows[idx].SortOrder = idx + 1
	}
	return ReplaceChannelModelConfigsWithDB(db, channelID, rows)
}

func ReplaceChannelModelConfigsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	return replaceChannelModelRowsWithDB(db, channelID, rows)
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
	return db.Where("channel_id IN ?", normalizedIDs).Delete(&ChannelModel{}).Error
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
		rowsByChannelID[row.ChannelId] = append(rowsByChannelID[row.ChannelId], row)
	}
	return rowsByChannelID, nil
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
	for i := range rows {
		normalizeChannelModelRow(&rows[i])
	}
	return rows, nil
}

func normalizeChannelModelRow(row *ChannelModel) {
	if row == nil {
		return
	}
	row.ChannelId = strings.TrimSpace(row.ChannelId)
	row.Model = strings.TrimSpace(row.Model)
	row.UpstreamModel = strings.TrimSpace(row.UpstreamModel)
	if row.Model == "" && row.UpstreamModel != "" {
		row.Model = row.UpstreamModel
	}
	if row.UpstreamModel == "" {
		row.UpstreamModel = row.Model
	}
	row.Type = normalizeExplicitChannelModelType(row.Type)
	row.PriceUnit = strings.TrimSpace(strings.ToLower(row.PriceUnit))
	row.Currency = normalizeChannelModelCurrency(row.Currency)
	row.InputPrice = cloneNormalizedChannelModelPrice(row.InputPrice)
	row.OutputPrice = cloneNormalizedChannelModelPrice(row.OutputPrice)
}

func applyChannelModelRows(channel *Channel, rows []ChannelModel) {
	if channel == nil {
		return
	}
	normalized := NormalizeChannelModelConfigsPreserveOrder(rows)
	for i := range normalized {
		completeChannelModelRowDefaults(&normalized[i], channel.GetChannelProtocol())
	}
	channel.SetModelConfigs(normalized)
}

func buildChannelModelSelectionSet(modelIDs []string) map[string]struct{} {
	normalized := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	set := make(map[string]struct{}, len(normalized))
	for _, modelID := range normalized {
		set[modelID] = struct{}{}
	}
	return set
}

func replaceChannelModelRowsWithDB(db *gorm.DB, channelID string, rows []ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedRows := NormalizeChannelModelConfigsPreserveOrder(rows)
	channelProtocol, err := loadChannelProtocolByChannelIDWithDB(db, normalizedChannelID)
	if err != nil {
		return err
	}
	now := helper.GetTimestamp()
	dbRows := make([]ChannelModel, 0, len(normalizedRows))
	for idx, row := range normalizedRows {
		row.ChannelId = normalizedChannelID
		row.SortOrder = idx + 1
		row.UpdatedAt = now
		normalizeChannelModelRow(&row)
		completeChannelModelRowDefaults(&row, channelProtocol)
		dbRows = append(dbRows, row)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModel{}).Error; err != nil {
			return err
		}
		if len(dbRows) == 0 {
			return nil
		}
		return tx.Create(&dbRows).Error
	})
}

func BuildFetchedChannelModelConfigs(existingRows []ChannelModel, fetchedRows []ChannelModel, channelProtocol int, selectAll bool) []ChannelModel {
	normalizedFetchedRows := NormalizeChannelModelConfigsPreserveOrder(fetchedRows)
	normalizedExisting := NormalizeChannelModelConfigsPreserveOrder(existingRows)
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
		}
		row.UpstreamModel = upstreamModel
		if selectAll {
			row.Selected = true
		}
		row.SortOrder = idx + 1
		completeChannelModelRowDefaults(&row, channelProtocol)
		rows = append(rows, row)
	}
	return NormalizeChannelModelConfigsPreserveOrder(rows)
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
	normalizeChannelModelRow(row)
	row.Type = resolveChannelModelType(row.Type, channelProtocol, row.UpstreamModel, row.Model)
	row.PriceUnit = normalizeChannelModelPriceUnit(row.PriceUnit, row.Type, channelProtocol, row.UpstreamModel, row.Model)
	row.Currency = normalizeChannelModelCurrency(row.Currency)
	row.InputPrice = cloneNormalizedChannelModelPrice(row.InputPrice)
	row.OutputPrice = cloneNormalizedChannelModelPrice(row.OutputPrice)
}

func normalizeExplicitChannelModelType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case ModelProviderModelTypeImage:
		return ModelProviderModelTypeImage
	case ModelProviderModelTypeAudio:
		return ModelProviderModelTypeAudio
	default:
		return ""
	}
}

func resolveChannelModelProviderDetail(channelProtocol int, upstreamModel string, model string) (ResolvedModelPricing, bool) {
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
	if detail, ok := resolveChannelModelProviderDetail(channelProtocol, upstreamModel, model); ok {
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
	if detail, ok := resolveChannelModelProviderDetail(channelProtocol, upstreamModel, model); ok {
		providerPriceUnit := strings.TrimSpace(strings.ToLower(detail.PriceUnit))
		if providerPriceUnit == "" {
			providerPriceUnit = defaultPriceUnitByType(detail.Type, detail.Model)
		}
		if providerPriceUnit != "" && (priceUnit == "" || priceUnit == ModelProviderPriceUnitPer1KTokens) {
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
	return ModelProviderPriceCurrencyUSD
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
