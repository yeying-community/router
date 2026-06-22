package middleware

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
)

func TestGetRequestModel_VideosMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("model", "veo-3.0-generate-preview"); err != nil {
		t.Fatalf("WriteField(model) error: %v", err)
	}
	if err := writer.WriteField("prompt", "test"); err != nil {
		t.Fatalf("WriteField(prompt) error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close error: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/videos", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	modelName, err := getRequestModel(c)
	if err != nil {
		t.Fatalf("getRequestModel returned error: %v", err)
	}
	if modelName != "veo-3.0-generate-preview" {
		t.Fatalf("getRequestModel returned %q, want %q", modelName, "veo-3.0-generate-preview")
	}
}

func TestGetRequestModel_ImageEditsMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("model", "qwen-image-2.0"); err != nil {
		t.Fatalf("WriteField(model) error: %v", err)
	}
	if err := writer.WriteField("prompt", "make it blue"); err != nil {
		t.Fatalf("WriteField(prompt) error: %v", err)
	}
	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile(image) error: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47}); err != nil {
		t.Fatalf("Write(image) error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close error: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/images/edits", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	modelName, err := getRequestModel(c)
	if err != nil {
		t.Fatalf("getRequestModel returned error: %v", err)
	}
	if modelName != "qwen-image-2.0" {
		t.Fatalf("getRequestModel returned %q, want %q", modelName, "qwen-image-2.0")
	}
}

func TestGetRequestModel_VideoStatusQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("GET", "/v1/videos/task_123?model=veo-3.0-generate-preview", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	modelName, err := getRequestModel(c)
	if err != nil {
		t.Fatalf("getRequestModel returned error: %v", err)
	}
	if modelName != "veo-3.0-generate-preview" {
		t.Fatalf("getRequestModel returned %q, want %q", modelName, "veo-3.0-generate-preview")
	}
}

func TestGetRequestModel_RealtimeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/realtime/calls?model=gpt-realtime-2", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	modelName, err := getRequestModel(c)
	if err != nil {
		t.Fatalf("getRequestModel returned error: %v", err)
	}
	if modelName != "gpt-realtime-2" {
		t.Fatalf("getRequestModel returned %q, want %q", modelName, "gpt-realtime-2")
	}
}

func TestGetRequestModel_RealtimeNestedSessionModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/realtime/client_secrets", bytes.NewBufferString(`{"session":{"model":"gpt-realtime-1.5"}}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	modelName, err := getRequestModel(c)
	if err != nil {
		t.Fatalf("getRequestModel returned error: %v", err)
	}
	if modelName != "gpt-realtime-1.5" {
		t.Fatalf("getRequestModel returned %q, want %q", modelName, "gpt-realtime-1.5")
	}
}

func TestHydrateResponsesRelayContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/api/v1/public/responses", bytes.NewBufferString(`{
		"model":"gpt-5.4",
		"previous_response_id":"resp_prev",
		"input":[{"type":"function_call_output","call_id":"call_123","output":"ok"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	hydrateResponsesRelayContext(c)

	if got := c.GetString(ctxkey.ResponsesPreviousResponseID); got != "resp_prev" {
		t.Fatalf("ResponsesPreviousResponseID = %q, want resp_prev", got)
	}
	if !c.GetBool(ctxkey.ResponsesStatefulRequest) {
		t.Fatal("ResponsesStatefulRequest = false, want true")
	}
}

func TestUserAuthAcceptsUcan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevVerifyWalletJWT := verifyWalletJWTFunc
	prevIsUcanToken := isUcanTokenFunc
	prevResolveCapabilitySets := resolveUcanCapabilitySetsFunc
	prevResolveAudience := resolveUcanAudienceFunc
	prevVerifyUcanInvocationAny := verifyUcanInvocationAnyFunc
	prevValidateAccessToken := validateAccessTokenFunc
	prevGetUserByID := getUserByIDFunc
	prevFindOrCreateWalletUser := findOrCreateWalletUserFunc
	defer func() {
		verifyWalletJWTFunc = prevVerifyWalletJWT
		isUcanTokenFunc = prevIsUcanToken
		resolveUcanCapabilitySetsFunc = prevResolveCapabilitySets
		resolveUcanAudienceFunc = prevResolveAudience
		verifyUcanInvocationAnyFunc = prevVerifyUcanInvocationAny
		validateAccessTokenFunc = prevValidateAccessToken
		getUserByIDFunc = prevGetUserByID
		findOrCreateWalletUserFunc = prevFindOrCreateWalletUser
	}()

	verifyWalletJWTFunc = func(token string) (*common.WalletClaims, error) {
		return nil, errors.New("not jwt")
	}
	isUcanTokenFunc = func(token string) bool {
		return token == "ucan-token"
	}
	resolveUcanCapabilitySetsFunc = func() [][]common.UcanCapability {
		return [][]common.UcanCapability{{{Resource: "app:*", Action: "invoke"}}}
	}
	resolveUcanAudienceFunc = func() string {
		return "did:web:localhost:3011"
	}
	verifyUcanInvocationAnyFunc = func(token string, audience string, requiredSets [][]common.UcanCapability) (string, error) {
		if token != "ucan-token" {
			return "", errors.New("unexpected token")
		}
		return "0x1234567890abcdef1234567890abcdef12345678", nil
	}
	validateAccessTokenFunc = func(token string) *model.User {
		return nil
	}
	getUserByIDFunc = func(id string, selectAll bool) (*model.User, error) {
		return &model.User{
			Id:       id,
			Username: "wallet_user",
			Role:     model.RoleCommonUser,
			Status:   model.UserStatusEnabled,
		}, nil
	}
	findOrCreateWalletUserFunc = func(addr string, ctx context.Context) (*model.User, error) {
		return &model.User{
			Id:       "user-ucan-1",
			Username: "wallet_user",
			Role:     model.RoleCommonUser,
			Status:   model.UserStatusEnabled,
		}, nil
	}

	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	engine.Use(UserAuth())

	called := false
	engine.GET("/api/v1/public/token/", func(ctx *gin.Context) {
		called = true
		if got := ctx.GetString(ctxkey.Id); got != "user-ucan-1" {
			t.Fatalf("ctx user id = %q, want %q", got, "user-ucan-1")
		}
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/token/?page=1", nil)
	req.Header.Set("Authorization", "Bearer ucan-token")
	engine.ServeHTTP(recorder, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("response code = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestTokenAuthRejectsUcan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevIsUcanToken := isUcanTokenFunc
	prevValidateUserToken := validateUserTokenFunc
	defer func() {
		isUcanTokenFunc = prevIsUcanToken
		validateUserTokenFunc = prevValidateUserToken
	}()

	isUcanTokenFunc = func(token string) bool {
		return token == "ucan-token"
	}
	validateUserTokenFunc = func(key string) (*model.Token, error) {
		t.Fatalf("TokenAuth should reject UCAN before API token validation, got key %q", key)
		return nil, errors.New("unexpected validation")
	}

	recorder := httptest.NewRecorder()
	engine := gin.New()
	engine.Use(TokenAuth())

	called := false
	engine.POST("/v1/chat/completions", func(ctx *gin.Context) {
		called = true
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-5.4","messages":[]}`))
	req.Header.Set("Authorization", "Bearer ucan-token")
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)

	if called {
		t.Fatal("expected TokenAuth to abort UCAN request")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("response code = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
