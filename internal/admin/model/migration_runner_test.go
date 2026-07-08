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

func TestBackfillLogRouteModelNamesWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	rows := []Log{
		{Id: "legacy-empty", ModelName: "qwen3.7-plus"},
		{Id: "keep-request", ModelName: "qwen3.7-plus-2026-05-26", RequestModelName: "qwen3.7-plus"},
		{Id: "keep-actual", ModelName: "gpt-5", ActualModelName: "gpt-5-upstream"},
		{Id: "blank-model"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	if err := backfillLogRouteModelNamesWithDB(db); err != nil {
		t.Fatalf("backfill route model names: %v", err)
	}

	var legacy Log
	if err := db.First(&legacy, "id = ?", "legacy-empty").Error; err != nil {
		t.Fatalf("load legacy-empty: %v", err)
	}
	if legacy.RequestModelName != "qwen3.7-plus" || legacy.ActualModelName != "qwen3.7-plus" {
		t.Fatalf("legacy names request=%q actual=%q", legacy.RequestModelName, legacy.ActualModelName)
	}
	var keepRequest Log
	if err := db.First(&keepRequest, "id = ?", "keep-request").Error; err != nil {
		t.Fatalf("load keep-request: %v", err)
	}
	if keepRequest.RequestModelName != "qwen3.7-plus" || keepRequest.ActualModelName != "qwen3.7-plus-2026-05-26" {
		t.Fatalf("keep-request names request=%q actual=%q", keepRequest.RequestModelName, keepRequest.ActualModelName)
	}
	var keepActual Log
	if err := db.First(&keepActual, "id = ?", "keep-actual").Error; err != nil {
		t.Fatalf("load keep-actual: %v", err)
	}
	if keepActual.RequestModelName != "gpt-5" || keepActual.ActualModelName != "gpt-5-upstream" {
		t.Fatalf("keep-actual names request=%q actual=%q", keepActual.RequestModelName, keepActual.ActualModelName)
	}
	var blank Log
	if err := db.First(&blank, "id = ?", "blank-model").Error; err != nil {
		t.Fatalf("load blank-model: %v", err)
	}
	if blank.RequestModelName != "" || blank.ActualModelName != "" {
		t.Fatalf("blank-model names request=%q actual=%q", blank.RequestModelName, blank.ActualModelName)
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

func TestRemoveDefaultUserGroupAndLegacyBalanceSourcesWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&Option{},
		&User{},
		&GroupCatalog{},
		&TopupOrder{},
		&UserBalanceLot{},
		&UserBalanceLotTransaction{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&Option{Key: "DefaultUserGroup", Value: "group-1"}).Error; err != nil {
		t.Fatalf("create option: %v", err)
	}
	if err := db.Create(&GroupCatalog{Id: "group-1", Name: "Group 1", Enabled: true}).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&User{Id: "user-1", Username: "user-1", Group: "group-1", Status: UserStatusEnabled}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	lot := UserBalanceLot{
		Id:              "lot-1",
		UserID:          "user-1",
		SourceType:      "legacy_migration",
		SourceID:        "legacy-source-1",
		TotalAmount:     1000,
		UsedAmount:      200,
		RemainingAmount: 800,
		Status:          UserBalanceLotStatusActive,
		GrantedAt:       100,
		CreatedAt:       90,
	}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatalf("create lot: %v", err)
	}
	tx := UserBalanceLotTransaction{
		Id:                 "tx-1",
		UserID:             "user-1",
		LotID:              "lot-1",
		SourceType:         "legacy_migration",
		SourceID:           "legacy-source-1",
		TxType:             UserBalanceLotTxTypeCredit,
		DeltaAmount:        1000,
		LotRemainingBefore: 0,
		LotRemainingAfter:  1000,
		OccurredAt:         100,
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	if err := removeDefaultUserGroupAndLegacyBalanceSourcesWithDB(db); err != nil {
		t.Fatalf("remove legacy sources: %v", err)
	}

	optionCount := int64(0)
	if err := db.Model(&Option{}).Where("key = ?", "DefaultUserGroup").Count(&optionCount).Error; err != nil {
		t.Fatalf("count option: %v", err)
	}
	if optionCount != 0 {
		t.Fatalf("DefaultUserGroup option count=%d, want 0", optionCount)
	}
	migratedLot := UserBalanceLot{}
	if err := db.First(&migratedLot, "id = ?", "lot-1").Error; err != nil {
		t.Fatalf("load migrated lot: %v", err)
	}
	if migratedLot.SourceType != UserBalanceLotSourceTopup {
		t.Fatalf("lot source_type=%q, want %q", migratedLot.SourceType, UserBalanceLotSourceTopup)
	}
	if migratedLot.SourceID == "" || migratedLot.SourceID == "legacy-source-1" {
		t.Fatalf("lot source_id=%q, want generated topup order id", migratedLot.SourceID)
	}
	order := TopupOrder{}
	if err := db.First(&order, "id = ?", migratedLot.SourceID).Error; err != nil {
		t.Fatalf("load migration order: %v", err)
	}
	if order.BusinessType != TopupOrderBusinessBalance || order.Status != TopupOrderStatusFulfilled {
		t.Fatalf("order business/status=%q/%q, want balance fulfilled", order.BusinessType, order.Status)
	}
	if order.GroupID != "group-1" {
		t.Fatalf("order group_id=%q, want group-1", order.GroupID)
	}
	if order.CreditOrigin != TopupOrderCreditOriginReconcile {
		t.Fatalf("order credit_origin=%q, want %q", order.CreditOrigin, TopupOrderCreditOriginReconcile)
	}
	migratedTx := UserBalanceLotTransaction{}
	if err := db.First(&migratedTx, "id = ?", "tx-1").Error; err != nil {
		t.Fatalf("load migrated transaction: %v", err)
	}
	if migratedTx.SourceType != UserBalanceLotSourceTopup || migratedTx.SourceID != migratedLot.SourceID {
		t.Fatalf("tx source=%q/%q, want topup/%q", migratedTx.SourceType, migratedTx.SourceID, migratedLot.SourceID)
	}
}

func TestMigrateBalanceCreditOriginsReconcilesLegacyUserQuotaAsGiftLots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&User{},
		&TopupOrder{},
		&UserBalanceLot{},
		&UserBalanceLotTransaction{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Exec("ALTER TABLE users ADD COLUMN quota bigint DEFAULT 0").Error; err != nil {
		t.Fatalf("add legacy quota column: %v", err)
	}
	if err := db.Create(&User{Id: "user-legacy", Username: "legacy"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Exec("UPDATE users SET quota = ? WHERE id = ?", 100, "user-legacy").Error; err != nil {
		t.Fatalf("set legacy quota: %v", err)
	}
	existingOrder := TopupOrder{
		Id:            "order-existing",
		UserID:        "user-legacy",
		Username:      "legacy",
		Status:        TopupOrderStatusFulfilled,
		Source:        TopupOrderSourceTopUpAPI,
		ProviderName:  "admin",
		TransactionID: "existing-admin-order",
		BusinessType:  TopupOrderBusinessBalance,
		OperationType: TopupOrderOperationTopup,
		CreditOrigin:  TopupOrderCreditOriginAdmin,
		Title:         "existing admin balance",
		Quota:         40,
		CreatedAt:     100,
		UpdatedAt:     100,
	}
	if err := db.Create(&existingOrder).Error; err != nil {
		t.Fatalf("create existing order: %v", err)
	}
	if _, _, err := CreditUserBalanceLotWithDB(db, UserBalanceLotCreditInput{
		UserID:      "user-legacy",
		SourceType:  UserBalanceLotSourceTopup,
		SourceID:    existingOrder.Id,
		TotalAmount: 40,
		GrantedAt:   100,
		ExpiresAt:   0,
	}); err != nil {
		t.Fatalf("credit existing lot: %v", err)
	}

	if err := migrateBalanceCreditOriginsWithDB(db); err != nil {
		t.Fatalf("migrate balance origins: %v", err)
	}
	if err := migrateBalanceCreditOriginsWithDB(db); err != nil {
		t.Fatalf("migrate balance origins second run: %v", err)
	}

	var reconcileOrders []TopupOrder
	if err := db.Where("transaction_id = ?", "balance-reconciliation-user-legacy").Find(&reconcileOrders).Error; err != nil {
		t.Fatalf("find reconcile orders: %v", err)
	}
	if len(reconcileOrders) != 1 {
		t.Fatalf("reconcile order count=%d, want 1", len(reconcileOrders))
	}
	if reconcileOrders[0].CreditOrigin != TopupOrderCreditOriginReconcile {
		t.Fatalf("reconcile credit_origin=%q, want %q", reconcileOrders[0].CreditOrigin, TopupOrderCreditOriginReconcile)
	}
	if reconcileOrders[0].Quota != 60 {
		t.Fatalf("reconcile quota=%d, want 60", reconcileOrders[0].Quota)
	}
	var lots []UserBalanceLot
	if err := db.Where("source_type = ? AND source_id = ?", UserBalanceLotSourceTopup, reconcileOrders[0].Id).Find(&lots).Error; err != nil {
		t.Fatalf("find reconcile lots: %v", err)
	}
	if len(lots) != 1 {
		t.Fatalf("reconcile lot count=%d, want 1", len(lots))
	}
	if lots[0].RemainingAmount != 60 || lots[0].Status != UserBalanceLotStatusActive {
		t.Fatalf("reconcile lot remaining/status=%d/%q, want 60/%q", lots[0].RemainingAmount, lots[0].Status, UserBalanceLotStatusActive)
	}
}

func TestRemoveAutomaticChannelToggleOptionsWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := db.Create(&Option{Key: "AutomaticDisableChannelEnabled", Value: "true"}).Error; err != nil {
		t.Fatalf("create deprecated option: %v", err)
	}
	if err := db.Create(&Option{Key: "AutomaticEnableChannelEnabled", Value: "true"}).Error; err != nil {
		t.Fatalf("create deprecated option: %v", err)
	}
	if err := db.Create(&Option{Key: "ChannelDisableThreshold", Value: "5"}).Error; err != nil {
		t.Fatalf("create retained option: %v", err)
	}

	if err := removeAutomaticChannelToggleOptionsWithDB(db); err != nil {
		t.Fatalf("remove deprecated options: %v", err)
	}

	var removedCount int64
	if err := db.Model(&Option{}).Where("key IN ?", []string{"AutomaticDisableChannelEnabled", "AutomaticEnableChannelEnabled"}).Count(&removedCount).Error; err != nil {
		t.Fatalf("count removed options: %v", err)
	}
	if removedCount != 0 {
		t.Fatalf("deprecated automatic channel option count=%d, want 0", removedCount)
	}
	var retainedCount int64
	if err := db.Model(&Option{}).Where("key = ?", "ChannelDisableThreshold").Count(&retainedCount).Error; err != nil {
		t.Fatalf("count retained option: %v", err)
	}
	if retainedCount != 1 {
		t.Fatalf("ChannelDisableThreshold option count=%d, want 1", retainedCount)
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
