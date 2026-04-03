package model

import "context"

const EventLogsTableName = "event_logs"

type Log struct {
	Id                    string  `json:"id" gorm:"type:char(36);primaryKey"`
	UserId                string  `json:"user_id" gorm:"type:char(36);index"`
	CreatedAt             int64   `json:"created_at" gorm:"bigint;index:idx_created_at_type"`
	Type                  int     `json:"type" gorm:"index:idx_created_at_type"`
	Content               string  `json:"content"`
	Username              string  `json:"username" gorm:"index:index_username_model_name,priority:2;default:''"`
	TokenName             string  `json:"token_name" gorm:"index;default:''"`
	ModelName             string  `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	GroupId               string  `json:"group_id" gorm:"type:varchar(64);index"`
	GroupName             string  `json:"group_name,omitempty" gorm:"-"`
	Quota                 int     `json:"quota" gorm:"default:0"`
	BillingSource         string  `json:"billing_source" gorm:"type:varchar(32);index;default:''"`
	UserDailyQuota        int     `json:"user_daily_quota" gorm:"column:user_daily_quota;default:0"`
	UserEmergencyQuota    int     `json:"user_emergency_quota" gorm:"column:user_emergency_quota;default:0"`
	BillingPriceUnit      string  `json:"billing_price_unit" gorm:"type:varchar(64);default:''"`
	BillingCurrency       string  `json:"billing_currency" gorm:"type:varchar(16);default:''"`
	BillingGroupRatio     float64 `json:"billing_group_ratio" gorm:"type:double precision;default:0"`
	BillingYYCRate        float64 `json:"billing_yyc_rate" gorm:"type:double precision;default:0"`
	BillingInputQuantity  float64 `json:"billing_input_quantity" gorm:"type:double precision;default:0"`
	BillingOutputQuantity float64 `json:"billing_output_quantity" gorm:"type:double precision;default:0"`
	BillingInputAmount    float64 `json:"billing_input_amount" gorm:"type:double precision;default:0"`
	BillingOutputAmount   float64 `json:"billing_output_amount" gorm:"type:double precision;default:0"`
	BillingAmount         float64 `json:"billing_amount" gorm:"type:double precision;default:0"`
	BillingYYCAmount      int64   `json:"billing_yyc_amount" gorm:"bigint;default:0"`
	PromptTokens          int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens      int     `json:"completion_tokens" gorm:"default:0"`
	ChannelId             string  `json:"channel" gorm:"type:varchar(64);index"`
	ChannelName           string  `json:"channel_name,omitempty" gorm:"-"`
	TraceID               string  `json:"trace_id" gorm:"column:trace_id;default:''"`
	ElapsedTime           int64   `json:"elapsed_time" gorm:"default:0"`
	IsStream              bool    `json:"is_stream" gorm:"default:false"`
	SystemPromptReset     bool    `json:"system_prompt_reset" gorm:"default:false"`
}

func (Log) TableName() string {
	return EventLogsTableName
}

const (
	LogTypeAll = iota
	LogTypeTopup
	LogTypeConsume
	LogTypeManage
	LogTypeSystem
	LogTypeTest
)

const (
	LogBillingSourceBalance = "balance"
	LogBillingSourcePackage = "package"
)

func ResolveConsumeLogBillingSource(chargeUserBalance bool) string {
	if chargeUserBalance {
		return LogBillingSourceBalance
	}
	return LogBillingSourcePackage
}

func RecordLog(ctx context.Context, userId string, logType int, content string) {
	mustLogRepo().RecordLog(ctx, userId, logType, content)
}

func RecordTopupLog(ctx context.Context, userId string, content string, quota int) {
	mustLogRepo().RecordTopupLog(ctx, userId, content, quota)
}

func RecordConsumeLog(ctx context.Context, log *Log) {
	mustLogRepo().RecordConsumeLog(ctx, log)
}

func RecordTestLog(ctx context.Context, log *Log) {
	mustLogRepo().RecordTestLog(ctx, log)
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, groupID string, startIdx int, num int, channel string) ([]*Log, error) {
	return mustLogRepo().GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, groupID, startIdx, num, channel)
}

func GetUserLogs(userId string, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int) ([]*Log, error) {
	return mustLogRepo().GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num)
}

func GetLogByID(logID string) (*Log, error) {
	return mustLogRepo().GetLogByID(logID)
}

func GetUserLogByID(userId string, logID string) (*Log, error) {
	return mustLogRepo().GetUserLogByID(userId, logID)
}

func SearchAllLogs(keyword string) ([]*Log, error) {
	return mustLogRepo().SearchAllLogs(keyword)
}

func SearchUserLogs(userId string, keyword string) ([]*Log, error) {
	return mustLogRepo().SearchUserLogs(userId, keyword)
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel string) int64 {
	return mustLogRepo().SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel)
}

func SumUsedQuotaByUserIdWithModels(logType int, userId string, startTimestamp int64, endTimestamp int64, models []string) (int64, error) {
	return mustLogRepo().SumUsedQuotaByUserIdWithModels(logType, userId, startTimestamp, endTimestamp, models)
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

func SearchLogsByPeriodAndModel(userId string, start, end int, granularity string, models []string) ([]*LogStatistic, error) {
	return mustLogRepo().SearchLogsByPeriodAndModel(userId, start, end, granularity, models)
}

func SearchLogModelsByPeriod(userId string, start, end int) ([]string, error) {
	return mustLogRepo().SearchLogModelsByPeriod(userId, start, end)
}
