package user

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWalletAddressQueriesAreCaseInsensitive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
	})

	storedAddress := "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if err := db.Create(&model.User{
		Id:            "wallet-user",
		Username:      "wallet_user",
		Password:      "password",
		WalletAddress: &storedAddress,
		Status:        model.UserStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if !IsWalletAddressAlreadyTaken(" 0xABCDEFabcdefABCDEFabcdefABCDEFabcdefABCD ") {
		t.Fatalf("mixed-case wallet address was not detected")
	}
	queryAddress := " 0xABCDEFabcdefABCDEFabcdefABCDEFabcdefABCD "
	found := model.User{WalletAddress: &queryAddress}
	if err := FillByWalletAddress(&found); err != nil {
		t.Fatalf("fill by mixed-case wallet address: %v", err)
	}
	if found.Id != "wallet-user" {
		t.Fatalf("found user=%s, want wallet-user", found.Id)
	}
}
