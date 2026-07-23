package model

import (
	"fmt"
	"strings"
	"testing"

	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func float64Ptr(value float64) *float64 {
	return &value
}

func openChannelModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	return db
}

func TestBuildDefaultChannelModelsWithProtocol_AnthropicPrefersMessages(t *testing.T) {
	rows := BuildDefaultChannelModelsWithProtocol([]string{"claude-sonnet-4-6"}, relaychannel.Anthropic)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Endpoint != ChannelModelEndpointMessages {
		t.Fatalf("rows[0].Endpoint = %q, want %q", rows[0].Endpoint, ChannelModelEndpointMessages)
	}
	if len(rows[0].Endpoints) == 0 || rows[0].Endpoints[0] != ChannelModelEndpointMessages {
		t.Fatalf("rows[0].Endpoints = %#v, want first %q", rows[0].Endpoints, ChannelModelEndpointMessages)
	}
}

func TestNormalizeChannelModelRow_PreservesExplicitPrimaryEndpoint(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointResponses,
			ChannelModelEndpointChat,
		},
	}
	normalizeChannelModelRow(&row)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
	if len(row.Endpoints) != 2 {
		t.Fatalf("len(row.Endpoints) = %d, want 2", len(row.Endpoints))
	}
}

func TestCompleteChannelModelRowDefaults_PreservesExplicitPrimaryEndpoint(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointResponses,
			ChannelModelEndpointChat,
		},
	}
	completeChannelModelRowDefaults(&row, relaychannel.OpenAI)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
	if len(row.Endpoints) != 2 {
		t.Fatalf("len(row.Endpoints) = %d, want 2", len(row.Endpoints))
	}
}

func TestCompleteChannelModelRowDefaults_PreservesExplicitEmbeddingType(t *testing.T) {
	row := ChannelModel{
		Model:         "custom-embedding",
		UpstreamModel: "custom-embedding",
		Type:          ProviderModelTypeEmbedding,
		Endpoint:      ChannelModelEndpointEmbeddings,
		Endpoints: []string{
			ChannelModelEndpointEmbeddings,
		},
	}
	completeChannelModelRowDefaults(&row, relaychannel.OpenAI)
	if row.Type != ProviderModelTypeEmbedding {
		t.Fatalf("row.Type = %q, want %q", row.Type, ProviderModelTypeEmbedding)
	}
	if row.Endpoint != ChannelModelEndpointEmbeddings {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointEmbeddings)
	}
}

func TestNormalizeChannelModelRow_UsesFirstEndpointWhenPrimaryMissing(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      "",
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}
	normalizeChannelModelRow(&row)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
}

func TestApplyChannelModelEndpointState_PreservesExplicitPrimaryEvenIfDisabled(t *testing.T) {
	row := ChannelModel{
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Type:          ProviderModelTypeText,
		Endpoint:      ChannelModelEndpointChat,
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
	}
	state := channelModelEndpointState{
		Endpoints: []string{
			ChannelModelEndpointChat,
			ChannelModelEndpointResponses,
		},
		Enabled: map[string]bool{
			ChannelModelEndpointChat:      false,
			ChannelModelEndpointResponses: true,
		},
	}
	applyChannelModelEndpointState(&row, state)
	if row.Endpoint != ChannelModelEndpointChat {
		t.Fatalf("row.Endpoint = %q, want %q", row.Endpoint, ChannelModelEndpointChat)
	}
}

func TestBuildFetchedChannelModelsPreservesExistingSelectionsAndMarksMissingRowsUnselected(t *testing.T) {
	existingRows := []ChannelModel{
		{
			Model:         "alias-gpt-4.1",
			UpstreamModel: "gpt-4.1",
			Type:          ProviderModelTypeText,
			Selected:      false,
			InputPrice:    float64Ptr(1.2),
			OutputPrice:   float64Ptr(2.4),
			PriceUnit:     "per_1k_tokens",
			Currency:      "USD",
		},
		{
			Model:         "legacy-removed",
			UpstreamModel: "legacy-removed",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	fetchedRows := []ChannelModel{
		{
			Model:         "gpt-4.1",
			UpstreamModel: "gpt-4.1",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			Model:         "gpt-image-1",
			UpstreamModel: "gpt-image-1",
			Type:          ProviderModelTypeImage,
			Selected:      true,
		},
	}

	rows := BuildFetchedChannelModels(existingRows, fetchedRows, 0, false)
	if len(rows) != 3 {
		t.Fatalf("BuildFetchedChannelModels returned %d rows, want 3", len(rows))
	}

	if rows[0].Model != "alias-gpt-4.1" {
		t.Fatalf("rows[0].Model = %q, want alias-gpt-4.1", rows[0].Model)
	}
	if rows[0].UpstreamModel != "gpt-4.1" {
		t.Fatalf("rows[0].UpstreamModel = %q, want gpt-4.1", rows[0].UpstreamModel)
	}
	if rows[0].Selected {
		t.Fatalf("rows[0].Selected = true, want false")
	}
	if rows[0].InputPrice == nil || *rows[0].InputPrice != 1.2 {
		t.Fatalf("rows[0].InputPrice = %#v, want 1.2", rows[0].InputPrice)
	}
	if rows[0].OutputPrice == nil || *rows[0].OutputPrice != 2.4 {
		t.Fatalf("rows[0].OutputPrice = %#v, want 2.4", rows[0].OutputPrice)
	}
	if rows[1].Model != "gpt-image-1" {
		t.Fatalf("rows[1].Model = %q, want gpt-image-1", rows[1].Model)
	}
	if rows[1].Selected {
		t.Fatalf("rows[1].Selected = true, want false")
	}

	if rows[2].Model != "legacy-removed" {
		t.Fatalf("rows[2].Model = %q, want legacy-removed", rows[2].Model)
	}
	if rows[2].Selected {
		t.Fatalf("rows[2].Selected = true, want false")
	}
}

func TestBuildDisabledChannelModelConfigsMarksOnlyTargetModelUnselected(t *testing.T) {
	rows := []ChannelModel{
		{
			Model:         "gpt-5.3-codex",
			UpstreamModel: "gpt-5.3-codex",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	updated, changed := buildDisabledChannelModels(rows, "gpt-5.3-codex", "model not found", "runtime")
	if !changed {
		t.Fatalf("changed = false, want true")
	}
	if len(updated) != 2 {
		t.Fatalf("updated len = %d, want 2", len(updated))
	}
	if updated[0].Selected {
		t.Fatalf("updated[0].Selected = true, want false")
	}
	if updated[0].DisabledReason != "model not found" || updated[0].DisabledBy != "runtime" || updated[0].DisabledAt == 0 {
		t.Fatalf("updated[0] disable metadata = reason:%q by:%q at:%d, want populated", updated[0].DisabledReason, updated[0].DisabledBy, updated[0].DisabledAt)
	}
	if !updated[1].Selected {
		t.Fatalf("updated[1].Selected = false, want true")
	}
}

func TestBuildDisabledChannelModelConfigsNoopWhenTargetMissing(t *testing.T) {
	rows := []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}

	updated, changed := buildDisabledChannelModels(rows, "gpt-5.3-codex", "", "")
	if changed {
		t.Fatalf("changed = true, want false")
	}
	if len(updated) != 1 {
		t.Fatalf("updated len = %d, want 1", len(updated))
	}
	if !updated[0].Selected {
		t.Fatalf("updated[0].Selected = false, want true")
	}
}

func TestReplaceChannelModelsWithDBClearsRuntimeDisableMetadataWhenModelRestored(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
		&ProviderModel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:      "channel-1",
		Model:          "gpt-5.4",
		UpstreamModel:  "gpt-5.4",
		Provider:       "openai",
		Type:           ProviderModelTypeText,
		Selected:       false,
		DisabledReason: "model not found",
		DisabledAt:     123,
		DisabledBy:     "runtime",
	}).Error; err != nil {
		t.Fatalf("create disabled channel model: %v", err)
	}

	if err := ReplaceChannelModelsWithDB(db, "channel-1", []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}); err != nil {
		t.Fatalf("ReplaceChannelModelsWithDB: %v", err)
	}

	rows, err := listChannelModelRowsByChannelIDWithDB(db, "channel-1")
	if err != nil {
		t.Fatalf("list channel models: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if !rows[0].Selected {
		t.Fatalf("rows[0].Selected = false, want true")
	}
	if rows[0].DisabledReason != "" || rows[0].DisabledAt != 0 || rows[0].DisabledBy != "" {
		t.Fatalf("disable metadata = reason:%q at:%d by:%q, want cleared", rows[0].DisabledReason, rows[0].DisabledAt, rows[0].DisabledBy)
	}
}

func TestReplaceChannelModelsWithDBPreservesPublishStateForUnchangedSelectedModel(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
		&ProviderModel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:      "channel-1",
		Model:          "gpt-5.4",
		UpstreamModel:  "gpt-5.4",
		Provider:       "openai",
		Type:           ProviderModelTypeText,
		Selected:       true,
		PublishEnabled: true,
		PublishedAt:    123,
		PublishedBy:    "migration",
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}

	if err := ReplaceChannelModelsWithDB(db, "channel-1", []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}); err != nil {
		t.Fatalf("ReplaceChannelModelsWithDB: %v", err)
	}

	stored := ChannelModel{}
	if err := db.Where("channel_id = ? AND model = ?", "channel-1", "gpt-5.4").Take(&stored).Error; err != nil {
		t.Fatalf("load channel model: %v", err)
	}
	if !stored.PublishEnabled || stored.PublishedAt != 123 || stored.PublishedBy != "migration" {
		t.Fatalf("publish fields = enabled:%v at:%d by:%q, want preserved", stored.PublishEnabled, stored.PublishedAt, stored.PublishedBy)
	}
}

func TestReplaceChannelModelsWithDBClearsPublishStateWhenRoutingConfigChanges(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
		&ProviderModel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:      "channel-1",
		Model:          "gpt-5.4",
		UpstreamModel:  "gpt-5.4",
		Provider:       "openai",
		Type:           ProviderModelTypeText,
		Selected:       true,
		PublishEnabled: true,
		PublishedAt:    123,
		PublishedBy:    "migration",
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}

	if err := ReplaceChannelModelsWithDB(db, "channel-1", []ChannelModel{
		{
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4-preview",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}); err != nil {
		t.Fatalf("ReplaceChannelModelsWithDB: %v", err)
	}

	stored := ChannelModel{}
	if err := db.Where("channel_id = ? AND model = ?", "channel-1", "gpt-5.4").Take(&stored).Error; err != nil {
		t.Fatalf("load channel model: %v", err)
	}
	if stored.PublishEnabled || stored.PublishedAt != 0 || stored.PublishedBy != "" {
		t.Fatalf("publish fields = enabled:%v at:%d by:%q, want cleared", stored.PublishEnabled, stored.PublishedAt, stored.PublishedBy)
	}
}

func TestSetChannelModelPublishEnabledWithDBBlocksUnsupportedImageTokenBilling(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "unsupported-image",
		UpstreamModel: "unsupported-image",
		Provider:      "openai",
		Type:          ProviderModelTypeImage,
		Selected:      true,
		PriceUnit:     ProviderPriceUnitPer1KTokens,
		InputPrice:    float64Ptr(0.01),
		OutputPrice:   float64Ptr(0.04),
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: "channel-1",
		Model:     "unsupported-image",
		Endpoint:  ChannelModelEndpointImages,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	if err := db.Create(&ChannelModelEndpointTestResult{
		ChannelId:      "channel-1",
		Model:          "unsupported-image",
		Endpoint:       ChannelModelEndpointImages,
		LastSupported:  true,
		LastTestStatus: ChannelModelEndpointTestStatusSuccess,
	}).Error; err != nil {
		t.Fatalf("create endpoint test result: %v", err)
	}

	err := SetChannelModelPublishEnabledWithDB(db, "channel-1", "unsupported-image", true, "", "tester")
	if err == nil {
		t.Fatal("SetChannelModelPublishEnabledWithDB error = nil, want unsupported token image billing block")
	}
	if !strings.Contains(err.Error(), "不支持可靠本地估算") {
		t.Fatalf("error = %q, want unsupported image token billing block", err.Error())
	}
}

func TestSetChannelModelPublishEnabledWithDBAllowsGPTImage2TokenBilling(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
		&ChannelProcurementBatch{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-image-2",
		UpstreamModel: "gpt-image-2",
		Provider:      "openai",
		Type:          ProviderModelTypeImage,
		Selected:      true,
		PriceUnit:     ProviderPriceUnitPer1KTokens,
		OutputPrice:   float64Ptr(0.03),
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelPriceComponent{
		ChannelId:  "channel-1",
		Model:      "gpt-image-2",
		Component:  ProviderModelPriceComponentText,
		InputPrice: 0.008,
		PriceUnit:  ProviderPriceUnitPer1KTokens,
		Currency:   ProviderPriceCurrencyUSD,
	}).Error; err != nil {
		t.Fatalf("create channel model price component: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: "channel-1",
		Model:     "gpt-image-2",
		Endpoint:  ChannelModelEndpointImages,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	if err := db.Create(&ChannelModelEndpointTestResult{
		ChannelId:      "channel-1",
		Model:          "gpt-image-2",
		Endpoint:       ChannelModelEndpointImages,
		LastSupported:  true,
		LastTestStatus: ChannelModelEndpointTestStatusSuccess,
	}).Error; err != nil {
		t.Fatalf("create endpoint test result: %v", err)
	}
	if err := db.Create(&ChannelProcurementBatch{
		Id:                "batch-gpt-image-2",
		ChannelId:         "channel-1",
		ScopeType:         "model",
		ScopeValue:        "gpt-image-2",
		CapacityUnit:      "token",
		CapacityTotal:     1000000,
		CapacityEffective: 1000000,
		CapacityRemaining: 1000000,
		CostPerUnitAmount: 0.000001,
		CostSource:        ProcurementCostSourceActual,
		CostStatus:        ProcurementCostStatusActive,
	}).Error; err != nil {
		t.Fatalf("create procurement batch: %v", err)
	}

	if err := SetChannelModelPublishEnabledWithDB(db, "channel-1", "gpt-image-2", true, "gpt-image-2-public", "tester"); err != nil {
		t.Fatalf("SetChannelModelPublishEnabledWithDB error = %v", err)
	}
	stored := ChannelModel{}
	if err := db.Where("channel_id = ? AND model = ?", "channel-1", "gpt-image-2").Take(&stored).Error; err != nil {
		t.Fatalf("load channel model: %v", err)
	}
	if !stored.PublishEnabled {
		t.Fatalf("PublishEnabled = false, want true")
	}
	if stored.PublishedModel != "gpt-image-2-public" {
		t.Fatalf("PublishedModel = %q, want gpt-image-2-public", stored.PublishedModel)
	}
}

func TestSetChannelModelPublishEnabledWithDBBlocksDuplicatePublishedModel(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointTestResult{},
		&ChannelModelPriceComponent{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	rows := []ChannelModel{
		{
			ChannelId:      "channel-1",
			Model:          "qwen3.7-plus",
			UpstreamModel:  "qwen3.7-plus",
			Provider:       "qwen",
			Type:           ProviderModelTypeText,
			Selected:       true,
			PublishEnabled: true,
			PublishedModel: "qwen3.7-plus",
		},
		{
			ChannelId:     "channel-1",
			Model:         "qwen3.7-plus-2026-05-26",
			UpstreamModel: "qwen3.7-plus-2026-05-26",
			Provider:      "qwen",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create channel models: %v", err)
	}
	for _, modelName := range []string{"qwen3.7-plus", "qwen3.7-plus-2026-05-26"} {
		if err := db.Create(&ChannelModelEndpoint{
			ChannelId: "channel-1",
			Model:     modelName,
			Endpoint:  ChannelModelEndpointResponses,
			Enabled:   true,
		}).Error; err != nil {
			t.Fatalf("create endpoint %s: %v", modelName, err)
		}
		if err := db.Create(&ChannelModelEndpointTestResult{
			ChannelId:      "channel-1",
			Model:          modelName,
			Endpoint:       ChannelModelEndpointResponses,
			LastSupported:  true,
			LastTestStatus: ChannelModelEndpointTestStatusSuccess,
		}).Error; err != nil {
			t.Fatalf("create endpoint test result %s: %v", modelName, err)
		}
	}

	err := SetChannelModelPublishEnabledWithDB(db, "channel-1", "qwen3.7-plus-2026-05-26", true, "qwen3.7-plus", "tester")
	if err == nil {
		t.Fatal("SetChannelModelPublishEnabledWithDB error = nil, want duplicate published model block")
	}
	if !strings.Contains(err.Error(), "发布名称 qwen3.7-plus 已被该渠道其他模型使用") {
		t.Fatalf("error = %q, want duplicate published model block", err.Error())
	}
}

func TestValidateChannelModelDisableTransitionsWithDBBlocksWhenEnabledEndpointsExist(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(&ChannelModelEndpoint{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: "channel-1",
		Model:     "gpt-5.4",
		Endpoint:  ChannelModelEndpointResponses,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create endpoint row: %v", err)
	}
	existingRows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Selected:      true,
		},
	}
	nextRows := []ChannelModel{
		{
			ChannelId:     "channel-1",
			Model:         "gpt-5.4",
			UpstreamModel: "gpt-5.4",
			Selected:      false,
		},
	}
	err := ValidateChannelModelDisableTransitionsWithDB(db, "channel-1", existingRows, nextRows)
	if err == nil {
		t.Fatalf("ValidateChannelModelDisableTransitionsWithDB error = nil, want block")
	}
	if !strings.Contains(err.Error(), "仍有已启用端点") {
		t.Fatalf("error = %q, want enabled endpoint block", err.Error())
	}
}

func TestDeleteChannelModelWithDBBlocksWhenModelStillReturned(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointPolicy{},
		&ChannelModelSyncResult{},
		&ChannelTest{},
		&ChannelModelPriceComponent{},
		&GroupModelChannel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Selected:      true,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Returned:      true,
	}).Error; err != nil {
		t.Fatalf("create sync result: %v", err)
	}
	err := DeleteChannelModelWithDB(db, "channel-1", "gpt-5.4", "")
	if err == nil {
		t.Fatalf("DeleteChannelModelWithDB error = nil, want block")
	}
	if !strings.Contains(err.Error(), "最近一次上游返回仍包含") {
		t.Fatalf("error = %q, want returned block", err.Error())
	}
}

func TestDeleteChannelModelWithDBIgnoresReturnedDisplayModel(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointPolicy{},
		&ChannelModelSyncResult{},
		&ChannelTest{},
		&ChannelModelPriceComponent{},
		&GroupModelChannel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-5.1-codex-mini",
		UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
		Selected:      false,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-5.1-codex-mini",
		UpstreamModel: "gpt-5.1-codex-mini",
		Returned:      true,
	}).Error; err != nil {
		t.Fatalf("create sync result: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-5.1-codex-mini-2025-11-13",
		UpstreamModel: "gpt-5.1-codex-mini-2025-11-13",
		Returned:      false,
	}).Error; err != nil {
		t.Fatalf("create sync result: %v", err)
	}
	err := DeleteChannelModelWithDB(db, "channel-1", "gpt-5.1-codex-mini", "gpt-5.1-codex-mini-2025-11-13")
	if err != nil {
		t.Fatalf("DeleteChannelModelWithDB error = %v, want nil", err)
	}
}

func TestDeleteChannelModelWithDBPrefersExactUpstreamModel(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointPolicy{},
		&ChannelModelSyncResult{},
		&ChannelTest{},
		&ChannelModelPriceComponent{},
		&GroupModelChannel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-4o-mini-tts",
		UpstreamModel: "gpt-4o-mini-tts",
		Selected:      true,
		SortOrder:     1,
	}).Error; err != nil {
		t.Fatalf("create base channel model: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-4o-mini-tts-2025-12-15",
		UpstreamModel: "gpt-4o-mini-tts-2025-12-15",
		Selected:      false,
		SortOrder:     2,
	}).Error; err != nil {
		t.Fatalf("create dated channel model: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-4o-mini-tts",
		UpstreamModel: "gpt-4o-mini-tts",
		Returned:      true,
	}).Error; err != nil {
		t.Fatalf("create returned sync result: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-4o-mini-tts-2025-12-15",
		UpstreamModel: "gpt-4o-mini-tts-2025-12-15",
		Returned:      false,
	}).Error; err != nil {
		t.Fatalf("create not returned sync result: %v", err)
	}
	err := DeleteChannelModelWithDB(db, "channel-1", "gpt-4o-mini-tts-2025-12-15", "gpt-4o-mini-tts-2025-12-15")
	if err != nil {
		t.Fatalf("DeleteChannelModelWithDB error = %v, want nil", err)
	}
}

func TestDeleteChannelModelWithDBBlocksWhenEnabledEndpointsExist(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointPolicy{},
		&ChannelModelSyncResult{},
		&ChannelTest{},
		&ChannelModelPriceComponent{},
		&GroupModelChannel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Selected:      true,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: "channel-1",
		Model:     "gpt-5.4",
		Endpoint:  ChannelModelEndpointResponses,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create enabled endpoint: %v", err)
	}
	err := DeleteChannelModelWithDB(db, "channel-1", "gpt-5.4", "")
	if err == nil {
		t.Fatalf("DeleteChannelModelWithDB error = nil, want block")
	}
	if !strings.Contains(err.Error(), "仍有已启用端点") {
		t.Fatalf("error = %q, want enabled endpoint block", err.Error())
	}
}

func TestDeleteChannelModelWithDBAllowsNotReturnedModelWithEnabledEndpoints(t *testing.T) {
	db := openChannelModelTestDB(t)
	if err := db.AutoMigrate(
		&Channel{},
		&ChannelModel{},
		&ChannelModelEndpoint{},
		&ChannelModelEndpointPolicy{},
		&ChannelModelSyncResult{},
		&ChannelTest{},
		&ChannelModelPriceComponent{},
		&GroupModelChannel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&Channel{Id: "channel-1", Name: "channel-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     "channel-1",
		Model:         "gpt-realtime-1.5-2026-02-23",
		UpstreamModel: "gpt-realtime-1.5-2026-02-23",
		Selected:      false,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelSyncResult{
		ChannelId:     "channel-1",
		Model:         "gpt-realtime-1.5-2026-02-23",
		UpstreamModel: "gpt-realtime-1.5-2026-02-23",
		Returned:      false,
	}).Error; err != nil {
		t.Fatalf("create sync result: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: "channel-1",
		Model:     "gpt-realtime-1.5-2026-02-23",
		Endpoint:  ChannelModelEndpointRealtime,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create enabled endpoint: %v", err)
	}
	err := DeleteChannelModelWithDB(db, "channel-1", "gpt-realtime-1.5-2026-02-23", "gpt-realtime-1.5-2026-02-23")
	if err != nil {
		t.Fatalf("DeleteChannelModelWithDB error = %v, want nil", err)
	}
}
