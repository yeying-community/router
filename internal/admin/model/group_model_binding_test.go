package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openGroupModelBindingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&GroupCatalog{}, &GroupChannel{}, &GroupModel{}, &GroupModelChannel{}, &Channel{}, &ChannelModel{}, &ChannelModelEndpoint{}, &ChannelModelEndpointTestResult{}, &ChannelModelPriceComponent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestBuildGroupModelBindingItemsPreservesDisabledGroupModelState(t *testing.T) {
	priority := int64(3)
	rows := []GroupModelChannel{
		{
			Group:         "group-a",
			Model:         "gpt-5.1-codex-mini",
			ChannelId:     "channel-1",
			UpstreamModel: "gpt-5.1-codex-mini",
			Priority:      &priority,
		},
	}
	channelByID := map[string]Channel{
		"channel-1": {
			Id:       "channel-1",
			Name:     "channel-1",
			Protocol: "openai",
			Status:   ChannelStatusEnabled,
		},
	}
	enabledByModel := map[string]bool{
		"gpt-5.1-codex-mini": false,
	}

	items := buildGroupModelBindingItems(rows, channelByID, enabledByModel)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Enabled == nil {
		t.Fatalf("items[0].Enabled is nil, want false pointer")
	}
	if *items[0].Enabled {
		t.Fatalf("items[0].Enabled = true, want false")
	}
}

func TestReplaceSingleGroupModelWithDB_PreservesDisabledState(t *testing.T) {
	db := openGroupModelBindingTestDB(t)

	group := GroupCatalog{
		Id:      "group-1",
		Name:    "group-1",
		Enabled: true,
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	channel := Channel{
		Id:       "channel-1",
		Name:     "channel-1",
		Protocol: "openai",
		Status:   ChannelStatusEnabled,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&GroupChannel{
		Group:     group.Id,
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  10,
	}).Error; err != nil {
		t.Fatalf("create group channel: %v", err)
	}
	if err := db.Create(&ChannelModel{
		ChannelId:     channel.Id,
		Model:         "gpt-5.1",
		UpstreamModel: "gpt-5.1",
		Provider:      "openai",
		Type:          ProviderModelTypeText,
		Selected:      true,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&ChannelModelEndpoint{
		ChannelId: channel.Id,
		Model:     "gpt-5.1",
		Endpoint:  ChannelModelEndpointResponses,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create channel model endpoint: %v", err)
	}
	if err := db.Create(&ChannelModelEndpointTestResult{
		ChannelId:      channel.Id,
		Model:          "gpt-5.1",
		Endpoint:       ChannelModelEndpointResponses,
		LastTestStatus: ChannelModelEndpointTestStatusSuccess,
		LastSupported:  true,
	}).Error; err != nil {
		t.Fatalf("create channel model endpoint test result: %v", err)
	}

	enabled := true
	if err := replaceSingleGroupModelWithDB(db, group.Id, "gpt-5.1", []GroupModelBindingItem{
		{
			Model:         "gpt-5.1",
			ChannelId:     channel.Id,
			UpstreamModel: "gpt-5.1",
			Enabled:       &enabled,
		},
	}); err != nil {
		t.Fatalf("replace group model enabled: %v", err)
	}

	disabled := false
	if err := replaceSingleGroupModelWithDB(db, group.Id, "gpt-5.1", []GroupModelBindingItem{
		{
			Model:         "gpt-5.1",
			ChannelId:     channel.Id,
			UpstreamModel: "gpt-5.1",
			Enabled:       &disabled,
		},
	}); err != nil {
		t.Fatalf("replace group model disabled: %v", err)
	}

	rows, err := listGroupModelRowsWithDB(db, group.Id, false)
	if err != nil {
		t.Fatalf("list group model rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Enabled {
		t.Fatalf("rows[0].Enabled = true, want false")
	}

	items, err := listGroupModelBindingItemsWithDB(db, group.Id)
	if err != nil {
		t.Fatalf("list group model binding items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Enabled == nil {
		t.Fatalf("items[0].Enabled is nil, want false pointer")
	}
	if *items[0].Enabled {
		t.Fatalf("items[0].Enabled = true, want false")
	}
}

func TestReplaceGroupModelRowsWithDB_PreservesDisabledState(t *testing.T) {
	db := openGroupModelBindingTestDB(t)
	group := GroupCatalog{
		Id:      "group-1",
		Name:    "group-1",
		Enabled: true,
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	if err := replaceGroupModelRowsWithDB(db, group.Id, []GroupModel{
		{
			Group:    group.Id,
			Model:    "gpt-image-2",
			Provider: "openai",
			Enabled:  false,
		},
	}); err != nil {
		t.Fatalf("replace group model rows: %v", err)
	}

	rows, err := listGroupModelRowsWithDB(db, group.Id, false)
	if err != nil {
		t.Fatalf("list group model rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].Enabled {
		t.Fatalf("rows[0].Enabled = true, want false")
	}
}

func TestBuildGroupChannelModelOptionsOnlyIncludesPublishedModels(t *testing.T) {
	db := openGroupModelBindingTestDB(t)
	channel := Channel{
		Id:       "channel-1",
		Name:     "channel-1",
		Protocol: "openai",
		Status:   ChannelStatusEnabled,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&[]ChannelModel{
		{
			ChannelId:     channel.Id,
			Model:         "published-model",
			UpstreamModel: "published-model",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			ChannelId:     channel.Id,
			Model:         "pending-config-model",
			UpstreamModel: "pending-config-model",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
		{
			ChannelId:     channel.Id,
			Model:         "pending-test-model",
			UpstreamModel: "pending-test-model",
			Provider:      "openai",
			Type:          ProviderModelTypeText,
			Selected:      true,
		},
	}).Error; err != nil {
		t.Fatalf("create channel models: %v", err)
	}
	if err := db.Create(&[]ChannelModelEndpoint{
		{
			ChannelId: channel.Id,
			Model:     "published-model",
			Endpoint:  ChannelModelEndpointResponses,
			Enabled:   true,
		},
		{
			ChannelId: channel.Id,
			Model:     "pending-test-model",
			Endpoint:  ChannelModelEndpointResponses,
			Enabled:   true,
		},
	}).Error; err != nil {
		t.Fatalf("create channel endpoints: %v", err)
	}
	if err := db.Create(&ChannelModelEndpointTestResult{
		ChannelId:      channel.Id,
		Model:          "published-model",
		Endpoint:       ChannelModelEndpointResponses,
		LastTestStatus: ChannelModelEndpointTestStatusSuccess,
		LastSupported:  true,
	}).Error; err != nil {
		t.Fatalf("create endpoint test result: %v", err)
	}

	loaded := Channel{}
	if err := db.First(&loaded, "id = ?", channel.Id).Error; err != nil {
		t.Fatalf("load channel: %v", err)
	}
	if err := HydrateChannelWithModels(db, &loaded); err != nil {
		t.Fatalf("hydrate channel: %v", err)
	}
	options := buildGroupChannelModelOptions(&loaded)
	if len(options) != 1 {
		t.Fatalf("len(options) = %d, want 1: %#v", len(options), options)
	}
	if options[0].Model != "published-model" {
		t.Fatalf("option model = %q, want published-model", options[0].Model)
	}

	statusByModel := make(map[string]string)
	for _, row := range loaded.GetChannelModels() {
		statusByModel[row.Model] = row.PublishStatus
	}
	if statusByModel["published-model"] != ChannelModelPublishStatusPublished {
		t.Fatalf("published status = %q, want published", statusByModel["published-model"])
	}
	if statusByModel["pending-config-model"] != ChannelModelPublishStatusPendingConfig {
		t.Fatalf("pending config status = %q, want pending_config", statusByModel["pending-config-model"])
	}
	if statusByModel["pending-test-model"] != ChannelModelPublishStatusPendingTest {
		t.Fatalf("pending test status = %q, want pending_test", statusByModel["pending-test-model"])
	}
}
