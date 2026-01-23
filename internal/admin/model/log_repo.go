package model

import "context"

type LogRepository struct {
	RecordLog                  func(ctx context.Context, userId int, logType int, content string)
	RecordTopupLog             func(ctx context.Context, userId int, content string, quota int)
	RecordConsumeLog           func(ctx context.Context, log *Log)
	RecordTestLog              func(ctx context.Context, log *Log)
	GetAllLogs                 func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int) ([]*Log, error)
	GetUserLogs                func(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*Log, error)
	SearchAllLogs              func(keyword string) ([]*Log, error)
	SearchUserLogs             func(userId int, keyword string) ([]*Log, error)
	SumUsedQuota               func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int) int64
	SumUsedToken               func(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) int
	DeleteOldLog               func(targetTimestamp int64) (int64, error)
	SearchLogsByPeriodAndModel func(userId int, start int, end int, granularity string, models []string) ([]*LogStatistic, error)
	SearchLogModelsByPeriod    func(userId int, start int, end int) ([]string, error)
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
