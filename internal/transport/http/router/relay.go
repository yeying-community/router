package router

import (
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/internal/admin/controller"
	channel "github.com/yeying-community/router/internal/admin/controller/channel"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func SetRelayRouter(engine *gin.Engine) {
	engine.Use(middleware.CORS())
	// Negotiate the supported transport before body-based token distribution.
	// This GET never relays inference; POST retains its normal authentication.
	engine.GET("/v1/responses", controller.ResponsesHTTPTransport)

	modelsRouter := engine.Group("/v1/models")
	modelsRouter.Use(middleware.TokenAuth())
	{
		modelsRouter.GET("", controller.ListModels)
		modelsRouter.GET("/:model", controller.RetrieveModel)
	}

	// Provider catalog discovery is useful to OpenRouter/LiteLLM-style clients.
	// Keep the existing token-authenticated catalog handler and expose it beside
	// the standard model endpoints.
	providerCatalogRouter := engine.Group("/v1/providers")
	providerCatalogRouter.Use(middleware.TokenAuth())
	providerCatalogRouter.GET("/models", channel.GetPublicProviderModels)

	relayV1Router := engine.Group("/v1")
	relayV1Router.Use(middleware.RelayLogger(), middleware.TokenAuth(), middleware.Distribute())
	{
		relayV1Router.POST("/completions", controller.Relay)
		relayV1Router.POST("/chat/completions", controller.Relay)
		relayV1Router.POST("/messages", controller.Relay)
		relayV1Router.POST("/responses", controller.Relay)
		relayV1Router.POST("/edits", controller.Relay)
		relayV1Router.POST("/images/generations", controller.Relay)
		relayV1Router.POST("/images/edits", controller.Relay)
		relayV1Router.POST("/images/variations", controller.RelayNotImplemented)
		relayV1Router.POST("/embeddings", controller.Relay)
		relayV1Router.POST("/engines/:model/embeddings", controller.Relay)
		relayV1Router.POST("/audio/transcriptions", controller.Relay)
		relayV1Router.POST("/audio/translations", controller.Relay)
		relayV1Router.POST("/audio/speech", controller.Relay)
		relayV1Router.GET("/realtime", controller.Relay)
		relayV1Router.POST("/realtime/client_secrets", controller.Relay)
		relayV1Router.POST("/realtime/sessions", controller.Relay)
		relayV1Router.POST("/realtime/transcription_sessions", controller.Relay)
		relayV1Router.POST("/realtime/calls", controller.Relay)
		relayV1Router.POST("/videos", controller.Relay)
		relayV1Router.GET("/videos/:id", controller.Relay)
		relayV1Router.GET("/files", controller.RelayNotImplemented)
		relayV1Router.POST("/files", controller.RelayNotImplemented)
		relayV1Router.DELETE("/files/:id", controller.RelayNotImplemented)
		relayV1Router.GET("/files/:id", controller.RelayNotImplemented)
		relayV1Router.GET("/files/:id/content", controller.RelayNotImplemented)
		relayV1Router.POST("/fine_tuning/jobs", controller.RelayNotImplemented)
		relayV1Router.GET("/fine_tuning/jobs", controller.RelayNotImplemented)
		relayV1Router.GET("/fine_tuning/jobs/:id", controller.RelayNotImplemented)
		relayV1Router.POST("/fine_tuning/jobs/:id/cancel", controller.RelayNotImplemented)
		relayV1Router.GET("/fine_tuning/jobs/:id/events", controller.RelayNotImplemented)
		relayV1Router.DELETE("/models/:model", controller.RelayNotImplemented)
		relayV1Router.POST("/moderations", controller.Relay)
		relayV1Router.POST("/assistants", controller.RelayNotImplemented)
		relayV1Router.GET("/assistants/:id", controller.RelayNotImplemented)
		relayV1Router.POST("/assistants/:id", controller.RelayNotImplemented)
		relayV1Router.DELETE("/assistants/:id", controller.RelayNotImplemented)
		relayV1Router.GET("/assistants", controller.RelayNotImplemented)
		relayV1Router.POST("/assistants/:id/files", controller.RelayNotImplemented)
		relayV1Router.GET("/assistants/:id/files/:fileId", controller.RelayNotImplemented)
		relayV1Router.DELETE("/assistants/:id/files/:fileId", controller.RelayNotImplemented)
		relayV1Router.GET("/assistants/:id/files", controller.RelayNotImplemented)
		relayV1Router.POST("/threads", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id", controller.RelayNotImplemented)
		relayV1Router.DELETE("/threads/:id", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/messages", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/messages/:messageId", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/messages/:messageId", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/messages/:messageId/files/:filesId", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/messages/:messageId/files", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/runs", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/runs/:runsId", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/runs", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/runs/:runsId/submit_tool_outputs", controller.RelayNotImplemented)
		relayV1Router.POST("/threads/:id/runs/:runsId/cancel", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/runs/:runsId/steps/:stepId", controller.RelayNotImplemented)
		relayV1Router.GET("/threads/:id/runs/:runsId/steps", controller.RelayNotImplemented)
	}
}
