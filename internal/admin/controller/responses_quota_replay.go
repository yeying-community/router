package controller

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	dbmodel "github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/relay/model"
	"github.com/yeying-community/router/internal/relay/responsestate"
	"github.com/yeying-community/router/internal/relay/routing"
)

// An item ID alone does not require the old account when the client supplies
// the text and completed local tool pairs. Detach only after an explicit quota
// rejection, before any output, and keep normal auth/entitlement/retry selection.
func prepareResponsesQuotaReplay(c *gin.Context, err *model.ErrorWithStatusCode) bool {
	if c == nil || c.Request == nil || !isPinnedResponsesRequest(c) ||
		!isUpstreamQuotaRelayError(err) || isLocalQuotaRelayError(err) ||
		c.Writer.Written() || c.Request.Context().Err() != nil {
		return false
	}
	if _, pinned := c.Get(ctxkey.SpecificChannelId); pinned {
		return false
	}
	channelID := c.GetString(ctxkey.ChannelId)
	if dbmodel.IsPersonalChannelCandidateID(channelID) || dbmodel.IsCommunityOfferChannelID(channelID) ||
		requestProviderRoutingPolicy(c).RetryScope == routing.RetryScopeNone ||
		strings.TrimSpace(c.GetHeader("x-codex-turn-state")) != "" {
		return false
	}
	raw, readErr := common.GetRequestBody(c)
	if readErr != nil {
		return false
	}
	replay, ok := responsestate.PrepareToolReplay(raw)
	if !ok {
		return false
	}
	c.Set(ctxkey.KeyRequestBody, replay)
	c.Request.Body = io.NopCloser(bytes.NewReader(replay))
	c.Request.ContentLength = int64(len(replay))
	if c.Request.Header.Get("Content-Length") != "" {
		c.Request.Header.Set("Content-Length", strconv.Itoa(len(replay)))
	}
	c.Set(ctxkey.ResponsesPreviousResponseID, "")
	c.Set(ctxkey.ResponsesItemIDs, nil)
	c.Set(ctxkey.ResponsesStatefulRequest, false)
	logger.RelayInfof(c.Request.Context(), "RETRY decision=detach_tool_history reason=upstream_quota endpoint=%s", c.Request.URL.Path)
	return true
}
