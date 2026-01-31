package meta

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/channeltype"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Meta struct {
	Mode         int
	ChannelType  int
	ChannelId    int
	TokenId      int
	TokenName    string
	UserId       int
	Group        string
	ModelMapping map[string]string
	// ChannelModelRatio is the optional per-channel model ratio JSON string.
	ChannelModelRatio string
	// ChannelCompletionRatio is the optional per-channel completion ratio JSON string.
	ChannelCompletionRatio string
	// BaseURL is the proxy url set in the channel config
	BaseURL  string
	APIKey   string
	APIType  int
	Config   model.ChannelConfig
	IsStream bool
	// OriginModelName is the model name from the raw user request
	OriginModelName string
	// ActualModelName is the model name after mapping
	ActualModelName    string
	RequestURLPath     string
	PromptTokens       int // only for DoResponse
	ForcedSystemPrompt string
	StartTime          time.Time
}

func GetByContext(c *gin.Context) *Meta {
	normalizedPath := relaymode.NormalizePath(c.Request.URL.String())
	meta := Meta{
		Mode:                   relaymode.GetByPath(c.Request.URL.Path),
		ChannelType:            c.GetInt(ctxkey.Channel),
		ChannelId:              c.GetInt(ctxkey.ChannelId),
		TokenId:                c.GetInt(ctxkey.TokenId),
		TokenName:              c.GetString(ctxkey.TokenName),
		UserId:                 c.GetInt(ctxkey.Id),
		Group:                  c.GetString(ctxkey.Group),
		ModelMapping:           c.GetStringMapString(ctxkey.ModelMapping),
		ChannelModelRatio:      c.GetString(ctxkey.ModelRatio),
		ChannelCompletionRatio: c.GetString(ctxkey.CompletionRatio),
		OriginModelName:        c.GetString(ctxkey.RequestModel),
		BaseURL:                c.GetString(ctxkey.BaseURL),
		APIKey:                 strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer "),
		RequestURLPath:         normalizedPath,
		ForcedSystemPrompt:     c.GetString(ctxkey.SystemPrompt),
		StartTime:              time.Now(),
	}
	cfg, ok := c.Get(ctxkey.Config)
	if ok {
		meta.Config = cfg.(model.ChannelConfig)
	}
	if meta.BaseURL == "" {
		meta.BaseURL = channeltype.ChannelBaseURLs[meta.ChannelType]
	}
	meta.APIType = channeltype.ToAPIType(meta.ChannelType)
	if meta.Config.UseResponses {
		meta.Mode = relaymode.Responses
		meta.RequestURLPath = "/v1/responses"
	}
	return &meta
}
