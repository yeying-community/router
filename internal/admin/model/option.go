package model

import (
	"strconv"
	"strings"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
)

const SystemSettingsTableName = "system_settings"

var deprecatedRewardOptionKeys = map[string]struct{}{
	"QuotaForNewUser": {},
	"QuotaForInviter": {},
	"QuotaForInvitee": {},
}

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

func (Option) TableName() string {
	return SystemSettingsTableName
}

func AllOption() ([]*Option, error) {
	return mustOptionRepo().AllOption()
}

func InitOptionMap() {
	config.OptionMapRWMutex.Lock()
	config.OptionMap = make(map[string]string)
	config.OptionMap["PasswordLoginEnabled"] = strconv.FormatBool(config.PasswordLoginEnabled)
	config.OptionMap["PasswordRegisterEnabled"] = strconv.FormatBool(config.PasswordRegisterEnabled)
	config.OptionMap["RegisterEnabled"] = strconv.FormatBool(config.RegisterEnabled)
	config.OptionMap["AutomaticDisableChannelEnabled"] = strconv.FormatBool(config.AutomaticDisableChannelEnabled)
	config.OptionMap["AutomaticEnableChannelEnabled"] = strconv.FormatBool(config.AutomaticEnableChannelEnabled)
	config.OptionMap["LogConsumeEnabled"] = strconv.FormatBool(config.LogConsumeEnabled)
	config.OptionMap["FXAutoSyncEnabled"] = strconv.FormatBool(config.FXAutoSyncEnabled)
	config.OptionMap["FXAutoSyncIntervalSeconds"] = strconv.Itoa(config.FXAutoSyncIntervalSeconds)
	config.OptionMap["FXAutoSyncProvider"] = config.FXAutoSyncProvider
	config.OptionMap["FXAutoSyncLastRunAt"] = strconv.FormatInt(config.FXAutoSyncLastRunAt, 10)
	config.OptionMap["FXAutoSyncLastSuccessAt"] = strconv.FormatInt(config.FXAutoSyncLastSuccessAt, 10)
	config.OptionMap["FXAutoSyncLastError"] = config.FXAutoSyncLastError
	config.OptionMap["ChannelDisableThreshold"] = strconv.FormatFloat(config.ChannelDisableThreshold, 'f', -1, 64)
	config.OptionMap["SMTPServer"] = ""
	config.OptionMap["SMTPFrom"] = ""
	config.OptionMap["SMTPPort"] = strconv.Itoa(config.SMTPPort)
	config.OptionMap["SMTPAccount"] = ""
	config.OptionMap["SMTPToken"] = ""
	config.OptionMap["Notice"] = ""
	config.OptionMap["About"] = ""
	config.OptionMap["HomePageContent"] = ""
	config.OptionMap["Footer"] = config.Footer
	config.OptionMap["SystemName"] = config.SystemName
	config.OptionMap["Logo"] = config.Logo
	config.OptionMap["NewUserRewardTopupPlanID"] = config.NewUserRewardTopupPlanID
	config.OptionMap["DefaultUserGroup"] = config.DefaultUserGroup
	config.OptionMap["InviterRewardTopupPlanID"] = config.InviterRewardTopupPlanID
	config.OptionMap["QuotaRemindThreshold"] = strconv.FormatInt(config.QuotaRemindThreshold, 10)
	config.OptionMap["PreConsumedQuota"] = strconv.FormatInt(config.PreConsumedQuota, 10)
	config.OptionMap["QuotaPerUnit"] = strconv.FormatFloat(config.QuotaPerUnit, 'f', -1, 64)
	config.OptionMap["RetryTimes"] = strconv.Itoa(config.RetryTimes)
	config.OptionMapRWMutex.Unlock()
	loadOptionsFromDatabase()
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		logger.SysError("failed to sync group runtime caches from groups table: " + err.Error())
	}
	if err := SyncBillingCurrencyCatalogWithDB(DB); err != nil {
		logger.SysError("failed to sync billing currencies from database: " + err.Error())
	}
}

func loadOptionsFromDatabase() {
	options, _ := AllOption()
	for _, option := range options {
		err := UpdateOptionMap(option.Key, option.Value)
		if err != nil {
			logger.SysError("failed to update option map: " + err.Error())
		}
	}
}

func SyncOptions(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		logger.SysLog("syncing system settings from database")
		loadOptionsFromDatabase()
		if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
			logger.SysError("failed to sync group runtime caches from groups table: " + err.Error())
		}
		if err := SyncBillingCurrencyCatalogWithDB(DB); err != nil {
			logger.SysError("failed to sync billing currencies from database: " + err.Error())
		}
	}
}

func UpdateOption(key string, value string) error {
	normalizedKey := strings.TrimSpace(key)
	if _, ok := deprecatedRewardOptionKeys[normalizedKey]; ok {
		if DB != nil {
			if err := DB.Where("key = ?", normalizedKey).Delete(&Option{}).Error; err != nil {
				return err
			}
		}
		return UpdateOptionMap(normalizedKey, "")
	}
	return mustOptionRepo().UpdateOption(key, value)
}

func UpdateOptionMap(key string, value string) (err error) {
	config.OptionMapRWMutex.Lock()
	defer config.OptionMapRWMutex.Unlock()
	switch key {
	case "WalletLoginEnabled", "WalletAutoRegisterEnabled", "WalletAllowedChains", "AutoRegisterEnabled", "Theme",
		"ServerAddress", "TopUpLink", "TopUpSignSecret", "TopUpCallbackToken", "ChatLink":
		delete(config.OptionMap, key)
		return nil
	}
	if _, ok := deprecatedRewardOptionKeys[key]; ok {
		delete(config.OptionMap, key)
		return nil
	}
	config.OptionMap[key] = value
	if strings.HasSuffix(key, "Enabled") {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			config.PasswordRegisterEnabled = boolValue
		case "PasswordLoginEnabled":
			config.PasswordLoginEnabled = boolValue
		case "RegisterEnabled":
			config.RegisterEnabled = boolValue
		case "AutomaticDisableChannelEnabled":
			config.AutomaticDisableChannelEnabled = boolValue
		case "AutomaticEnableChannelEnabled":
			config.AutomaticEnableChannelEnabled = boolValue
		case "LogConsumeEnabled":
			config.LogConsumeEnabled = boolValue
		case "FXAutoSyncEnabled":
			config.FXAutoSyncEnabled = boolValue
		}
	}
	switch key {
	case "SMTPServer":
		config.SMTPServer = value
	case "SMTPPort":
		intValue, _ := strconv.Atoi(value)
		config.SMTPPort = intValue
	case "SMTPAccount":
		config.SMTPAccount = value
	case "SMTPFrom":
		config.SMTPFrom = value
	case "SMTPToken":
		config.SMTPToken = value
	case "Footer":
		config.Footer = value
	case "SystemName":
		config.SystemName = value
	case "Logo":
		config.Logo = value
	case "NewUserRewardTopupPlanID":
		config.NewUserRewardTopupPlanID = strings.TrimSpace(value)
	case "DefaultUserGroup":
		config.DefaultUserGroup = strings.TrimSpace(value)
	case "InviterRewardTopupPlanID":
		config.InviterRewardTopupPlanID = strings.TrimSpace(value)
	case "QuotaRemindThreshold":
		config.QuotaRemindThreshold, _ = strconv.ParseInt(value, 10, 64)
	case "PreConsumedQuota":
		config.PreConsumedQuota, _ = strconv.ParseInt(value, 10, 64)
	case "RetryTimes":
		limit, _ := strconv.Atoi(value)
		if limit < 0 {
			limit = 0
			config.OptionMap[key] = strconv.Itoa(limit)
		}
		config.RetryTimes = limit
	case "FXAutoSyncIntervalSeconds":
		interval, _ := strconv.Atoi(value)
		if interval < 60 {
			interval = 60
			config.OptionMap[key] = strconv.Itoa(interval)
		}
		config.FXAutoSyncIntervalSeconds = interval
	case "FXAutoSyncProvider":
		provider := strings.TrimSpace(value)
		if provider == "" {
			provider = "frankfurter"
			config.OptionMap[key] = provider
		}
		config.FXAutoSyncProvider = provider
	case "FXAutoSyncLastRunAt":
		config.FXAutoSyncLastRunAt, _ = strconv.ParseInt(value, 10, 64)
	case "FXAutoSyncLastSuccessAt":
		config.FXAutoSyncLastSuccessAt, _ = strconv.ParseInt(value, 10, 64)
	case "FXAutoSyncLastError":
		config.FXAutoSyncLastError = strings.TrimSpace(value)
	case "ChannelDisableThreshold":
		config.ChannelDisableThreshold, _ = strconv.ParseFloat(value, 64)
	case "QuotaPerUnit":
		config.QuotaPerUnit, _ = strconv.ParseFloat(value, 64)
	}
	if key == "QuotaPerUnit" && DB != nil {
		if err := SyncBillingCurrencyCatalogWithDB(DB); err != nil {
			logger.SysError("failed to sync billing currencies after QuotaPerUnit update: " + err.Error())
		}
	}
	return err
}
