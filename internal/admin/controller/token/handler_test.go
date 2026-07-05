package token

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
	return db
}

func seedUserTokenForTest(t *testing.T, db *gorm.DB, token model.Token) {
	t.Helper()
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
}

func decodeTokenResponseBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal response %q: %v", string(body), err)
	}
	return payload
}

func TestGetAllTokensReturnsRawKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTokenControllerTestDB(t)
	seedUserTokenForTest(t, db, model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "sk-secretTokenValue1234",
		Status:      model.TokenStatusEnabled,
		Name:        "alpha",
		CreatedTime: 100,
		UpdatedTime: 100,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/token/?page=1", nil)

	GetAllTokens(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); !success {
		t.Fatalf("expected success response, got %v", payload)
	}
	data, ok := payload["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data=%T %#v, want one token row", payload["data"], payload["data"])
	}
	row, ok := data[0].(map[string]any)
	if !ok {
		t.Fatalf("row=%T %#v, want object", data[0], data[0])
	}
	key, _ := row["key"].(string)
	if key != "sk-secretTokenValue1234" {
		t.Fatalf("key=%q, want raw value", key)
	}
}

func TestGetTokenReturnsRawKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTokenControllerTestDB(t)
	seedUserTokenForTest(t, db, model.Token{
		Id:          "token-1",
		UserId:      "user-1",
		Key:         "secretTokenValue1234",
		Status:      model.TokenStatusEnabled,
		Name:        "alpha",
		CreatedTime: 100,
		UpdatedTime: 100,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Params = gin.Params{{Key: "id", Value: "token-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/token/token-1", nil)

	GetToken(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); !success {
		t.Fatalf("expected success response, got %v", payload)
	}
	row, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%T %#v, want object", payload["data"], payload["data"])
	}
	key, _ := row["key"].(string)
	if key != "secretTokenValue1234" {
		t.Fatalf("key=%q, want raw value", key)
	}
}

func TestAddTokenReturnsRawKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = newTokenControllerTestDB(t)
	originalBuildEntitlements := buildUserEntitlementModelsFn
	buildUserEntitlementModelsFn = func(ctx context.Context, userID string) (model.UserEntitlementModelsPayload, error) {
		return model.UserEntitlementModelsPayload{
			Models: []string{"gpt-4o-mini"},
		}, nil
	}
	t.Cleanup(func() {
		buildUserEntitlementModelsFn = originalBuildEntitlements
	})

	body := map[string]any{
		"name":            "created-token",
		"remain_quota":    1000,
		"unlimited_quota": false,
	}
	payloadBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/token/", bytes.NewReader(payloadBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	AddToken(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); !success {
		t.Fatalf("expected success response, got %v", payload)
	}
	row, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%T %#v, want object", payload["data"], payload["data"])
	}
	key, _ := row["key"].(string)
	if key == "" {
		t.Fatal("expected raw key on create response")
	}
	if strings.Contains(key, "****") {
		t.Fatalf("create response key=%q, should remain raw", key)
	}
}

func TestAddTokenRejectsUserWithoutAvailableModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = newTokenControllerTestDB(t)
	originalBuildEntitlements := buildUserEntitlementModelsFn
	buildUserEntitlementModelsFn = func(ctx context.Context, userID string) (model.UserEntitlementModelsPayload, error) {
		return model.UserEntitlementModelsPayload{}, nil
	}
	t.Cleanup(func() {
		buildUserEntitlementModelsFn = originalBuildEntitlements
	})

	body := map[string]any{
		"name":            "created-token",
		"remain_quota":    1000,
		"unlimited_quota": false,
	}
	payloadBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/token/", bytes.NewReader(payloadBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	AddToken(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); success {
		t.Fatalf("expected failure response, got %v", payload)
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "暂无可用模型") {
		t.Fatalf("message=%q, want available model error", message)
	}
}

func TestAddTokenRejectsUnavailableManualModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = newTokenControllerTestDB(t)
	originalBuildEntitlements := buildUserEntitlementModelsFn
	buildUserEntitlementModelsFn = func(ctx context.Context, userID string) (model.UserEntitlementModelsPayload, error) {
		return model.UserEntitlementModelsPayload{
			Models: []string{"gpt-4o-mini"},
		}, nil
	}
	t.Cleanup(func() {
		buildUserEntitlementModelsFn = originalBuildEntitlements
	})

	modelScope := "gpt-4o-mini,not-entitled"
	body := map[string]any{
		"name":            "created-token",
		"remain_quota":    1000,
		"unlimited_quota": false,
		"models":          modelScope,
	}
	payloadBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/token/", bytes.NewReader(payloadBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	AddToken(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); success {
		t.Fatalf("expected failure response, got %v", payload)
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "not-entitled") {
		t.Fatalf("message=%q, want missing model detail", message)
	}
}

func TestGetTokenStatusReturnsUsageSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTokenControllerTestDB(t)
	seedUserTokenForTest(t, db, model.Token{
		Id:             "token-1",
		UserId:         "user-1",
		Key:            "sk-secretTokenValue1234",
		Status:         model.TokenStatusEnabled,
		Name:           "alpha",
		CreatedTime:    100,
		UpdatedTime:    120,
		AccessedTime:   130,
		ExpiredTime:    150,
		RemainQuota:    900,
		UsedQuota:      100,
		UnlimitedQuota: false,
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Set(ctxkey.TokenId, "token-1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/token/status", nil)

	GetTokenStatus(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); !success {
		t.Fatalf("expected success response, got %v", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%T %#v, want object", payload["data"], payload["data"])
	}
	if got, _ := data["object"].(string); got != "token_credit_summary" {
		t.Fatalf("object=%q, want token_credit_summary", got)
	}
	if got, _ := data["token_id"].(string); got != "token-1" {
		t.Fatalf("token_id=%q, want token-1", got)
	}
	if got, _ := data["token_name"].(string); got != "alpha" {
		t.Fatalf("token_name=%q, want alpha", got)
	}
	if got, _ := data["total_granted"].(float64); got != 1000 {
		t.Fatalf("total_granted=%v, want 1000", got)
	}
	if got, _ := data["total_used"].(float64); got != 100 {
		t.Fatalf("total_used=%v, want 100", got)
	}
	if got, _ := data["total_available"].(float64); got != 900 {
		t.Fatalf("total_available=%v, want 900", got)
	}
	if got, _ := data["remaining_amount"].(float64); got != 900 {
		t.Fatalf("remaining_amount=%v, want 900", got)
	}
	if got, _ := data["used_amount"].(float64); got != 100 {
		t.Fatalf("used_amount=%v, want 100", got)
	}
	if got, _ := data["expires_at"].(float64); got != 150000 {
		t.Fatalf("expires_at=%v, want 150000", got)
	}
}

func TestGetTokenStatusRejectsWhenNoConcreteTokenBound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = newTokenControllerTestDB(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/token/status", nil)

	GetTokenStatus(c)

	payload := decodeTokenResponseBody(t, recorder.Body.Bytes())
	if success, _ := payload["success"].(bool); success {
		t.Fatalf("expected failure response, got %v", payload)
	}
	if got, _ := payload["message"].(string); got != "当前访问凭证未绑定具体令牌" {
		t.Fatalf("message=%q, want 当前访问凭证未绑定具体令牌", got)
	}
}
