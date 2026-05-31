package channel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChannelEndpointControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.ChannelModel{},
		&model.ChannelTest{},
		&model.ProviderModel{},
		&model.ChannelModelEndpoint{},
		&model.ChannelModelEndpointTestResult{},
		&model.ChannelModelPriceComponent{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
	return db
}

func callUpdateChannelEndpointForTest(t *testing.T, channelID string, body any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: channelID}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/channel/"+channelID+"/endpoints", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateChannelEndpoint(c)

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", recorder.Body.String(), err)
	}
	return resp
}

func callGetChannelEndpointsForTest(t *testing.T, channelID string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: channelID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel/"+channelID+"/endpoints", nil)

	GetChannelEndpoints(c)

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response %q: %v", recorder.Body.String(), err)
	}
	return resp
}

func seedEndpointControllerChannel(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&model.Channel{
		Id:       "channel-1",
		Name:     "channel-1",
		Protocol: "openai",
		Status:   model.ChannelStatusEnabled,
	}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.ChannelModel{
		ChannelId:     "channel-1",
		Model:         "qwen3.7-max",
		UpstreamModel: "qwen3.7-max",
		Provider:      "qwen",
		Type:          model.ProviderModelTypeText,
		Selected:      true,
	}).Error; err != nil {
		t.Fatalf("create channel model: %v", err)
	}
	if err := db.Create(&model.ProviderModel{
		Provider:           "qwen",
		Model:              "qwen3.7-max",
		Tags:               model.ProviderModelTypeText,
		Status:             model.ProviderModelStatusActive,
		SupportedEndpoints: model.ChannelModelEndpointChat,
	}).Error; err != nil {
		t.Fatalf("create provider model: %v", err)
	}
}

func TestUpdateChannelEndpointRequiresExactModelTestResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newChannelEndpointControllerTestDB(t)
	seedEndpointControllerChannel(t, db)
	if err := db.Create(&model.ChannelModelEndpointTestResult{
		ChannelId:      "channel-1",
		Model:          "other-model",
		Endpoint:       model.ChannelModelEndpointChat,
		UpstreamModel:  "qwen3.7-max",
		LastTestStatus: model.ChannelModelEndpointTestStatusSuccess,
		LastSupported:  true,
	}).Error; err != nil {
		t.Fatalf("create loose endpoint test result: %v", err)
	}

	resp := callUpdateChannelEndpointForTest(t, "channel-1", map[string]any{
		"model":    "qwen3.7-max",
		"endpoint": model.ChannelModelEndpointChat,
		"enabled":  true,
	})
	if resp["success"] == true {
		t.Fatalf("UpdateChannelEndpoint success=true, want false")
	}
	if !strings.Contains(resp["message"].(string), "缺少最近一次成功测试结果") {
		t.Fatalf("UpdateChannelEndpoint message=%v, want exact test result error", resp["message"])
	}

	if err := db.Create(&model.ChannelModelEndpointTestResult{
		ChannelId:      "channel-1",
		Model:          "qwen3.7-max",
		Endpoint:       model.ChannelModelEndpointChat,
		LastTestStatus: model.ChannelModelEndpointTestStatusSuccess,
		LastSupported:  true,
	}).Error; err != nil {
		t.Fatalf("create exact endpoint test result: %v", err)
	}
	resp = callUpdateChannelEndpointForTest(t, "channel-1", map[string]any{
		"model":    "qwen3.7-max",
		"endpoint": model.ChannelModelEndpointChat,
		"enabled":  true,
	})
	if resp["success"] != true {
		t.Fatalf("UpdateChannelEndpoint success=%v, message=%v, want true", resp["success"], resp["message"])
	}
}

func TestGetChannelEndpointsDoesNotShowLooseUpstreamModelSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newChannelEndpointControllerTestDB(t)
	seedEndpointControllerChannel(t, db)
	if err := db.Create(&model.ChannelModelEndpointTestResult{
		ChannelId:      "channel-1",
		Model:          "other-model",
		Endpoint:       model.ChannelModelEndpointChat,
		UpstreamModel:  "qwen3.7-max",
		LastTestStatus: model.ChannelModelEndpointTestStatusSuccess,
		LastSupported:  true,
	}).Error; err != nil {
		t.Fatalf("create loose endpoint test result: %v", err)
	}

	resp := callGetChannelEndpointsForTest(t, "channel-1")
	if resp["success"] != true {
		t.Fatalf("GetChannelEndpoints success=%v message=%v, want true", resp["success"], resp["message"])
	}
	data := resp["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len=%d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if status, ok := item["last_test_status"].(string); ok && status != "" {
		t.Fatalf("last_test_status=%q, want empty because exact model has no result", status)
	}
	reason, _ := item["enable_block_reason"].(string)
	if !strings.Contains(reason, "缺少最近一次成功测试结果") {
		t.Fatalf("enable_block_reason=%q, want exact test result block reason", reason)
	}
}

func TestCollectRestoredChannelModelCapabilities(t *testing.T) {
	currentRows := []model.ChannelModel{
		{
			Model:          "qwen3.7-max",
			Inactive:       true,
			Selected:       false,
			DisabledReason: "model not found",
			DisabledAt:     123,
			DisabledBy:     "runtime",
		},
		{
			Model:    "deepseek-chat",
			Inactive: true,
			Selected: false,
		},
	}
	nextRows := []model.ChannelModel{
		{
			Model:    "qwen3.7-max",
			Selected: true,
		},
		{
			Model:    "deepseek-chat",
			Selected: true,
		},
	}

	restored := collectRestoredChannelModelCapabilities(currentRows, nextRows)
	if len(restored) != 1 || restored[0] != "qwen3.7-max" {
		t.Fatalf("restored=%v, want [qwen3.7-max]", restored)
	}
}

func TestIsChannelModelEndpointRuntimeDisabled(t *testing.T) {
	rows := []model.ChannelModelEndpoint{
		{
			Model:          "qwen3.7-max",
			Endpoint:       model.ChannelModelEndpointResponses,
			Enabled:        false,
			DisabledReason: "unsupported endpoint",
			DisabledAt:     123,
			DisabledBy:     "runtime",
		},
		{
			Model:    "qwen3.7-max",
			Endpoint: model.ChannelModelEndpointChat,
			Enabled:  false,
		},
	}

	if !isChannelModelEndpointRuntimeDisabled(rows, "qwen3.7-max", model.ChannelModelEndpointResponses) {
		t.Fatalf("responses endpoint disabled=false, want true")
	}
	if isChannelModelEndpointRuntimeDisabled(rows, "qwen3.7-max", model.ChannelModelEndpointChat) {
		t.Fatalf("chat endpoint disabled=true, want false")
	}
}
