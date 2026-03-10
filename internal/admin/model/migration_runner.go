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
			Version:     "202603102345_main_baseline_v24",
			Description: "baseline: create current main schema, drop legacy channel test payload and channel model test summary columns, add channel model inactive state, and seed current catalogs",
			Up: func(tx *gorm.DB) error {
				return runMainBaselineMigrationWithDB(tx)
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
