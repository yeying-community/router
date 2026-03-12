package common

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"gopkg.in/yaml.v3"
)

var (
	GinMode              = "release"
	ChannelTestFrequency = 0

	DisableOpenAICompat = false
	FrontendBaseURL     = ""

	SQLDSN                = ""
	LogSQLDSN             = ""
	SQLMaxIdleConns       = 100
	SQLMaxOpenConns       = 1000
	SQLMaxLifetimeSeconds = 60

	RedisConnString = ""
	RedisMasterName = ""
	RedisPassword   = ""
)

type RuntimeConfig struct {
	Server    ServerRuntimeConfig    `yaml:"server"`
	Database  DatabaseRuntimeConfig  `yaml:"database"`
	Redis     RedisRuntimeConfig     `yaml:"redis"`
	Node      NodeRuntimeConfig      `yaml:"node"`
	Cache     CacheRuntimeConfig     `yaml:"cache"`
	Auth      AuthRuntimeConfig      `yaml:"auth"`
	CORS      CORSRuntimeConfig      `yaml:"cors"`
	UCAN      UCANRuntimeConfig      `yaml:"ucan"`
	Feature   FeatureRuntimeConfig   `yaml:"feature"`
	Relay     RelayRuntimeConfig     `yaml:"relay"`
	RateLimit RateLimitRuntimeConfig `yaml:"rate_limit"`
	Metrics   MetricsRuntimeConfig   `yaml:"metrics"`
	Bootstrap BootstrapRuntimeConfig `yaml:"bootstrap"`
	Logging   LoggingRuntimeConfig   `yaml:"logging"`
}

type ServerRuntimeConfig struct {
	Port    int    `yaml:"port"`
	GinMode string `yaml:"gin_mode"`
	LogDir  string `yaml:"log_dir"`
}

type DatabaseRuntimeConfig struct {
	SQLDSN             string `yaml:"sql_dsn"`
	LogSQLDSN          string `yaml:"log_sql_dsn"`
	MaxIdleConns       int    `yaml:"max_idle_conns"`
	MaxOpenConns       int    `yaml:"max_open_conns"`
	MaxLifetimeSeconds int    `yaml:"max_lifetime_seconds"`
}

type RedisRuntimeConfig struct {
	ConnString string `yaml:"conn_string"`
	MasterName string `yaml:"master_name"`
	Password   string `yaml:"password"`
}

type NodeRuntimeConfig struct {
	Type                   string `yaml:"type"`
	PollingIntervalSeconds int    `yaml:"polling_interval_seconds"`
}

type CacheRuntimeConfig struct {
	MemoryCacheEnabled         bool `yaml:"memory_cache_enabled"`
	SyncFrequencySeconds       int  `yaml:"sync_frequency_seconds"`
	ChannelTestFrequency       int  `yaml:"channel_test_frequency"`
	BatchUpdateEnabled         bool `yaml:"batch_update_enabled"`
	BatchUpdateIntervalSeconds int  `yaml:"batch_update_interval_seconds"`
}

type AuthRuntimeConfig struct {
	SessionSecret               string   `yaml:"session_secret"`
	AutoRegisterEnabled         bool     `yaml:"auto_register_enabled"`
	WalletJWTSecret             string   `yaml:"wallet_jwt_secret"`
	WalletJWTFallbackSecrets    []string `yaml:"wallet_jwt_fallback_secrets"`
	WalletJWTExpireHours        int      `yaml:"wallet_jwt_expire_hours"`
	WalletRefreshExpireHours    int      `yaml:"wallet_refresh_expire_hours"`
	WalletNonceTTLMinutes       int      `yaml:"wallet_nonce_ttl_minutes"`
	WalletRefreshCookieDomain   string   `yaml:"wallet_refresh_cookie_domain"`
	WalletRefreshCookieSecure   bool     `yaml:"wallet_refresh_cookie_secure"`
	WalletRefreshCookieSameSite string   `yaml:"wallet_refresh_cookie_samesite"`
}

type CORSRuntimeConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type UCANRuntimeConfig struct {
	Aud      string `yaml:"aud"`
	Resource string `yaml:"resource"`
	Action   string `yaml:"action"`
}

type FeatureRuntimeConfig struct {
	Debug               bool   `yaml:"debug"`
	DebugSQL            bool   `yaml:"debug_sql"`
	DisableOpenAICompat bool   `yaml:"disable_openai_compat"`
	FrontendBaseURL     string `yaml:"frontend_base_url"`
}

type RelayRuntimeConfig struct {
	TimeoutSeconds                int    `yaml:"timeout_seconds"`
	Proxy                         string `yaml:"proxy"`
	UserContentRequestProxy       string `yaml:"user_content_request_proxy"`
	UserContentRequestTimeoutSecs int    `yaml:"user_content_request_timeout_seconds"`
	GeminiSafetySetting           string `yaml:"gemini_safety_setting"`
	GeminiVersion                 string `yaml:"gemini_version"`
	EnforceIncludeUsage           bool   `yaml:"enforce_include_usage"`
	TestPrompt                    string `yaml:"test_prompt"`
}

type RateLimitRuntimeConfig struct {
	GlobalAPIRateLimit                int `yaml:"global_api_rate_limit"`
	GlobalAPIRateLimitDurationSeconds int `yaml:"global_api_rate_limit_duration_seconds"`
	GlobalWebRateLimit                int `yaml:"global_web_rate_limit"`
	GlobalWebRateLimitDurationSeconds int `yaml:"global_web_rate_limit_duration_seconds"`
}

type MetricsRuntimeConfig struct {
	Enabled              bool    `yaml:"enabled"`
	QueueSize            int     `yaml:"queue_size"`
	SuccessRateThreshold float64 `yaml:"success_rate_threshold"`
	SuccessChanSize      int     `yaml:"success_chan_size"`
	FailChanSize         int     `yaml:"fail_chan_size"`
}

type BootstrapRuntimeConfig struct {
	InitialRootToken       string `yaml:"initial_root_token"`
	InitialRootAccessToken string `yaml:"initial_root_access_token"`
	RootWalletAddress      string `yaml:"root_wallet_address"`
}

type LoggingRuntimeConfig struct {
	OnlyOneLogFile   bool `yaml:"only_one_log_file"`
	RotateMaxSizeMB  int  `yaml:"rotate_max_size_mb"`
	RotateMaxBackups int  `yaml:"rotate_max_backups"`
	RotateMaxAgeDays int  `yaml:"rotate_max_age_days"`
	RotateCompress   bool `yaml:"rotate_compress"`
}

func defaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Server: ServerRuntimeConfig{
			Port:    3011,
			GinMode: "release",
			LogDir:  "./logs",
		},
		Database: DatabaseRuntimeConfig{
			SQLDSN:             "postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable",
			LogSQLDSN:          "",
			MaxIdleConns:       100,
			MaxOpenConns:       1000,
			MaxLifetimeSeconds: 60,
		},
		Redis: RedisRuntimeConfig{
			ConnString: "",
			MasterName: "",
			Password:   "",
		},
		Node: NodeRuntimeConfig{
			Type:                   "master",
			PollingIntervalSeconds: 0,
		},
		Cache: CacheRuntimeConfig{
			MemoryCacheEnabled:         false,
			SyncFrequencySeconds:       10 * 60,
			ChannelTestFrequency:       0,
			BatchUpdateEnabled:         false,
			BatchUpdateIntervalSeconds: 5,
		},
		Auth: AuthRuntimeConfig{
			SessionSecret:               "",
			AutoRegisterEnabled:         false,
			WalletJWTSecret:             "",
			WalletJWTFallbackSecrets:    []string{},
			WalletJWTExpireHours:        72,
			WalletRefreshExpireHours:    24 * 30,
			WalletNonceTTLMinutes:       10,
			WalletRefreshCookieDomain:   "",
			WalletRefreshCookieSecure:   false,
			WalletRefreshCookieSameSite: "lax",
		},
		CORS: CORSRuntimeConfig{
			AllowedOrigins: []string{},
		},
		UCAN: UCANRuntimeConfig{
			Aud:      "",
			Resource: "profile",
			Action:   "read",
		},
		Feature: FeatureRuntimeConfig{
			Debug:               false,
			DebugSQL:            false,
			DisableOpenAICompat: false,
			FrontendBaseURL:     "",
		},
		Relay: RelayRuntimeConfig{
			TimeoutSeconds:                0,
			Proxy:                         "",
			UserContentRequestProxy:       "",
			UserContentRequestTimeoutSecs: 30,
			GeminiSafetySetting:           "BLOCK_NONE",
			GeminiVersion:                 "v1",
			EnforceIncludeUsage:           false,
			TestPrompt:                    "Output only your specific model name with no additional text.",
		},
		RateLimit: RateLimitRuntimeConfig{
			GlobalAPIRateLimit:                480,
			GlobalAPIRateLimitDurationSeconds: 3 * 60,
			GlobalWebRateLimit:                240,
			GlobalWebRateLimitDurationSeconds: 3 * 60,
		},
		Metrics: MetricsRuntimeConfig{
			Enabled:              false,
			QueueSize:            10,
			SuccessRateThreshold: 0.8,
			SuccessChanSize:      1024,
			FailChanSize:         128,
		},
		Bootstrap: BootstrapRuntimeConfig{
			InitialRootToken:       "",
			InitialRootAccessToken: "",
			RootWalletAddress:      "",
		},
		Logging: LoggingRuntimeConfig{
			OnlyOneLogFile:   false,
			RotateMaxSizeMB:  100,
			RotateMaxBackups: 10,
			RotateMaxAgeDays: 14,
			RotateCompress:   false,
		},
	}
}

func LoadRuntimeConfig(path string) (*RuntimeConfig, error) {
	configPath := strings.TrimSpace(path)
	if configPath == "" {
		configPath = "./config.yaml"
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config file %q not found; copy config.yaml.template to config.yaml first", configPath)
		}
		return nil, fmt.Errorf("read config file %q failed: %w", configPath, err)
	}

	cfg := defaultRuntimeConfig()
	if err = yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q failed: %w", configPath, err)
	}
	return &cfg, nil
}

func ApplyRuntimeConfig(cfg *RuntimeConfig, portFlagSet bool, logDirFlagSet bool) error {
	if cfg == nil {
		return errors.New("runtime config is nil")
	}

	if !portFlagSet {
		if cfg.Server.Port <= 0 {
			return fmt.Errorf("invalid server.port: %d", cfg.Server.Port)
		}
		*Port = cfg.Server.Port
	}
	if !logDirFlagSet {
		*LogDir = strings.TrimSpace(cfg.Server.LogDir)
		if *LogDir == "" {
			*LogDir = "./logs"
		}
	}

	GinMode = normalizeGinMode(cfg.Server.GinMode)
	ChannelTestFrequency = cfg.Cache.ChannelTestFrequency
	if ChannelTestFrequency < 0 {
		return fmt.Errorf("invalid cache.channel_test_frequency: %d", ChannelTestFrequency)
	}

	DisableOpenAICompat = cfg.Feature.DisableOpenAICompat
	FrontendBaseURL = strings.TrimSpace(cfg.Feature.FrontendBaseURL)

	SQLDSN = strings.TrimSpace(cfg.Database.SQLDSN)
	LogSQLDSN = strings.TrimSpace(cfg.Database.LogSQLDSN)
	SQLMaxIdleConns = cfg.Database.MaxIdleConns
	if SQLMaxIdleConns <= 0 {
		SQLMaxIdleConns = 100
	}
	SQLMaxOpenConns = cfg.Database.MaxOpenConns
	if SQLMaxOpenConns <= 0 {
		SQLMaxOpenConns = 1000
	}
	SQLMaxLifetimeSeconds = cfg.Database.MaxLifetimeSeconds
	if SQLMaxLifetimeSeconds <= 0 {
		SQLMaxLifetimeSeconds = 60
	}

	RedisConnString = strings.TrimSpace(cfg.Redis.ConnString)
	RedisMasterName = strings.TrimSpace(cfg.Redis.MasterName)
	RedisPassword = strings.TrimSpace(cfg.Redis.Password)

	if sessionSecret := strings.TrimSpace(cfg.Auth.SessionSecret); sessionSecret != "" {
		if sessionSecret == "random_string" {
			logger.SysError("auth.session_secret is set to an example value, please change it to a random string.")
		} else {
			config.SessionSecret = sessionSecret
		}
	}
	config.AutoRegisterEnabled = cfg.Auth.AutoRegisterEnabled
	config.WalletJWTSecret = strings.TrimSpace(cfg.Auth.WalletJWTSecret)
	config.WalletJWTFallbackSecrets = normalizeStringSlice(cfg.Auth.WalletJWTFallbackSecrets)
	if cfg.Auth.WalletJWTExpireHours > 0 {
		config.WalletJWTExpireHours = cfg.Auth.WalletJWTExpireHours
	}
	if cfg.Auth.WalletRefreshExpireHours > 0 {
		config.WalletRefreshTokenExpireHours = cfg.Auth.WalletRefreshExpireHours
	}
	if cfg.Auth.WalletNonceTTLMinutes > 0 {
		config.WalletNonceTTLMinutes = cfg.Auth.WalletNonceTTLMinutes
	}
	config.WalletRefreshCookieDomain = strings.TrimSpace(cfg.Auth.WalletRefreshCookieDomain)
	config.WalletRefreshCookieSecure = cfg.Auth.WalletRefreshCookieSecure
	if sameSite := strings.ToLower(strings.TrimSpace(cfg.Auth.WalletRefreshCookieSameSite)); sameSite != "" {
		config.WalletRefreshCookieSameSite = sameSite
	}
	if config.WalletJWTSecret == "" {
		config.WalletJWTSecret = config.SessionSecret
	}

	config.CorsAllowedOrigins = normalizeStringSlice(cfg.CORS.AllowedOrigins)
	config.UcanAud = strings.TrimSpace(cfg.UCAN.Aud)
	if resource := strings.TrimSpace(cfg.UCAN.Resource); resource != "" {
		config.UcanResource = resource
	}
	if action := strings.TrimSpace(cfg.UCAN.Action); action != "" {
		config.UcanAction = action
	}

	config.DebugEnabled = cfg.Feature.Debug
	config.DebugSQLEnabled = cfg.Feature.DebugSQL
	config.MemoryCacheEnabled = cfg.Cache.MemoryCacheEnabled
	config.BatchUpdateEnabled = cfg.Cache.BatchUpdateEnabled
	if cfg.Cache.BatchUpdateIntervalSeconds > 0 {
		config.BatchUpdateInterval = cfg.Cache.BatchUpdateIntervalSeconds
	} else {
		config.BatchUpdateInterval = 5
	}
	if cfg.Cache.SyncFrequencySeconds > 0 {
		config.SyncFrequency = cfg.Cache.SyncFrequencySeconds
	} else {
		config.SyncFrequency = 10 * 60
	}

	nodeType := strings.ToLower(strings.TrimSpace(cfg.Node.Type))
	config.IsMasterNode = nodeType != "slave"
	config.RequestInterval = time.Duration(cfg.Node.PollingIntervalSeconds) * time.Second

	config.RelayTimeout = cfg.Relay.TimeoutSeconds
	if config.RelayTimeout < 0 {
		return fmt.Errorf("invalid relay.timeout_seconds: %d", config.RelayTimeout)
	}
	config.RelayProxy = strings.TrimSpace(cfg.Relay.Proxy)
	config.UserContentRequestProxy = strings.TrimSpace(cfg.Relay.UserContentRequestProxy)
	if cfg.Relay.UserContentRequestTimeoutSecs > 0 {
		config.UserContentRequestTimeout = cfg.Relay.UserContentRequestTimeoutSecs
	} else {
		config.UserContentRequestTimeout = 30
	}
	if geminiSafetySetting := strings.TrimSpace(cfg.Relay.GeminiSafetySetting); geminiSafetySetting != "" {
		config.GeminiSafetySetting = geminiSafetySetting
	} else {
		config.GeminiSafetySetting = "BLOCK_NONE"
	}
	if geminiVersion := strings.TrimSpace(cfg.Relay.GeminiVersion); geminiVersion != "" {
		config.GeminiVersion = geminiVersion
	} else {
		config.GeminiVersion = "v1"
	}
	config.EnforceIncludeUsage = cfg.Relay.EnforceIncludeUsage
	if testPrompt := strings.TrimSpace(cfg.Relay.TestPrompt); testPrompt != "" {
		config.TestPrompt = testPrompt
	} else {
		config.TestPrompt = "Output only your specific model name with no additional text."
	}

	if cfg.RateLimit.GlobalAPIRateLimit > 0 {
		config.GlobalApiRateLimitNum = cfg.RateLimit.GlobalAPIRateLimit
	} else {
		config.GlobalApiRateLimitNum = 480
	}
	if cfg.RateLimit.GlobalAPIRateLimitDurationSeconds > 0 {
		config.GlobalApiRateLimitDuration = int64(cfg.RateLimit.GlobalAPIRateLimitDurationSeconds)
	} else {
		config.GlobalApiRateLimitDuration = 3 * 60
	}
	if cfg.RateLimit.GlobalWebRateLimit > 0 {
		config.GlobalWebRateLimitNum = cfg.RateLimit.GlobalWebRateLimit
	} else {
		config.GlobalWebRateLimitNum = 240
	}
	if cfg.RateLimit.GlobalWebRateLimitDurationSeconds > 0 {
		config.GlobalWebRateLimitDuration = int64(cfg.RateLimit.GlobalWebRateLimitDurationSeconds)
	} else {
		config.GlobalWebRateLimitDuration = 3 * 60
	}

	config.EnableMetric = cfg.Metrics.Enabled
	if cfg.Metrics.QueueSize > 0 {
		config.MetricQueueSize = cfg.Metrics.QueueSize
	} else {
		config.MetricQueueSize = 10
	}
	if cfg.Metrics.SuccessRateThreshold > 0 {
		config.MetricSuccessRateThreshold = cfg.Metrics.SuccessRateThreshold
	} else {
		config.MetricSuccessRateThreshold = 0.8
	}
	if cfg.Metrics.SuccessChanSize > 0 {
		config.MetricSuccessChanSize = cfg.Metrics.SuccessChanSize
	} else {
		config.MetricSuccessChanSize = 1024
	}
	if cfg.Metrics.FailChanSize > 0 {
		config.MetricFailChanSize = cfg.Metrics.FailChanSize
	} else {
		config.MetricFailChanSize = 128
	}

	config.InitialRootToken = strings.TrimSpace(cfg.Bootstrap.InitialRootToken)
	config.InitialRootAccessToken = strings.TrimSpace(cfg.Bootstrap.InitialRootAccessToken)
	config.RootWalletAddress = strings.TrimSpace(cfg.Bootstrap.RootWalletAddress)
	config.RootWalletAddresses = nil
	for _, item := range strings.Split(config.RootWalletAddress, ",") {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		duplicated := false
		for _, existing := range config.RootWalletAddresses {
			if existing == normalized {
				duplicated = true
				break
			}
		}
		if !duplicated {
			config.RootWalletAddresses = append(config.RootWalletAddresses, normalized)
		}
	}
	config.OnlyOneLogFile = cfg.Logging.OnlyOneLogFile
	if cfg.Logging.RotateMaxSizeMB > 0 {
		config.LogRotateMaxSizeMB = cfg.Logging.RotateMaxSizeMB
	} else {
		config.LogRotateMaxSizeMB = 100
	}
	if cfg.Logging.RotateMaxBackups >= 0 {
		config.LogRotateMaxBackups = cfg.Logging.RotateMaxBackups
	} else {
		config.LogRotateMaxBackups = 10
	}
	if cfg.Logging.RotateMaxAgeDays >= 0 {
		config.LogRotateMaxAgeDays = cfg.Logging.RotateMaxAgeDays
	} else {
		config.LogRotateMaxAgeDays = 14
	}
	config.LogRotateCompress = cfg.Logging.RotateCompress

	setCompatibilityEnvs()
	return nil
}

func normalizeGinMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return "release"
	}
	switch normalized {
	case "debug", "release", "test":
		return normalized
	default:
		return "release"
	}
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func setCompatibilityEnvs() {
	_ = os.Setenv("PORT", strconv.Itoa(*Port))
	_ = os.Setenv("GIN_MODE", GinMode)
	_ = os.Setenv("SESSION_SECRET", config.SessionSecret)
	_ = os.Setenv("AUTO_REGISTER_ENABLED", strconv.FormatBool(config.AutoRegisterEnabled))
	_ = os.Setenv("WALLET_JWT_SECRET", config.WalletJWTSecret)
	_ = os.Setenv("WALLET_JWT_FALLBACK_SECRETS", strings.Join(config.WalletJWTFallbackSecrets, ","))
	_ = os.Setenv("WALLET_JWT_EXPIRE_HOURS", strconv.Itoa(config.WalletJWTExpireHours))
	_ = os.Setenv("WALLET_REFRESH_EXPIRE_HOURS", strconv.Itoa(config.WalletRefreshTokenExpireHours))
	_ = os.Setenv("WALLET_NONCE_TTL_MINUTES", strconv.Itoa(config.WalletNonceTTLMinutes))
	_ = os.Setenv("WALLET_REFRESH_COOKIE_DOMAIN", config.WalletRefreshCookieDomain)
	_ = os.Setenv("WALLET_REFRESH_COOKIE_SECURE", strconv.FormatBool(config.WalletRefreshCookieSecure))
	_ = os.Setenv("WALLET_REFRESH_COOKIE_SAMESITE", config.WalletRefreshCookieSameSite)
	_ = os.Setenv("CORS_ALLOWED_ORIGINS", strings.Join(config.CorsAllowedOrigins, ","))
	_ = os.Setenv("UCAN_AUD", config.UcanAud)
	_ = os.Setenv("UCAN_RESOURCE", config.UcanResource)
	_ = os.Setenv("UCAN_ACTION", config.UcanAction)
	_ = os.Setenv("SQL_DSN", SQLDSN)
	_ = os.Setenv("LOG_SQL_DSN", LogSQLDSN)
	_ = os.Setenv("SQL_MAX_IDLE_CONNS", strconv.Itoa(SQLMaxIdleConns))
	_ = os.Setenv("SQL_MAX_OPEN_CONNS", strconv.Itoa(SQLMaxOpenConns))
	_ = os.Setenv("SQL_MAX_LIFETIME", strconv.Itoa(SQLMaxLifetimeSeconds))
	_ = os.Setenv("REDIS_CONN_STRING", RedisConnString)
	_ = os.Setenv("REDIS_MASTER_NAME", RedisMasterName)
	_ = os.Setenv("REDIS_PASSWORD", RedisPassword)
	if config.IsMasterNode {
		_ = os.Setenv("NODE_TYPE", "master")
	} else {
		_ = os.Setenv("NODE_TYPE", "slave")
	}
	_ = os.Setenv("POLLING_INTERVAL", strconv.Itoa(int(config.RequestInterval.Seconds())))
	_ = os.Setenv("SYNC_FREQUENCY", strconv.Itoa(config.SyncFrequency))
	_ = os.Setenv("MEMORY_CACHE_ENABLED", strconv.FormatBool(config.MemoryCacheEnabled))
	_ = os.Setenv("CHANNEL_TEST_FREQUENCY", strconv.Itoa(ChannelTestFrequency))
	_ = os.Setenv("BATCH_UPDATE_ENABLED", strconv.FormatBool(config.BatchUpdateEnabled))
	_ = os.Setenv("BATCH_UPDATE_INTERVAL", strconv.Itoa(config.BatchUpdateInterval))
	_ = os.Setenv("DISABLE_OPENAI_COMPAT", strconv.FormatBool(DisableOpenAICompat))
	_ = os.Setenv("FRONTEND_BASE_URL", FrontendBaseURL)
	_ = os.Setenv("DEBUG", strconv.FormatBool(config.DebugEnabled))
	_ = os.Setenv("DEBUG_SQL", strconv.FormatBool(config.DebugSQLEnabled))
	_ = os.Setenv("RELAY_TIMEOUT", strconv.Itoa(config.RelayTimeout))
	_ = os.Setenv("GEMINI_SAFETY_SETTING", config.GeminiSafetySetting)
	_ = os.Setenv("GLOBAL_API_RATE_LIMIT", strconv.Itoa(config.GlobalApiRateLimitNum))
	_ = os.Setenv("GLOBAL_WEB_RATE_LIMIT", strconv.Itoa(config.GlobalWebRateLimitNum))
	_ = os.Setenv("ENABLE_METRIC", strconv.FormatBool(config.EnableMetric))
	_ = os.Setenv("METRIC_QUEUE_SIZE", strconv.Itoa(config.MetricQueueSize))
	_ = os.Setenv("METRIC_SUCCESS_RATE_THRESHOLD", strconv.FormatFloat(config.MetricSuccessRateThreshold, 'f', -1, 64))
	_ = os.Setenv("METRIC_SUCCESS_CHAN_SIZE", strconv.Itoa(config.MetricSuccessChanSize))
	_ = os.Setenv("METRIC_FAIL_CHAN_SIZE", strconv.Itoa(config.MetricFailChanSize))
	_ = os.Setenv("INITIAL_ROOT_TOKEN", config.InitialRootToken)
	_ = os.Setenv("INITIAL_ROOT_ACCESS_TOKEN", config.InitialRootAccessToken)
	_ = os.Setenv("ROOT_WALLET_ADDRESS", config.RootWalletAddress)
	_ = os.Setenv("GEMINI_VERSION", config.GeminiVersion)
	_ = os.Setenv("ONLY_ONE_LOG_FILE", strconv.FormatBool(config.OnlyOneLogFile))
	_ = os.Setenv("LOG_ROTATE_MAX_SIZE_MB", strconv.Itoa(config.LogRotateMaxSizeMB))
	_ = os.Setenv("LOG_ROTATE_MAX_BACKUPS", strconv.Itoa(config.LogRotateMaxBackups))
	_ = os.Setenv("LOG_ROTATE_MAX_AGE_DAYS", strconv.Itoa(config.LogRotateMaxAgeDays))
	_ = os.Setenv("LOG_ROTATE_COMPRESS", strconv.FormatBool(config.LogRotateCompress))
	_ = os.Setenv("RELAY_PROXY", config.RelayProxy)
	_ = os.Setenv("USER_CONTENT_REQUEST_PROXY", config.UserContentRequestProxy)
	_ = os.Setenv("USER_CONTENT_REQUEST_TIMEOUT", strconv.Itoa(config.UserContentRequestTimeout))
	_ = os.Setenv("ENFORCE_INCLUDE_USAGE", strconv.FormatBool(config.EnforceIncludeUsage))
	_ = os.Setenv("TEST_PROMPT", config.TestPrompt)
}
