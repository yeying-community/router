package model

import (
	"testing"

	"github.com/yeying-community/router/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureProcurementCostTablesWithDBRepairsMissingCostPerUnitColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE channel_procurement_batches (
			id char(36) PRIMARY KEY,
			channel_id char(36) NOT NULL,
			resource_type varchar(32) NOT NULL DEFAULT '',
			quota_type varchar(32) NOT NULL DEFAULT '',
			scope_type varchar(32) NOT NULL DEFAULT 'global',
			scope_value varchar(191) NOT NULL DEFAULT '',
			capacity_unit varchar(32) NOT NULL DEFAULT '',
			capacity_total double precision NOT NULL DEFAULT 0,
			capacity_effective double precision NOT NULL DEFAULT 0,
			capacity_remaining double precision NOT NULL DEFAULT 0,
			purchase_currency varchar(16) NOT NULL DEFAULT '',
			purchase_amount double precision NOT NULL DEFAULT 0,
			purchase_fx_rate double precision NOT NULL DEFAULT 0,
			purchase_cost_amount double precision NOT NULL DEFAULT 0,
			cost_source varchar(32) NOT NULL DEFAULT '',
			cost_status varchar(32) NOT NULL DEFAULT 'cost_unconfigured',
			valid_from bigint NOT NULL DEFAULT 0,
			expire_at bigint NOT NULL DEFAULT 0,
			reset_cycle varchar(32) NOT NULL DEFAULT 'none',
			source_snapshot_id char(36) NOT NULL DEFAULT '',
			source_snapshot_item_id char(36) NOT NULL DEFAULT '',
			source_ref varchar(191) NOT NULL DEFAULT '',
			metadata text,
			created_at bigint,
			updated_at bigint
		)
	`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if db.Migrator().HasColumn(&ChannelProcurementBatch{}, "CostPerUnitAmount") {
		t.Fatalf("legacy table unexpectedly has cost_per_unit_amount")
	}

	if err := ensureProcurementCostTablesWithDB(db); err != nil {
		t.Fatalf("ensure procurement tables: %v", err)
	}

	if !db.Migrator().HasColumn(&ChannelProcurementBatch{}, "CostPerUnitAmount") {
		t.Fatalf("cost_per_unit_amount column was not repaired")
	}
	if !db.Migrator().HasTable(&RequestProcurementConsumption{}) {
		t.Fatalf("request procurement consumption table was not created")
	}
}

func TestEnsureUserWalletAddressCaseInsensitiveUniqueCleansDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE users (
			id char(36) PRIMARY KEY,
			username varchar(255),
			password varchar(255),
			wallet_address varchar(128),
			role int,
			status int,
			created_at bigint,
			updated_at bigint
		)
	`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}
	rootWallets := config.RootWalletAddresses
	config.RootWalletAddresses = []string{"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"}
	t.Cleanup(func() {
		config.RootWalletAddresses = rootWallets
	})

	users := []User{
		{
			Id:            "root-user",
			Username:      "root_user",
			Password:      "password",
			WalletAddress: stringPtr(" 0xABCDEFabcdefABCDEFabcdefABCDEFabcdefABCD "),
			Role:          RoleCommonUser,
			Status:        UserStatusEnabled,
			CreatedAt:     5,
		},
		{
			Id:            "enabled-user",
			Username:      "enabled_user",
			Password:      "password",
			WalletAddress: stringPtr("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			Role:          RoleCommonUser,
			Status:        UserStatusEnabled,
			CreatedAt:     10,
		},
		{
			Id:            "disabled-user",
			Username:      "disabled_user",
			Password:      "password",
			WalletAddress: stringPtr("0xABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD"),
			Role:          RoleCommonUser,
			Status:        UserStatusDisabled,
			CreatedAt:     15,
		},
		{
			Id:            "single-user",
			Username:      "single_user",
			Password:      "password",
			WalletAddress: stringPtr(" 0x1111111111111111111111111111111111111111 "),
			Role:          RoleCommonUser,
			Status:        UserStatusEnabled,
			CreatedAt:     40,
		},
	}
	for _, user := range users {
		walletAddress := any(nil)
		if user.WalletAddress != nil {
			walletAddress = *user.WalletAddress
		}
		if err := db.Table("users").Create(map[string]any{
			"id":             user.Id,
			"username":       user.Username,
			"password":       user.Password,
			"wallet_address": walletAddress,
			"role":           user.Role,
			"status":         user.Status,
			"created_at":     user.CreatedAt,
			"updated_at":     user.UpdatedAt,
		}).Error; err != nil {
			t.Fatalf("create user %s: %v", user.Id, err)
		}
	}

	if err := ensureUserWalletAddressCaseInsensitiveUniqueWithDB(db); err != nil {
		t.Fatalf("ensure wallet uniqueness: %v", err)
	}

	var root User
	if err := db.First(&root, "id = ?", "root-user").Error; err != nil {
		t.Fatalf("load root user: %v", err)
	}
	if root.WalletAddress == nil || *root.WalletAddress != "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Fatalf("root wallet=%v, want normalized keeper wallet", root.WalletAddress)
	}
	for _, id := range []string{"enabled-user", "disabled-user"} {
		var duplicate User
		if err := db.First(&duplicate, "id = ?", id).Error; err != nil {
			t.Fatalf("load duplicate %s: %v", id, err)
		}
		if duplicate.WalletAddress != nil {
			t.Fatalf("duplicate %s wallet=%v, want nil", id, *duplicate.WalletAddress)
		}
	}
	var single User
	if err := db.First(&single, "id = ?", "single-user").Error; err != nil {
		t.Fatalf("load single user: %v", err)
	}
	if single.WalletAddress == nil || *single.WalletAddress != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("single wallet=%v, want normalized", single.WalletAddress)
	}
	var auditCount int64
	if err := db.Model(&WalletAddressCleanupAuditLog{}).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("audit count=%d, want 2", auditCount)
	}
}

func TestSelectWalletAddressDuplicateKeeperPrefersEnabledOldest(t *testing.T) {
	got := selectWalletAddressDuplicateKeeper([]walletAddressCleanupCandidate{
		{ID: "disabled-oldest", WalletAddress: "0x2222222222222222222222222222222222222222", Status: UserStatusDisabled, CreatedAt: 1},
		{ID: "enabled-newest", WalletAddress: "0x2222222222222222222222222222222222222222", Status: UserStatusEnabled, CreatedAt: 20},
		{ID: "enabled-oldest", WalletAddress: "0x2222222222222222222222222222222222222222", Status: UserStatusEnabled, CreatedAt: 10},
	})
	if got.ID != "enabled-oldest" {
		t.Fatalf("keeper=%s, want enabled-oldest", got.ID)
	}
}

func TestSelectWalletAddressDuplicateKeeperBreaksRootAddressTiesByStatusThenOldest(t *testing.T) {
	rootWallets := config.RootWalletAddresses
	config.RootWalletAddresses = []string{"0x3333333333333333333333333333333333333333"}
	t.Cleanup(func() {
		config.RootWalletAddresses = rootWallets
	})

	got := selectWalletAddressDuplicateKeeper([]walletAddressCleanupCandidate{
		{ID: "disabled-oldest", WalletAddress: "0x3333333333333333333333333333333333333333", Status: UserStatusDisabled, CreatedAt: 1},
		{ID: "enabled-newest", WalletAddress: "0x3333333333333333333333333333333333333333", Status: UserStatusEnabled, CreatedAt: 20},
		{ID: "enabled-oldest", WalletAddress: "0x3333333333333333333333333333333333333333", Status: UserStatusEnabled, CreatedAt: 10},
	})
	if got.ID != "enabled-oldest" {
		t.Fatalf("keeper=%s, want enabled-oldest when all rows have configured root wallet address", got.ID)
	}
}

func stringPtr(value string) *string {
	return &value
}
