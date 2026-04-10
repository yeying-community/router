package config

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

var SystemName = "Router"
var ServerAddress = "http://localhost:3011"
var Footer = ""
var Logo = ""
var TopUpMode = ""
var TopUpLink = ""
var TopUpAPICreateURL = ""
var TopUpAPIQueryURL = ""
var TopUpAPIUniacid = 1
var TopUpMerchantApp = "router"
var TopUpAPITimeoutSeconds = 15
var TopUpSignSecret = ""
var TopUpCallbackToken = ""
var ChatLink = ""
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens
var FXAutoSyncEnabled = false
var FXAutoSyncIntervalSeconds = 6 * 60 * 60
var FXAutoSyncProvider = "frankfurter"
var FXAutoSyncLastRunAt int64 = 0
var FXAutoSyncLastSuccessAt int64 = 0
var FXAutoSyncLastError = ""

// Any options with "Secret", "Token" in its key won't be return by GetOptions

var SessionSecret = uuid.New().String()

var OptionMap map[string]string
var OptionMapRWMutex sync.RWMutex

var ItemsPerPage = 10
var MaxRecentItems = 100

var PasswordLoginEnabled = true
var PasswordRegisterEnabled = true
var EmailVerificationEnabled = false
var GitHubOAuthEnabled = false
var OidcEnabled = false
var WeChatAuthEnabled = false
var TurnstileCheckEnabled = false
var RegisterEnabled = true

var EmailDomainRestrictionEnabled = false
var EmailDomainWhitelist = []string{
	"gmail.com",
	"163.com",
	"126.com",
	"qq.com",
	"outlook.com",
	"hotmail.com",
	"icloud.com",
	"yahoo.com",
	"foxmail.com",
}

var DebugEnabled = false
var DebugSQLEnabled = false
var MemoryCacheEnabled = false

var LogConsumeEnabled = true

var SMTPServer = ""
var SMTPPort = 587
var SMTPAccount = ""
var SMTPFrom = ""
var SMTPToken = ""

var GitHubClientId = ""
var GitHubClientSecret = ""

var LarkClientId = ""
var LarkClientSecret = ""

var OidcClientId = ""
var OidcClientSecret = ""
var OidcWellKnown = ""
var OidcAuthorizationEndpoint = ""
var OidcTokenEndpoint = ""
var OidcUserinfoEndpoint = ""

var WeChatServerAddress = ""
var WeChatServerToken = ""
var WeChatAccountQRCodeImageURL = ""

// Wallet login
var AutoRegisterEnabled = false
var WalletJWTSecret = ""
var WalletJWTExpireHours = 72
var WalletRefreshTokenExpireHours = 24 * 30
var WalletNonceTTLMinutes = 10
var WalletRefreshCookieDomain = ""
var WalletRefreshCookieSecure = false
var WalletRefreshCookieSameSite = "lax"

// Optional fallback secrets (comma-separated env WALLET_JWT_FALLBACK_SECRETS) for verifying wallet JWTs issued by external services.
var WalletJWTFallbackSecrets []string

// UCAN auth
var UcanAud = ""

const DefaultUcanResourcePrefix = "llm:"
const DefaultUcanAction = "invoke"

const AppCompatUcanResource = "app:*"
const AppCompatUcanAction = "invoke"
const AppScopedCompatUcanResource = "app:all:*"
const AppScopedCompatUcanAction = "invoke"
const CompatUcanResource = "router:llm"
const CompatUcanAction = "invoke"

// Compatibility alias for previously issued tokens.
const ProfileCompatUcanResource = "profile"
const ProfileCompatUcanAction = "read"

var UcanResource = ""
var UcanAction = DefaultUcanAction

// CORS allowlist (comma-separated env CORS_ALLOWED_ORIGINS)
var CorsAllowedOrigins []string

var MessagePusherAddress = ""
var MessagePusherToken = ""

var TurnstileSiteKey = ""
var TurnstileSecretKey = ""

var NewUserRewardTopupPlanID = ""
var DefaultUserGroup = ""
var InviterRewardTopupPlanID = ""
var ChannelDisableThreshold = 5.0
var AutomaticDisableChannelEnabled = false
var AutomaticEnableChannelEnabled = false
var QuotaRemindThreshold int64 = 1000
var PreConsumedQuota int64 = 500
var RetryTimes = 0

var RootUserEmail = ""

var IsMasterNode = true

var RequestInterval = time.Duration(0)

var SyncFrequency = 10 * 60 // unit is second

var BatchUpdateEnabled = false
var BatchUpdateInterval = 5

var RelayTimeout = 0 // unit is second

var GeminiSafetySetting = "BLOCK_NONE"

// All duration's unit is seconds
// Shouldn't larger then RateLimitKeyExpirationDuration
var (
	GlobalApiRateLimitNum            = 480
	GlobalApiRateLimitDuration int64 = 3 * 60

	GlobalWebRateLimitNum            = 240
	GlobalWebRateLimitDuration int64 = 3 * 60

	UploadRateLimitNum            = 10
	UploadRateLimitDuration int64 = 60

	DownloadRateLimitNum            = 10
	DownloadRateLimitDuration int64 = 60

	CriticalRateLimitNum            = 20
	CriticalRateLimitDuration int64 = 20 * 60
)

var RateLimitKeyExpirationDuration = 20 * time.Minute

var EnableMetric = false
var MetricQueueSize = 10
var MetricSuccessRateThreshold = 0.8
var MetricSuccessChanSize = 1024
var MetricFailChanSize = 128

var InitialRootToken = ""

var InitialRootAccessToken = ""

var RootWalletAddress = ""
var RootWalletAddresses []string

var GeminiVersion = "v1"

var OnlyOneLogFile = false
var LogRotateMaxSizeMB = 100
var LogRotateMaxBackups = 10
var LogRotateMaxAgeDays = 14
var LogRotateCompress = false

var RelayProxy = ""
var UserContentRequestProxy = ""
var UserContentRequestTimeout = 30

var EnforceIncludeUsage = false
var TestPrompt = "Output only your specific model name with no additional text."
