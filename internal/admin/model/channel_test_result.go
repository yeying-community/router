package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelTestsTableName = "channel_tests"

	ChannelTestStatusSupported   = "supported"
	ChannelTestStatusUnsupported = "unsupported"
	ChannelTestStatusSkipped     = "skipped"
)

type ChannelTest struct {
	ChannelId     string `json:"channel_id,omitempty" gorm:"primaryKey;type:varchar(64);index"`
	Model         string `json:"model" gorm:"primaryKey;type:varchar(255)"`
	Round         int64  `json:"round,omitempty" gorm:"primaryKey;type:bigint"`
	UpstreamModel string `json:"upstream_model,omitempty" gorm:"type:varchar(255);default:'';index"`
	Type          string `json:"type" gorm:"type:varchar(32);default:'text';index"`
	Endpoint      string `json:"endpoint" gorm:"type:varchar(255);index"`
	Status        string `json:"status" gorm:"type:varchar(32);index"`
	Supported     bool   `json:"supported" gorm:"not null;default:false"`
	Message       string `json:"message,omitempty" gorm:"type:text"`
	LatencyMs     int64  `json:"latency_ms,omitempty" gorm:"bigint"`
	SortOrder     int64  `json:"sort_order,omitempty" gorm:"bigint;default:0"`
	TestedAt      int64  `json:"tested_at,omitempty" gorm:"bigint;index"`
}

func (ChannelTest) TableName() string {
	return ChannelTestsTableName
}

func NormalizeChannelTestStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case ChannelTestStatusSupported:
		return ChannelTestStatusSupported
	case ChannelTestStatusSkipped:
		return ChannelTestStatusSkipped
	default:
		return ChannelTestStatusUnsupported
	}
}

func NormalizeChannelTestRows(rows []ChannelTest) []ChannelTest {
	if len(rows) == 0 {
		return []ChannelTest{}
	}
	result := make([]ChannelTest, 0, len(rows))
	indexByKey := make(map[string]int, len(rows))
	for idx, row := range rows {
		normalized := ChannelTest{
			ChannelId:     strings.TrimSpace(row.ChannelId),
			Model:         strings.TrimSpace(row.Model),
			Round:         row.Round,
			UpstreamModel: strings.TrimSpace(row.UpstreamModel),
			Type:          normalizeModelType(row.Type, row.Model),
			Endpoint:      NormalizeChannelModelEndpoint(row.Type, row.Endpoint),
			Status:        NormalizeChannelTestStatus(row.Status),
			Supported:     row.Supported && NormalizeChannelTestStatus(row.Status) == ChannelTestStatusSupported,
			Message:       strings.TrimSpace(row.Message),
			LatencyMs:     row.LatencyMs,
			SortOrder:     row.SortOrder,
			TestedAt:      row.TestedAt,
		}
		if normalized.Model == "" && normalized.UpstreamModel != "" {
			normalized.Model = normalized.UpstreamModel
		}
		if normalized.UpstreamModel == "" {
			normalized.UpstreamModel = normalized.Model
		}
		if normalized.ChannelId == "" || normalized.Model == "" || normalized.Endpoint == "" {
			continue
		}
		if normalized.Round < 0 {
			normalized.Round = 0
		}
		if normalized.SortOrder == 0 {
			normalized.SortOrder = int64(idx + 1)
		}
		key := normalized.ChannelId + "::" + normalized.Model
		if normalized.Round > 0 {
			key += "::round:" + strconv.FormatInt(normalized.Round, 10)
		} else {
			key += "::endpoint:" + normalized.Endpoint
		}
		if existingIdx, ok := indexByKey[key]; ok {
			existing := result[existingIdx]
			if existing.Round < normalized.Round || existing.TestedAt <= normalized.TestedAt {
				result[existingIdx] = normalized
			}
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, normalized)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		if result[i].Round != result[j].Round {
			return result[i].Round > result[j].Round
		}
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		if result[i].Model != result[j].Model {
			return result[i].Model < result[j].Model
		}
		return result[i].Endpoint < result[j].Endpoint
	})
	return result
}

func HydrateChannelWithTests(db *gorm.DB, channel *Channel) error {
	if channel == nil {
		return nil
	}
	return HydrateChannelsWithTests(db, []*Channel{channel})
}

func HydrateChannelsWithTests(db *gorm.DB, channels []*Channel) error {
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
			channel.SetChannelTests(nil)
			continue
		}
		channelIDs = append(channelIDs, channel.Id)
		normalizedChannels = append(normalizedChannels, channel)
	}
	if len(normalizedChannels) == 0 {
		return nil
	}
	rowsByChannelID, err := loadChannelTestRowsByChannelIDs(db, channelIDs)
	if err != nil {
		return err
	}
	for _, channel := range normalizedChannels {
		channel.SetChannelTests(rowsByChannelID[channel.Id])
	}
	return nil
}

func ReplaceChannelTestsWithDB(db *gorm.DB, channelID string, rows []ChannelTest) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedRows := NormalizeChannelTestRows(rows)
	now := helper.GetTimestamp()
	roundByModel := make(map[string]int64, len(normalizedRows))
	for i := range normalizedRows {
		normalizedRows[i].ChannelId = normalizedChannelID
		normalizedRows[i].SortOrder = int64(i + 1)
		if normalizedRows[i].TestedAt == 0 {
			normalizedRows[i].TestedAt = now
		}
		if normalizedRows[i].Round <= 0 {
			roundByModel[normalizedRows[i].Model]++
			normalizedRows[i].Round = roundByModel[normalizedRows[i].Model]
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelTest{}).Error; err != nil {
			return err
		}
		if len(normalizedRows) == 0 {
			return nil
		}
		return tx.Create(&normalizedRows).Error
	})
}

func AppendChannelTestsForModelsWithDB(db *gorm.DB, channelID string, modelIDs []string, rows []ChannelTest) ([]ChannelTest, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelTest{}, nil
	}
	targetModels := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	normalizedRows := NormalizeChannelTestRows(rows)
	now := helper.GetTimestamp()
	for i := range normalizedRows {
		normalizedRows[i].ChannelId = normalizedChannelID
		if normalizedRows[i].SortOrder == 0 {
			normalizedRows[i].SortOrder = int64(i + 1)
		}
		if normalizedRows[i].TestedAt == 0 {
			normalizedRows[i].TestedAt = now
		}
		normalizedRows[i].Round = 0
		if normalizedRows[i].Model != "" {
			targetModels = append(targetModels, normalizedRows[i].Model)
		}
	}
	targetModels = NormalizeChannelModelIDsPreserveOrder(targetModels)
	if len(normalizedRows) == 0 {
		return []ChannelTest{}, nil
	}
	inserted := make([]ChannelTest, 0, len(normalizedRows))
	err := db.Transaction(func(tx *gorm.DB) error {
		roundsByModel, err := loadMaxChannelTestRoundsByModelsWithDB(tx, normalizedChannelID, targetModels)
		if err != nil {
			return err
		}
		nextRoundByModel := make(map[string]int64, len(roundsByModel))
		for i := range normalizedRows {
			modelID := normalizedRows[i].Model
			if modelID == "" {
				continue
			}
			if _, ok := nextRoundByModel[modelID]; !ok {
				nextRoundByModel[modelID] = roundsByModel[modelID] + 1
			}
			normalizedRows[i].Round = nextRoundByModel[modelID]
		}
		if err := tx.Create(&normalizedRows).Error; err != nil {
			return err
		}
		inserted = append(inserted, normalizedRows...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return NormalizeChannelTestRows(inserted), nil
}

func DeleteChannelTestsByChannelIDWithDB(db *gorm.DB, channelID string) error {
	return DeleteChannelTestsByChannelIDsWithDB(db, []string{channelID})
}

func DeleteChannelTestsByChannelIDsWithDB(db *gorm.DB, channelIDs []string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return nil
	}
	return db.Where("channel_id IN ?", normalizedIDs).Delete(&ChannelTest{}).Error
}

func ListLatestChannelTestsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelTest, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelTest{}, nil
	}
	rowsByChannelID, err := loadChannelTestRowsByChannelIDs(db, []string{normalizedChannelID})
	if err != nil {
		return nil, err
	}
	return NormalizeChannelTestRows(rowsByChannelID[normalizedChannelID]), nil
}

func loadChannelTestRowsByChannelIDs(db *gorm.DB, channelIDs []string) (map[string][]ChannelTest, error) {
	rowsByChannelID := make(map[string][]ChannelTest)
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return rowsByChannelID, nil
	}
	rows := make([]ChannelTest, 0)
	if err := db.Where("channel_id IN ?", normalizedIDs).
		Order("channel_id asc, model asc, round desc, tested_at desc, sort_order asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	latestByKey := make(map[string]ChannelTest, len(rows))
	for _, row := range NormalizeChannelTestRows(rows) {
		key := row.ChannelId + "::" + row.Model
		if existing, ok := latestByKey[key]; ok {
			if existing.Round > row.Round || (existing.Round == row.Round && existing.TestedAt >= row.TestedAt) {
				continue
			}
		}
		latestByKey[key] = row
	}
	latestRows := make([]ChannelTest, 0, len(latestByKey))
	for _, row := range latestByKey {
		latestRows = append(latestRows, row)
	}
	for _, row := range NormalizeChannelTestRows(latestRows) {
		rowsByChannelID[row.ChannelId] = append(rowsByChannelID[row.ChannelId], row)
	}
	return rowsByChannelID, nil
}

func loadMaxChannelTestRoundsByModelsWithDB(db *gorm.DB, channelID string, modelIDs []string) (map[string]int64, error) {
	result := make(map[string]int64)
	if db == nil {
		return result, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModels := NormalizeChannelModelIDsPreserveOrder(modelIDs)
	if normalizedChannelID == "" || len(normalizedModels) == 0 {
		return result, nil
	}
	type record struct {
		Model string `gorm:"column:model"`
		Round int64  `gorm:"column:max_round"`
	}
	rows := make([]record, 0, len(normalizedModels))
	if err := db.Model(&ChannelTest{}).
		Select("model, COALESCE(MAX(round), 0) AS max_round").
		Where("channel_id = ? AND model IN ?", normalizedChannelID, normalizedModels).
		Group("model").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[strings.TrimSpace(row.Model)] = row.Round
	}
	return result, nil
}

func CalcChannelTestsLastTestedAt(rows []ChannelTest) int64 {
	var testedAt int64
	for _, row := range rows {
		if row.TestedAt > testedAt {
			testedAt = row.TestedAt
		}
	}
	return testedAt
}
