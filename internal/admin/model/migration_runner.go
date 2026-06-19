package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"gorm.io/gorm"
)

const (
	migrationScopeMain = "main"
	migrationScopeLog  = "log"
)

var (
	allProviderMigrationCatalogProviders = []string{
		"anthropic",
		"baidu",
		"cohere",
		"deepseek",
		"google",
		"hunyuan",
		"minimax",
		"mistral",
		"openai",
		"qwen",
		"stepfun",
		"volcengine",
		"xai",
		"zhipu",
	}
	openAIAndXAIProviderMigrationCatalogProviders  = []string{"openai", "xai"}
	googleAndQwenProviderMigrationCatalogProviders = []string{"google", "qwen"}
)

// SchemaMigration records Flyway-style versioned migrations.
type SchemaMigration struct {
	Scope       string `gorm:"primaryKey;type:varchar(32)"`
	Version     string `gorm:"primaryKey;type:varchar(128)"`
	Description string `gorm:"type:varchar(255);default:''"`
	AppliedAt   int64  `gorm:"index"`
}

func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

type versionedMigration struct {
	Version     string
	Description string
	Up          func(tx *gorm.DB) error
}

func runMainVersionedMigrations(db *gorm.DB) error {
	migrations := []versionedMigration{
		{
			Version:     "202603122230_main_baseline_v30",
			Description: "baseline: create current main schema with user password-state flag, current task tables, and current provider data",
			Up: func(tx *gorm.DB) error {
				return runMainBaselineMigrationWithDB(tx)
			},
		},
		{
			Version:     "202603131030_openai_gpt51_provider_catalog",
			Description: "upsert openai provider migration rows for gpt-5.1 and gpt-5.1-codex pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202603131600_channel_test_artifacts",
			Description: "add persisted artifact metadata columns for channel model test downloads",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelTest{})
			},
		},
		{
			Version:     "202603131830_redemption_redeemed_by_user",
			Description: "add redeemed_by_user_id column for redemption detail tracking",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Redemption{})
			},
		},
		{
			Version:     "202603131930_fix_channel_test_task_status",
			Description: "mark unsupported channel model test tasks as failed",
			Up: func(tx *gorm.DB) error {
				rows := make([]AsyncTask, 0)
				if err := tx.
					Where("type = ? AND status = ?", AsyncTaskTypeChannelModelTest, AsyncTaskStatusSucceeded).
					Find(&rows).Error; err != nil {
					return err
				}
				for _, row := range rows {
					status, message, ok := ResolveAsyncTaskBusinessOutcome(row.Type, row.Result)
					if !ok || status == AsyncTaskStatusSucceeded {
						continue
					}
					if err := tx.Model(&AsyncTask{}).
						Where("id = ?", row.Id).
						Updates(map[string]any{
							"status":        status,
							"error_message": strings.TrimSpace(message),
						}).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202603151130_channel_model_endpoints",
			Description: "add channel model endpoint capability table and seed current endpoint rows",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ChannelModelEndpoint{}); err != nil {
					return err
				}
				channelIDs := make([]string, 0)
				if err := tx.Model(&Channel{}).Distinct("id").Pluck("id", &channelIDs).Error; err != nil {
					return err
				}
				for _, channelID := range channelIDs {
					rows, err := listChannelModelRowsByChannelIDWithDB(tx, channelID)
					if err != nil {
						return err
					}
					if err := SyncChannelModelEndpointsWithDB(tx, channelID, rows); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202603201030_main_event_log_group_id",
			Description: "add group_id index column to event logs in main database",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202603201500_group_daily_quota_limits",
			Description: "add group daily quota limit columns and daily counter table",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&GroupCatalog{}); err != nil {
					return err
				}
				return tx.AutoMigrate(&GroupQuotaCounter{})
			},
		},
		{
			Version:     "202603202030_user_group_daily_quota_counters",
			Description: "switch group daily quota counters to user+group scoped counters",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&GroupQuotaCounter{})
			},
		},
		{
			Version:     "202603311200_user_daily_emergency_quota",
			Description: "add user daily quota and monthly emergency quota models, counters, and log fields",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&User{}, &UserQuotaCounter{}, &Log{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE users SET quota_reset_timezone = ? WHERE COALESCE(quota_reset_timezone, '') = ''",
					DefaultGroupQuotaResetTimezone,
				).Error
			},
		},
		{
			Version:     "202603311700_billing_currency_catalog",
			Description: "add billing currencies catalog and seed default USD/CNY yyc rates",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&BillingCurrency{}); err != nil {
					return err
				}
				return syncDefaultBillingCurrenciesWithDB(tx)
			},
		},
		{
			Version:     "202603311900_log_billing_snapshots",
			Description: "add billing snapshot fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202603312040_generic_quota_counters",
			Description: "migrate legacy daily quota tables to generic quota counter tables",
			Up: func(tx *gorm.DB) error {
				return migrateLegacyQuotaCountersToGenericWithDB(tx)
			},
		},
		{
			Version:     "202603312230_topup_orders",
			Description: "add persisted topup order table for external recharge redirects",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&TopupOrder{})
			},
		},
		{
			Version:     "202603312355_topup_order_callback_flow",
			Description: "add topup order callback fields and redemption linkage",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&TopupOrder{}, &Redemption{})
			},
		},
		{
			Version:     "202604081630_topup_plan_catalog",
			Description: "add persisted topup plan catalog and seed default plans",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&TopupPlan{}); err != nil {
					return err
				}
				return seedDefaultTopupPlansWithDB(tx)
			},
		},
		{
			Version:     "202604141130_channel_model_direct_endpoints",
			Description: "reseed channel model endpoint rows to direct upstream endpoints",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ChannelModelEndpoint{}); err != nil {
					return err
				}
				channelIDs := make([]string, 0)
				if err := tx.Model(&Channel{}).Distinct("id").Pluck("id", &channelIDs).Error; err != nil {
					return err
				}
				for _, channelID := range normalizeTrimmedValuesPreserveOrder(channelIDs) {
					channelProtocol, err := loadChannelProtocolByChannelIDWithDB(tx, channelID)
					if err != nil {
						return err
					}
					rows := make([]ChannelModel, 0)
					if err := tx.
						Where("channel_id = ?", channelID).
						Order("sort_order asc, model asc").
						Find(&rows).Error; err != nil {
						return err
					}
					for i := range rows {
						normalizeChannelModelRow(&rows[i])
						completeChannelModelRowDefaults(&rows[i], channelProtocol)
					}
					if err := SyncChannelModelEndpointsWithDB(tx, channelID, rows); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202604301030_channel_model_endpoint_policies",
			Description: "add channel model endpoint policy table for request compatibility rules",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModelEndpointPolicy{})
			},
		},
		{
			Version:     "202604011030_billing_currency_cny_decouple",
			Description: "decouple CNY yyc rate from system default linkage and switch legacy default source to manual",
			Up: func(tx *gorm.DB) error {
				return decoupleCNYYYCFromSystemDefaultWithDB(tx)
			},
		},
		{
			Version:     "202604011430_service_packages",
			Description: "add service package catalog and user package subscriptions",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ServicePackage{}, &UserPackageSubscription{})
			},
		},
		{
			Version:     "202604041030_service_package_sale_fields",
			Description: "add sale price and sale currency fields for service packages",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ServicePackage{})
			},
		},
		{
			Version:     "202604011630_drop_group_quota_columns",
			Description: "drop legacy group daily quota and timezone columns from groups table",
			Up: func(tx *gorm.DB) error {
				return dropLegacyGroupQuotaColumnsWithDB(tx)
			},
		},
		{
			Version:     "202604021730_channel_model_provider",
			Description: "add persisted provider field for channel model selection",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModel{})
			},
		},
		{
			Version:     "202604021930_group_created_at",
			Description: "add created_at column to groups and backfill existing rows from updated_at",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&GroupCatalog{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE groups SET created_at = COALESCE(NULLIF(updated_at, 0), ?) WHERE COALESCE(created_at, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202604022030_provider_created_at",
			Description: "add created_at column to providers and backfill existing rows from updated_at",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Provider{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE providers SET created_at = COALESCE(NULLIF(updated_at, 0), ?) WHERE COALESCE(created_at, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202604022130_channel_updated_at",
			Description: "add updated_at column to channels and backfill existing rows from created_time",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Channel{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE channels SET updated_at = COALESCE(NULLIF(created_time, 0), ?) WHERE COALESCE(updated_at, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202604022330_redemption_group_face_value",
			Description: "add redemption group binding and multi-unit face value fields",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Redemption{}); err != nil {
					return err
				}
				if err := tx.Exec(
					`ALTER TABLE redemptions
					 ALTER COLUMN face_value_amount TYPE numeric(30,8)
					 USING face_value_amount::numeric`,
				).Error; err != nil {
					return err
				}
				if err := tx.Exec(
					"UPDATE redemptions SET face_value_unit = ? WHERE COALESCE(face_value_unit, '') = ''",
					RedemptionFaceValueUnitYYC,
				).Error; err != nil {
					return err
				}
				return tx.Exec(
					`UPDATE redemptions
					 SET face_value_amount = LEAST(
					   quota::numeric,
					   9999999999999999999999.99999999::numeric
					 )
					 WHERE COALESCE(face_value_amount, 0) = 0
					   AND COALESCE(quota, 0) > 0`,
				).Error
			},
		},
		{
			Version:     "202604030010_redemption_default_group_backfill",
			Description: "backfill historical redemptions with the configured default user group",
			Up: func(tx *gorm.DB) error {
				return backfillRedemptionGroupWithDefaultGroupWithDB(tx)
			},
		},
		{
			Version:     "202604031030_service_package_created_at",
			Description: "add created_at column to service packages and backfill from updated_at",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ServicePackage{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE service_packages SET created_at = COALESCE(NULLIF(updated_at, 0), ?) WHERE COALESCE(created_at, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202605191030_service_package_visibility",
			Description: "add package visibility scope and visible user mapping",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ServicePackage{}, &ServicePackageVisibleUser{}); err != nil {
					return err
				}
				if err := tx.Exec(
					"UPDATE service_packages SET visibility_scope = ? WHERE COALESCE(visibility_scope, '') = ''",
					ServicePackageVisibilityScopeAll,
				).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version:     "202604031130_user_created_updated_at",
			Description: "add created_at and updated_at columns to users and backfill existing rows",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&User{}); err != nil {
					return err
				}
				now := helper.GetTimestamp()
				if err := tx.Exec(
					"UPDATE users SET created_at = ? WHERE COALESCE(created_at, 0) = 0",
					now,
				).Error; err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE users SET updated_at = COALESCE(NULLIF(created_at, 0), ?) WHERE COALESCE(updated_at, 0) = 0",
					now,
				).Error
			},
		},
		{
			Version:     "202604031500_log_billing_source",
			Description: "add billing source field to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202604031730_billing_currency_created_at",
			Description: "add created_at column to billing currencies and backfill existing rows from updated_at",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&BillingCurrency{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE billing_currencies SET created_at = COALESCE(NULLIF(updated_at, 0), ?) WHERE COALESCE(created_at, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202604031930_fx_market_rates",
			Description: "add persisted fiat market rates and normalize legacy auto-managed yyc currency sources to manual",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&FXMarketRate{}); err != nil {
					return err
				}
				return normalizeBillingCurrencyAutoSourcesWithDB(tx)
			},
		},
		{
			Version:     "202604032130_billing_currency_yyc_default",
			Description: "seed default yyc currency row into billing currency catalog",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&BillingCurrency{}); err != nil {
					return err
				}
				return syncDefaultBillingCurrenciesWithDB(tx)
			},
		},
		{
			Version:     "202604032230_billing_currency_minor_unit_six",
			Description: "standardize billing currency minor_unit: yyc=0, fiat=6",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&BillingCurrency{}); err != nil {
					return err
				}
				return tx.Exec(
					`UPDATE billing_currencies
					 SET minor_unit = CASE
					   WHEN UPPER(TRIM(code)) = 'YYC' THEN 0
					   ELSE 6
					 END
					 WHERE COALESCE(minor_unit, -1) <> CASE
					   WHEN UPPER(TRIM(code)) = 'YYC' THEN 0
					   ELSE 6
					 END`,
				).Error
			},
		},
		{
			Version:     "202604041200_package_emergency_quota_columns",
			Description: "migrate monthly_emergency_quota_limit columns to package_emergency_quota_limit and drop legacy columns",
			Up: func(tx *gorm.DB) error {
				return migratePackageEmergencyQuotaColumnsWithDB(tx)
			},
		},
		{
			Version:     "202604042200_topup_order_business_type",
			Description: "ensure topup order business_type column exists and backfill historical rows",
			Up: func(tx *gorm.DB) error {
				return ensureTopupOrderBusinessTypeWithDB(tx)
			},
		},
		{
			Version:     "202604071030_anthropic_claude46_provider_catalog",
			Description: "upsert anthropic provider migration rows for claude 4.6/4.5/3.5 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "anthropic")
			},
		},
		{
			Version:     "202604301100_anthropic_claude47_provider_catalog",
			Description: "upsert anthropic provider migration rows for claude opus 4.7 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "anthropic")
			},
		},
		{
			Version:     "202604301230_openai_gpt54mini_provider_catalog",
			Description: "upsert openai provider migration rows for gpt-5.4-mini pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202604071800_package_emergency_counter_type",
			Description: "rename user quota counter type monthly_emergency to package_emergency",
			Up: func(tx *gorm.DB) error {
				return migrateUserQuotaCounterTypePackageEmergencyWithDB(tx)
			},
		},
		{
			Version:     "202604091100_topup_order_operation_type",
			Description: "add topup order operation_type and backfill package purchases to new_purchase",
			Up: func(tx *gorm.DB) error {
				return ensureTopupOrderOperationTypeWithDB(tx)
			},
		},
		{
			Version:     "202604101600_drop_legacy_reward_option_keys",
			Description: "remove legacy reward option keys from system settings",
			Up: func(tx *gorm.DB) error {
				return tx.Exec(
					"DELETE FROM system_settings WHERE key IN ?",
					[]string{"QuotaForNewUser", "QuotaForInviter", "QuotaForInvitee"},
				).Error
			},
		},
		{
			Version:     "202604131130_topup_validity_balance_lots",
			Description: "add topup plan validity and per-credit user balance lots",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&TopupPlan{}, &TopupOrder{}, &UserBalanceLot{})
			},
		},
		{
			Version:     "202604131930_balance_lot_transactions",
			Description: "add user balance lot transaction ledger table",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&UserBalanceLotTransaction{})
			},
		},
		{
			Version:     "202604132230_redemption_validity_fields",
			Description: "add redemption code validity and redeemed credit validity fields",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Redemption{})
			},
		},
		{
			Version:     "202604141030_drop_user_quota_snapshot_columns",
			Description: "drop legacy users daily/package emergency snapshot columns",
			Up: func(tx *gorm.DB) error {
				return dropLegacyUserQuotaSnapshotColumnsWithDB(tx)
			},
		},
		{
			Version:     "202604151130_channel_test_is_stream",
			Description: "add is_stream flag to channel model test results",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelTest{})
			},
		},
		{
			Version:     "202604151900_drop_channel_model_stream_only",
			Description: "drop legacy is_stream_only column from channel models",
			Up: func(tx *gorm.DB) error {
				return dropChannelModelStreamOnlyWithDB(tx)
			},
		},
		{
			Version:     "202604161230_group_model_providers",
			Description: "add canonical group_model_providers table and backfill provider mapping from current runtime group model routes",
			Up: func(tx *gorm.DB) error {
				return migrateGroupModelProvidersWithDB(tx)
			},
		},
		{
			Version:     "202604161700_channel_model_provider_catalog_backfill",
			Description: "rebuild channel model provider from provider data unique matches and resync group model provider mapping",
			Up: func(tx *gorm.DB) error {
				return backfillChannelModelProviderFromProviderModelsWithDB(tx)
			},
		},
		{
			Version:     "202604161830_channel_model_provider_catalog_reconcile",
			Description: "reconcile channel/group model provider mappings strictly from provider data unique matches",
			Up: func(tx *gorm.DB) error {
				return backfillChannelModelProviderFromProviderModelsWithDB(tx)
			},
		},
		{
			Version:     "202604181030_drop_topup_redemption_link_columns",
			Description: "drop historical topup/redemption mutual reference columns",
			Up: func(tx *gorm.DB) error {
				return dropTopupRedemptionLinkColumnsWithDB(tx)
			},
		},
		{
			Version:     "202604221030_topup_plan_public_visible",
			Description: "add topup plan public visibility flag for user-side plan exposure control",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&TopupPlan{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE topup_plans SET public_visible = TRUE WHERE public_visible IS NULL",
				).Error
			},
		},
		{
			Version:     "202604251130_openai_gpt55_provider_catalog",
			Description: "upsert openai provider migration rows for gpt-5.5 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202605051230_openai_gpt_image_2_provider_catalog",
			Description: "upsert openai provider migration rows for gpt-image-2 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202605091200_openai_realtime_2_provider_catalog",
			Description: "upsert openai provider migration rows for gpt-realtime-2 and gpt-realtime-1.5 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202605091330_openai_realtime_endpoint_candidates",
			Description: "upsert openai realtime provider endpoint candidates",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202605101230_log_billing_observability",
			Description: "add billing observability fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202605101650_channel_model_price_components",
			Description: "add channel-level model price component overrides",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModelPriceComponent{})
			},
		},
		{
			Version:     "202605041030_provider_model_supported_endpoints",
			Description: "add provider model supported endpoints as channel endpoint candidates",
			Up: func(tx *gorm.DB) error {
				if err := upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...); err != nil {
					return err
				}
				channelIDs := make([]string, 0)
				if err := tx.Model(&Channel{}).Distinct("id").Pluck("id", &channelIDs).Error; err != nil {
					return err
				}
				for _, channelID := range channelIDs {
					rows, err := listChannelModelRowsByChannelIDWithDB(tx, channelID)
					if err != nil {
						return err
					}
					if err := SyncChannelModelEndpointsWithDB(tx, channelID, rows); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605051030_openai_text_model_endpoint_candidates",
			Description: "backfill openai text provider models with responses and chat completion endpoint candidates",
			Up: func(tx *gorm.DB) error {
				if err := upsertProviderMigrationProvidersWithDB(tx, "openai"); err != nil {
					return err
				}
				return backfillOpenAITextProviderModelEndpointCandidatesWithDB(tx)
			},
		},
		{
			Version:     "202605051130_drop_provider_model_capabilities",
			Description: "drop provider model capabilities column in favor of model type",
			Up: func(tx *gorm.DB) error {
				return dropProviderModelCapabilitiesWithDB(tx)
			},
		},
		{
			Version:     "202605051330_repair_openai_text_model_endpoint_candidates",
			Description: "repair openai text provider model endpoint candidates to include responses and chat completions",
			Up: func(tx *gorm.DB) error {
				return backfillOpenAITextProviderModelEndpointCandidatesWithDB(tx)
			},
		},
		{
			Version:     "202605061700_channel_endpoint_policy_template_key",
			Description: "add template key column to channel endpoint policies",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModelEndpointPolicy{})
			},
		},
		{
			Version:     "202605061830_drop_legacy_and_reject_endpoint_policies",
			Description: "delete deprecated drop_fields and reject_unsupported_input endpoint policies",
			Up: func(tx *gorm.DB) error {
				if err := tx.Where("template_key IN ?", []string{
					"DROP_LEGACY_PENALTIES",
					"REJECT_ANTHROPIC_IMAGE_URL",
				}).Delete(&ChannelModelEndpointPolicy{}).Error; err != nil {
					return err
				}
				return tx.Where(
					"request_policy LIKE ? OR request_policy LIKE ?",
					"%\"type\":\"drop_fields\"%",
					"%\"type\":\"reject_unsupported_input\"%",
				).Delete(&ChannelModelEndpointPolicy{}).Error
			},
		},
		{
			Version:     "202605071030_group_channel_bindings",
			Description: "add canonical group channels table and backfill from current runtime group model routes",
			Up: func(tx *gorm.DB) error {
				return migrateGroupChannelsWithDB(tx)
			},
		},
		{
			Version:     "202605161130_rename_group_channel_bindings_to_group_channels",
			Description: "rename group channel bindings table to group channels",
			Up: func(tx *gorm.DB) error {
				return renameLegacyGroupChannelsTableWithDB(tx)
			},
		},
		{
			Version:     "202605071330_ability_provider_and_drop_group_model_providers",
			Description: "store provider on runtime group model routes, backfill from channel/provider datas, and drop group model providers table",
			Up: func(tx *gorm.DB) error {
				if err := backfillGroupModelChannelProviderFromChannelModelsWithDB(tx); err != nil {
					return err
				}
				return dropGroupModelProvidersTableWithDB(tx)
			},
		},
		{
			Version:     "202605071530_group_models",
			Description: "add canonical group models table and backfill from current runtime group model routes",
			Up: func(tx *gorm.DB) error {
				return migrateGroupModelsWithDB(tx)
			},
		},
		{
			Version:     "202605071700_group_model_routes",
			Description: "rename runtime group model channel table to group model routes",
			Up: func(tx *gorm.DB) error {
				return migrateGroupModelRoutesTableWithDB(tx)
			},
		},
		{
			Version:     "202605171900_group_model_channels",
			Description: "rename runtime group model routes table to group model channels",
			Up: func(tx *gorm.DB) error {
				return migrateGroupModelChannelsTableWithDB(tx)
			},
		},
		{
			Version:     "202605071830_channel_endpoint_policy_unique_index",
			Description: "add unique index for channel endpoint policy upsert key",
			Up: func(tx *gorm.DB) error {
				if err := tx.Exec(`
					CREATE UNIQUE INDEX IF NOT EXISTS uniq_channel_model_endpoint_policy
					ON channel_model_endpoint_policies (channel_id, model, endpoint)
				`).Error; err != nil {
					return err
				}
				return tx.AutoMigrate(&ChannelModelEndpointPolicy{})
			},
		},
		{
			Version:     "202605101700_channel_api_base_url_backfill",
			Description: "backfill channel config.api_base_url from legacy channels.base_url",
			Up: func(tx *gorm.DB) error {
				rows := make([]Channel, 0)
				if err := tx.Select("id", "base_url", "config").Find(&rows).Error; err != nil {
					return err
				}
				for _, row := range rows {
					legacyBaseURL := normalizeConfiguredBaseURL(row.GetBaseURL())
					if legacyBaseURL == "" {
						continue
					}
					cfg, err := row.LoadConfig()
					if err != nil {
						return err
					}
					if cfg.GetAPIBaseURL() != "" {
						continue
					}
					cfg.APIBaseURL = legacyBaseURL
					raw, err := json.Marshal(cfg)
					if err != nil {
						return err
					}
					if err := tx.Model(&Channel{}).
						Where("id = ?", row.Id).
						Update("config", strings.TrimSpace(string(raw))).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605101900_channel_endpoint_base_url_table",
			Description: "move channel endpoint base urls from channel config into channel_model_endpoints.base_url",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ChannelModelEndpoint{}); err != nil {
					return err
				}
				type legacyChannelConfig struct {
					Region            string            `json:"region,omitempty"`
					SK                string            `json:"sk,omitempty"`
					AK                string            `json:"ak,omitempty"`
					UserID            string            `json:"user_id,omitempty"`
					APIVersion        string            `json:"api_version,omitempty"`
					LibraryID         string            `json:"library_id,omitempty"`
					Plugin            string            `json:"plugin,omitempty"`
					APIBaseURL        string            `json:"api_base_url,omitempty"`
					AccountBaseURL    string            `json:"account_base_url,omitempty"`
					EndpointBaseURLs  map[string]string `json:"endpoint_base_urls,omitempty"`
					VertexAIProjectID string            `json:"vertex_ai_project_id,omitempty"`
					VertexAIADC       string            `json:"vertex_ai_adc,omitempty"`
				}
				channels := make([]Channel, 0)
				if err := tx.Select("id", "config").Find(&channels).Error; err != nil {
					return err
				}
				for _, channel := range channels {
					if strings.TrimSpace(channel.Config) == "" {
						continue
					}
					cfg := legacyChannelConfig{}
					if err := json.Unmarshal([]byte(channel.Config), &cfg); err != nil {
						return err
					}
					if len(cfg.EndpointBaseURLs) == 0 {
						continue
					}
					rows, err := listChannelModelEndpointRowsByChannelIDWithDB(tx, channel.Id)
					if err != nil {
						return err
					}
					updated := false
					for i := range rows {
						if baseURL, ok := cfg.EndpointBaseURLs[NormalizeRequestedChannelModelEndpoint(rows[i].Endpoint)]; ok {
							normalizedBaseURL := normalizeConfiguredBaseURL(baseURL)
							if normalizedBaseURL != "" && normalizeConfiguredBaseURL(rows[i].BaseURL) != normalizedBaseURL {
								rows[i].BaseURL = normalizedBaseURL
								updated = true
							}
						}
					}
					if updated {
						if err := replaceChannelModelEndpointRowsWithDB(tx, channel.Id, rows); err != nil {
							return err
						}
					}
					cfg.EndpointBaseURLs = nil
					raw, err := json.Marshal(cfg)
					if err != nil {
						return err
					}
					if err := tx.Model(&Channel{}).
						Where("id = ?", channel.Id).
						Update("config", strings.TrimSpace(string(raw))).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605102100_provider_model_descriptions",
			Description: "upsert provider model descriptions from migration data",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605111030_refresh_provider_model_descriptions_and_defaults",
			Description: "refresh provider model descriptions from official model catalogs and add newly tracked migration models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605111130_refresh_openai_and_xai_official_models",
			Description: "refresh openai and xai official model descriptions and add newly tracked openai video models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, openAIAndXAIProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605111210_refresh_retired_anthropic_model_descriptions",
			Description: "clear descriptions for retired anthropic models while keeping catalog rows for backward compatibility",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "anthropic")
			},
		},
		{
			Version:     "202605111330_refresh_retired_google_and_qwen_model_descriptions",
			Description: "clear descriptions for retired or stopped-updating google and qwen models while keeping catalog rows for backward compatibility",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, googleAndQwenProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605111430_provider_model_soft_delete_flag",
			Description: "add provider model soft delete flag and mark upstream-retired migration models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605111530_provider_model_official_status",
			Description: "add provider model official status and mark deprecated migration models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605141130_refresh_default_provider_model_pricing",
			Description: "upsert provider migration pricing for newly priced official models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605141330_refresh_component_based_provider_model_pricing",
			Description: "upsert component-based provider migration pricing for complex official models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605141430_refresh_tiered_provider_model_pricing",
			Description: "upsert tiered provider migration pricing details",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, allProviderMigrationCatalogProviders...)
			},
		},
		{
			Version:     "202605141530_refresh_google_multimodal_provider_model_pricing",
			Description: "upsert google multimodal provider migration pricing components",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "google")
			},
		},
		{
			Version:     "202605141630_refresh_minimax_provider_model_pricing",
			Description: "upsert minimax provider migration pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "minimax")
			},
		},
		{
			Version:     "202605141730_refresh_zhipu_provider_model_pricing",
			Description: "upsert zhipu provider migration pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202605141830_refresh_hunyuan_provider_model_pricing",
			Description: "upsert hunyuan provider migration pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "hunyuan")
			},
		},
		{
			Version:     "202605142030_add_volcengine_doubao_provider_catalog",
			Description: "upsert volcengine doubao provider migration models and pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "volcengine")
			},
		},
		{
			Version:     "202605142130_refresh_volcengine_doubao_thinking_pricing",
			Description: "upsert volcengine doubao thinking provider migration pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "volcengine")
			},
		},
		{
			Version:     "202605142230_add_embedding_model_type_and_volcengine_seed_embedding",
			Description: "upsert embedding model type support and volcengine seed embedding model",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "volcengine")
			},
		},
		{
			Version:     "202605151030_refresh_gpt_image2_endpoint_candidates",
			Description: "upsert openai gpt-image-2 provider endpoint candidates",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "openai")
			},
		},
		{
			Version:     "202605161200_channel_model_observation_facts",
			Description: "add channel model sync fact table and endpoint test fact table without mutating manual channel config",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModelSyncResult{}, &ChannelModelEndpointTestResult{})
			},
		},
		{
			Version:     "202605161330_channel_model_sync_results_returned_column",
			Description: "rename channel model sync result observed column to returned",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ChannelModelSyncResult{}); err != nil {
					return err
				}
				if tx.Migrator().HasColumn(&ChannelModelSyncResult{}, "observed") && !tx.Migrator().HasColumn(&ChannelModelSyncResult{}, "returned") {
					if err := tx.Migrator().RenameColumn(&ChannelModelSyncResult{}, "observed", "returned"); err != nil {
						return err
					}
				}
				return tx.AutoMigrate(&ChannelModelSyncResult{})
			},
		},
		{
			Version:     "202605171930_backfill_channel_endpoint_test_results",
			Description: "backfill latest endpoint test facts from channel test history",
			Up: func(tx *gorm.DB) error {
				return BackfillChannelModelEndpointTestResultsFromChannelTestsWithDB(tx)
			},
		},
		{
			Version:     "202605191030_channel_billing_resources",
			Description: "add channel billing profile, snapshot, and action tables",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelBillingProfile{}, &ChannelBillingSnapshot{}, &ChannelBillingAction{})
			},
		},
		{
			Version:     "202605191230_channel_billing_snapshot_items",
			Description: "add channel billing snapshot item table and backfill explicit billing profiles",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ChannelBillingSnapshotItem{}); err != nil {
					return err
				}
				rows := make([]Channel, 0)
				if err := tx.Find(&rows).Error; err != nil {
					return err
				}
				for _, row := range rows {
					if _, err := GetChannelBillingProfileByChannelIDWithDB(tx, row.Id); err == nil {
						continue
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
					profile, ok := BuildChannelBillingProfileFromChannelConfig(&row)
					if !ok {
						continue
					}
					if _, err := SaveChannelBillingProfileWithDB(tx, profile); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605191430_channel_billing_fetch_config_api_base_url",
			Description: "backfill billing fetch api base url from legacy account base url config",
			Up: func(tx *gorm.DB) error {
				rows := make([]Channel, 0)
				if err := tx.Find(&rows).Error; err != nil {
					return err
				}
				for _, row := range rows {
					profile, err := GetChannelBillingProfileByChannelIDWithDB(tx, row.Id)
					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							continue
						}
						return err
					}
					if strings.TrimSpace(profile.BillingConfig) != "" {
						continue
					}
					if strings.TrimSpace(profile.BillingMode) == ChannelBillingModeManual {
						continue
					}
					cfg, configErr := row.LoadConfig()
					if configErr != nil {
						continue
					}
					accountBaseURL := cfg.GetAccountBaseURL()
					if accountBaseURL == "" || deriveChannelActivateURLTemplate(accountBaseURL) != "" {
						continue
					}
					profile.BillingConfig = marshalJSONString(channelBillingConfig{
						APIBaseURL: accountBaseURL,
					})
					if _, err := SaveChannelBillingProfileWithDB(tx, profile); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605200930_drop_channel_balance_columns",
			Description: "drop legacy channel balance columns",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&Channel{}, "balance_updated_time") {
					if err := tx.Migrator().DropColumn(&Channel{}, "balance_updated_time"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&Channel{}, "balance") {
					if err := tx.Migrator().DropColumn(&Channel{}, "balance"); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Version:     "202605201030_rename_channel_billing_task_and_capabilities",
			Description: "rename channel billing task type and billing capability values",
			Up: func(tx *gorm.DB) error {
				if err := tx.Model(&AsyncTask{}).
					Where("type = ?", "channel_refresh_balance").
					Update("type", AsyncTaskTypeChannelRefreshBilling).Error; err != nil {
					return err
				}
				if err := tx.Exec(
					"UPDATE "+AdminTasksTableName+
						" SET dedupe_key = REPLACE(dedupe_key, ?, ?), result = REPLACE(result, ?, ?)"+
						" WHERE dedupe_key LIKE ? OR result LIKE ?",
					"channel_refresh_balance:",
					AsyncTaskTypeChannelRefreshBilling+":",
					"\"balance_urls\":",
					"\"billing_request_urls\":",
					"channel_refresh_balance:%",
					"%\"balance_urls\":%",
				).Error; err != nil {
					return err
				}
				if err := tx.Exec(
					"UPDATE "+ChannelBillingProfilesTableName+
						" SET action_capabilities = REPLACE(REPLACE(action_capabilities, ?, ?), ?, ?)"+
						" WHERE action_capabilities LIKE ? OR action_capabilities LIKE ?",
					"refresh_balance",
					ChannelBillingCapabilityRefreshBilling,
					"manual_update_balance",
					ChannelBillingCapabilityManualUpdateSnapshot,
					"%refresh_balance%",
					"%manual_update_balance%",
				).Error; err != nil {
					return err
				}
				if err := tx.Model(&ChannelBillingAction{}).
					Where("action_type = ?", "manual_update_balance").
					Update("action_type", ChannelBillingActionTypeManualUpdateSnapshot).Error; err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version:     "202605201130_rename_channel_billing_profile_columns",
			Description: "rename channel billing profile mode and config columns",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasColumn(&ChannelBillingProfile{}, "balance_fetch_mode") &&
					!tx.Migrator().HasColumn(&ChannelBillingProfile{}, "billing_mode") {
					if err := tx.Migrator().RenameColumn(&ChannelBillingProfile{}, "balance_fetch_mode", "billing_mode"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&ChannelBillingProfile{}, "balance_fetch_config") &&
					!tx.Migrator().HasColumn(&ChannelBillingProfile{}, "billing_config") {
					if err := tx.Migrator().RenameColumn(&ChannelBillingProfile{}, "balance_fetch_config", "billing_config"); err != nil {
						return err
					}
				}
				return tx.AutoMigrate(&ChannelBillingProfile{})
			},
		},
		{
			Version:     "202605231230_provider_migration_owned_qwen_data",
			Description: "normalize provider migration sources and upsert current qwen provider data",
			Up: func(tx *gorm.DB) error {
				if err := normalizeProviderMigrationLegacySourcesWithDB(tx); err != nil {
					return err
				}
				if err := normalizeProviderPricingLegacySourcesWithDB(tx); err != nil {
					return err
				}
				if err := upsertProviderMigrationProvidersWithDB(tx, "qwen"); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version:     "202605231330_qwen_provider_response_and_image_endpoints",
			Description: "upsert qwen provider response and image endpoint support",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "qwen")
			},
		},
		{
			Version:     "202605231430_qwen_provider_image_edit_endpoint",
			Description: "upsert qwen provider image edit endpoint support",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "qwen")
			},
		},
		{
			Version:     "202605231500_qwen_image_provider_pricing",
			Description: "upsert qwen image provider per-image pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "qwen")
			},
		},
		{
			Version:     "202605231620_provider_model_tags",
			Description: "add provider model tags and backfill from existing model classification",
			Up: func(tx *gorm.DB) error {
				return backfillProviderModelTagsWithDB(tx)
			},
		},
		{
			Version:     "202605251930_channel_protocol_ali_label",
			Description: "rename ali channel protocol label to Ali",
			Up: func(tx *gorm.DB) error {
				return tx.Model(&ChannelProtocolCatalog{}).
					Where("name = ?", "ali").
					Updates(map[string]any{
						"label":      "Ali",
						"updated_at": helper.GetTimestamp(),
					}).Error
			},
		},
		{
			Version:     "202605261030_channel_billing_entitlement_framework",
			Description: "upgrade channel billing items to standard entitlement fields and add alert events",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelBillingSnapshotItem{}, &ChannelBillingAlertEvent{})
			},
		},
		{
			Version:     "202605261230_refresh_deepseek_provider_catalog",
			Description: "upsert deepseek provider migration rows for v4 models and current aliases",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "deepseek")
			},
		},
		{
			Version:     "202605261700_deepseek_messages_endpoint_support",
			Description: "refresh deepseek provider migration rows to expose chat and messages endpoints",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "deepseek")
			},
		},
		{
			Version:     "202606091030_deepseek_reasoning_model_tags",
			Description: "mark deepseek reasoning-capable provider models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "deepseek")
			},
		},
		{
			Version:     "202605271030_channel_capability_disable_observability",
			Description: "add runtime disable metadata to channel model and endpoint capabilities",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelModel{}, &ChannelModelEndpoint{})
			},
		},
		{
			Version:     "202605271130_channel_circuit_breaker_state",
			Description: "add persisted channel circuit breaker state for metric auto recovery",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelCircuitBreakerState{})
			},
		},
		{
			Version:     "202605291030_channel_circuit_breaker_events",
			Description: "add channel circuit breaker event history",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelCircuitBreakerEvent{})
			},
		},
		{
			Version:     "202605291100_redemption_issue_audit_logs",
			Description: "add redemption issue audit logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&RedemptionIssueAuditLog{})
			},
		},
		{
			Version:     "202606051130_anthropic_claude48_provider_catalog",
			Description: "upsert anthropic provider migration rows for claude opus 4.8 pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "anthropic")
			},
		},
		{
			Version:     "202606091030_add_qwen37_plus_provider_model",
			Description: "upsert qwen3.7-plus official provider model and tiered pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "qwen")
			},
		},
		{
			Version:     "202606091130_group_channel_billing_ratio",
			Description: "add group channel billing ratio and backfill from group ratio",
			Up: func(tx *gorm.DB) error {
				return backfillGroupChannelBillingRatioWithDB(tx)
			},
		},
		{
			Version:     "202606101000_api_tokens_updated_time",
			Description: "add token updated_time and backfill existing rows from created_time",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&Token{}); err != nil {
					return err
				}
				return tx.Exec(
					"UPDATE api_tokens SET updated_time = COALESCE(NULLIF(created_time, 0), ?) WHERE COALESCE(updated_time, 0) = 0",
					helper.GetTimestamp(),
				).Error
			},
		},
		{
			Version:     "202606111030_refresh_zhipu_provider_models",
			Description: "upsert zhipu official provider models and pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606111130_zhipu_messages_endpoint_support",
			Description: "upsert zhipu official messages endpoint support for claude-compatible models",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606111140_channel_protocol_zhipu_label",
			Description: "rename zhipu channel protocol label to ChatGLM",
			Up: func(tx *gorm.DB) error {
				return tx.Model(&ChannelProtocolCatalog{}).
					Where("name = ?", "zhipu").
					Updates(map[string]any{
						"label":      "ChatGLM",
						"updated_at": helper.GetTimestamp(),
					}).Error
			},
		},
		{
			Version:     "202606111150_zhipu_channel_endpoint_baseline_cleanup",
			Description: "rebuild zhipu channel endpoints from official provider endpoint baseline",
			Up: func(tx *gorm.DB) error {
				return cleanupChannelEndpointBaselineWithDB(tx, "zhipu", "zhipu", true)
			},
		},
		{
			Version:     "202606111160_qwen_channel_endpoint_baseline_cleanup",
			Description: "rebuild qwen channel endpoints from official provider endpoint baseline",
			Up: func(tx *gorm.DB) error {
				if err := upsertProviderMigrationProvidersWithDB(tx, "qwen"); err != nil {
					return err
				}
				return cleanupChannelEndpointBaselineWithDB(tx, "ali", "qwen", false)
			},
		},
		{
			Version:     "202606111210_refresh_volcengine_provider_models",
			Description: "refresh volcengine official provider models and channel endpoint baseline",
			Up: func(tx *gorm.DB) error {
				if err := upsertProviderMigrationProvidersWithDB(tx, "volcengine"); err != nil {
					return err
				}
				if err := renameVolcengineOldModelNamesWithDB(tx); err != nil {
					return err
				}
				return cleanupChannelEndpointBaselineWithDB(tx, "doubao", "volcengine", false)
			},
		},
		{
			Version:     "202606111230_channel_protocol_volcengine_label",
			Description: "rename doubao channel protocol label to Volcengine",
			Up: func(tx *gorm.DB) error {
				return tx.Model(&ChannelProtocolCatalog{}).
					Where("name = ?", "doubao").
					Updates(map[string]any{
						"label":       "Volcengine",
						"description": "Volcengine Ark",
						"updated_at":  helper.GetTimestamp(),
					}).Error
			},
		},
		{
			Version:     "202606151030_add_zhipu_glm52_provider_model",
			Description: "upsert zhipu official glm-5.2 provider model",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606171030_procurement_cost_tables",
			Description: "add procurement cost batch and request consumption tables",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelProcurementBatch{}, &RequestProcurementConsumption{})
			},
		},
		{
			Version:     "202606181030_channel_billing_purchase_fields",
			Description: "add purchase fields to channel billing snapshots",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&ChannelBillingSnapshot{})
			},
		},
		{
			Version:     "202606191030_refresh_zhipu_glm52_pricing",
			Description: "refresh zhipu provider migration data for glm-5.2 pricing and official metadata",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606191100_refresh_zhipu_glm_image_pricing",
			Description: "refresh zhipu provider migration data for glm-image pricing",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606191140_provider_model_specification",
			Description: "add provider model specification field and refresh zhipu official image usage spec",
			Up: func(tx *gorm.DB) error {
				if err := tx.AutoMigrate(&ProviderModel{}); err != nil {
					return err
				}
				return upsertProviderMigrationProvidersWithDB(tx, "zhipu")
			},
		},
		{
			Version:     "202606191230_refresh_provider_image_model_specifications",
			Description: "refresh official image model specifications for supported providers",
			Up: func(tx *gorm.DB) error {
				return upsertProviderMigrationProvidersWithDB(
					tx,
					"openai",
					"google",
					"hunyuan",
					"minimax",
					"qwen",
					"stepfun",
					"volcengine",
					"zhipu",
				)
			},
		},
	}
	return runVersionedMigrations(db, migrationScopeMain, migrations)
}

func volcengineOldModelNameToOfficialModelMap() map[string]string {
	return map[string]string{
		"doubao-seed-2.0-pro":          "doubao-seed-2-0-pro-260215",
		"doubao-seed-2.0-lite":         "doubao-seed-2-0-lite-260428",
		"doubao-seed-2.0-mini":         "doubao-seed-2-0-mini-260428",
		"doubao-seed-2.0-code":         "doubao-seed-2-0-code-preview-260215",
		"doubao-seed-1.8":              "doubao-seed-1-8-251228",
		"doubao-seed-1.6-vision":       "doubao-seed-1-6-vision-250815",
		"doubao-seed-code":             "doubao-seed-code-preview-251028",
		"doubao-seed-translation":      "doubao-seed-translation-250915",
		"doubao-seed-character":        "doubao-seed-character-251128",
		"doubao-embedding-vision":      "doubao-embedding-vision-251215",
		"doubao-seedream-5.0-lite":     "doubao-seedream-5-0-lite-260128",
		"doubao-seedream-4.5":          "doubao-seedream-4-5-251128",
		"doubao-seedream-4.0":          "doubao-seedream-4-0-250828",
		"doubao-seedance-2.0":          "doubao-seedance-2-0-260128",
		"doubao-seedance-2.0-fast":     "doubao-seedance-2-0-fast-260128",
		"doubao-seedance-1.5-pro":      "doubao-seedance-1-5-pro-251215",
		"doubao-seedance-1.0-pro":      "doubao-seedance-1-0-pro-250528",
		"doubao-seedance-1.0-pro-fast": "doubao-seedance-1-0-pro-fast-251015",
	}
}

func renameVolcengineOldModelNamesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	for oldModelName, officialModel := range volcengineOldModelNameToOfficialModelMap() {
		for _, tableName := range []string{
			ChannelModelsTableName,
			ChannelModelEndpointsTableName,
			ChannelModelEndpointTestResultsTableName,
			ChannelModelSyncResultsTableName,
			ChannelTestsTableName,
			ChannelModelPriceComponentsTableName,
			ChannelModelEndpointPoliciesTableName,
		} {
			if err := db.Table(tableName).
				Where("channel_id IN (?)", db.Model(&Channel{}).Select("id").Where("protocol = ?", "doubao")).
				Where("model = ?", oldModelName).
				Updates(map[string]any{"model": officialModel}).Error; err != nil {
				return err
			}
		}
		for _, tableName := range []string{
			ChannelModelsTableName,
			ChannelModelEndpointTestResultsTableName,
			ChannelModelSyncResultsTableName,
			ChannelTestsTableName,
		} {
			if err := db.Table(tableName).
				Where("channel_id IN (?)", db.Model(&Channel{}).Select("id").Where("protocol = ?", "doubao")).
				Where("upstream_model = ? OR upstream_model = ?", oldModelName, officialModel).
				Updates(map[string]any{"upstream_model": officialModel}).Error; err != nil {
				return err
			}
		}
		if err := db.Table(GroupModelChannelsTableName).
			Where("provider = ?", "volcengine").
			Where("upstream_model = ? OR upstream_model = ?", oldModelName, officialModel).
			Updates(map[string]any{"upstream_model": officialModel}).Error; err != nil {
			return err
		}
		for _, tableName := range []string{
			GroupModelsTableName,
			GroupModelChannelsTableName,
		} {
			if err := db.Table(tableName).
				Where("provider = ?", "volcengine").
				Where("model = ?", oldModelName).
				Updates(map[string]any{"model": officialModel}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func cleanupChannelEndpointBaselineWithDB(db *gorm.DB, channelProtocol string, provider string, assignAllRows bool) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedProtocol := strings.TrimSpace(strings.ToLower(channelProtocol))
	normalizedProvider := NormalizeGroupModelProviderValue(provider)
	if normalizedProtocol == "" || normalizedProvider == "" {
		return nil
	}
	channels := make([]Channel, 0)
	if err := db.
		Select("id").
		Where("protocol = ?", normalizedProtocol).
		Find(&channels).Error; err != nil {
		return err
	}
	for _, channel := range channels {
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		rows, err := listChannelModelRowsByChannelIDWithDB(db, channelID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			continue
		}
		modelNames := channelModelProviderLookupCandidates(rows)
		endpointsByModel, err := LoadProviderModelEndpointMapByModelsWithDB(db, normalizedProvider, modelNames)
		if err != nil {
			return err
		}
		if len(endpointsByModel) == 0 {
			continue
		}
		providerEndpoints := make(map[string][]string, len(endpointsByModel))
		for modelName, endpoints := range endpointsByModel {
			key := buildProviderModelEndpointKey(normalizedProvider, modelName)
			if key == "" {
				continue
			}
			providerEndpoints[key] = endpoints
		}
		now := helper.GetTimestamp()
		changed := false
		for idx := range rows {
			if !assignAllRows && !channelModelMatchesProviderEndpointBaseline(rows[idx], normalizedProvider, providerEndpoints) {
				continue
			}
			rows[idx].Provider = normalizedProvider
			supportedEndpoints := resolveProviderEndpointCandidatesForChannelModel(rows[idx], providerEndpoints)
			currentEndpoint := NormalizeRequestedChannelModelEndpoint(rows[idx].Endpoint)
			if len(supportedEndpoints) > 0 && !channelModelEndpointInSet(currentEndpoint, supportedEndpoints) {
				rows[idx].Endpoint = supportedEndpoints[0]
			}
			changed = true
			if err := db.Model(&ChannelModel{}).
				Where("channel_id = ? AND model = ?", channelID, rows[idx].Model).
				Updates(map[string]any{
					"provider":   rows[idx].Provider,
					"endpoint":   rows[idx].Endpoint,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}
		if !changed {
			continue
		}
		if err := SyncChannelModelEndpointsWithDB(db, channelID, rows); err != nil {
			return err
		}
	}
	return nil
}

func channelModelMatchesProviderEndpointBaseline(row ChannelModel, provider string, providerEndpoints map[string][]string) bool {
	if NormalizeGroupModelProviderValue(row.Provider) == NormalizeGroupModelProviderValue(provider) {
		return true
	}
	normalized := row
	normalizeChannelModelRow(&normalized)
	for _, modelName := range NormalizeProviderLookupCandidates(normalized.Model, normalized.UpstreamModel) {
		key := buildProviderModelEndpointKey(provider, modelName)
		if len(providerEndpoints[key]) > 0 {
			return true
		}
	}
	return false
}

func channelModelEndpointInSet(endpoint string, candidates []string) bool {
	normalizedEndpoint := NormalizeRequestedChannelModelEndpoint(endpoint)
	if normalizedEndpoint == "" {
		return false
	}
	for _, candidate := range candidates {
		if normalizedEndpoint == NormalizeRequestedChannelModelEndpoint(candidate) {
			return true
		}
	}
	return false
}

func backfillProviderModelTagsWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&ProviderModel{}); err != nil {
		return err
	}
	type providerModelTagBackfillRow struct {
		Provider string `gorm:"column:provider"`
		Model    string `gorm:"column:model"`
		Type     string `gorm:"column:type"`
		Tags     string `gorm:"column:tags"`
	}
	rows := make([]providerModelTagBackfillRow, 0)
	query := db.Table(ProviderModelsTableName).Select("provider", "model", "tags")
	if db.Migrator().HasColumn(ProviderModelsTableName, "type") {
		query = db.Table(ProviderModelsTableName).Select("provider", "model", "type", "tags")
	}
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	now := helper.GetTimestamp()
	for _, row := range rows {
		nextTags := joinProviderModelTags(row.Model, append([]string{row.Type}, splitProviderModelTags(row.Tags)...))
		if strings.TrimSpace(row.Tags) == nextTags {
			continue
		}
		if err := db.Model(&ProviderModel{}).
			Where("provider = ? AND model = ?", row.Provider, row.Model).
			Updates(map[string]interface{}{
				"tags":       nextTags,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(ProviderModelsTableName, "type") {
		if err := db.Migrator().DropColumn(ProviderModelsTableName, "type"); err != nil {
			return err
		}
	}
	return nil
}

func backfillOpenAITextProviderModelEndpointCandidatesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&ProviderModel{}); err != nil {
		return err
	}
	rows := make([]ProviderModel, 0)
	if err := db.
		Where("provider = ?", "openai").
		Find(&rows).Error; err != nil {
		return err
	}
	now := helper.GetTimestamp()
	for _, row := range rows {
		if ProviderModelTypeFromTags(splitProviderModelTags(row.Tags)) != ProviderModelTypeText {
			continue
		}
		nextEndpoints := openAITextProviderModelEndpointCandidates(row.SupportedEndpoints)
		if strings.TrimSpace(row.SupportedEndpoints) == nextEndpoints {
			continue
		}
		if err := db.Model(&ProviderModel{}).
			Where("provider = ? AND model = ?", row.Provider, row.Model).
			Updates(map[string]interface{}{
				"supported_endpoints": nextEndpoints,
				"updated_at":          now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func openAITextProviderModelEndpointCandidates(raw string) string {
	endpoints := splitProviderModelSupportedEndpoints(raw)
	endpoints = append(endpoints, ChannelModelEndpointResponses, ChannelModelEndpointChat)
	return joinProviderModelSupportedEndpoints(ProviderModelTypeText, endpoints)
}

func runLogVersionedMigrations(db *gorm.DB) error {
	migrations := []versionedMigration{
		{
			Version:     "202603101930_log_baseline_v6",
			Description: "baseline: create current log schema",
			Up: func(tx *gorm.DB) error {
				return runLogBaselineMigrationWithDB(tx)
			},
		},
		{
			Version:     "202603201030_log_event_log_group_id",
			Description: "add group_id index column to event logs in log database",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202603311200_log_user_quota_usage_fields",
			Description: "add user quota usage fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202603311900_log_billing_snapshots",
			Description: "add billing snapshot fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202604031500_log_billing_source",
			Description: "add billing source field to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202605101230_log_billing_observability",
			Description: "add billing observability fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202605231130_log_normalize_provider_pricing_sources",
			Description: "rewrite legacy provider pricing source provider_default rows to provider_migration in log database",
			Up: func(tx *gorm.DB) error {
				return normalizeProviderPricingLegacySourcesWithDB(tx)
			},
		},
		{
			Version:     "202605281030_log_token_estimate_observability",
			Description: "add token estimate observability fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202605300830_log_token_estimate_observability_columns",
			Description: "add missing token estimate estimator fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
		{
			Version:     "202606171040_log_procurement_cost_snapshot",
			Description: "add procurement cost and gross margin fields to consume logs",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&Log{})
			},
		},
	}
	return runVersionedMigrations(db, migrationScopeLog, migrations)
}

func runVersionedMigrations(db *gorm.DB, scope string, migrations []versionedMigration) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if strings.TrimSpace(scope) == "" {
		return fmt.Errorf("migration scope cannot be empty")
	}
	keepVersions, err := configuredMigrationVersions(migrations)
	if err != nil {
		return err
	}
	// Run migrations without prepared statements. Schema changes can invalidate
	// cached plans for queries such as SELECT *, especially when columns are dropped.
	migrationDB := db.Session(&gorm.Session{
		NewDB:       true,
		PrepareStmt: false,
	})
	if err := migrationDB.AutoMigrate(&SchemaMigration{}); err != nil {
		return err
	}

	applied := make([]SchemaMigration, 0)
	if err := migrationDB.Where("scope = ?", scope).Find(&applied).Error; err != nil {
		return err
	}
	appliedSet := make(map[string]struct{}, len(applied))
	for _, item := range applied {
		appliedSet[item.Version] = struct{}{}
	}

	for _, migration := range migrations {
		if migration.Up == nil {
			return fmt.Errorf("migration %s has nil up function", migration.Version)
		}
		if _, ok := appliedSet[migration.Version]; ok {
			continue
		}

		logger.SysLogf("migration[%s] applying %s (%s)", scope, migration.Version, migration.Description)
		err := migrationDB.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}
			record := SchemaMigration{
				Scope:       scope,
				Version:     migration.Version,
				Description: migration.Description,
				AppliedAt:   helper.GetTimestamp(),
			}
			return tx.Create(&record).Error
		})
		if err != nil {
			return fmt.Errorf("migration[%s] failed at %s: %w", scope, migration.Version, err)
		}
		logger.SysLogf("migration[%s] applied %s", scope, migration.Version)
	}
	return cleanupObsoleteSchemaMigrations(migrationDB, scope, keepVersions)
}

func configuredMigrationVersions(migrations []versionedMigration) ([]string, error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migrations configured")
	}
	seen := make(map[string]struct{}, len(migrations))
	versions := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		version := strings.TrimSpace(migration.Version)
		if version == "" {
			return nil, fmt.Errorf("migration version cannot be empty")
		}
		if _, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version: %s", version)
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	return versions, nil
}

func cleanupObsoleteSchemaMigrations(db *gorm.DB, scope string, keepVersions []string) error {
	if db == nil {
		return nil
	}
	if len(keepVersions) == 0 {
		return fmt.Errorf("no migration versions configured for scope %s", scope)
	}
	result := db.Where("scope = ? AND version NOT IN ?", scope, keepVersions).Delete(&SchemaMigration{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.SysLogf("migration[%s] removed %d obsolete schema_migrations rows", scope, result.RowsAffected)
	}
	return nil
}
