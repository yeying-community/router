package controller

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestGetImageRequestAppliesDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"prompt":"draw a city skyline"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	req, err := getImageRequest(ctx, 0)
	if err != nil {
		t.Fatalf("getImageRequest() error = %v", err)
	}
	if req.Prompt != "draw a city skyline" {
		t.Fatalf("Prompt = %q, want %q", req.Prompt, "draw a city skyline")
	}
	if req.N != 1 {
		t.Fatalf("N = %d, want %d", req.N, 1)
	}
	if req.Size != "1024x1024" {
		t.Fatalf("Size = %q, want %q", req.Size, "1024x1024")
	}
	if req.Model != "dall-e-2" {
		t.Fatalf("Model = %q, want %q", req.Model, "dall-e-2")
	}
}

func TestGetImageEditRequestParsesMultipartForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("model", "gpt-image-2"); err != nil {
		t.Fatalf("WriteField(model) error = %v", err)
	}
	if err := writer.WriteField("prompt", "edit this image"); err != nil {
		t.Fatalf("WriteField(prompt) error = %v", err)
	}
	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile(image) error = %v", err)
	}
	if _, err := part.Write([]byte("png-bytes")); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/images/edits", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	req, form, err := getImageEditRequest(ctx)
	if err != nil {
		t.Fatalf("getImageEditRequest() error = %v", err)
	}
	if req.Model != "gpt-image-2" {
		t.Fatalf("Model = %q, want %q", req.Model, "gpt-image-2")
	}
	if req.Prompt != "edit this image" {
		t.Fatalf("Prompt = %q, want %q", req.Prompt, "edit this image")
	}
	if req.N != 1 {
		t.Fatalf("N = %d, want %d", req.N, 1)
	}
	if form == nil || len(form.File["image"]) != 1 {
		t.Fatalf("image file count = %d, want 1", len(form.File["image"]))
	}
}

func TestBuildMultipartImageEditBodyRewritesModelField(t *testing.T) {
	form := &multipart.Form{
		Value: map[string][]string{
			"model":  {"old-model"},
			"prompt": {"old prompt"},
			"user":   {"user-1"},
		},
		File: map[string][]*multipart.FileHeader{},
	}
	req := &relaymodel.ImageRequest{
		Model:  "gpt-image-2",
		Prompt: "new prompt",
		N:      1,
		User:   "user-1",
	}

	body, contentType, err := buildMultipartImageEditBody(form, req)
	if err != nil {
		t.Fatalf("buildMultipartImageEditBody() error = %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("contentType = %q, want multipart/form-data boundary", contentType)
	}
	payload := body.String()
	if !strings.Contains(payload, "gpt-image-2") {
		t.Fatalf("payload missing rewritten model: %q", payload)
	}
	if strings.Contains(payload, "old-model") {
		t.Fatalf("payload still contains old model: %q", payload)
	}
	if !strings.Contains(payload, "new prompt") {
		t.Fatalf("payload missing rewritten prompt: %q", payload)
	}
}

func TestValidateImageRequest(t *testing.T) {
	tests := []struct {
		name       string
		request    *relaymodel.ImageRequest
		wantOK     bool
		wantErrMsg string
	}{
		{
			name: "valid request",
			request: &relaymodel.ImageRequest{
				Model:  "dall-e-3",
				Prompt: "draw a city skyline",
				Size:   "1024x1024",
				N:      1,
			},
			wantOK: true,
		},
		{
			name: "missing prompt",
			request: &relaymodel.ImageRequest{
				Model: "dall-e-3",
				Size:  "1024x1024",
				N:     1,
			},
			wantErrMsg: "prompt is required",
		},
		{
			name: "unsupported size",
			request: &relaymodel.ImageRequest{
				Model:  "dall-e-3",
				Prompt: "draw a city skyline",
				Size:   "512x512",
				N:      1,
			},
			wantErrMsg: "size not supported for this image model",
		},
		{
			name: "prompt too long",
			request: &relaymodel.ImageRequest{
				Model:  "dall-e-2",
				Prompt: strings.Repeat("a", 1001),
				Size:   "1024x1024",
				N:      1,
			},
			wantErrMsg: "prompt is too long",
		},
		{
			name: "invalid n",
			request: &relaymodel.ImageRequest{
				Model:  "dall-e-3",
				Prompt: "draw a city skyline",
				Size:   "1024x1024",
				N:      2,
			},
			wantErrMsg: "invalid value of n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageRequest(tt.request, nil)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("validateImageRequest() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateImageRequest() error = nil, want %q", tt.wantErrMsg)
			}
			if err.Error.Message != tt.wantErrMsg {
				t.Fatalf("validateImageRequest() message = %q, want %q", err.Error.Message, tt.wantErrMsg)
			}
		})
	}
}

func TestGetImageCostRatio(t *testing.T) {
	tests := []struct {
		name      string
		request   *relaymodel.ImageRequest
		wantRatio float64
		wantErr   bool
	}{
		{
			name: "dall-e-3 standard",
			request: &relaymodel.ImageRequest{
				Model: "dall-e-3",
				Size:  "1024x1792",
			},
			wantRatio: 2,
		},
		{
			name: "dall-e-3 hd square",
			request: &relaymodel.ImageRequest{
				Model:   "dall-e-3",
				Size:    "1024x1024",
				Quality: "hd",
			},
			wantRatio: 2,
		},
		{
			name: "dall-e-3 hd portrait",
			request: &relaymodel.ImageRequest{
				Model:   "dall-e-3",
				Size:    "1024x1792",
				Quality: "hd",
			},
			wantRatio: 3,
		},
		{
			name:    "nil request",
			request: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getImageCostRatio(tt.request)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("getImageCostRatio() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("getImageCostRatio() error = %v", err)
			}
			if got != tt.wantRatio {
				t.Fatalf("getImageCostRatio() = %v, want %v", got, tt.wantRatio)
			}
		})
	}
}

func TestValidateImageBillingPricing(t *testing.T) {
	tests := []struct {
		name    string
		pricing adminmodel.ResolvedModelPricing
		wantErr bool
	}{
		{
			name: "per image allowed",
			pricing: adminmodel.ResolvedModelPricing{
				Model:     "dall-e-3",
				Type:      adminmodel.ProviderModelTypeImage,
				PriceUnit: adminmodel.ProviderPriceUnitPerImage,
			},
		},
		{
			name: "per request allowed",
			pricing: adminmodel.ResolvedModelPricing{
				Model:     "foo-image",
				Type:      adminmodel.ProviderModelTypeImage,
				PriceUnit: adminmodel.ProviderPriceUnitPerRequest,
			},
		},
		{
			name: "token based image endpoint blocked",
			pricing: adminmodel.ResolvedModelPricing{
				Model:     "gpt-image-2",
				Type:      adminmodel.ProviderModelTypeImage,
				PriceUnit: adminmodel.ProviderPriceUnitPer1KTokens,
			},
			wantErr: true,
		},
		{
			name: "char based image endpoint blocked",
			pricing: adminmodel.ResolvedModelPricing{
				Model:     "weird-image",
				Type:      adminmodel.ProviderModelTypeImage,
				PriceUnit: adminmodel.ProviderPriceUnitPer1KChars,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageBillingPricing(tt.pricing)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateImageBillingPricing() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("validateImageBillingPricing() error = %v", err)
			}
		})
	}
}
