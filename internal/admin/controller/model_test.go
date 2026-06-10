package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

func TestBuildOpenAIModelsForRequestOwnedByFromProviderStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4":           "openai",
			"claude-sonnet-4-6": "anthropic",
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {"/v1/responses", "/v1/chat/completions"},
			"claude-sonnet-4-6": {"/v1/messages"},
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {"text", "tool_calling"},
			"claude-sonnet-4-6": {"text"},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
	})

	items, itemMap, err := buildOpenAIModelsForRequest(c)
	if err != nil {
		t.Fatalf("buildOpenAIModelsForRequest returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if got := itemMap["gpt-5.4"].OwnedBy; got != "openai" {
		t.Fatalf("gpt-5.4 owned_by = %q, want %q", got, "openai")
	}
	if got := itemMap["claude-sonnet-4-6"].OwnedBy; got != "anthropic" {
		t.Fatalf("claude-sonnet-4-6 owned_by = %q, want %q", got, "anthropic")
	}
	if got := itemMap["gpt-5.4"].SupportedEndpoints; len(got) != 2 || got[0] != "/v1/chat/completions" || got[1] != "/v1/responses" {
		t.Fatalf("gpt-5.4 supported_endpoints = %#v, want [/v1/chat/completions /v1/responses]", got)
	}
	if got := itemMap["claude-sonnet-4-6"].SupportedEndpoints; len(got) != 1 || got[0] != "/v1/messages" {
		t.Fatalf("claude-sonnet-4-6 supported_endpoints = %#v, want [/v1/messages]", got)
	}
	if got := itemMap["gpt-5.4"].Tags; len(got) != 2 || got[0] != "text" || got[1] != "tool_calling" {
		t.Fatalf("gpt-5.4 tags = %#v, want [text tool_calling]", got)
	}
}

func TestBuildOpenAIModelsForRequestFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4": "openai",
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadProviderModelTagsFn = originalTags
	})

	_, _, err := buildOpenAIModelsForRequest(c)
	if err == nil {
		t.Fatalf("expected error when provider mapping missing")
	}
}

func TestListModelsOwnedByUsesProviderStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4":           "openai",
			"claude-sonnet-4-6": "anthropic",
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {"/v1/chat/completions", "/v1/responses"},
			"claude-sonnet-4-6": {"/v1/messages"},
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {"text", "tool_calling"},
			"claude-sonnet-4-6": {"text"},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
	})

	ListModels(c)
	if recorder.Code != 200 {
		t.Fatalf("status code = %d, want 200", recorder.Code)
	}

	payload := struct {
		Object string         `json:"object"`
		Data   []OpenAIModels `json:"data"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if payload.Object != "list" {
		t.Fatalf("object = %q, want %q", payload.Object, "list")
	}
	if len(payload.Data) != 2 {
		t.Fatalf("data length = %d, want 2", len(payload.Data))
	}
	ownedBy := map[string]string{}
	for _, item := range payload.Data {
		ownedBy[item.Id] = item.OwnedBy
	}
	if got := ownedBy["gpt-5.4"]; got != "openai" {
		t.Fatalf("gpt-5.4 owned_by = %q, want %q", got, "openai")
	}
	if got := ownedBy["claude-sonnet-4-6"]; got != "anthropic" {
		t.Fatalf("claude-sonnet-4-6 owned_by = %q, want %q", got, "anthropic")
	}
	supportedEndpoints := map[string][]string{}
	tagsByModel := map[string][]string{}
	for _, item := range payload.Data {
		supportedEndpoints[item.Id] = item.SupportedEndpoints
		tagsByModel[item.Id] = item.Tags
	}
	if got := supportedEndpoints["gpt-5.4"]; len(got) != 2 || got[0] != "/v1/chat/completions" || got[1] != "/v1/responses" {
		t.Fatalf("gpt-5.4 supported_endpoints = %#v, want [/v1/chat/completions /v1/responses]", got)
	}
	if got := supportedEndpoints["claude-sonnet-4-6"]; len(got) != 1 || got[0] != "/v1/messages" {
		t.Fatalf("claude-sonnet-4-6 supported_endpoints = %#v, want [/v1/messages]", got)
	}
	if got := tagsByModel["gpt-5.4"]; len(got) != 2 || got[0] != "text" || got[1] != "tool_calling" {
		t.Fatalf("gpt-5.4 tags = %#v, want [text tool_calling]", got)
	}
}

func TestListModelsFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4")

	original := loadGroupModelProvidersFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadProviderModelTagsFn = originalTags
	})

	ListModels(c)
	if recorder.Code != 400 {
		t.Fatalf("status code = %d, want 400", recorder.Code)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if _, ok := payload["error"]; !ok {
		t.Fatalf("expected error field in payload, got %v", payload)
	}
}

func TestRetrieveModelSharesListOwnedByLogic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	original := loadGroupModelProvidersFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4": "openai",
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4": {"/v1/responses"},
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4": {"text", "reasoning"},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
	})

	{
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(ctxkey.AvailableModels, "gpt-5.4")
		c.Params = gin.Params{{Key: "model", Value: "gpt-5.4"}}

		RetrieveModel(c)
		if recorder.Code != 200 {
			t.Fatalf("status code = %d, want 200", recorder.Code)
		}
		item := OpenAIModels{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &item); err != nil {
			t.Fatalf("json.Unmarshal item returned error: %v", err)
		}
		if item.OwnedBy != "openai" {
			t.Fatalf("owned_by = %q, want %q", item.OwnedBy, "openai")
		}
		if len(item.SupportedEndpoints) != 1 || item.SupportedEndpoints[0] != "/v1/responses" {
			t.Fatalf("supported_endpoints = %#v, want [/v1/responses]", item.SupportedEndpoints)
		}
		if len(item.Tags) != 2 || item.Tags[0] != "text" || item.Tags[1] != "reasoning" {
			t.Fatalf("tags = %#v, want [text reasoning]", item.Tags)
		}
	}

	{
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set(ctxkey.AvailableModels, "gpt-5.4")
		c.Params = gin.Params{{Key: "model", Value: "not-exist"}}

		RetrieveModel(c)
		if recorder.Code != 200 {
			t.Fatalf("status code = %d, want 200", recorder.Code)
		}
		payload := map[string]any{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal error payload returned error: %v", err)
		}
		if _, ok := payload["error"]; !ok {
			t.Fatalf("expected error field in payload, got %v", payload)
		}
	}
}

func TestLoadDashboardProtocolModels_UsesProviderCatalogForDeepSeek(t *testing.T) {
	original := loadProviderProtocolModelsFn
	defer func() {
		loadProviderProtocolModelsFn = original
	}()

	model.DB = nil
	loadProviderProtocolModelsFn = loadDashboardProtocolModels

	got, err := loadDashboardProtocolModels(relaychannel.DeepSeek)
	if err == nil {
		t.Fatalf("expected error when database handle is nil")
	}
	if got != nil {
		t.Fatalf("models = %#v, want nil on db error", got)
	}
}

func TestRetrieveModelFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4")
	c.Params = gin.Params{{Key: "model", Value: "gpt-5.4"}}

	original := loadGroupModelProvidersFn
	originalTags := loadProviderModelTagsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadProviderModelTagsFn = originalTags
	})

	RetrieveModel(c)
	if recorder.Code != 400 {
		t.Fatalf("status code = %d, want 400", recorder.Code)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if _, ok := payload["error"]; !ok {
		t.Fatalf("expected error field in payload, got %v", payload)
	}
}
