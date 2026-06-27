package token

import (
	"testing"

	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTokenRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
	})
	return db
}

func TestUpdateInvalidatesTokenCache(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	if err := db.Create(&model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "before",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
		Models:      stringPtr("gpt-4o-mini"),
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	var invalidatedKey string
	previousInvalidate := invalidateTokenCacheFn
	invalidateTokenCacheFn = func(key string) error {
		invalidatedKey = key
		return nil
	}
	t.Cleanup(func() {
		invalidateTokenCacheFn = previousInvalidate
	})

	token := &model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "after",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 200,
		Models:      stringPtr("gpt-4o-mini,gpt-4.1"),
	}
	if err := Update(token); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if invalidatedKey != "secret-key-1" {
		t.Fatalf("invalidated key=%q, want secret-key-1", invalidatedKey)
	}

	stored, err := GetByID("token-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Name != "after" {
		t.Fatalf("stored name=%q, want after", stored.Name)
	}
	if stored.Models == nil || *stored.Models != "gpt-4o-mini,gpt-4.1" {
		t.Fatalf("stored models=%v, want updated models", stored.Models)
	}
}

func TestDeleteInvalidatesTokenCache(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secret-key-1",
		Name:        "alpha",
		Status:      model.TokenStatusEnabled,
		CreatedTime: 100,
		UpdatedTime: 100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	var invalidatedKey string
	previousInvalidate := invalidateTokenCacheFn
	invalidateTokenCacheFn = func(key string) error {
		invalidatedKey = key
		return nil
	}
	t.Cleanup(func() {
		invalidateTokenCacheFn = previousInvalidate
	})

	if err := Delete(token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if invalidatedKey != "secret-key-1" {
		t.Fatalf("invalidated key=%q, want secret-key-1", invalidatedKey)
	}

	if _, err := GetByID("token-1"); err == nil {
		t.Fatalf("expected deleted token lookup to fail")
	}
}

func stringPtr(value string) *string {
	return &value
}
