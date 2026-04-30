package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	relaylogging "github.com/yeying-community/router/internal/relay/logging"
	"github.com/yeying-community/router/internal/relay/relaymode"
)

func RelayLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		begin := relaylogging.NewFields("START").
			String("method", c.Request.Method).
			String("path", c.Request.URL.Path).
			String("mode", relayModeName(c.Request.URL.Path)).
			String("ip", c.ClientIP()).
			String("ua", c.Request.UserAgent())
		logger.RelayInfof(c.Request.Context(), begin.Build())

		c.Next()

		status := c.Writer.Status()
		end := relaylogging.NewFields("END").
			String("method", c.Request.Method).
			String("path", c.Request.URL.Path).
			String("mode", relayModeName(c.Request.URL.Path)).
			Int("status", status).
			Duration("latency", time.Since(startedAt)).
			String("ip", c.ClientIP()).
			String("user_id", c.GetString(ctxkey.Id)).
			String("token_id", c.GetString(ctxkey.TokenId)).
			String("token_name", c.GetString(ctxkey.TokenName)).
			String("channel_id", c.GetString(ctxkey.ChannelId)).
			String("channel_name", c.GetString(ctxkey.ChannelName)).
			String("group", c.GetString(ctxkey.Group)).
			String("request_model", c.GetString(ctxkey.RequestModel)).
			String("original_model", c.GetString(ctxkey.OriginalModel)).
			String("protocol", relayProtocolName(c)).
			String("upstream_url", c.GetString(ctxkey.UpstreamURL)).
			Int("upstream_status", c.GetInt(ctxkey.UpstreamStatus)).
			Int("retry_count", c.GetInt(ctxkey.RelayRetryCount)).
			String("termination", c.GetString(ctxkey.RelayTermination)).
			String("error_type", c.GetString(ctxkey.RelayErrorType)).
			String("error_code", c.GetString(ctxkey.RelayErrorCode)).
			String("error", c.GetString(ctxkey.RelayError))

		switch {
		case c.GetString(ctxkey.RelayTermination) != "":
			logger.RelayWarnf(c.Request.Context(), end.Build())
		case status >= 500 || c.GetString(ctxkey.RelayError) != "":
			logger.RelayErrorf(c.Request.Context(), end.Build())
		case status >= 400:
			logger.RelayWarnf(c.Request.Context(), end.Build())
		default:
			logger.RelayInfof(c.Request.Context(), end.Build())
		}
	}
}

func relayModeName(path string) string {
	switch relaymode.GetByPath(path) {
	case relaymode.ChatCompletions:
		return "chat_completions"
	case relaymode.Messages:
		return "messages"
	case relaymode.Completions:
		return "completions"
	case relaymode.Embeddings:
		return "embeddings"
	case relaymode.Moderations:
		return "moderations"
	case relaymode.ImagesGenerations:
		return "images_generations"
	case relaymode.Edits:
		return "edits"
	case relaymode.AudioSpeech:
		return "audio_speech"
	case relaymode.AudioTranscription:
		return "audio_transcription"
	case relaymode.AudioTranslation:
		return "audio_translation"
	case relaymode.Proxy:
		return "proxy"
	case relaymode.Responses:
		return "responses"
	case relaymode.Videos:
		return "videos"
	default:
		return "unknown"
	}
}

func relayProtocolName(c *gin.Context) string {
	protocol := c.GetInt(ctxkey.Channel)
	if protocol == 0 {
		return ""
	}
	return relaychannel.ProtocolByType(protocol)
}
