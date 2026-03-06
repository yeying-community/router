package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelCapabilityResultsTableName = "channel_capability_results"

	ChannelCapabilityStatusSupported   = "supported"
	ChannelCapabilityStatusUnsupported = "unsupported"
	ChannelCapabilityStatusSkipped     = "skipped"
)

type ChannelCapabilityResult struct {
	ChannelId  string `json:"channel_id,omitempty" gorm:"primaryKey;type:char(36);index"`
	Capability string `json:"capability" gorm:"primaryKey;type:varchar(128)"`
	Label      string `json:"label" gorm:"type:varchar(255)"`
	Endpoint   string `json:"endpoint" gorm:"type:varchar(255)"`
	Model      string `json:"model,omitempty" gorm:"type:varchar(255)"`
	Status     string `json:"status" gorm:"type:varchar(32);index"`
	Supported  bool   `json:"supported" gorm:"not null;default:false"`
	Message    string `json:"message,omitempty" gorm:"type:text"`
	LatencyMs  int64  `json:"latency_ms,omitempty" gorm:"bigint"`
	SortOrder  int64  `json:"sort_order,omitempty" gorm:"bigint;default:0"`
	TestedAt   int64  `json:"tested_at,omitempty" gorm:"bigint;index"`
}

func (ChannelCapabilityResult) TableName() string {
	return ChannelCapabilityResultsTableName
}

func NormalizeChannelCapabilityStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case ChannelCapabilityStatusSupported:
		return ChannelCapabilityStatusSupported
	case ChannelCapabilityStatusSkipped:
		return ChannelCapabilityStatusSkipped
	default:
		return ChannelCapabilityStatusUnsupported
	}
}

func normalizeChannelCapabilityResultName(capability string) string {
	normalized := strings.TrimSpace(strings.ToLower(capability))
	switch {
	case normalized == "":
		return ""
	case strings.HasPrefix(normalized, "responses:"):
		return "responses"
	default:
		return normalized
	}
}

func compareChannelCapabilityResultPriority(left ChannelCapabilityResult, right ChannelCapabilityResult) int {
	leftStatus := NormalizeChannelCapabilityStatus(left.Status)
	rightStatus := NormalizeChannelCapabilityStatus(right.Status)
	leftRank := 2
	rightRank := 2
	if left.Supported || leftStatus == ChannelCapabilityStatusSupported {
		leftRank = 0
	} else if leftStatus == ChannelCapabilityStatusSkipped {
		leftRank = 1
	}
	if right.Supported || rightStatus == ChannelCapabilityStatusSupported {
		rightRank = 0
	} else if rightStatus == ChannelCapabilityStatusSkipped {
		rightRank = 1
	}
	if leftRank != rightRank {
		if leftRank < rightRank {
			return -1
		}
		return 1
	}
	if left.TestedAt != right.TestedAt {
		if left.TestedAt > right.TestedAt {
			return -1
		}
		return 1
	}
	if left.SortOrder != right.SortOrder {
		if left.SortOrder < right.SortOrder {
			return -1
		}
		return 1
	}
	return 0
}

func NormalizeChannelCapabilityResultRows(rows []ChannelCapabilityResult) []ChannelCapabilityResult {
	if len(rows) == 0 {
		return []ChannelCapabilityResult{}
	}
	normalized := make([]ChannelCapabilityResult, 0, len(rows))
	indexByKey := make(map[string]int, len(rows))
	for idx, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		capability := normalizeChannelCapabilityResultName(row.Capability)
		if channelID == "" || capability == "" {
			continue
		}
		sortOrder := row.SortOrder
		if sortOrder == 0 && idx > 0 {
			sortOrder = int64(idx)
		}
		candidate := ChannelCapabilityResult{
			ChannelId:  channelID,
			Capability: capability,
			Label:      strings.TrimSpace(row.Label),
			Endpoint:   strings.TrimSpace(row.Endpoint),
			Model:      strings.TrimSpace(row.Model),
			Status:     NormalizeChannelCapabilityStatus(row.Status),
			Supported:  row.Supported && NormalizeChannelCapabilityStatus(row.Status) == ChannelCapabilityStatusSupported,
			Message:    strings.TrimSpace(row.Message),
			LatencyMs:  row.LatencyMs,
			SortOrder:  sortOrder,
			TestedAt:   row.TestedAt,
		}
		if candidate.Capability == "responses" {
			candidate.Label = "Responses"
			candidate.Endpoint = "/v1/responses"
		}
		key := channelID + "::" + capability
		if existingIdx, ok := indexByKey[key]; ok {
			if compareChannelCapabilityResultPriority(candidate, normalized[existingIdx]) < 0 {
				normalized[existingIdx] = candidate
			}
			continue
		}
		indexByKey[key] = len(normalized)
		normalized = append(normalized, candidate)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder != normalized[j].SortOrder {
			return normalized[i].SortOrder < normalized[j].SortOrder
		}
		return normalized[i].Capability < normalized[j].Capability
	})
	return normalized
}

func HydrateChannelWithCapabilityResults(db *gorm.DB, channel *Channel) error {
	if channel == nil {
		return nil
	}
	return HydrateChannelsWithCapabilityResults(db, []*Channel{channel})
}

func HydrateChannelsWithCapabilityResults(db *gorm.DB, channels []*Channel) error {
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
			channel.SetCapabilityResults(nil)
			channel.CapabilityLastTestedAt = 0
			continue
		}
		channelIDs = append(channelIDs, channel.Id)
		normalizedChannels = append(normalizedChannels, channel)
	}
	if len(normalizedChannels) == 0 {
		return nil
	}
	rowsByChannelID, err := loadChannelCapabilityResultRowsByChannelIDs(db, channelIDs)
	if err != nil {
		return err
	}
	for _, channel := range normalizedChannels {
		rows := rowsByChannelID[channel.Id]
		channel.SetCapabilityResults(rows)
		channel.CapabilityLastTestedAt = calcChannelCapabilityLastTestedAt(rows)
	}
	return nil
}

func ReplaceChannelCapabilityResultsWithDB(db *gorm.DB, channelID string, rows []ChannelCapabilityResult) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedRows := NormalizeChannelCapabilityResultRows(rows)
	now := helper.GetTimestamp()
	for i := range normalizedRows {
		normalizedRows[i].ChannelId = normalizedChannelID
		if normalizedRows[i].TestedAt == 0 {
			normalizedRows[i].TestedAt = now
		}
		normalizedRows[i].SortOrder = int64(i)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelCapabilityResult{}).Error; err != nil {
			return err
		}
		if len(normalizedRows) == 0 {
			return nil
		}
		return tx.Create(&normalizedRows).Error
	})
}

func DeleteChannelCapabilityResultsByChannelIDWithDB(db *gorm.DB, channelID string) error {
	return DeleteChannelCapabilityResultsByChannelIDsWithDB(db, []string{channelID})
}

func DeleteChannelCapabilityResultsByChannelIDsWithDB(db *gorm.DB, channelIDs []string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return nil
	}
	return db.Where("channel_id IN ?", normalizedIDs).Delete(&ChannelCapabilityResult{}).Error
}

func loadChannelCapabilityResultRowsByChannelIDs(db *gorm.DB, channelIDs []string) (map[string][]ChannelCapabilityResult, error) {
	rowsByChannelID := make(map[string][]ChannelCapabilityResult)
	normalizedIDs := normalizeTrimmedValuesPreserveOrder(channelIDs)
	if len(normalizedIDs) == 0 {
		return rowsByChannelID, nil
	}
	rows := make([]ChannelCapabilityResult, 0)
	if err := db.Where("channel_id IN ?", normalizedIDs).
		Order("channel_id asc, sort_order asc, capability asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range NormalizeChannelCapabilityResultRows(rows) {
		rowsByChannelID[row.ChannelId] = append(rowsByChannelID[row.ChannelId], row)
	}
	return rowsByChannelID, nil
}

func calcChannelCapabilityLastTestedAt(rows []ChannelCapabilityResult) int64 {
	var testedAt int64
	for _, row := range rows {
		if row.TestedAt > testedAt {
			testedAt = row.TestedAt
		}
	}
	return testedAt
}
