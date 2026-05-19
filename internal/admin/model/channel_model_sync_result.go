package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelModelSyncResultsTableName = "channel_model_sync_results"
	ChannelModelSyncSourceRefresh    = "refresh"
)

type ChannelModelSyncResult struct {
	ChannelId      string `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model          string `json:"model" gorm:"primaryKey;type:varchar(255)"`
	UpstreamModel  string `json:"upstream_model" gorm:"type:varchar(255);default:'';index"`
	Returned       bool   `json:"returned" gorm:"column:returned;not null;default:false;index"`
	SyncSource     string `json:"sync_source" gorm:"type:varchar(32);default:'refresh'"`
	LastSyncTaskId string `json:"last_sync_task_id" gorm:"type:char(36);default:''"`
	LastSyncedAt   int64  `json:"last_synced_at" gorm:"bigint;index"`
	LastError      string `json:"last_error" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func (ChannelModelSyncResult) TableName() string {
	return ChannelModelSyncResultsTableName
}

func ReplaceChannelModelSyncResultsWithDB(db *gorm.DB, channelID string, existingRows []ChannelModel, fetchedRows []ChannelModel, taskID string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	now := helper.GetTimestamp()
	type syncCandidate struct {
		Model         string
		UpstreamModel string
		Returned      bool
	}
	candidateByKey := make(map[string]syncCandidate)
	order := make([]string, 0, len(existingRows)+len(fetchedRows))
	appendCandidate := func(modelName string, upstreamModel string, returned bool) {
		modelName = strings.TrimSpace(modelName)
		upstreamModel = strings.TrimSpace(upstreamModel)
		if modelName == "" && upstreamModel != "" {
			modelName = upstreamModel
		}
		if upstreamModel == "" {
			upstreamModel = modelName
		}
		if modelName == "" {
			return
		}
		key := modelName + "\x00" + upstreamModel
		current, exists := candidateByKey[key]
		if !exists {
			order = append(order, key)
		}
		candidateByKey[key] = syncCandidate{
			Model:         modelName,
			UpstreamModel: upstreamModel,
			Returned:      returned || current.Returned,
		}
	}
	for _, row := range NormalizeChannelModelsPreserveOrder(existingRows) {
		appendCandidate(row.Model, row.UpstreamModel, false)
	}
	for _, row := range NormalizeChannelModelsPreserveOrder(fetchedRows) {
		appendCandidate(row.Model, row.UpstreamModel, true)
	}
	rows := make([]ChannelModelSyncResult, 0, len(order))
	for _, key := range order {
		item := candidateByKey[key]
		rows = append(rows, ChannelModelSyncResult{
			ChannelId:      normalizedChannelID,
			Model:          item.Model,
			UpstreamModel:  item.UpstreamModel,
			Returned:       item.Returned,
			SyncSource:     ChannelModelSyncSourceRefresh,
			LastSyncTaskId: strings.TrimSpace(taskID),
			LastSyncedAt:   now,
			LastError:      "",
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockChannelRowForUpdateWithDB(tx, normalizedChannelID); err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModelSyncResult{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func HasReturnedChannelModelSyncResultWithDB(db *gorm.DB, channelID string, candidates ...string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	modelCandidates := NormalizeProviderLookupCandidates(candidates...)
	if normalizedChannelID == "" || len(modelCandidates) == 0 {
		return false, nil
	}
	count := int64(0)
	err := db.Model(&ChannelModelSyncResult{}).
		Where("channel_id = ? AND returned = ? AND (model IN ? OR upstream_model IN ?)", normalizedChannelID, true, modelCandidates, modelCandidates).
		Count(&count).Error
	return count > 0, err
}

func ListChannelModelSyncResultsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelModelSyncResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModelSyncResult{}, nil
	}
	rows := make([]ChannelModelSyncResult, 0)
	if err := db.Where("channel_id = ?", normalizedChannelID).
		Order("last_synced_at desc, model asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
