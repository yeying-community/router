package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
)

func TestBuildOpenAIModelsForRequestOwnedByFromProviderStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4":           "openai",
			"claude-sonnet-4-6": "anthropic",
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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
}

func TestBuildOpenAIModelsForRequestFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4": "openai",
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4":           "openai",
			"claude-sonnet-4-6": "anthropic",
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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
}

func TestListModelsFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4")

	original := loadGroupModelProvidersFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4": "openai",
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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

func TestRetrieveModelFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4")
	c.Params = gin.Params{{Key: "model", Value: "gpt-5.4"}}

	original := loadGroupModelProvidersFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
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
