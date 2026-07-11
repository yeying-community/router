package model

import "context"

const EventLogsTableName = "event_logs"

type Log struct {
	Id                               string  `json:"id" gorm:"type:char(36);primaryKey"`
	UserId                           string  `json:"user_id" gorm:"type:char(36);index"`
	CreatedAt                        int64   `json:"created_at" gorm:"bigint;index:idx_created_at_type"`
	Type                             int     `json:"type" gorm:"index:idx_created_at_type"`
	Content                          string  `json:"content"`
	Username                         string  `json:"username" gorm:"index:index_username_model_name,priority:2;default:''"`
	TokenName                        string  `json:"token_name" gorm:"index;default:''"`
	ModelName                        string  `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	GroupId                          string  `json:"group_id" gorm:"type:varchar(64);index"`
	GroupName                        string  `json:"group_name,omitempty" gorm:"-"`
	Quota                            int     `json:"quota" gorm:"default:0"`
	BillingSource                    string  `json:"billing_source" gorm:"type:varchar(32);index;default:''"`
	BillingSourceID                  string  `json:"billing_source_id" gorm:"type:char(36);index;default:''"`
	BillingSourceName                string  `json:"billing_source_name" gorm:"type:varchar(255);default:''"`
	BillingSourceDetail              string  `json:"billing_source_detail" gorm:"type:varchar(255);default:''"`
	UserDailyQuota                   int     `json:"user_daily_quota" gorm:"column:user_daily_quota;default:0"`
	UserEmergencyQuota               int     `json:"user_emergency_quota" gorm:"column:user_emergency_quota;default:0"`
	BillingPriceUnit                 string  `json:"billing_price_unit" gorm:"type:varchar(64);default:''"`
	BillingCurrency                  string  `json:"billing_currency" gorm:"type:varchar(16);default:''"`
	BillingPricingSource             string  `json:"billing_pricing_source" gorm:"type:varchar(64);default:''"`
	BillingUsageSource               string  `json:"billing_usage_source" gorm:"type:varchar(64);default:''"`
	BillingEstimateSource            string  `json:"billing_estimate_source" gorm:"type:varchar(64);default:''"`
	BillingEstimateEstimator         string  `json:"billing_estimate_estimator" gorm:"type:varchar(64);default:''"`
	BillingEstimatePrecision         string  `json:"billing_estimate_precision" gorm:"type:varchar(32);default:''"`
	BillingSettlementMode            string  `json:"billing_settlement_mode" gorm:"type:varchar(64);default:''"`
	BillingGroupRatio                float64 `json:"billing_group_ratio" gorm:"type:double precision;default:0"`
	BillingChargeRate                float64 `json:"billing_charge_rate" gorm:"type:double precision;default:0"`
	BillingInputQuantity             float64 `json:"billing_input_quantity" gorm:"type:double precision;default:0"`
	BillingOutputQuantity            float64 `json:"billing_output_quantity" gorm:"type:double precision;default:0"`
	BillingCacheReadQuantity         float64 `json:"billing_cache_read_quantity" gorm:"type:double precision;default:0"`
	BillingCacheWriteQuantity        float64 `json:"billing_cache_write_quantity" gorm:"type:double precision;default:0"`
	BillingInputAmount               float64 `json:"billing_input_amount" gorm:"type:double precision;default:0"`
	BillingOutputAmount              float64 `json:"billing_output_amount" gorm:"type:double precision;default:0"`
	BillingCacheReadAmount           float64 `json:"billing_cache_read_amount" gorm:"type:double precision;default:0"`
	BillingCacheWriteAmount          float64 `json:"billing_cache_write_amount" gorm:"type:double precision;default:0"`
	BillingAmount                    float64 `json:"billing_amount" gorm:"type:double precision;default:0"`
	BillingChargeAmount              int64   `json:"billing_charge_amount" gorm:"bigint;default:0"`
	BillingImageToolCalls            int     `json:"billing_image_tool_calls" gorm:"default:0"`
	BillingImageToolOutputTokens     int     `json:"billing_image_tool_output_tokens" gorm:"default:0"`
	BillingImageToolAmount           float64 `json:"billing_image_tool_amount" gorm:"type:double precision;default:0"`
	BillingImageToolChargeAmount     int64   `json:"billing_image_tool_charge_amount" gorm:"bigint;default:0"`
	BillingSettlementTruthMode       string  `json:"billing_settlement_truth_mode" gorm:"type:varchar(64);default:''"`
	BillingOfficialAnchorAmount      float64 `json:"billing_official_anchor_amount" gorm:"type:double precision;default:0"`
	BillingOfficialAnchorCurrency    string  `json:"billing_official_anchor_currency" gorm:"type:varchar(16);default:''"`
	BillingOfficialAnchorBaseAmount  float64 `json:"billing_official_anchor_base_amount" gorm:"type:double precision;default:0"`
	BillingProcurementCostBaseAmount float64 `json:"billing_procurement_cost_base_amount" gorm:"type:double precision;default:0"`
	BillingProcurementCostSource     string  `json:"billing_procurement_cost_source" gorm:"type:varchar(32);default:''"`
	BillingProcurementCostConfidence string  `json:"billing_procurement_cost_confidence" gorm:"type:varchar(64);default:''"`
	BillingSellBaseAmount            float64 `json:"billing_sell_base_amount" gorm:"type:double precision;default:0"`
	BillingGrossProfitBaseAmount     float64 `json:"billing_gross_profit_base_amount" gorm:"type:double precision;default:0"`
	BillingGrossMargin               float64 `json:"billing_gross_margin" gorm:"type:double precision;default:0"`
	BillingPricingRuleVersion        string  `json:"billing_pricing_rule_version" gorm:"type:varchar(64);default:''"`
	BillingCostRuleVersion           string  `json:"billing_cost_rule_version" gorm:"type:varchar(64);default:''"`
	EstimatedPromptTokens            int     `json:"estimated_prompt_tokens" gorm:"default:0"`
	EstimatedOutputTokens            int     `json:"estimated_output_tokens" gorm:"default:0"`
	EstimatedChargeAmount            int64   `json:"estimated_charge_amount" gorm:"bigint;default:0"`
	BillingPromptTokenDelta          int     `json:"billing_prompt_token_delta" gorm:"default:0"`
	BillingOutputTokenDelta          int     `json:"billing_output_token_delta" gorm:"default:0"`
	BillingChargeDeltaAmount         int64   `json:"billing_charge_delta_amount" gorm:"bigint;default:0"`
	PromptTokens                     int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens                 int     `json:"completion_tokens" gorm:"default:0"`
	ChannelId                        string  `json:"channel" gorm:"type:varchar(64);index"`
	ChannelName                      string  `json:"channel_name,omitempty" gorm:"-"`
	RequestModelName                 string  `json:"request_model_name" gorm:"type:varchar(191);index;default:''"`
	ActualModelName                  string  `json:"actual_model_name" gorm:"type:varchar(191);index;default:''"`
	UpstreamEndpoint                 string  `json:"upstream_endpoint" gorm:"type:varchar(191);index;default:''"`
	UpstreamProtocol                 string  `json:"upstream_protocol" gorm:"type:varchar(64);index;default:''"`
	FallbackCount                    int     `json:"fallback_count" gorm:"default:0"`
	FallbackAttempts                 string  `json:"fallback_attempts" gorm:"type:text"`
	RelayErrorType                   string  `json:"relay_error_type" gorm:"type:varchar(64);default:''"`
	RelayErrorCode                   string  `json:"relay_error_code" gorm:"type:varchar(128);default:''"`
	RelayErrorMessage                string  `json:"relay_error_message" gorm:"type:text"`
	TraceID                          string  `json:"trace_id" gorm:"column:trace_id;default:''"`
	ElapsedTime                      int64   `json:"elapsed_time" gorm:"default:0"`
	IsStream                         bool    `json:"is_stream" gorm:"default:false"`
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
	LogTypeRelayFailure
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

func ApplyConsumeLogBillingSource(log *Log, chargeUserBalance bool, packageSource LogBillingSourceSnapshot, balanceSource LogBillingSourceSnapshot) {
	if log == nil {
		return
	}
	log.BillingSource = ResolveConsumeLogBillingSource(chargeUserBalance)
	source := packageSource
	if chargeUserBalance {
		source = balanceSource
	}
	log.BillingSourceID = source.ID
	log.BillingSourceName = source.Name
	log.BillingSourceDetail = source.Detail
}

type LogBillingSourceSnapshot struct {
	ID     string
	Name   string
	Detail string
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

func RecordRelayFailureLog(ctx context.Context, log *Log) {
	mustLogRepo().RecordRelayFailureLog(ctx, log)
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

func SumUsedQuotaByUserIdWithModelAndToken(logType int, userId string, startTimestamp int64, endTimestamp int64, modelName string, tokenName string) (int64, error) {
	return mustLogRepo().SumUsedQuotaByUserIdWithModelAndToken(logType, userId, startTimestamp, endTimestamp, modelName, tokenName)
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
