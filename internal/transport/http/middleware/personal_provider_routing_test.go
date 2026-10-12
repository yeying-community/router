package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/responsestate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPersonalProviderRoutingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PersonalProviderConnection{}, &model.PersonalModelRoute{}, &model.CommunityOfferModelRoute{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := model.DB
	previousSecret := config.JWTSecret
	previousFallbacks := config.JWTFallbackSecrets
	model.DB = db
	config.JWTSecret = "personal-provider-routing-test-secret"
	config.JWTFallbackSecrets = nil
	t.Cleanup(func() {
		model.DB = previousDB
		config.JWTSecret = previousSecret
		config.JWTFallbackSecrets = previousFallbacks
	})
	return db
}

func newPersonalProviderRelayContext(t *testing.T, modelName string, policy string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/chat/completions", bytes.NewBufferString(`{"model":"`+modelName+`","messages":[{"role":"user","content":"hello"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.Id, "user-a")
	c.Set(ctxkey.RequestModel, modelName)
	c.Set(ctxkey.PersonalRoutePolicy, policy)
	c.Set(ctxkey.TokenRemainQuota, int64(0))
	c.Set(ctxkey.TokenUnlimitedQuota, false)
	return c, recorder
}

func TestDistributePersonalOnlyUsesPrivateConnectionWithoutCommunityEntitlement(t *testing.T) {
	newPersonalProviderRoutingTestDB(t)
	connection := &model.PersonalProviderConnection{
		UserId: "user-a", Name: "private OpenAI", Protocol: "openai", BaseURL: "",
		Models: []string{"private-model"}, Priority: 20,
	}
	if err := model.CreatePersonalProviderConnection(connection, "sk-personal"); err != nil {
		t.Fatalf("create private connection: %v", err)
	}
	c, recorder := newPersonalProviderRelayContext(t, "private-model", model.PersonalRoutePolicyPersonalOnly)

	Distribute()(c)

	if recorder.Code >= http.StatusBadRequest {
		t.Fatalf("distribute status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := c.GetString(ctxkey.ChannelId); got != model.PersonalProviderChannelPrefix+connection.Id {
		t.Fatalf("channel ID = %q, want personal connection", got)
	}
	if got := c.GetString(ctxkey.Group); got != "" {
		t.Fatalf("personal-only request unexpectedly resolved community group %q", got)
	}
	if got := c.GetString(ctxkey.PersonalProviderID); got != connection.Id {
		t.Fatalf("personal provider ID = %q, want %q", got, connection.Id)
	}
}

func TestTokenMonetaryQuotaExhaustedUsesTokenAuthSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if tokenMonetaryQuotaExhausted(c) {
		t.Fatal("missing token snapshot must not block request")
	}
	c.Set(ctxkey.TokenRemainQuota, int64(0))
	c.Set(ctxkey.TokenUnlimitedQuota, false)
	if !tokenMonetaryQuotaExhausted(c) {
		t.Fatal("zero finite monetary quota must block a community selection")
	}
	c.Set(ctxkey.TokenUnlimitedQuota, true)
	if tokenMonetaryQuotaExhausted(c) {
		t.Fatal("unlimited monetary quota must not block request")
	}
}

func TestDistributeResponsesKeepsPinnedPersonalConnectionAcrossPolicies(t *testing.T) {
	newPersonalProviderRoutingTestDB(t)
	responsestate.ResetForTest()
	t.Cleanup(responsestate.ResetForTest)
	pinned := &model.PersonalProviderConnection{
		UserId: "user-a", Name: "pinned connection", Protocol: "openai", BaseURL: "",
		Models: []string{"private-model"}, Priority: 1,
	}
	preferred := &model.PersonalProviderConnection{
		UserId: "user-a", Name: "new preferred connection", Protocol: "openai", BaseURL: "",
		Models: []string{"private-model"}, Priority: 100,
	}
	if err := model.CreatePersonalProviderConnection(pinned, "sk-pinned"); err != nil {
		t.Fatalf("create pinned connection: %v", err)
	}
	if err := model.CreatePersonalProviderConnection(preferred, "sk-preferred"); err != nil {
		t.Fatalf("create preferred connection: %v", err)
	}
	responsestate.StoreRoute("resp_pinned", model.PersonalProviderChannelPrefix+pinned.Id)
	c, recorder := newPersonalProviderRelayContext(t, "private-model", model.PersonalRoutePolicyPersonalFirst)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/responses", bytes.NewBufferString(`{"model":"private-model","previous_response_id":"resp_pinned"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.ResponsesPreviousResponseID, "resp_pinned")

	Distribute()(c)

	if recorder.Code >= http.StatusBadRequest {
		t.Fatalf("distribute status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := c.GetString(ctxkey.ChannelId); got != model.PersonalProviderChannelPrefix+pinned.Id {
		t.Fatalf("channel ID = %q, want pinned %q", got, pinned.Id)
	}
}

func TestDistributeResponsesRejectsDeletedPinnedPersonalConnection(t *testing.T) {
	db := newPersonalProviderRoutingTestDB(t)
	responsestate.ResetForTest()
	t.Cleanup(responsestate.ResetForTest)
	connection := &model.PersonalProviderConnection{
		UserId: "user-a", Name: "deleted connection", Protocol: "openai", BaseURL: "",
		Models: []string{"private-model"}, Priority: 1,
	}
	if err := model.CreatePersonalProviderConnection(connection, "sk-deleted"); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	responsestate.StoreRoute("resp_deleted", model.PersonalProviderChannelPrefix+connection.Id)
	if err := db.Delete(&model.PersonalProviderConnection{}, "id = ?", connection.Id).Error; err != nil {
		t.Fatalf("delete connection: %v", err)
	}
	c, recorder := newPersonalProviderRelayContext(t, "private-model", model.PersonalRoutePolicyPersonalFirst)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/responses", bytes.NewBufferString(`{"model":"private-model","previous_response_id":"resp_deleted"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.ResponsesPreviousResponseID, "resp_deleted")

	Distribute()(c)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "已绑定的 Responses 会话上游不可用") {
		t.Fatalf("response = %s", recorder.Body.String())
	}
}

func TestDistributePersonalOnlyRejectsMissingConnectionBeforeCommunityLookup(t *testing.T) {
	newPersonalProviderRoutingTestDB(t)
	c, recorder := newPersonalProviderRelayContext(t, "private-model", model.PersonalRoutePolicyPersonalOnly)

	Distribute()(c)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "没有可用的个人供应商连接") {
		t.Fatalf("missing connection response = %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "no such table") {
		t.Fatalf("personal-only request attempted community entitlement lookup: %s", recorder.Body.String())
	}
}

// resetConflictingResponsesState underpins the regular-community-channel pin
// failover (task 149): when a pinned channel is unavailable, the distributor
// resets the Responses state and falls through to normal selection instead of
// returning 409. This verifies the reset strips the now-invalid identifiers and
// clears the stateful context so downstream selection is not re-pinned.
func TestResetConflictingResponsesStateStripsPinnedIdentifiers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"m","previous_response_id":"resp_old","input":[{"type":"message","id":"msg_old","role":"user","content":"hi"}]}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/responses", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.ResponsesPreviousResponseID, "resp_old")
	c.Set(ctxkey.ResponsesItemIDs, []string{"msg_old"})
	c.Set(ctxkey.ResponsesStatefulRequest, true)

	if err := resetConflictingResponsesState(c); err != nil {
		t.Fatalf("reset error: %v", err)
	}

	if got := c.GetString(ctxkey.ResponsesPreviousResponseID); got != "" {
		t.Fatalf("previous_response_id context = %q, want cleared", got)
	}
	if c.GetBool(ctxkey.ResponsesStatefulRequest) {
		t.Fatal("stateful flag must be cleared after reset")
	}
	raw, _ := common.GetRequestBody(c)
	if strings.Contains(string(raw), "resp_old") {
		t.Fatalf("rewritten body still carries previous_response_id: %s", string(raw))
	}
	if strings.Contains(string(raw), "msg_old") {
		t.Fatalf("rewritten body still carries stale item id: %s", string(raw))
	}
	if !strings.Contains(string(raw), "\"content\":\"hi\"") {
		t.Fatalf("rewritten body dropped replayable content: %s", string(raw))
	}
}
