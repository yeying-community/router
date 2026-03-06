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
			Version:     "202603071300_main_baseline",
			Description: "baseline: create current main schema and seed initial catalogs",
			Up: func(tx *gorm.DB) error {
				return runMainBaselineMigrationWithDB(tx)
			},
		},
		{
			Version:     "202603071700_add_channel_capability_results",
			Description: "add channel capability result table",
			Up: func(tx *gorm.DB) error {
				return runChannelCapabilityResultsMigrationWithDB(tx)
			},
		},
		{
			Version:     "202603072100_drop_channel_capability_profiles_and_client_profiles",
			Description: "drop response whitelist tables no longer used",
			Up: func(tx *gorm.DB) error {
				return dropChannelCapabilityProfileTablesWithDB(tx)
			},
		},
		{
			Version:     "202603072130_simplify_channel_capability_results",
			Description: "drop legacy client profile fields from channel capability results",
			Up: func(tx *gorm.DB) error {
				return simplifyChannelCapabilityResultsWithDB(tx)
			},
		},
	}
	return runVersionedMigrations(db, migrationScopeMain, migrations)
}

func runLogVersionedMigrations(db *gorm.DB) error {
	migrations := []versionedMigration{
		{
			Version:     "202603070101_log_baseline",
			Description: "baseline: create current log schema",
			Up: func(tx *gorm.DB) error {
				return runLogBaselineMigrationWithDB(tx)
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
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return err
	}

	applied := make([]SchemaMigration, 0)
	if err := db.Where("scope = ?", scope).Find(&applied).Error; err != nil {
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
		err := db.Transaction(func(tx *gorm.DB) error {
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
	return nil
}
