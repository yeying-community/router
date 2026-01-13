package log

import (
	"context"
	"fmt"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindLogRepository(model.LogRepository{
		RecordLog:               RecordLog,
		RecordTopupLog:          RecordTopupLog,
		RecordConsumeLog:        RecordConsumeLog,
		RecordTestLog:           RecordTestLog,
		GetAllLogs:              GetAll,
		GetUserLogs:             GetUser,
		SearchAllLogs:           SearchAll,
		SearchUserLogs:          SearchUser,
		SumUsedQuota:            SumUsedQuota,
		SumUsedToken:            SumUsedToken,
		DeleteOldLog:            DeleteOld,
		SearchLogsByDayAndModel: SearchLogsByDayAndModel,
	})
}

func recordLogHelper(ctx context.Context, log *model.Log) {
	requestId := helper.GetRequestID(ctx)
	log.RequestId = requestId
	err := model.LOG_DB.Create(log).Error
	if err != nil {
		logger.Error(ctx, "failed to record log: "+err.Error())
		return
	}
	logger.Infof(ctx, "record log: %+v", log)
}

func RecordLog(ctx context.Context, userId int, logType int, content string) {
	if logType == model.LogTypeConsume && !config.LogConsumeEnabled {
		return
	}
	log := &model.Log{
		UserId:    userId,
		Username:  model.GetUsernameById(userId),
		CreatedAt: helper.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	recordLogHelper(ctx, log)
}

func RecordTopupLog(ctx context.Context, userId int, content string, quota int) {
	log := &model.Log{
		UserId:    userId,
		Username:  model.GetUsernameById(userId),
		CreatedAt: helper.GetTimestamp(),
		Type:      model.LogTypeTopup,
		Content:   content,
		Quota:     quota,
	}
	recordLogHelper(ctx, log)
}

func RecordConsumeLog(ctx context.Context, log *model.Log) {
	if !config.LogConsumeEnabled {
		return
	}
	log.Username = model.GetUsernameById(log.UserId)
	log.CreatedAt = helper.GetTimestamp()
	log.Type = model.LogTypeConsume
	recordLogHelper(ctx, log)
}

func RecordTestLog(ctx context.Context, log *model.Log) {
	log.CreatedAt = helper.GetTimestamp()
	log.Type = model.LogTypeTest
	recordLogHelper(ctx, log)
}

func GetAll(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int) ([]*model.Log, error) {
	var tx = model.LOG_DB
	if logType != model.LogTypeUnknown {
		tx = tx.Where("type = ?", logType)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	var logs []*model.Log
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, err
}

func GetUser(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*model.Log, error) {
	var tx = model.LOG_DB
	if logType == model.LogTypeUnknown {
		tx = tx.Where("user_id = ?", userId)
	} else {
		tx = tx.Where("user_id = ? and type = ?", userId, logType)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	var logs []*model.Log
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Omit("id").Find(&logs).Error
	return logs, err
}

func SearchAll(keyword string) ([]*model.Log, error) {
	var logs []*model.Log
	err := model.LOG_DB.Where("type = ? or content LIKE ?", keyword, keyword+"%").Order("id desc").Limit(config.MaxRecentItems).Find(&logs).Error
	return logs, err
}

func SearchUser(userId int, keyword string) ([]*model.Log, error) {
	var logs []*model.Log
	err := model.LOG_DB.Where("user_id = ? and type = ?", userId, keyword).Order("id desc").Limit(config.MaxRecentItems).Omit("id").Find(&logs).Error
	return logs, err
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int) int64 {
	ifnull := "ifnull"
	if common.UsingPostgreSQL {
		ifnull = "COALESCE"
	}
	tx := model.LOG_DB.Table("logs").Select(fmt.Sprintf("%s(sum(quota),0)", ifnull))
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	var quota int64
	tx.Where("type = ?", model.LogTypeConsume).Scan(&quota)
	return quota
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) int {
	ifnull := "ifnull"
	if common.UsingPostgreSQL {
		ifnull = "COALESCE"
	}
	tx := model.LOG_DB.Table("logs").Select(fmt.Sprintf("%s(sum(prompt_tokens),0) + %s(sum(completion_tokens),0)", ifnull, ifnull))
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	var token int
	tx.Where("type = ?", model.LogTypeConsume).Scan(&token)
	return token
}

func DeleteOld(targetTimestamp int64) (int64, error) {
	result := model.LOG_DB.Where("created_at < ?", targetTimestamp).Delete(&model.Log{})
	return result.RowsAffected, result.Error
}

func SearchLogsByDayAndModel(userId, start, end int) ([]*model.LogStatistic, error) {
	groupSelect := "DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d') as day"

	if common.UsingPostgreSQL {
		groupSelect = "TO_CHAR(date_trunc('day', to_timestamp(created_at)), 'YYYY-MM-DD') as day"
	}

	if common.UsingSQLite {
		groupSelect = "strftime('%Y-%m-%d', datetime(created_at, 'unixepoch')) as day"
	}

	var stats []*model.LogStatistic
	err := model.LOG_DB.Raw(`
		SELECT `+groupSelect+`,
		model_name, count(1) as request_count,
		sum(quota) as quota,
		sum(prompt_tokens) as prompt_tokens,
		sum(completion_tokens) as completion_tokens
		FROM logs
		WHERE type=2
		AND user_id= ?
		AND created_at BETWEEN ? AND ?
		GROUP BY day, model_name
		ORDER BY day, model_name
	`, userId, start, end).Scan(&stats).Error

	return stats, err
}
