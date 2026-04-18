package model

import (
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
			Description: "baseline: create current main schema with user password-state flag, current task tables, and current provider catalog",
			Up: func(tx *gorm.DB) error {
				return runMainBaselineMigrationWithDB(tx)
			},
		},
		{
			Version:     "202603131030_openai_gpt51_provider_catalog",
			Description: "sync default provider catalog to add openai gpt-5.1 and gpt-5.1-codex pricing rows",
			Up: func(tx *gorm.DB) error {
				return syncDefaultProviderCatalogWithDB(tx)
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
			Description: "sync default provider catalog to add anthropic claude 4.6/4.5/3.5 pricing rows",
			Up: func(tx *gorm.DB) error {
				return syncDefaultProviderCatalogWithDB(tx)
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
			Description: "drop legacy is_stream_only column from channel model configs",
			Up: func(tx *gorm.DB) error {
				return dropChannelModelStreamOnlyWithDB(tx)
			},
		},
		{
			Version:     "202604161230_group_model_providers",
			Description: "add canonical group_model_providers table and backfill provider mapping from current abilities",
			Up: func(tx *gorm.DB) error {
				return migrateGroupModelProvidersWithDB(tx)
			},
		},
		{
			Version:     "202604161700_channel_model_provider_catalog_backfill",
			Description: "rebuild channel model provider from provider catalog unique matches and resync group model provider mapping",
			Up: func(tx *gorm.DB) error {
				return backfillChannelModelProviderFromCatalogWithDB(tx)
			},
		},
		{
			Version:     "202604161830_channel_model_provider_catalog_reconcile",
			Description: "reconcile channel/group model provider mappings strictly from provider catalog unique matches",
			Up: func(tx *gorm.DB) error {
				return backfillChannelModelProviderFromCatalogWithDB(tx)
			},
		},
		{
			Version:     "202604181030_drop_topup_redemption_link_columns",
			Description: "drop historical topup/redemption mutual reference columns",
			Up: func(tx *gorm.DB) error {
				return dropTopupRedemptionLinkColumnsWithDB(tx)
			},
		},
	}
	return runVersionedMigrations(db, migrationScopeMain, migrations)
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
