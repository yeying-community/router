package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdimage "image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	adminmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay"
	aliadaptor "github.com/yeying-community/router/internal/relay/adaptor/ali"
	"github.com/yeying-community/router/internal/relay/adaptor/openai"
	"github.com/yeying-community/router/internal/relay/billing"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/imagerule"
	"github.com/yeying-community/router/internal/relay/meta"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/relaymode"
	_ "golang.org/x/image/webp"
)

type traditionalImageTokenEstimate struct {
	PromptTokens      int
	ImageOutputTokens int
}

type gptImage2Estimate struct {
	PromptTokens      int
	ImageInputTokens  int
	OutputAmount      float64
	OutputQuantity    float64
	NormalizedSize    string
	NormalizedQuality string
}

const (
	imageUsageSourceLocalEstimate             = "local_estimate"
	imageEstimateSourceTraditionalImageTokens = "traditional_image_tokens_local"
	imageEstimateSourceGPTImage2Examples      = "gpt_image_2_examples_local"
	imageEstimateSourceGPTImage2Edits         = "gpt_image_2_edits_local"
	imageEstimateSourceImageCountRatio        = "image_count_ratio"
	imageUsageSourceProviderResponse          = "provider_response"
	imageEstimateSourceQwenImageOutputCount   = "qwen_image_output_count"
	imageSettlementModeEstimateOnly           = "estimate_only"
	imageSettlementModeLocalEstimateFinal     = "local_estimate_final"
	imageSettlementModeProviderUsageFinal     = "provider_usage_final"
)

func validateImageBillingPricing(pricing adminmodel.ResolvedModelPricing) error {
	switch billing.ResolveImageBillingMode(pricing) {
	case billing.ImageBillingModePerImage, billing.ImageBillingModePerCall:
		return nil
	case billing.ImageBillingModeTokenBased:
		if !supportsTraditionalImageTokenBilling(pricing) {
			return fmt.Errorf("image billing strategy is not supported for model %s with price_unit %s on traditional image endpoints", strings.TrimSpace(pricing.Model), strings.TrimSpace(pricing.PriceUnit))
		}
		if _, err := resolveTraditionalImagePromptInputPrice(pricing); err != nil {
			return err
		}
		if pricing.OutputPrice <= 0 {
			return fmt.Errorf("traditional image token billing output price is not configured for model %s", strings.TrimSpace(pricing.Model))
		}
		return nil
	default:
		return fmt.Errorf("image billing strategy is not supported for model %s with price_unit %s on traditional image endpoints", strings.TrimSpace(pricing.Model), strings.TrimSpace(pricing.PriceUnit))
	}
}

func supportsTraditionalImageTokenBilling(pricing adminmodel.ResolvedModelPricing) bool {
	if billing.ResolveImageBillingMode(pricing) != billing.ImageBillingModeTokenBased {
		return false
	}
	if isGPTImage2Model(pricing.Model) {
		return true
	}
	return supportsLegacyImageTokenTable(pricing.Model)
}

func supportsLegacyImageTokenTable(modelName string) bool {
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	return strings.HasPrefix(modelName, "gpt-image-") && modelName != "gpt-image-2"
}

func isGPTImage2Model(modelName string) bool {
	return strings.EqualFold(strings.TrimSpace(modelName), "gpt-image-2")
}

func normalizeTraditionalImageBillingSize(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "auto":
		return "1024x1024"
	case "1024x1024", "1024x1536", "1536x1024":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return ""
	}
}

func normalizeTraditionalImageBillingQuality(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "auto":
		return "high"
	case "low", "medium", "high":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return ""
	}
}

func resolveTraditionalImagePromptInputPrice(pricing adminmodel.ResolvedModelPricing) (float64, error) {
	if pricing.HasChannelInputPriceOverride && pricing.InputPrice > 0 {
		return pricing.InputPrice, nil
	}
	for _, component := range pricing.PriceComponents {
		if strings.TrimSpace(strings.ToLower(component.Component)) != adminmodel.ProviderModelPriceComponentText {
			continue
		}
		if component.InputPrice > 0 {
			return component.InputPrice, nil
		}
	}
	return 0, fmt.Errorf("traditional image token billing text input price is not configured for model %s", strings.TrimSpace(pricing.Model))
}

func resolveTraditionalImageImageInputPrice(pricing adminmodel.ResolvedModelPricing) (float64, error) {
	if pricing.HasChannelInputPriceOverride && pricing.InputPrice > 0 {
		return pricing.InputPrice, nil
	}
	for _, component := range pricing.PriceComponents {
		if strings.TrimSpace(strings.ToLower(component.Component)) != adminmodel.ProviderModelPriceComponentImageGeneration {
			continue
		}
		if component.InputPrice > 0 {
			return component.InputPrice, nil
		}
	}
	if pricing.InputPrice > 0 {
		return pricing.InputPrice, nil
	}
	return 0, fmt.Errorf("traditional image token billing image input price is not configured for model %s", strings.TrimSpace(pricing.Model))
}

func estimateTraditionalImageOutputTokens(modelName string, size string, quality string, imageCount int) (int, error) {
	if imageCount <= 0 {
		return 0, nil
	}
	if !supportsLegacyImageTokenTable(modelName) {
		return 0, fmt.Errorf("traditional image token estimate is not supported for model %s", strings.TrimSpace(modelName))
	}
	normalizedSize := normalizeTraditionalImageBillingSize(size)
	if normalizedSize == "" {
		return 0, fmt.Errorf("unsupported image size %q for traditional token billing", strings.TrimSpace(size))
	}
	normalizedQuality := normalizeTraditionalImageBillingQuality(quality)
	if normalizedQuality == "" {
		return 0, fmt.Errorf("unsupported image quality %q for traditional token billing", strings.TrimSpace(quality))
	}
	tokenTable := map[string]map[string]int{
		"low": {
			"1024x1024": 272,
			"1024x1536": 408,
			"1536x1024": 400,
		},
		"medium": {
			"1024x1024": 1056,
			"1024x1536": 1584,
			"1536x1024": 1568,
		},
		"high": {
			"1024x1024": 4160,
			"1024x1536": 6240,
			"1536x1024": 6208,
		},
	}
	perImageTokens := tokenTable[normalizedQuality][normalizedSize]
	if perImageTokens <= 0 {
		return 0, fmt.Errorf("traditional image token estimate is not configured for size=%s quality=%s", normalizedSize, normalizedQuality)
	}
	return perImageTokens * imageCount, nil
}

func estimateGPTImage2OutputAmount(size string, quality string, imageCount int) (float64, string, string, error) {
	if imageCount <= 0 {
		return 0, "", "", nil
	}
	normalizedSize := normalizeTraditionalImageBillingSize(size)
	if normalizedSize == "" {
		return 0, "", "", fmt.Errorf("unsupported image size %q for gpt-image-2 local estimate", strings.TrimSpace(size))
	}
	normalizedQuality := normalizeTraditionalImageBillingQuality(quality)
	if normalizedQuality == "" {
		return 0, "", "", fmt.Errorf("unsupported image quality %q for gpt-image-2 local estimate", strings.TrimSpace(quality))
	}
	amountTable := map[string]map[string]float64{
		"low": {
			"1024x1024": 0.006,
			"1024x1536": 0.005,
			"1536x1024": 0.005,
		},
		"medium": {
			"1024x1024": 0.053,
			"1024x1536": 0.041,
			"1536x1024": 0.041,
		},
		"high": {
			"1024x1024": 0.211,
			"1024x1536": 0.165,
			"1536x1024": 0.165,
		},
	}
	perImageAmount := amountTable[normalizedQuality][normalizedSize]
	if perImageAmount <= 0 {
		return 0, "", "", fmt.Errorf("gpt-image-2 local estimate is not configured for size=%s quality=%s", normalizedSize, normalizedQuality)
	}
	return perImageAmount * float64(imageCount), normalizedSize, normalizedQuality, nil
}

func estimateGPTImage2Usage(imageRequest *relaymodel.ImageRequest, pricing adminmodel.ResolvedModelPricing, imageCount int) (gptImage2Estimate, error) {
	if imageRequest == nil {
		return gptImage2Estimate{}, errors.New("image request is nil")
	}
	if !isGPTImage2Model(pricing.Model) {
		return gptImage2Estimate{}, fmt.Errorf("gpt-image-2 local estimate is not supported for model %s", strings.TrimSpace(pricing.Model))
	}
	if pricing.OutputPrice <= 0 {
		return gptImage2Estimate{}, fmt.Errorf("gpt-image-2 output price is not configured for model %s", strings.TrimSpace(pricing.Model))
	}
	outputAmount, normalizedSize, normalizedQuality, err := estimateGPTImage2OutputAmount(imageRequest.Size, imageRequest.Quality, imageCount)
	if err != nil {
		return gptImage2Estimate{}, err
	}
	outputQuantity := outputAmount * 1000 / pricing.OutputPrice
	return gptImage2Estimate{
		PromptTokens:      openai.CountTokenText(strings.TrimSpace(imageRequest.Prompt), strings.TrimSpace(pricing.Model)),
		OutputAmount:      outputAmount,
		OutputQuantity:    outputQuantity,
		NormalizedSize:    normalizedSize,
		NormalizedQuality: normalizedQuality,
	}, nil
}

func readMultipartImageSize(fileHeader *multipart.FileHeader) (int, int, error) {
	if fileHeader == nil {
		return 0, 0, errors.New("image file header is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	cfg, _, err := stdimage.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func estimateGPTImage2EditImageInputTokens(form *multipart.Form) (int, error) {
	if form == nil {
		return 0, errors.New("multipart form is required")
	}
	fileHeaders := form.File["image"]
	if len(fileHeaders) == 0 {
		return 0, errors.New("image file is required")
	}
	total := 0
	for _, fileHeader := range fileHeaders {
		width, height, err := readMultipartImageSize(fileHeader)
		if err != nil {
			return 0, err
		}
		tokens, err := estimateGPTImage2InputImageTokens(width, height)
		if err != nil {
			return 0, err
		}
		total += tokens
	}
	return total, nil
}

func estimateGPTImage2InputImageTokens(width int, height int) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, fmt.Errorf("invalid image size %dx%d", width, height)
	}
	scaledWidth := width
	scaledHeight := height
	if scaledWidth > 2048 || scaledHeight > 2048 {
		ratio := float64(2048) / maxFloat(float64(scaledWidth), float64(scaledHeight))
		scaledWidth = int(float64(scaledWidth) * ratio)
		scaledHeight = int(float64(scaledHeight) * ratio)
	}
	if scaledWidth > 512 && scaledHeight > 512 {
		ratio := float64(512) / minFloat(float64(scaledWidth), float64(scaledHeight))
		scaledWidth = int(float64(scaledWidth) * ratio)
		scaledHeight = int(float64(scaledHeight) * ratio)
	}
	tiles := int(math.Ceil(float64(scaledWidth)/512.0) * math.Ceil(float64(scaledHeight)/512.0))
	if tiles <= 0 {
		return 0, fmt.Errorf("invalid tile count for image size %dx%d", width, height)
	}
	baseTokens := 65
	perTileTokens := 129
	aspectRatioTokens := 4160
	if scaledWidth != scaledHeight {
		aspectRatioTokens = 6240
	}
	return baseTokens + tiles*perTileTokens + aspectRatioTokens, nil
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a float64, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func estimateGPTImage2EditUsage(imageRequest *relaymodel.ImageRequest, form *multipart.Form, pricing adminmodel.ResolvedModelPricing, imageCount int) (gptImage2Estimate, error) {
	estimate, err := estimateGPTImage2Usage(imageRequest, pricing, imageCount)
	if err != nil {
		return gptImage2Estimate{}, err
	}
	imageInputTokens, err := estimateGPTImage2EditImageInputTokens(form)
	if err != nil {
		return gptImage2Estimate{}, err
	}
	estimate.ImageInputTokens = imageInputTokens
	return estimate, nil
}

func estimateTraditionalImageTokenUsage(imageRequest *relaymodel.ImageRequest, pricing adminmodel.ResolvedModelPricing, imageCount int) (traditionalImageTokenEstimate, error) {
	if imageRequest == nil {
		return traditionalImageTokenEstimate{}, errors.New("image request is nil")
	}
	if !supportsTraditionalImageTokenBilling(pricing) {
		return traditionalImageTokenEstimate{}, fmt.Errorf("traditional image token billing is not supported for model %s", strings.TrimSpace(pricing.Model))
	}
	outputTokens, err := estimateTraditionalImageOutputTokens(pricing.Model, imageRequest.Size, imageRequest.Quality, imageCount)
	if err != nil {
		return traditionalImageTokenEstimate{}, err
	}
	return traditionalImageTokenEstimate{
		PromptTokens:      openai.CountTokenText(strings.TrimSpace(imageRequest.Prompt), strings.TrimSpace(pricing.Model)),
		ImageOutputTokens: outputTokens,
	}, nil
}

func getImageRequest(c *gin.Context, _ int) (*relaymodel.ImageRequest, error) {
	imageRequest := &relaymodel.ImageRequest{}
	err := common.UnmarshalBodyReusable(c, imageRequest)
	if err != nil {
		return nil, err
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	if imageRequest.Size == "" {
		imageRequest.Size = "1024x1024"
	}
	if imageRequest.Model == "" {
		imageRequest.Model = "dall-e-2"
	}
	return imageRequest, nil
}

func getImageEditRequest(c *gin.Context) (*relaymodel.ImageRequest, *multipart.Form, error) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, nil, err
	}
	form := c.Request.MultipartForm
	if form == nil {
		return nil, nil, errors.New("multipart form is required")
	}
	imageRequest := &relaymodel.ImageRequest{
		Model:          strings.TrimSpace(c.Request.FormValue("model")),
		Prompt:         strings.TrimSpace(c.Request.FormValue("prompt")),
		Size:           strings.TrimSpace(c.Request.FormValue("size")),
		Quality:        strings.TrimSpace(c.Request.FormValue("quality")),
		ResponseFormat: strings.TrimSpace(c.Request.FormValue("response_format")),
		Style:          strings.TrimSpace(c.Request.FormValue("style")),
		User:           strings.TrimSpace(c.Request.FormValue("user")),
	}
	if rawN := strings.TrimSpace(c.Request.FormValue("n")); rawN != "" {
		n, err := strconv.Atoi(rawN)
		if err != nil {
			return nil, nil, err
		}
		imageRequest.N = n
	}
	if imageRequest.N == 0 {
		imageRequest.N = 1
	}
	if len(form.File["image"]) == 0 {
		return nil, nil, errors.New("image file is required")
	}
	return imageRequest, form, nil
}

func buildMultipartImageEditBody(form *multipart.Form, imageRequest *relaymodel.ImageRequest) (*bytes.Buffer, string, error) {
	if form == nil {
		return nil, "", errors.New("multipart form is required")
	}
	if imageRequest == nil {
		return nil, "", errors.New("image request is nil")
	}
	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)
	knownFields := map[string]struct{}{
		"model":           {},
		"prompt":          {},
		"n":               {},
		"size":            {},
		"quality":         {},
		"response_format": {},
		"style":           {},
		"user":            {},
	}
	writeField := func(key string, value string) error {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return writer.WriteField(key, value)
	}
	if err := writeField("model", imageRequest.Model); err != nil {
		return nil, "", err
	}
	if err := writeField("prompt", imageRequest.Prompt); err != nil {
		return nil, "", err
	}
	if imageRequest.N > 0 {
		if err := writer.WriteField("n", strconv.Itoa(imageRequest.N)); err != nil {
			return nil, "", err
		}
	}
	if err := writeField("size", imageRequest.Size); err != nil {
		return nil, "", err
	}
	if err := writeField("quality", imageRequest.Quality); err != nil {
		return nil, "", err
	}
	if err := writeField("response_format", imageRequest.ResponseFormat); err != nil {
		return nil, "", err
	}
	if err := writeField("style", imageRequest.Style); err != nil {
		return nil, "", err
	}
	if err := writeField("user", imageRequest.User); err != nil {
		return nil, "", err
	}
	for key, values := range form.Value {
		if _, known := knownFields[key]; known {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", err
			}
		}
	}
	for fieldName, files := range form.File {
		for _, header := range files {
			src, err := header.Open()
			if err != nil {
				return nil, "", err
			}
			part, err := writer.CreateFormFile(fieldName, header.Filename)
			if err != nil {
				_ = src.Close()
				return nil, "", err
			}
			if _, err := io.Copy(part, src); err != nil {
				_ = src.Close()
				return nil, "", err
			}
			if err := src.Close(); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return bodyBuffer, writer.FormDataContentType(), nil
}

func isValidImageSize(model string, size string) bool {
	if model == "cogview-3" || imagerule.ImageSizeRatios[model] == nil {
		return true
	}
	_, ok := imagerule.ImageSizeRatios[model][size]
	return ok
}

func isValidImagePromptLength(model string, promptLength int) bool {
	maxPromptLength, ok := imagerule.ImagePromptLengthLimitations[model]
	return !ok || promptLength <= maxPromptLength
}

func isWithinRange(element string, value int) bool {
	amounts, ok := imagerule.ImageGenerationAmounts[element]
	return !ok || (value >= amounts[0] && value <= amounts[1])
}

func getImageSizeRatio(model string, size string) float64 {
	if ratio, ok := imagerule.ImageSizeRatios[model][size]; ok {
		return ratio
	}
	return 1
}

func validateImageRequest(imageRequest *relaymodel.ImageRequest, _ *meta.Meta) *relaymodel.ErrorWithStatusCode {
	// check prompt length
	if imageRequest.Prompt == "" {
		return openai.ErrorWrapper(errors.New("prompt is required"), "prompt_missing", http.StatusBadRequest)
	}

	// model validation
	if !isValidImageSize(imageRequest.Model, imageRequest.Size) {
		return openai.ErrorWrapper(errors.New("size not supported for this image model"), "size_not_supported", http.StatusBadRequest)
	}

	if !isValidImagePromptLength(imageRequest.Model, len(imageRequest.Prompt)) {
		return openai.ErrorWrapper(errors.New("prompt is too long"), "prompt_too_long", http.StatusBadRequest)
	}

	// Number of generated images validation
	if !isWithinRange(imageRequest.Model, imageRequest.N) {
		return openai.ErrorWrapper(errors.New("invalid value of n"), "n_not_within_range", http.StatusBadRequest)
	}
	return nil
}

func getImageCostRatio(imageRequest *relaymodel.ImageRequest) (float64, error) {
	if imageRequest == nil {
		return 0, errors.New("imageRequest is nil")
	}
	imageCostRatio := getImageSizeRatio(imageRequest.Model, imageRequest.Size)
	if imageRequest.Quality == "hd" && imageRequest.Model == "dall-e-3" {
		if imageRequest.Size == "1024x1024" {
			imageCostRatio *= 2
		} else {
			imageCostRatio *= 1.5
		}
	}
	return imageCostRatio, nil
}

func resolveImageRequestPackageAmount(imageRequest *relaymodel.ImageRequest) int64 {
	if imageRequest == nil {
		return 1
	}
	count := int64(imageRequest.N)
	if count <= 0 {
		return 1
	}
	return count
}

func RelayImageHelper(c *gin.Context, relayMode int) *relaymodel.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)
	var (
		imageRequest *relaymodel.ImageRequest
		form         *multipart.Form
		err          error
	)
	if relayMode == relaymode.ImagesEdits {
		imageRequest, form, err = getImageEditRequest(c)
	} else {
		imageRequest, err = getImageRequest(c, meta.Mode)
	}
	if err != nil {
		logger.Errorf(ctx, "image relay get request failed user_id=%s group=%s channel_id=%s endpoint=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), c.Request.URL.Path, err.Error())
		return openai.ErrorWrapper(err, "invalid_image_request", http.StatusBadRequest)
	}

	// map model name
	var isModelMapped bool
	meta.OriginModelName = imageRequest.Model
	imageRequest.Model, isModelMapped = getMappedModelName(imageRequest.Model, meta.ModelMapping)
	meta.ActualModelName = imageRequest.Model

	// model validation
	bizErr := validateImageRequest(imageRequest, meta)
	if bizErr != nil {
		return bizErr
	}

	imageCostRatio, err := getImageCostRatio(imageRequest)
	if err != nil {
		return openai.ErrorWrapper(err, "get_image_cost_ratio_failed", http.StatusInternalServerError)
	}

	imageModel := imageRequest.Model
	// Convert the original image model
	imageRequest.Model, _ = getMappedModelName(imageRequest.Model, imagerule.ImageOriginModelName)
	c.Set("response_format", imageRequest.ResponseFormat)
	groupRatio := adminmodel.GetGroupChannelBillingRatio(meta.Group, meta.ChannelId)
	pricing, pricingErr := adminmodel.ResolveChannelModelPricing(meta.ChannelProtocol, meta.ChannelModelConfigs, imageModel)
	if pricingErr != nil {
		if groupRatio == 0 {
			pricing = adminmodel.ResolvedModelPricing{
				Model:     imageModel,
				Type:      adminmodel.InferModelType(imageModel),
				PriceUnit: adminmodel.ProviderPriceUnitPerImage,
				Currency:  adminmodel.ProviderPriceCurrencyUSD,
				Source:    "group_free",
			}
		} else {
			return openai.ErrorWrapper(pricingErr, "model_pricing_not_configured", http.StatusServiceUnavailable)
		}
	}
	pricing = adminmodel.ResolveImageRequestPricing(pricing, imageRequest.Size, imageRequest.Quality)
	if pricing.MatchedComponent != "" {
		imageCostRatio = 1
	}
	if billingErr := validateImageBillingPricing(pricing); billingErr != nil {
		return openai.ErrorWrapper(billingErr, "unsupported_image_billing", http.StatusServiceUnavailable)
	}

	var requestBody io.Reader
	if relayMode == relaymode.ImagesEdits {
		if meta.ChannelProtocol == relaychannel.Ali && aliadaptor.IsQwenImageModel(imageRequest.Model) {
			finalRequest, buildErr := aliadaptor.ConvertQwenImageEditRequest(*imageRequest, form)
			if buildErr != nil {
				return openai.ErrorWrapper(buildErr, "convert_image_request_failed", http.StatusInternalServerError)
			}
			jsonStr, buildErr := json.Marshal(finalRequest)
			if buildErr != nil {
				return openai.ErrorWrapper(buildErr, "marshal_image_request_failed", http.StatusInternalServerError)
			}
			c.Request.Header.Set("Content-Type", "application/json")
			requestBody = bytes.NewBuffer(jsonStr)
		} else {
			requestBodyBuffer, contentType, buildErr := buildMultipartImageEditBody(form, imageRequest)
			if buildErr != nil {
				return openai.ErrorWrapper(buildErr, "marshal_image_request_failed", http.StatusInternalServerError)
			}
			c.Request.Header.Set("Content-Type", contentType)
			requestBody = bytes.NewBuffer(requestBodyBuffer.Bytes())
		}
	} else if isModelMapped || meta.ChannelProtocol == relaychannel.Azure { // make Azure channel request body
		jsonStr, err := json.Marshal(imageRequest)
		if err != nil {
			return openai.ErrorWrapper(err, "marshal_image_request_failed", http.StatusInternalServerError)
		}
		requestBody = bytes.NewBuffer(jsonStr)
	} else {
		requestBody = c.Request.Body
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	// these adaptors need to convert the request
	if relayMode != relaymode.ImagesEdits {
		switch meta.ChannelProtocol {
		case relaychannel.Zhipu,
			relaychannel.Ali,
			relaychannel.Replicate,
			relaychannel.Baidu:
			finalRequest, err := adaptor.ConvertImageRequest(imageRequest)
			if err != nil {
				return openai.ErrorWrapper(err, "convert_image_request_failed", http.StatusInternalServerError)
			}
			jsonStr, err := json.Marshal(finalRequest)
			if err != nil {
				return openai.ErrorWrapper(err, "marshal_image_request_failed", http.StatusInternalServerError)
			}
			requestBody = bytes.NewBuffer(jsonStr)
		}
	}

	requestPackageAmount := resolveImageRequestPackageAmount(imageRequest)
	if requestPackagePlan, matched, quotaErr := tryBuildRequestPackageBillingPlanWithAmount(ctx, meta, requestPackageAmount); quotaErr != nil || (matched && requestPackagePlan.UsesRequestPackage()) {
		if quotaErr != nil {
			return quotaErr
		}
		groupQuotaSettled := false
		defer func() {
			if !groupQuotaSettled {
				releaseRelayBillingPlan(ctx, requestPackagePlan)
			}
		}()
		resp, err := adaptor.DoRequest(c, meta, requestBody)
		if err != nil {
			return openai.ErrorWrapper(err, classifyImageRequestErrorCode(err), http.StatusInternalServerError)
		}
		if resp != nil &&
			resp.StatusCode != http.StatusCreated &&
			resp.StatusCode != http.StatusOK {
			return RelayErrorHandler(meta, resp)
		}
		if _, respErr := adaptor.DoResponse(c, resp, meta); respErr != nil {
			logger.Errorf(ctx, "image request package response failed user_id=%s group=%s channel_id=%s model=%s endpoint=%s err=%+v", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), c.Request.URL.Path, respErr)
			return respErr
		}
		go settleRequestPackageOnlyConsumption(ctx, meta, imageRequest.Model, c.GetString(ctxkey.TokenName), 0, 0, helper.CalcElapsedTime(meta.StartTime), false, requestPackagePlan)
		groupQuotaSettled = true
		return nil
	}

	imageCount := imageRequest.N
	if meta.ChannelProtocol == relaychannel.Replicate {
		imageCount = 1
	}
	billingSnapshot := billing.BillingSnapshot{}
	switch billing.ResolveImageBillingMode(pricing) {
	case billing.ImageBillingModeTokenBased:
		promptInputPrice, promptPriceErr := resolveTraditionalImagePromptInputPrice(pricing)
		if promptPriceErr != nil {
			return openai.ErrorWrapper(promptPriceErr, "calculate_image_quota_failed", http.StatusInternalServerError)
		}
		pricingForBilling := pricing
		pricingForBilling.InputPrice = promptInputPrice
		if isGPTImage2Model(pricing.Model) {
			var (
				estimate    gptImage2Estimate
				estimateErr error
			)
			if relayMode == relaymode.ImagesEdits {
				estimate, estimateErr = estimateGPTImage2EditUsage(imageRequest, form, pricingForBilling, imageCount)
			} else {
				estimate, estimateErr = estimateGPTImage2Usage(imageRequest, pricingForBilling, imageCount)
			}
			if estimateErr != nil {
				return openai.ErrorWrapper(estimateErr, "calculate_image_quota_failed", http.StatusInternalServerError)
			}
			promptInputAmount := float64(estimate.PromptTokens) * promptInputPrice / 1000
			outputAmount := estimate.OutputAmount
			var (
				inputQuantity float64
				inputAmount   float64
				snapshotErr   error
			)
			inputQuantity = float64(estimate.PromptTokens)
			inputAmount = promptInputAmount
			if relayMode == relaymode.ImagesEdits {
				imageInputPrice, imageInputPriceErr := resolveTraditionalImageImageInputPrice(pricing)
				if imageInputPriceErr != nil {
					return openai.ErrorWrapper(imageInputPriceErr, "calculate_image_quota_failed", http.StatusInternalServerError)
				}
				inputQuantity += float64(estimate.ImageInputTokens)
				inputAmount += float64(estimate.ImageInputTokens) * imageInputPrice / 1000
				billingSnapshot, snapshotErr = billing.ComputeExplicitAmountBillingSnapshot(
					inputQuantity,
					estimate.OutputQuantity,
					inputAmount,
					outputAmount,
					pricingForBilling,
					groupRatio,
					inputQuantity > 0 || estimate.OutputQuantity > 0,
				)
			} else {
				billingSnapshot, snapshotErr = billing.ComputeExplicitAmountBillingSnapshot(
					inputQuantity,
					estimate.OutputQuantity,
					inputAmount,
					outputAmount,
					pricingForBilling,
					groupRatio,
					inputQuantity > 0 || estimate.OutputQuantity > 0,
				)
			}
			if snapshotErr != nil {
				return openai.ErrorWrapper(snapshotErr, "calculate_image_quota_failed", http.StatusInternalServerError)
			}
			billingSnapshot.PricingSource = strings.TrimSpace(pricing.Source)
			billingSnapshot.UsageSource = imageUsageSourceLocalEstimate
			if relayMode == relaymode.ImagesEdits {
				billingSnapshot.EstimateSource = imageEstimateSourceGPTImage2Edits
			} else {
				billingSnapshot.EstimateSource = imageEstimateSourceGPTImage2Examples
			}
			billingSnapshot.SettlementMode = imageSettlementModeLocalEstimateFinal
			logger.Debugf(
				ctx,
				"[image_gpt_image_2_estimate] model=%s prompt_tokens=%d image_input_tokens=%d output_quantity=%.3f output_amount=%.6f size=%s quality=%s count=%d endpoint=%s",
				strings.TrimSpace(pricingForBilling.Model),
				estimate.PromptTokens,
				estimate.ImageInputTokens,
				estimate.OutputQuantity,
				estimate.OutputAmount,
				estimate.NormalizedSize,
				estimate.NormalizedQuality,
				imageCount,
				c.Request.URL.Path,
			)
			break
		}
		tokenEstimate, estimateErr := estimateTraditionalImageTokenUsage(imageRequest, pricing, imageCount)
		if estimateErr != nil {
			return openai.ErrorWrapper(estimateErr, "calculate_image_quota_failed", http.StatusInternalServerError)
		}
		var snapshotErr error
		billingSnapshot, snapshotErr = billing.ComputeTraditionalImageTokenBasedBillingSnapshot(
			tokenEstimate.PromptTokens,
			tokenEstimate.ImageOutputTokens,
			pricingForBilling,
			groupRatio,
		)
		if snapshotErr != nil {
			return openai.ErrorWrapper(snapshotErr, "calculate_image_quota_failed", http.StatusInternalServerError)
		}
		billingSnapshot.PricingSource = strings.TrimSpace(pricing.Source)
		billingSnapshot.UsageSource = imageUsageSourceLocalEstimate
		billingSnapshot.EstimateSource = imageEstimateSourceTraditionalImageTokens
		billingSnapshot.SettlementMode = imageSettlementModeLocalEstimateFinal
		logger.Debugf(
			ctx,
			"[image_token_estimate] model=%s prompt_tokens=%d image_output_tokens=%d size=%s quality=%s count=%d",
			strings.TrimSpace(pricingForBilling.Model),
			tokenEstimate.PromptTokens,
			tokenEstimate.ImageOutputTokens,
			strings.TrimSpace(imageRequest.Size),
			strings.TrimSpace(imageRequest.Quality),
			imageCount,
		)
	default:
		var snapshotErr error
		billingSnapshot, snapshotErr = billing.ComputeImageBillingSnapshot(imageCount, imageCostRatio, pricing, groupRatio)
		if snapshotErr != nil {
			logger.Errorf(ctx, "image billing snapshot failed user_id=%s group=%s channel_id=%s model=%s image_count=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), imageCount, snapshotErr.Error())
			return openai.ErrorWrapper(snapshotErr, "calculate_image_quota_failed", http.StatusInternalServerError)
		}
		billingSnapshot.PricingSource = strings.TrimSpace(pricing.Source)
		billingSnapshot.UsageSource = ""
		billingSnapshot.EstimateSource = imageEstimateSourceImageCountRatio
		billingSnapshot.SettlementMode = imageSettlementModeEstimateOnly
	}
	if err := billing.ApplyEstimatedProcurementCostFloor(&billingSnapshot, meta.ChannelId, imageRequest.Model); err != nil {
		logger.Errorf(ctx, "image billing procurement cost estimate failed user_id=%s group=%s channel_id=%s model=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), err.Error())
		return openai.ErrorWrapper(err, "calculate_image_quota_failed", http.StatusInternalServerError)
	}
	quota := billingSnapshot.ChargeAmount
	billingPlan, quotaErr := reserveRelayQuota(ctx, meta, quota)
	if quotaErr != nil {
		return quotaErr
	}
	groupQuotaSettled := false
	defer func() {
		if !groupQuotaSettled {
			releaseRelayBillingPlan(ctx, billingPlan)
		}
	}()

	if billingPlan.ChargeUserBalance() {
		if _, expireErr := model.ExpireUserBalanceLots(meta.UserId); expireErr != nil {
			logger.Errorf(ctx, "image billing expire lots failed user_id=%s group=%s channel_id=%s model=%s err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), expireErr.Error())
		}
		userQuota, err := model.CacheGetUserQuota(ctx, meta.UserId)
		if err != nil {
			return openai.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
		}
		if userQuota-quota < 0 {
			return openai.ErrorWrapper(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusForbidden)
		}
	}

	// do request
	resp, err := adaptor.DoRequest(c, meta, requestBody)
	if err != nil {
		return openai.ErrorWrapper(err, classifyImageRequestErrorCode(err), http.StatusInternalServerError)
	}

	responseSettled := false
	defer func(ctx context.Context) {
		if resp != nil &&
			resp.StatusCode != http.StatusCreated && // replicate returns 201
			resp.StatusCode != http.StatusOK {
			releaseRelayBillingPlan(ctx, billingPlan)
			return
		}
		if !responseSettled {
			releaseRelayBillingPlan(ctx, billingPlan)
			return
		}
		userDailyQuota := 0
		userEmergencyQuota := 0
		if !billingPlan.ChargeUserBalance() {
			dailyConsumed, emergencyConsumed := settleRelayBillingPlan(ctx, billingPlan, quota)
			userDailyQuota = int(dailyConsumed)
			userEmergencyQuota = int(emergencyConsumed)
		}

		if strings.TrimSpace(meta.TokenId) != "" && billingPlan.ChargeTokenQuota() {
			if billingPlan.ChargeUserBalance() {
				err := model.PostConsumeTokenQuota(meta.TokenId, quota)
				if err != nil {
					logger.Errorf(ctx, "image billing failed code=post_consume_token_quota_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), strings.TrimSpace(meta.TokenId), quota, billingPlan.ChargeUserBalance(), err.Error())
				}
			} else {
				err := model.PostConsumeTokenRemainQuota(meta.TokenId, quota)
				if err != nil {
					logger.Errorf(ctx, "image billing failed code=post_consume_token_remain_quota_failed user_id=%s group=%s channel_id=%s model=%s token_id=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), strings.TrimSpace(meta.TokenId), quota, billingPlan.ChargeUserBalance(), err.Error())
				}
			}
		} else if billingPlan.ChargeUserBalance() && quota != 0 {
			if err := model.DecreaseUserQuota(meta.UserId, quota); err != nil {
				logger.Errorf(ctx, "image billing failed code=post_consume_user_quota_failed user_id=%s group=%s channel_id=%s model=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, billingPlan.ChargeUserBalance(), err.Error())
			}
		}
		if billingPlan.ChargeUserBalance() {
			if err := model.CacheUpdateUserQuota(ctx, meta.UserId); err != nil {
				logger.Errorf(ctx, "image billing failed code=update_user_quota_cache_failed user_id=%s group=%s channel_id=%s model=%s quota=%d charge_user_balance=%t err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, billingPlan.ChargeUserBalance(), err.Error())
			}
			if quota > 0 {
				consumedFromLots, consumeErr := model.ConsumeUserBalanceLotsForGroup(meta.UserId, meta.Group, quota)
				if consumeErr != nil {
					logger.Errorf(ctx, "image billing lots consume failed user_id=%s group=%s channel_id=%s model=%s quota=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), quota, consumeErr.Error())
				} else if consumedFromLots < quota {
					logger.Warnf(ctx, "image billing lots consume partial user_id=%s group=%s channel_id=%s model=%s consumed=%d requested=%d", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), consumedFromLots, quota)
				}
			}
		}
		tokenName := c.GetString(ctxkey.TokenName)
		billingSnapshot.ChargeAmount = quota
		entry := &model.Log{
			UserId:             meta.UserId,
			GroupId:            meta.Group,
			ChannelId:          meta.ChannelId,
			PromptTokens:       int(billingSnapshot.InputQuantity),
			CompletionTokens:   int(billingSnapshot.OutputQuantity),
			ModelName:          imageRequest.Model,
			TokenName:          tokenName,
			Quota:              int(quota),
			BillingSource:      model.ResolveConsumeLogBillingSource(billingPlan.ChargeUserBalance()),
			UserDailyQuota:     userDailyQuota,
			UserEmergencyQuota: userEmergencyQuota,
			Content:            billing.FormatPricingLog(pricing, groupRatio),
		}
		applyRouteObservabilityToLog(entry, meta, imageRequest.Model)
		billingSnapshot.ApplyToLog(entry)
		billing.ApplyProcurementCostObservation(entry)
		model.RecordConsumeLog(ctx, entry)
		billing.RecordProcurementConsumptionObservation(ctx, entry)
		model.UpdateUserUsedQuotaAndRequestCount(meta.UserId, quota)
		channelId := c.GetString(ctxkey.ChannelId)
		model.UpdateChannelUsedQuota(channelId, quota)
	}(c.Request.Context())
	groupQuotaSettled = true

	_, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "image relay response failed user_id=%s group=%s channel_id=%s model=%s endpoint=%s err=%+v", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), c.Request.URL.Path, respErr)
		return respErr
	}
	responseSettled = true
	if outputCount, ok := aliadaptor.QwenImageOutputCount(c); ok && billing.ResolveImageBillingMode(pricing) == billing.ImageBillingModePerImage {
		finalSnapshot, snapshotErr := billing.ComputeImageBillingSnapshot(outputCount, 1, pricing, groupRatio)
		if snapshotErr != nil {
			logger.Errorf(ctx, "qwen image final billing snapshot failed user_id=%s group=%s channel_id=%s model=%s output_count=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), outputCount, snapshotErr.Error())
			return openai.ErrorWrapper(snapshotErr, "calculate_image_quota_failed", http.StatusInternalServerError)
		}
		finalSnapshot.PricingSource = strings.TrimSpace(pricing.Source)
		finalSnapshot.UsageSource = imageUsageSourceProviderResponse
		finalSnapshot.EstimateSource = imageEstimateSourceQwenImageOutputCount
		finalSnapshot.SettlementMode = imageSettlementModeProviderUsageFinal
		if err := billing.ApplyEstimatedProcurementCostFloor(&finalSnapshot, meta.ChannelId, imageRequest.Model); err != nil {
			logger.Errorf(ctx, "qwen image final procurement cost estimate failed user_id=%s group=%s channel_id=%s model=%s output_count=%d err=%q", strings.TrimSpace(meta.UserId), strings.TrimSpace(meta.Group), strings.TrimSpace(meta.ChannelId), strings.TrimSpace(imageRequest.Model), outputCount, err.Error())
		}
		billingSnapshot = finalSnapshot
		quota = billingSnapshot.ChargeAmount
	}

	return nil
}

func classifyImageRequestErrorCode(err error) string {
	if err == nil {
		return "do_request_failed"
	}
	lowerMessage := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(lowerMessage, "server sent goaway"):
		return "upstream_transport_goaway"
	default:
		return "do_request_failed"
	}
}
