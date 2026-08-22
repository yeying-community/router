package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSyncIdentityEmailWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate user: %v", err)
	}
	user := User{Id: "user-1", Username: "test", Email: ""}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := SyncIdentityEmailWithDB(db, "user-1", " Person@Example.COM "); err != nil {
		t.Fatalf("sync email: %v", err)
	}
	stored := User{}
	if err := db.First(&stored, "id = ?", "user-1").Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if stored.Email != "person@example.com" {
		t.Fatalf("unexpected email: %s", stored.Email)
	}
}
