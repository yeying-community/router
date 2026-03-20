package model

import "context"

type LogRepository struct {
	RecordLog                      func(ctx context.Context, userId string, logType int, content string)
	RecordTopupLog                 func(ctx context.Context, userId string, content string, quota int)
	RecordConsumeLog               func(ctx context.Context, log *Log)
	RecordTestLog                  func(ctx context.Context, log *Log)
	GetAllLogs                     func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, groupID string, startIdx int, num int, channel string) ([]*Log, error)
	GetUserLogs                    func(userId string, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*Log, error)
	GetLogByID                     func(logID string) (*Log, error)
	GetUserLogByID                 func(userId string, logID string) (*Log, error)
	SearchAllLogs                  func(keyword string) ([]*Log, error)
	SearchUserLogs                 func(userId string, keyword string) ([]*Log, error)
	SumUsedQuota                   func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel string) int64
	SumUsedQuotaByUserIdWithModels func(logType int, userId string, startTimestamp int64, endTimestamp int64, models []string) (int64, error)
	SumUsedToken                   func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) int
	DeleteOldLog                   func(targetTimestamp int64) (int64, error)
	SearchLogsByPeriodAndModel     func(userId string, start int, end int, granularity string, models []string) ([]*LogStatistic, error)
	SearchLogModelsByPeriod        func(userId string, start int, end int) ([]string, error)
}

var logRepo LogRepository

func BindLogRepository(repo LogRepository) {
	logRepo = repo
}

func mustLogRepo() LogRepository {
	if logRepo.GetAllLogs == nil {
		panic("log repository not initialized")
	}
	return logRepo
}
