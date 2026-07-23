package controller

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/healthtrend"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/driver/sqlite"
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
	originalSpecifications := loadProviderModelSpecificationsFn
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
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{
			"gpt-5.4": {
				Version: 1,
				Endpoints: map[string]model.ProviderModelEndpointSpecification{
					model.ChannelModelEndpointResponses: {
						InputModalities:  []string{"text"},
						OutputModalities: []string{"text"},
					},
				},
			},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
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
	if itemMap["gpt-5.4"].Specification == nil {
		t.Fatal("expected gpt-5.4 specification to be included")
	}
}

func TestBuildOpenAIModelsForRequestFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")

	original := loadGroupModelProvidersFn
	originalTags := loadProviderModelTagsFn
	originalSpecifications := loadProviderModelSpecificationsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4": "openai",
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
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
	originalSpecifications := loadProviderModelSpecificationsFn
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
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{
			"claude-sonnet-4-6": {
				Version: 1,
				Endpoints: map[string]model.ProviderModelEndpointSpecification{
					model.ChannelModelEndpointMessages: {
						InputModalities:  []string{"text"},
						OutputModalities: []string{"text"},
					},
				},
			},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
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
	for _, item := range payload.Data {
		if item.Id == "claude-sonnet-4-6" && item.Specification == nil {
			t.Fatal("expected claude-sonnet-4-6 specification in list payload")
		}
	}
}

func TestListModelsUsesAllUserEntitlementSourcesForUnscopedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")

	originalEntitlements := buildRequestUserEntitlementModelsFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	originalSpecifications := loadProviderModelSpecificationsFn
	buildRequestUserEntitlementModelsFn = func(ctx context.Context, userID string) (model.UserEntitlementModelsPayload, error) {
		return model.UserEntitlementModelsPayload{
			Models: []string{"qwen3-coder-next", "gpt-5.4"},
			Items: []model.UserAvailableModel{
				{
					Model:    "qwen3-coder-next",
					Provider: "qwen",
					Sources: []model.UserEntitlementModelSource{
						{SourceType: model.UserEntitlementSourcePackage, SourceID: "sub-qwen", GroupID: "group-qwen", Provider: "qwen", Priority: 10},
					},
				},
				{
					Model:    "gpt-5.4",
					Provider: "openai",
					Sources: []model.UserEntitlementModelSource{
						{SourceType: model.UserEntitlementSourcePackage, SourceID: "sub-openai", GroupID: "group-openai", Provider: "openai", Priority: 10},
					},
				},
			},
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		switch groupID {
		case "group-qwen":
			return map[string][]string{"qwen3-coder-next": {model.ChannelModelEndpointResponses}}, nil
		case "group-openai":
			return map[string][]string{"gpt-5.4": {model.ChannelModelEndpointChat, model.ChannelModelEndpointResponses}}, nil
		default:
			return map[string][]string{}, nil
		}
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"qwen3-coder-next": {"text"},
			"gpt-5.4":          {"text", "tool_calling"},
		}, nil
	}
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{}, nil
	}
	t.Cleanup(func() {
		buildRequestUserEntitlementModelsFn = originalEntitlements
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
	})

	ListModels(c)
	if recorder.Code != 200 {
		t.Fatalf("status code = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	payload := struct {
		Object string         `json:"object"`
		Data   []OpenAIModels `json:"data"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("data length = %d, want 2", len(payload.Data))
	}
	ownedBy := map[string]string{}
	for _, item := range payload.Data {
		ownedBy[item.Id] = item.OwnedBy
	}
	if got := ownedBy["qwen3-coder-next"]; got != "qwen" {
		t.Fatalf("qwen3-coder-next owned_by = %q, want qwen", got)
	}
	if got := ownedBy["gpt-5.4"]; got != "openai" {
		t.Fatalf("gpt-5.4 owned_by = %q, want openai", got)
	}
}

func TestListModelsIntersectsTokenScopeWithUserEntitlements(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.Id, "user-1")
	c.Set(ctxkey.AvailableModels, "qwen3-coder-next")

	originalEntitlements := buildRequestUserEntitlementModelsFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	originalSpecifications := loadProviderModelSpecificationsFn
	buildRequestUserEntitlementModelsFn = func(ctx context.Context, userID string) (model.UserEntitlementModelsPayload, error) {
		return model.UserEntitlementModelsPayload{
			Models: []string{"qwen3-coder-next", "gpt-5.4"},
			Items: []model.UserAvailableModel{
				{
					Model:    "qwen3-coder-next",
					Provider: "qwen",
					Sources: []model.UserEntitlementModelSource{
						{SourceType: model.UserEntitlementSourcePackage, SourceID: "sub-qwen", GroupID: "group-qwen", Provider: "qwen", Priority: 10},
					},
				},
				{
					Model:    "gpt-5.4",
					Provider: "openai",
					Sources: []model.UserEntitlementModelSource{
						{SourceType: model.UserEntitlementSourcePackage, SourceID: "sub-openai", GroupID: "group-openai", Provider: "openai", Priority: 10},
					},
				},
			},
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{"qwen3-coder-next": {model.ChannelModelEndpointResponses}}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{}, nil
	}
	t.Cleanup(func() {
		buildRequestUserEntitlementModelsFn = originalEntitlements
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
	})

	ListModels(c)
	if recorder.Code != 200 {
		t.Fatalf("status code = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	payload := struct {
		Data []OpenAIModels `json:"data"`
	}{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("data length = %d, want 1", len(payload.Data))
	}
	if got := payload.Data[0].Id; got != "qwen3-coder-next" {
		t.Fatalf("model id = %q, want qwen3-coder-next", got)
	}
}

func TestListGroupModelSupportedEndpointsUsesUpstreamModelMapping(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.ChannelModel{},
		&model.GroupModelChannel{},
		&model.ChannelModelEndpoint{},
		&model.ChannelModelEndpointPolicy{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	channel := model.Channel{
		Id:       "channel-1",
		Name:     "qwen-lx",
		Protocol: "qwen",
		Status:   model.ChannelStatusEnabled,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.GroupModelChannel{
		Group:         "group-1",
		Model:         "qwen3.7-plus",
		ChannelId:     "channel-1",
		UpstreamModel: "qwen3.7-plus-2026-05-26",
		Provider:      "qwen",
	}).Error; err != nil {
		t.Fatalf("create group model channel: %v", err)
	}
	if err := db.Create(&[]model.ChannelModelEndpoint{
		{
			ChannelId: "channel-1",
			Model:     "qwen3.7-plus",
			Endpoint:  model.ChannelModelEndpointChat,
			Enabled:   false,
		},
		{
			ChannelId: "channel-1",
			Model:     "qwen3.7-plus",
			Endpoint:  model.ChannelModelEndpointResponses,
			Enabled:   false,
		},
		{
			ChannelId: "channel-1",
			Model:     "qwen3.7-plus-2026-05-26",
			Endpoint:  model.ChannelModelEndpointChat,
			Enabled:   true,
		},
		{
			ChannelId: "channel-1",
			Model:     "qwen3.7-plus-2026-05-26",
			Endpoint:  model.ChannelModelEndpointResponses,
			Enabled:   true,
		},
	}).Error; err != nil {
		t.Fatalf("create channel endpoints: %v", err)
	}

	originalDB := model.DB
	originalMemoryCacheEnabled := config.MemoryCacheEnabled
	model.DB = db
	config.MemoryCacheEnabled = true
	model.InitChannelCache()
	t.Cleanup(func() {
		model.DB = originalDB
		config.MemoryCacheEnabled = originalMemoryCacheEnabled
		if originalDB != nil && originalMemoryCacheEnabled {
			model.InitChannelCache()
		}
	})

	endpointsByModel, err := listGroupModelSupportedEndpoints("group-1", []string{"qwen3.7-plus"})
	if err != nil {
		t.Fatalf("listGroupModelSupportedEndpoints returned error: %v", err)
	}
	got := endpointsByModel["qwen3.7-plus"]
	if len(got) != 2 || got[0] != model.ChannelModelEndpointChat || got[1] != model.ChannelModelEndpointResponses {
		t.Fatalf("supported endpoints = %#v, want [%s %s]", got, model.ChannelModelEndpointChat, model.ChannelModelEndpointResponses)
	}
}

func TestListModelsFailsWhenProviderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4")

	original := loadGroupModelProvidersFn
	originalTags := loadProviderModelTagsFn
	originalSpecifications := loadProviderModelSpecificationsFn
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
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
	originalSpecifications := loadProviderModelSpecificationsFn
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
	loadProviderModelSpecificationsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string]*model.ProviderModelSpecification, error) {
		return map[string]*model.ProviderModelSpecification{
			"gpt-5.4": {
				Version: 1,
				Endpoints: map[string]model.ProviderModelEndpointSpecification{
					model.ChannelModelEndpointResponses: {
						InputModalities:  []string{"text"},
						OutputModalities: []string{"text"},
					},
				},
			},
		}, nil
	}
	t.Cleanup(func() {
		loadGroupModelProvidersFn = original
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadProviderModelSpecificationsFn = originalSpecifications
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
		if item.Specification == nil {
			t.Fatal("expected specification in retrieve payload")
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

func TestBuildUserModelStatusPayloadAggregatesGroupModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(ctxkey.AvailableModels, "gpt-5.4,claude-sonnet-4-6")
	now := healthtrend.BucketStart(helper.GetTimestamp())

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ChannelTest{}); err != nil {
		t.Fatalf("auto migrate channel tests: %v", err)
	}
	if err := db.Create(&[]model.ChannelTest{
		{
			ChannelId: "channel-1",
			Model:     "gpt-5.4",
			Round:     1,
			Type:      "text",
			Endpoint:  model.ChannelModelEndpointChat,
			Status:    model.ChannelTestStatusSupported,
			Supported: true,
			LatencyMs: 1200,
			TestedAt:  now,
		},
		{
			ChannelId: "channel-2",
			Model:     "gpt-5.4",
			Round:     1,
			Type:      "text",
			Endpoint:  model.ChannelModelEndpointResponses,
			Status:    model.ChannelTestStatusUnsupported,
			Supported: false,
			LatencyMs: 3000,
			TestedAt:  now,
		},
		{
			ChannelId: "channel-3",
			Model:     "claude-sonnet-4-6",
			Round:     1,
			Type:      "text",
			Endpoint:  model.ChannelModelEndpointMessages,
			Status:    model.ChannelTestStatusSkipped,
			Supported: false,
			LatencyMs: 0,
			TestedAt:  now,
		},
	}).Error; err != nil {
		t.Fatalf("create channel tests: %v", err)
	}

	originalDB := model.DB
	originalProviders := loadGroupModelProvidersFn
	originalEndpoints := loadGroupModelSupportedEndpointsFn
	originalTags := loadProviderModelTagsFn
	originalChannels := loadSatisfiedChannelsFn
	model.DB = db
	loadGroupModelProvidersFn = func(groupID string, modelNames []string) (map[string]string, error) {
		return map[string]string{
			"gpt-5.4":           "openai",
			"claude-sonnet-4-6": "anthropic",
		}, nil
	}
	loadGroupModelSupportedEndpointsFn = func(groupID string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {model.ChannelModelEndpointResponses, model.ChannelModelEndpointChat},
			"claude-sonnet-4-6": {model.ChannelModelEndpointMessages},
		}, nil
	}
	loadProviderModelTagsFn = func(_ *gorm.DB, providerByModel map[string]string, modelNames []string) (map[string][]string, error) {
		return map[string][]string{
			"gpt-5.4":           {"text"},
			"claude-sonnet-4-6": {"text", "tool_calling"},
		}, nil
	}
	loadSatisfiedChannelsFn = func(groupID string, modelName string) ([]*model.Channel, error) {
		switch modelName {
		case "gpt-5.4":
			return []*model.Channel{{Id: "channel-1"}, {Id: "channel-2"}}, nil
		case "claude-sonnet-4-6":
			return []*model.Channel{{Id: "channel-3"}}, nil
		default:
			return []*model.Channel{}, nil
		}
	}
	t.Cleanup(func() {
		model.DB = originalDB
		loadGroupModelProvidersFn = originalProviders
		loadGroupModelSupportedEndpointsFn = originalEndpoints
		loadProviderModelTagsFn = originalTags
		loadSatisfiedChannelsFn = originalChannels
	})

	payload, err := buildUserModelStatusPayload(c)
	if err != nil {
		t.Fatalf("buildUserModelStatusPayload returned error: %v", err)
	}
	if payload.Summary.ModelCount != 2 {
		t.Fatalf("model count = %d, want 2", payload.Summary.ModelCount)
	}
	byModel := map[string]UserModelStatusItem{}
	for _, item := range payload.Models {
		byModel[item.Model] = item
	}
	gpt := byModel["gpt-5.4"]
	if gpt.Provider != "openai" {
		t.Fatalf("gpt provider = %q, want openai", gpt.Provider)
	}
	if gpt.SupportedCount != 1 || gpt.UnsupportedCount != 1 {
		t.Fatalf("gpt supported/unsupported = %d/%d, want 1/1", gpt.SupportedCount, gpt.UnsupportedCount)
	}
	if len(gpt.HealthPoints) != healthtrend.BucketCount {
		t.Fatalf("gpt health points = %d, want %d", len(gpt.HealthPoints), healthtrend.BucketCount)
	}
	gptObserved := make([]healthtrend.Point, 0)
	for _, point := range gpt.HealthPoints {
		if point.TotalCount > 0 {
			gptObserved = append(gptObserved, point)
		}
	}
	if len(gptObserved) != 1 {
		t.Fatalf("gpt observed points = %d, want 1 from channel tests", len(gptObserved))
	}
	if gptObserved[0].State != healthtrend.StateWarning || gptObserved[0].SuccessCount != 1 || gptObserved[0].FailureCount != 1 {
		t.Fatalf("gpt test point = %+v, want warning with 1 success and 1 failure", gptObserved[0])
	}
	claude := byModel["claude-sonnet-4-6"]
	if len(claude.HealthPoints) != healthtrend.BucketCount {
		t.Fatalf("claude health points = %d, want %d", len(claude.HealthPoints), healthtrend.BucketCount)
	}
	claudeObserved := make([]healthtrend.Point, 0)
	for _, point := range claude.HealthPoints {
		if point.TotalCount > 0 {
			claudeObserved = append(claudeObserved, point)
		}
	}
	if len(claudeObserved) != 1 || claudeObserved[0].State != healthtrend.StateWarning {
		t.Fatalf("claude observed point = %+v, want one warning point from skipped test", claudeObserved)
	}
	if len(claude.SupportedEndpoints) != 1 || claude.SupportedEndpoints[0] != model.ChannelModelEndpointMessages {
		t.Fatalf("claude endpoints = %#v, want messages", claude.SupportedEndpoints)
	}
}

func TestLoadUserModelStatusTrafficRowsFiltersClientAbort(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("auto migrate logs: %v", err)
	}
	now := helper.GetTimestamp()
	rows := []model.Log{
		{Id: "success", Type: model.LogTypeConsume, ChannelId: "channel-1", RequestModelName: "gpt-5.4", CreatedAt: now, ElapsedTime: 1200},
		{Id: "failure", Type: model.LogTypeRelayFailure, ChannelId: "channel-1", RequestModelName: "gpt-5.4", CreatedAt: now, RelayErrorCode: "upstream_unavailable"},
		{Id: "abort", Type: model.LogTypeRelayFailure, ChannelId: "channel-1", RequestModelName: "gpt-5.4", CreatedAt: now, RelayErrorType: "client_abort"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}
	originalLogDB := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = originalLogDB })
	got, err := loadUserModelStatusTrafficRows([]string{"channel-1"}, []string{"gpt-5.4"}, now-60)
	if err != nil {
		t.Fatalf("load traffic rows: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("traffic rows=%d, want 1 bucket", len(got))
	}
	if got[0].SuccessCount != 1 || got[0].FailureCount != 1 {
		t.Fatalf("bucket success/failure=%d/%d, want 1/1", got[0].SuccessCount, got[0].FailureCount)
	}
	if got[0].LatencyTotal != 1200 || got[0].LatencyCount != 1 {
		t.Fatalf("bucket latency total/count=%d/%d, want 1200/1", got[0].LatencyTotal, got[0].LatencyCount)
	}
}
