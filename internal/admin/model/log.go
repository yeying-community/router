package model

import "context"

type Log struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	ChannelId         int    `json:"channel" gorm:"index"`
	RequestId         string `json:"request_id" gorm:"default:''"`
	ElapsedTime       int64  `json:"elapsed_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream" gorm:"default:false"`
	SystemPromptReset bool   `json:"system_prompt_reset" gorm:"default:false"`
}

const (
	LogTypeUnknown = iota
	LogTypeTopup
	LogTypeConsume
	LogTypeManage
	LogTypeSystem
	LogTypeTest
)

func RecordLog(ctx context.Context, userId int, logType int, content string) {
	mustLogRepo().RecordLog(ctx, userId, logType, content)
}

func RecordTopupLog(ctx context.Context, userId int, content string, quota int) {
	mustLogRepo().RecordTopupLog(ctx, userId, content, quota)
}

func RecordConsumeLog(ctx context.Context, log *Log) {
	mustLogRepo().RecordConsumeLog(ctx, log)
}

func RecordTestLog(ctx context.Context, log *Log) {
	mustLogRepo().RecordTestLog(ctx, log)
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int) ([]*Log, error) {
	return mustLogRepo().GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, startIdx, num, channel)
}

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*Log, error) {
	return mustLogRepo().GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num)
}

func SearchAllLogs(keyword string) ([]*Log, error) {
	return mustLogRepo().SearchAllLogs(keyword)
}

func SearchUserLogs(userId int, keyword string) ([]*Log, error) {
	return mustLogRepo().SearchUserLogs(userId, keyword)
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int) int64 {
	return mustLogRepo().SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel)
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) int {
	return mustLogRepo().SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
}

func DeleteOldLog(targetTimestamp int64) (int64, error) {
	return mustLogRepo().DeleteOldLog(targetTimestamp)
}

type LogStatistic struct {
	Day              string `gorm:"column:day"`
	ModelName        string `gorm:"column:model_name"`
	RequestCount     int    `gorm:"column:request_count"`
	Quota            int    `gorm:"column:quota"`
	PromptTokens     int    `gorm:"column:prompt_tokens"`
	CompletionTokens int    `gorm:"column:completion_tokens"`
}

func SearchLogsByPeriodAndModel(userId, start, end int, granularity string, models []string) ([]*LogStatistic, error) {
	return mustLogRepo().SearchLogsByPeriodAndModel(userId, start, end, granularity, models)
}

func SearchLogModelsByPeriod(userId, start, end int) ([]string, error) {
	return mustLogRepo().SearchLogModelsByPeriod(userId, start, end)
}
