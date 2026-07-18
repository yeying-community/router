package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

const billingServiceQueryPath = "/api/v1/internal/billing:query"

type billingServiceQueryRequest struct {
	ChannelID  string         `json:"channel_id,omitempty"`
	Adapter    string         `json:"adapter"`
	BaseURL    string         `json:"base_url,omitempty"`
	Credential string         `json:"credential,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

type billingServiceQueryResponse struct {
	Data  billingServiceSnapshot `json:"data"`
	Error *billingServiceError   `json:"error,omitempty"`
}

type billingServiceError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type billingServiceSnapshot struct {
	ChannelID string               `json:"channel_id,omitempty"`
	Adapter   string               `json:"adapter"`
	FetchedAt time.Time            `json:"fetched_at"`
	Items     []billingServiceItem `json:"items"`
	Metadata  map[string]any       `json:"metadata,omitempty"`
}

type billingServiceItem struct {
	ResourceType    string         `json:"resource_type"`
	QuotaType       string         `json:"quota_type"`
	QuotaLabel      string         `json:"quota_label,omitempty"`
	Amount          float64        `json:"amount"`
	LimitAmount     float64        `json:"limit_amount"`
	UsedAmount      float64        `json:"used_amount"`
	RemainingAmount float64        `json:"remaining_amount"`
	Currency        string         `json:"currency,omitempty"`
	ResetAt         *time.Time     `json:"reset_at,omitempty"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	Status          string         `json:"status"`
	SourceRef       string         `json:"source_ref,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

func billingServiceConfigured() bool {
	return strings.TrimSpace(config.BillingServiceBaseURL) != ""
}

func resolveBillingServiceRequestURLs() []string {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BillingServiceBaseURL), "/")
	if baseURL == "" {
		return nil
	}
	return []string{baseURL + billingServiceQueryPath}
}

func resolveBillingServiceAdapter(profile model.ChannelBillingProfile) string {
	switch strings.TrimSpace(profile.BillingMode) {
	case model.ChannelBillingModeBuiltinOpenAI:
		return "openai-subscription"
	case model.ChannelBillingModeBuiltinCloseAI:
		return "closeai"
	case model.ChannelBillingModeBuiltinOpenAISB:
		return "openai-sb"
	case model.ChannelBillingModeBuiltinAIProxy:
		return "aiproxy"
	case model.ChannelBillingModeBuiltinAPI2GPT:
		return "api2gpt"
	case model.ChannelBillingModeBuiltinAIGC2D:
		return "aigc2d"
	case model.ChannelBillingModeBuiltinSiliconFlow:
		return "siliconflow"
	case model.ChannelBillingModeBuiltinDeepSeek:
		return "deepseek"
	case model.ChannelBillingModeBuiltinOpenRouter:
		return "openrouter"
	case model.ChannelBillingModeBuiltinCDK:
		return "aixhan"
	default:
		return ""
	}
}

func buildBillingServiceQuery(channel *model.Channel, profile model.ChannelBillingProfile) (billingServiceQueryRequest, error) {
	if channel == nil {
		return billingServiceQueryRequest{}, fmt.Errorf("渠道不存在")
	}
	adapter := resolveBillingServiceAdapter(profile)
	if adapter == "" {
		return billingServiceQueryRequest{}, fmt.Errorf("当前渠道不支持 Billing 服务刷新账务")
	}
	request := billingServiceQueryRequest{
		ChannelID:  strings.TrimSpace(channel.Id),
		Adapter:    adapter,
		BaseURL:    resolveChannelBillingAPIBaseURL(channel, profile),
		Credential: strings.TrimSpace(channel.Key),
	}
	if strings.TrimSpace(profile.BillingMode) == model.ChannelBillingModeBuiltinCDK {
		request.Config = map[string]any{
			"cdk":      resolveChannelCDKKey(channel, profile),
			"currency": resolveChannelCDKBillingCurrency(profile),
		}
		return request, nil
	}
	if currency := resolveChannelBillingSnapshotCurrency(channel); currency != "" {
		request.Config = map[string]any{"currency": currency}
	}
	return request, nil
}

func collectBillingServiceSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, messageText string) (collectedChannelBillingSnapshot, error) {
	request, err := buildBillingServiceQuery(channel, profile)
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(config.BillingServiceBaseURL), "/") + billingServiceQueryPath
	payload, err := json.Marshal(request)
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	timeout := time.Duration(config.BillingServiceTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if apiKey := strings.TrimSpace(config.BillingServiceAPIKey); apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return collectedChannelBillingSnapshot{}, fmt.Errorf("调用 Billing 服务失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return collectedChannelBillingSnapshot{}, err
	}
	decoded := billingServiceQueryResponse{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			return collectedChannelBillingSnapshot{}, fmt.Errorf("解析 Billing 服务响应失败: %w", err)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil {
			return collectedChannelBillingSnapshot{}, fmt.Errorf("Billing 服务返回 %s: %s", strings.TrimSpace(decoded.Error.Code), strings.TrimSpace(decoded.Error.Message))
		}
		return collectedChannelBillingSnapshot{}, fmt.Errorf("Billing 服务返回 HTTP %d", resp.StatusCode)
	}
	if decoded.Data.Adapter == "" && len(decoded.Data.Items) == 0 {
		return collectedChannelBillingSnapshot{}, fmt.Errorf("Billing 服务响应为空")
	}
	return convertBillingServiceSnapshot(channel, profile, decoded.Data, messageText), nil
}

func convertBillingServiceSnapshot(channel *model.Channel, profile model.ChannelBillingProfile, snapshot billingServiceSnapshot, messageText string) collectedChannelBillingSnapshot {
	items := make([]model.ChannelBillingSnapshotItem, 0, len(snapshot.Items))
	for index, item := range snapshot.Items {
		items = append(items, model.ChannelBillingSnapshotItem{
			ResourceType:    strings.TrimSpace(strings.ToLower(item.ResourceType)),
			QuotaType:       strings.TrimSpace(strings.ToLower(item.QuotaType)),
			QuotaLabel:      strings.TrimSpace(item.QuotaLabel),
			Amount:          item.Amount,
			LimitAmount:     item.LimitAmount,
			UsedAmount:      item.UsedAmount,
			RemainingAmount: item.RemainingAmount,
			Currency:        strings.TrimSpace(strings.ToUpper(item.Currency)),
			ResetAt:         unixFromTime(item.ResetAt),
			ExpiresAt:       unixFromTime(item.ExpiresAt),
			Status:          strings.TrimSpace(strings.ToLower(item.Status)),
			SourceRef:       strings.TrimSpace(item.SourceRef),
			Metadata:        marshalMetadataString(item.Metadata),
			SortOrder:       index + 1,
		})
	}
	primaryAmount := primaryBillingServiceAmount(items)
	currency := primaryBillingServiceCurrency(items)
	messageParts := []string{}
	if message := strings.TrimSpace(messageText); message != "" {
		messageParts = append(messageParts, message)
	}
	if adapter := strings.TrimSpace(snapshot.Adapter); adapter != "" {
		messageParts = append(messageParts, "adapter="+adapter)
	}
	requestURLs := requestURLsFromBillingServiceMetadata(snapshot.Metadata)
	if len(requestURLs) == 0 {
		requestURLs = []string{strings.TrimRight(strings.TrimSpace(config.BillingServiceBaseURL), "/") + billingServiceQueryPath}
	}
	return collectedChannelBillingSnapshot{
		Snapshot: model.ChannelBillingSnapshot{
			ChannelId:  strings.TrimSpace(channel.Id),
			SourceType: model.ChannelBillingSnapshotSourceAPI,
			Balance:    primaryAmount,
			Currency:   currency,
			RawStatus:  "ok",
			Message:    strings.Join(messageParts, " | "),
			RequestURL: strings.Join(requestURLs, "\n"),
		},
		Items:          items,
		PrimaryAmount:  primaryAmount,
		ShouldHardStop: shouldHardStopBillingServiceSnapshot(profile, items, primaryAmount),
	}
}

func unixFromTime(value *time.Time) int64 {
	if value == nil || value.IsZero() {
		return 0
	}
	return value.Unix()
}

func marshalMetadataString(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	return marshalLogJSON(value)
}

func primaryBillingServiceCurrency(items []model.ChannelBillingSnapshotItem) string {
	for _, item := range items {
		if currency := strings.TrimSpace(strings.ToUpper(item.Currency)); currency != "" {
			return currency
		}
	}
	return ""
}

func primaryBillingServiceAmount(items []model.ChannelBillingSnapshotItem) float64 {
	normalized := model.NormalizeChannelBillingSnapshotItems(items)
	for _, item := range normalized {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		quotaType := strings.TrimSpace(strings.ToLower(item.QuotaType))
		if quotaType == "total" && (resourceType == model.ChannelBillingResourceTypeCredit || resourceType == model.ChannelBillingResourceTypeBalance || resourceType == model.ChannelBillingResourceTypeQuota) {
			if item.RemainingAmount > 0 || item.Amount > 0 {
				if item.RemainingAmount > 0 {
					return item.RemainingAmount
				}
				return item.Amount
			}
			return 0
		}
	}
	for _, item := range normalized {
		if item.RemainingAmount > 0 {
			return item.RemainingAmount
		}
		if item.Amount > 0 {
			return item.Amount
		}
	}
	return 0
}

func shouldHardStopBillingServiceSnapshot(profile model.ChannelBillingProfile, items []model.ChannelBillingSnapshotItem, primaryAmount float64) bool {
	if len(items) == 0 {
		return true
	}
	if strings.TrimSpace(profile.BillingMode) == model.ChannelBillingModeBuiltinCDK {
		return false
	}
	for _, item := range model.NormalizeChannelBillingSnapshotItems(items) {
		if strings.TrimSpace(strings.ToLower(item.QuotaType)) == "total" {
			return item.RemainingAmount <= 0 && item.Amount <= 0
		}
	}
	return primaryAmount <= 0
}

func requestURLsFromBillingServiceMetadata(metadata map[string]any) []string {
	raw, ok := metadata["request_urls"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return normalizeRequestURLs(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := strings.TrimSpace(fmt.Sprintf("%v", item)); value != "" {
				values = append(values, value)
			}
		}
		return normalizeRequestURLs(values)
	default:
		return nil
	}
}

func normalizeRequestURLs(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
