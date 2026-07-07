package controller

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/billing"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func TestGetAndValidateTextRequestMessagesPreservesMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{
		"model":"claude-opus-4-6",
		"system":[{"type":"text","text":"system prompt"}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":[{"type":"text","text":"world"}]}
		],
		"max_tokens":128,
		"stream":true
	}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	request, rawBody, err := getAndValidateTextRequest(ctx, relaymode.Messages)
	if err != nil {
		t.Fatalf("getAndValidateTextRequest returned error: %v", err)
	}
	if len(rawBody) == 0 {
		t.Fatalf("rawBody is empty")
	}
	if request == nil {
		t.Fatalf("request is nil")
	}
	if request.Model != "claude-opus-4-6" {
		t.Fatalf("request.Model = %q, want %q", request.Model, "claude-opus-4-6")
	}
	if len(request.Messages) != 3 {
		t.Fatalf("len(request.Messages) = %d, want 3", len(request.Messages))
	}
	if request.Messages[0].Role != "system" {
		t.Fatalf("request.Messages[0].Role = %q, want system", request.Messages[0].Role)
	}
	if request.Messages[1].Role != "user" || request.Messages[1].StringContent() != "hello" {
		t.Fatalf("unexpected user message: %#v", request.Messages[1])
	}
	if request.Messages[2].Role != "assistant" || request.Messages[2].StringContent() != "world" {
		t.Fatalf("unexpected assistant message: %#v", request.Messages[2])
	}
}

func TestResolveTextMaxOutputTokensUsesLargestLimit(t *testing.T) {
	maxCompletionTokens := 256
	maxOutputTokens := 384
	got := resolveTextMaxOutputTokens(&relaymodel.GeneralOpenAIRequest{
		MaxTokens:           128,
		MaxCompletionTokens: &maxCompletionTokens,
		MaxOutputTokens:     &maxOutputTokens,
	})
	if got != 384 {
		t.Fatalf("resolveTextMaxOutputTokens() = %d, want 384", got)
	}
}

func TestPreConsumeCanUseEstimatedTokenTierPricing(t *testing.T) {
	maxOutputTokens := 128
	basePricing := adminmodel.ResolvedModelPricing{
		Model:       "doubao-seed-1.6",
		Provider:    "volcengine",
		Type:        adminmodel.ProviderModelTypeText,
		InputPrice:  0.0008,
		OutputPrice: 0.008,
		PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
		Currency:    "CNY",
		Source:      "provider_migration",
		PriceComponents: []adminmodel.ProviderModelPriceComponentDetail{
			{
				Component:   adminmodel.ProviderModelPriceComponentText,
				Condition:   "prompt_tokens_lte=32000;completion_tokens_lte=200",
				InputPrice:  0.0008,
				OutputPrice: 0.002,
				PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:    "CNY",
				Source:      "migration",
			},
			{
				Component:   adminmodel.ProviderModelPriceComponentText,
				Condition:   "prompt_tokens_lte=32000;completion_tokens_gt=200",
				InputPrice:  0.0008,
				OutputPrice: 0.008,
				PriceUnit:   adminmodel.ProviderPriceUnitPer1KTokens,
				Currency:    "CNY",
				Source:      "migration",
			},
		},
	}
	preConsumedPricing := adminmodel.ResolveTextUsagePricing(basePricing, "/v1/chat/completions", 2048, maxOutputTokens)
	snapshot, err := billing.ComputeTextPreConsumedBillingSnapshot(2048, maxOutputTokens, preConsumedPricing, 0)
	if err != nil {
		t.Fatalf("ComputeTextPreConsumedBillingSnapshot() error = %v", err)
	}
	if preConsumedPricing.OutputPrice != 0.002 {
		t.Fatalf("pre-consume output price=%v, want 0.002", preConsumedPricing.OutputPrice)
	}
	wantOutputAmount := float64(config.PreConsumedQuota+int64(maxOutputTokens)) * 0.002 / 1000
	if math.Abs(snapshot.OutputAmount-wantOutputAmount) > 1e-12 {
		t.Fatalf("OutputAmount = %v, want %v", snapshot.OutputAmount, wantOutputAmount)
	}
}
