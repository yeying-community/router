package channel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCostQuoteHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Channel{},
		&model.ChannelModel{},
		&model.ChannelTest{},
		&model.ChannelModelEndpoint{},
		&model.ChannelModelEndpointTestResult{},
		&model.ChannelModelPriceComponent{},
		&model.ProviderModel{},
		&model.ChannelBillingProfile{},
		&model.ChannelProcurementBatch{},
		&model.ChannelModelCostRate{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	return db
}

func decodeCostHandlerJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, string(body))
	}
	return out
}

// With no billing service configured, the read-only reconciliation handler must
// succeed with service_available=false — informational, never a hard failure.
func TestGetChannelCostQuoteReconciliationServiceUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newCostQuoteHandlerTestDB(t)
	if err := db.Create(&model.Channel{Id: "ch-1", Name: "ch-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	origBase := config.BillingServiceBaseURL
	config.BillingServiceBaseURL = ""
	t.Cleanup(func() { config.BillingServiceBaseURL = origBase })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel/ch-1/billing/cost-quotes", nil)
	c.Params = gin.Params{{Key: "id", Value: "ch-1"}}

	GetChannelCostQuoteReconciliation(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	resp := decodeCostHandlerJSON(t, w.Body.Bytes())
	if resp["success"] != true {
		t.Fatalf("success = %v, want true", resp["success"])
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil || data["service_available"] != false {
		t.Fatalf("service_available = %v, want false; data=%+v", data["service_available"], data)
	}
}

func TestGetChannelCostQuoteReconciliationEmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCostQuoteHandlerTestDB(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/channel//billing/cost-quotes", nil)
	c.Params = gin.Params{{Key: "id", Value: ""}}

	GetChannelCostQuoteReconciliation(c)

	resp := decodeCostHandlerJSON(t, w.Body.Bytes())
	if resp["success"] != false {
		t.Fatalf("success = %v, want false for empty id", resp["success"])
	}
}

// Sync against an unconfigured service surfaces a plain failure (no panic, no
// partial write).
func TestSyncChannelCostQuotesServiceUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newCostQuoteHandlerTestDB(t)
	if err := db.Create(&model.Channel{Id: "ch-1", Name: "ch-1", Protocol: "openai"}).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	origBase := config.BillingServiceBaseURL
	config.BillingServiceBaseURL = ""
	t.Cleanup(func() { config.BillingServiceBaseURL = origBase })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/channel/ch-1/billing/cost-quotes/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: "ch-1"}}

	SyncChannelCostQuotes(c)

	resp := decodeCostHandlerJSON(t, w.Body.Bytes())
	if resp["success"] != false {
		t.Fatalf("success = %v, want false when service unconfigured", resp["success"])
	}
	// The cache must remain untouched.
	var count int64
	db.Model(&model.ChannelModelCostRate{}).Where("channel_id = ?", "ch-1").Count(&count)
	if count != 0 {
		t.Fatalf("cache rows = %d, want 0", count)
	}
}
