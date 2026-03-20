package log

import (
	"github.com/yeying-community/router/internal/admin/model"
	logrepo "github.com/yeying-community/router/internal/admin/repository/log"
)

func GetAll(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, groupID string, startIdx int, num int, channel string) ([]*model.Log, error) {
	return logrepo.GetAll(logType, startTimestamp, endTimestamp, modelName, username, tokenName, groupID, startIdx, num, channel)
}

func GetUser(userId string, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*model.Log, error) {
	return logrepo.GetUser(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num)
}

func GetByID(logID string) (*model.Log, error) {
	return logrepo.GetByID(logID)
}

func GetUserByID(userId string, logID string) (*model.Log, error) {
	return logrepo.GetUserByID(userId, logID)
}

func SearchAll(keyword string) ([]*model.Log, error) {
	return logrepo.SearchAll(keyword)
}

func SearchUser(userId string, keyword string) ([]*model.Log, error) {
	return logrepo.SearchUser(userId, keyword)
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel string) int64 {
	return logrepo.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel)
}

func SumUsedQuotaByUserId(logType int, userId string, startTimestamp int64, endTimestamp int64) (int64, error) {
	return logrepo.SumUsedQuotaByUserId(logType, userId, startTimestamp, endTimestamp)
}

func SumUsedQuotaByUserIdWithModels(logType int, userId string, startTimestamp int64, endTimestamp int64, models []string) (int64, error) {
	return logrepo.SumUsedQuotaByUserIdWithModels(logType, userId, startTimestamp, endTimestamp, models)
}

func MinLogTimestampByUserId(userId string, logTypes []int) (int64, error) {
	return logrepo.MinLogTimestampByUserId(userId, logTypes)
}

func DeleteOld(targetTimestamp int64) (int64, error) {
	return logrepo.DeleteOld(targetTimestamp)
}
