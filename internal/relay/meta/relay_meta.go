package meta

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

type Meta struct {
	Mode                  int
	ChannelProtocol       int
	ChannelId             string
	TokenId               string
	TokenName             string
	UserId                string
	Group                 string
	EntitlementSourceType string
	EntitlementSourceID   string
	EntitlementSourceName string
	ModelMapping          map[string]string
	ChannelModelConfigs   []model.ChannelModel
	EndpointPolicy        *model.ChannelModelEndpointPolicy
	// BaseURL is the proxy url set in the channel config
	BaseURL  string
	APIKey   string
	APIType  int
	Config   model.ChannelConfig
	IsStream bool
	// OriginModelName is the model name from the raw user request
	OriginModelName string
	// ActualModelName is the model name after mapping
	ActualModelName     string
	RequestURLPath      string
	UpstreamMode        int
	UpstreamRequestPath string
	PromptTokens        int // only for DoResponse
	StartTime           time.Time
	FallbackCount       int
	FallbackAttempts    string
	RelayErrorType      string
	RelayErrorCode      string
	RelayErrorMessage   string
}

func GetByContext(c *gin.Context) *Meta {
	normalizedPath := relaymode.NormalizePath(c.Request.URL.String())
	meta := Meta{
		Mode:                  relaymode.GetByPath(c.Request.URL.Path),
		ChannelProtocol:       c.GetInt(ctxkey.Channel),
		ChannelId:             c.GetString(ctxkey.ChannelId),
		TokenId:               c.GetString(ctxkey.TokenId),
		TokenName:             c.GetString(ctxkey.TokenName),
		UserId:                c.GetString(ctxkey.Id),
		Group:                 c.GetString(ctxkey.Group),
		EntitlementSourceType: c.GetString(ctxkey.EntitlementSourceType),
		EntitlementSourceID:   c.GetString(ctxkey.EntitlementSourceId),
		EntitlementSourceName: c.GetString(ctxkey.EntitlementSourceName),
		ModelMapping:          c.GetStringMapString(ctxkey.ModelMapping),
		OriginModelName:       c.GetString(ctxkey.RequestModel),
		BaseURL:               c.GetString(ctxkey.BaseURL),
		APIKey:                strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer "),
		RequestURLPath:        normalizedPath,
		UpstreamMode:          relaymode.GetByPath(c.Request.URL.Path),
		UpstreamRequestPath:   normalizedPath,
		StartTime:             time.Now(),
		FallbackCount:         c.GetInt(ctxkey.RelayRetryCount),
		FallbackAttempts:      c.GetString(ctxkey.RelayFallbackAttempts),
		RelayErrorType:        c.GetString(ctxkey.RelayErrorType),
		RelayErrorCode:        c.GetString(ctxkey.RelayErrorCode),
		RelayErrorMessage:     c.GetString(ctxkey.RelayError),
	}
	cfg, ok := c.Get(ctxkey.Config)
	if ok {
		meta.Config = cfg.(model.ChannelConfig)
	}
	if endpointBaseURL := model.CacheGetChannelModelEndpointBaseURL(
		meta.ChannelId,
		c.Request.URL.Path,
		meta.OriginModelName,
	); endpointBaseURL != "" {
		meta.BaseURL = endpointBaseURL
		c.Set(ctxkey.BaseURL, endpointBaseURL)
	} else if apiBaseURL := meta.Config.GetAPIBaseURL(); apiBaseURL != "" {
		meta.BaseURL = apiBaseURL
		c.Set(ctxkey.BaseURL, apiBaseURL)
	}
	if channelModelConfigs, ok := c.Get(ctxkey.ChannelModelConfigs); ok {
		if rows, castOK := channelModelConfigs.([]model.ChannelModel); castOK {
			meta.ChannelModelConfigs = rows
		}
	}
	if meta.BaseURL == "" {
		meta.BaseURL = relaychannel.BaseURLByProtocol(relaychannel.ProtocolByType(meta.ChannelProtocol))
	}
	meta.APIType = relaychannel.ToAPITypeForRequest(
		meta.ChannelProtocol,
		meta.UpstreamRequestPath,
		meta.RequestURLPath,
	)
	return &meta
}
