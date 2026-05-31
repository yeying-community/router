package model

import (
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	ChannelModelEndpointTestResultsTableName = "channel_model_endpoint_test_results"

	ChannelModelEndpointTestStatusSuccess = "success"
	ChannelModelEndpointTestStatusFailed  = "failed"
)

type ChannelModelEndpointTestResult struct {
	ChannelId      string `json:"channel_id" gorm:"primaryKey;type:varchar(64);index"`
	Model          string `json:"model" gorm:"primaryKey;type:varchar(255)"`
	Endpoint       string `json:"endpoint" gorm:"primaryKey;type:varchar(255)"`
	UpstreamModel  string `json:"upstream_model" gorm:"type:varchar(255);default:'';index"`
	LastTestTaskId string `json:"last_test_task_id" gorm:"type:char(36);default:''"`
	LastTestedAt   int64  `json:"last_tested_at" gorm:"bigint;index"`
	LastTestStatus string `json:"last_test_status" gorm:"type:varchar(32);default:'failed'"`
	LastSupported  bool   `json:"last_supported" gorm:"not null;default:false;index"`
	LastError      string `json:"last_error" gorm:"type:text"`
	LatencyMs      int64  `json:"latency_ms" gorm:"bigint"`
	IsStream       bool   `json:"is_stream" gorm:"not null;default:false"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

func (ChannelModelEndpointTestResult) TableName() string {
	return ChannelModelEndpointTestResultsTableName
}

func UpsertChannelModelEndpointTestResultsWithDB(db *gorm.DB, channelID string, taskID string, results []ChannelTest) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	normalizedResults := NormalizeChannelTestRows(results)
	if len(normalizedResults) == 0 {
		return nil
	}
	now := helper.GetTimestamp()
	return db.Transaction(func(tx *gorm.DB) error {
		for _, result := range normalizedResults {
			modelName := strings.TrimSpace(result.Model)
			endpoint := NormalizeRequestedChannelModelEndpoint(result.Endpoint)
			if modelName == "" || endpoint == "" {
				continue
			}
			status := ChannelModelEndpointTestStatusFailed
			lastError := strings.TrimSpace(result.Message)
			if result.Supported && NormalizeChannelTestStatus(result.Status) == ChannelTestStatusSupported {
				status = ChannelModelEndpointTestStatusSuccess
				lastError = ""
			}
			row := ChannelModelEndpointTestResult{
				ChannelId:      normalizedChannelID,
				Model:          modelName,
				Endpoint:       endpoint,
				UpstreamModel:  strings.TrimSpace(result.UpstreamModel),
				LastTestTaskId: strings.TrimSpace(taskID),
				LastTestedAt:   result.TestedAt,
				LastTestStatus: status,
				LastSupported:  status == ChannelModelEndpointTestStatusSuccess,
				LastError:      lastError,
				LatencyMs:      result.LatencyMs,
				IsStream:       result.IsStream,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if row.LastTestedAt == 0 {
				row.LastTestedAt = now
			}
			existing := ChannelModelEndpointTestResult{}
			err := tx.Where("channel_id = ? AND model = ? AND endpoint = ?", row.ChannelId, row.Model, row.Endpoint).
				First(&existing).Error
			if err == nil && existing.LastTestedAt > row.LastTestedAt {
				continue
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err := tx.Where("channel_id = ? AND model = ? AND endpoint = ?", row.ChannelId, row.Model, row.Endpoint).
				Assign(map[string]any{
					"upstream_model":    row.UpstreamModel,
					"last_test_task_id": row.LastTestTaskId,
					"last_tested_at":    row.LastTestedAt,
					"last_test_status":  row.LastTestStatus,
					"last_supported":    row.LastSupported,
					"last_error":        row.LastError,
					"latency_ms":        row.LatencyMs,
					"is_stream":         row.IsStream,
					"updated_at":        row.UpdatedAt,
				}).
				FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func HasSuccessfulExactChannelEndpointTestResultWithDB(db *gorm.DB, channelID string, modelName string, endpoint string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModel := strings.TrimSpace(modelName)
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedChannelID == "" || normalizedModel == "" || normalizedEndpoint == "" {
		return false, nil
	}
	count := int64(0)
	err := db.Model(&ChannelModelEndpointTestResult{}).
		Where("channel_id = ? AND model = ? AND endpoint = ? AND last_supported = ? AND last_test_status = ?", normalizedChannelID, normalizedModel, normalizedEndpoint, true, ChannelModelEndpointTestStatusSuccess).
		Count(&count).Error
	return count > 0, err
}

func ListChannelModelEndpointTestResultsByChannelIDWithDB(db *gorm.DB, channelID string) ([]ChannelModelEndpointTestResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelModelEndpointTestResult{}, nil
	}
	rows := make([]ChannelModelEndpointTestResult, 0)
	if err := db.Where("channel_id = ?", normalizedChannelID).
		Order("last_tested_at desc, model asc, endpoint asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
