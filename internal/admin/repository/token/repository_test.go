package token

import (
	"testing"

	"github.com/yeying-community/router/common"
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
	previousRedisEnabled := common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
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

func TestValidateUserTokenRejectsExhaustedRequestCount(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		ExpiredTime:           -1,
		RemainQuota:           1000,
		UnlimitedQuota:        false,
		RemainRequestCount:    0,
		UnlimitedRequestCount: false,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	_, err := ValidateUserToken("secret-key-1")
	if err == nil {
		t.Fatal("expected request-count exhausted token to be rejected")
	}
	if err.Error() != "该令牌请求次数已用尽" {
		t.Fatalf("ValidateUserToken error=%q, want request count exhausted", err.Error())
	}
}

func TestConsumeTokenRequestCountDecrementsFiniteLimit(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		RemainRequestCount:    3,
		UnlimitedRequestCount: false,
		UsedRequestCount:      2,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := model.ConsumeTokenRequestCount("token-1", 2); err != nil {
		t.Fatalf("ConsumeTokenRequestCount: %v", err)
	}

	var stored model.Token
	if err := db.First(&stored, "id = ?", "token-1").Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if stored.RemainRequestCount != 1 {
		t.Fatalf("RemainRequestCount=%d, want 1", stored.RemainRequestCount)
	}
	if stored.UsedRequestCount != 4 {
		t.Fatalf("UsedRequestCount=%d, want 4", stored.UsedRequestCount)
	}
}

func TestConsumeTokenRequestCountRejectsInsufficientFiniteLimit(t *testing.T) {
	db := newTokenRepositoryTestDB(t)
	token := &model.Token{
		Id:                    "token-1",
		UserId:                "user-1",
		Key:                   "secret-key-1",
		Name:                  "alpha",
		Status:                model.TokenStatusEnabled,
		RemainRequestCount:    1,
		UnlimitedRequestCount: false,
		UsedRequestCount:      2,
		CreatedTime:           100,
		UpdatedTime:           100,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	if err := model.ConsumeTokenRequestCount("token-1", 2); err == nil {
		t.Fatal("expected insufficient finite request count to fail")
	}

	var stored model.Token
	if err := db.First(&stored, "id = ?", "token-1").Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if stored.RemainRequestCount != 1 {
		t.Fatalf("RemainRequestCount=%d, want unchanged 1", stored.RemainRequestCount)
	}
	if stored.UsedRequestCount != 2 {
		t.Fatalf("UsedRequestCount=%d, want unchanged 2", stored.UsedRequestCount)
	}
}

func stringPtr(value string) *string {
	return &value
}
