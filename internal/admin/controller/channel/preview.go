package channel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/client"
	commonutils "github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/relay/channeltype"
)

type previewModelsRequest struct {
	Type          int             `json:"type"`
	Key           string          `json:"key"`
	BaseURL       string          `json:"base_url"`
	Config        json.RawMessage `json:"config"`
	ModelProvider string          `json:"model_provider"`
}

type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func isOpenAICompatibleType(channelType int) bool {
	return channelType == channeltype.OpenAICompatible || channelType == channeltype.GeminiOpenAICompatible
}

// PreviewChannelModels godoc
// @Summary Preview models for OpenAI-compatible channel (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.ChannelPreviewModelsRequest true "Preview payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/channel/preview/models [post]
func PreviewChannelModels(c *gin.Context) {
	var req previewModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if !isOpenAICompatibleType(req.Type) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前渠道类型暂不支持自动获取模型",
		})
		return
	}

	if strings.TrimSpace(req.Key) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请先填写 Key",
		})
		return
	}
	modelProvider := commonutils.NormalizeModelProvider(req.ModelProvider)

	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = channeltype.ChannelBaseURLs[req.Type]
		if baseURL == "" {
			baseURL = channeltype.ChannelBaseURLs[channeltype.OpenAI]
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	modelsURL := baseURL + "/v1/models"
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		modelsURL = baseURL + "/models"
	}

	httpReq, err := http.NewRequest(http.MethodGet, modelsURL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "创建请求失败",
		})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.Key))

	resp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请求模型列表失败",
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "读取模型列表失败",
		})
		return
	}

	var parsed openAIModelsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "解析模型列表失败",
		})
		return
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := fmt.Sprintf("模型列表请求失败（HTTP %d）", resp.StatusCode)
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": msg,
		})
		return
	}

	modelIDs := make([]string, 0, len(parsed.Data))
	seen := make(map[string]struct{}, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if !commonutils.MatchModelProvider(id, item.OwnedBy, modelProvider) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		modelIDs = append(modelIDs, id)
	}
	if len(modelIDs) == 0 {
		msg := "未返回可用模型"
		if modelProvider != "" {
			msg = "未找到符合所选模型供应商的模型"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": msg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    modelIDs,
	})
}
