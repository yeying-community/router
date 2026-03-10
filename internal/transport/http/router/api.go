package router

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	admin "github.com/yeying-community/router/internal/admin/controller"
	auth "github.com/yeying-community/router/internal/admin/controller/auth"
	channel "github.com/yeying-community/router/internal/admin/controller/channel"
	group "github.com/yeying-community/router/internal/admin/controller/group"
	log "github.com/yeying-community/router/internal/admin/controller/log"
	option "github.com/yeying-community/router/internal/admin/controller/option"
	token "github.com/yeying-community/router/internal/admin/controller/token"
	user "github.com/yeying-community/router/internal/admin/controller/user"
	"github.com/yeying-community/router/internal/transport/http/middleware"
)

func SetApiRouter(engine *gin.Engine) {
	publicAuthRouter := engine.Group("/api/v1/public/common/auth")
	publicAuthRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	publicAuthRouter.Use(middleware.GlobalAPIRateLimit())
	{
		publicAuthRouter.POST("/challenge", middleware.CriticalRateLimit(), auth.WalletChallengeProto)
		publicAuthRouter.POST("/verify", middleware.CriticalRateLimit(), auth.WalletVerifyProto)
		publicAuthRouter.POST("/refreshToken", middleware.CriticalRateLimit(), auth.WalletRefreshToken)
	}

	web3AuthRouter := engine.Group("/api/v1/public/auth")
	web3AuthRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	web3AuthRouter.Use(middleware.GlobalAPIRateLimit())
	{
		web3AuthRouter.POST("/challenge", middleware.CriticalRateLimit(), auth.WalletChallengeWeb3)
		web3AuthRouter.POST("/verify", middleware.CriticalRateLimit(), auth.WalletVerifyWeb3)
		web3AuthRouter.POST("/refresh", middleware.CriticalRateLimit(), auth.WalletRefreshWeb3)
		web3AuthRouter.POST("/logout", middleware.CriticalRateLimit(), auth.WalletLogoutWeb3)
	}

	publicRouter := engine.Group("/api/v1/public")
	publicRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	publicRouter.Use(middleware.GlobalAPIRateLimit())
	{
		publicRouter.GET("/profile", middleware.CriticalRateLimit(), auth.PublicProfile)

		publicRouter.GET("/status", admin.GetStatus)
		publicRouter.GET("/notice", admin.GetNotice)
		publicRouter.GET("/about", admin.GetAbout)
		publicRouter.GET("/home_page_content", admin.GetHomePageContent)

		// 仅保留密码找回，无额外人机验证
		publicRouter.GET("/reset_password", middleware.CriticalRateLimit(), admin.SendPasswordResetEmail)
		publicRouter.POST("/user/reset", middleware.CriticalRateLimit(), admin.ResetPassword)

		publicRouter.GET("/oauth/wallet/nonce", middleware.CriticalRateLimit(), auth.WalletNonce)
		publicRouter.POST("/oauth/wallet/login", middleware.CriticalRateLimit(), auth.WalletLogin)
		publicRouter.POST("/oauth/wallet/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.WalletBind)
		publicRouter.GET("/oauth/state", middleware.CriticalRateLimit(), auth.GenerateOAuthCode)
		publicRouter.GET("/oauth/github", middleware.CriticalRateLimit(), auth.GitHubOAuth)
		publicRouter.GET("/oauth/lark", middleware.CriticalRateLimit(), auth.LarkOAuth)

		publicUserRoute := publicRouter.Group("/user")
		{
			publicUserRoute.POST("/register", middleware.CriticalRateLimit(), user.Register)
			publicUserRoute.POST("/login", middleware.CriticalRateLimit(), user.Login)
			publicUserRoute.GET("/logout", user.Logout)

			publicSelfRoute := publicUserRoute.Group("/")
			publicSelfRoute.Use(middleware.UserAuth())
			{
				publicSelfRoute.GET("/self", user.GetSelf)
				publicSelfRoute.GET("/dashboard", user.GetUserDashboard)
				publicSelfRoute.GET("/spend/overview", user.GetUserSpendOverview)
				publicSelfRoute.GET("/available_models", admin.GetUserAvailableModels)
				publicSelfRoute.PUT("/self", user.UpdateSelf)
				publicSelfRoute.DELETE("/self", user.DeleteSelf)
				publicSelfRoute.GET("/token", user.GenerateAccessToken)
				publicSelfRoute.GET("/aff", user.GetAffCode)
				publicSelfRoute.POST("/topup", user.TopUp)
			}
		}

		publicTokenRoute := publicRouter.Group("/token")
		publicTokenRoute.Use(middleware.UserAuth())
		{
			publicTokenRoute.GET("/", token.GetAllTokens)
			publicTokenRoute.GET("/search", token.SearchTokens)
			publicTokenRoute.GET("/:id", token.GetToken)
			publicTokenRoute.POST("/", token.AddToken)
			publicTokenRoute.PUT("/", token.UpdateToken)
			publicTokenRoute.DELETE("/:id", token.DeleteToken)
		}

		publicLogRoute := publicRouter.Group("/log")
		publicLogRoute.Use(middleware.UserAuth())
		{
			publicLogRoute.GET("/self/stat", log.GetLogsSelfStat)
			publicLogRoute.GET("/self", log.GetUserLogs)
			publicLogRoute.GET("/self/search", log.SearchUserLogs)
		}

		publicChannelRoute := publicRouter.Group("/channel")
		{
			// 模型列表对所有登录用户开放，方便前端展示供应商/模型
			publicChannelRoute.GET("/models", middleware.UserAuth(), admin.DashboardListModels)
		}
	}

	publicModelsRouter := engine.Group("/api/v1/public/models")
	publicModelsRouter.Use(middleware.TokenAuth())
	{
		publicModelsRouter.GET("", admin.ListModels)
		publicModelsRouter.GET("/:model", admin.RetrieveModel)
	}

	publicRelayRouter := engine.Group("/api/v1/public")
	publicRelayRouter.Use(middleware.RelayLogger(), middleware.TokenAuth(), middleware.Distribute())
	{
		publicRelayRouter.POST("/completions", admin.Relay)
		publicRelayRouter.POST("/chat/completions", admin.Relay)
		publicRelayRouter.POST("/responses", admin.Relay)
		publicRelayRouter.POST("/edits", admin.Relay)
		publicRelayRouter.POST("/images/generations", admin.Relay)
		publicRelayRouter.POST("/images/edits", admin.RelayNotImplemented)
		publicRelayRouter.POST("/images/variations", admin.RelayNotImplemented)
		publicRelayRouter.POST("/embeddings", admin.Relay)
		publicRelayRouter.POST("/engines/:model/embeddings", admin.Relay)
		publicRelayRouter.POST("/audio/transcriptions", admin.Relay)
		publicRelayRouter.POST("/audio/translations", admin.Relay)
		publicRelayRouter.POST("/audio/speech", admin.Relay)
		publicRelayRouter.GET("/files", admin.RelayNotImplemented)
		publicRelayRouter.POST("/files", admin.RelayNotImplemented)
		publicRelayRouter.DELETE("/files/:id", admin.RelayNotImplemented)
		publicRelayRouter.GET("/files/:id", admin.RelayNotImplemented)
		publicRelayRouter.GET("/files/:id/content", admin.RelayNotImplemented)
		publicRelayRouter.POST("/fine_tuning/jobs", admin.RelayNotImplemented)
		publicRelayRouter.GET("/fine_tuning/jobs", admin.RelayNotImplemented)
		publicRelayRouter.GET("/fine_tuning/jobs/:id", admin.RelayNotImplemented)
		publicRelayRouter.POST("/fine_tuning/jobs/:id/cancel", admin.RelayNotImplemented)
		publicRelayRouter.GET("/fine_tuning/jobs/:id/events", admin.RelayNotImplemented)
		publicRelayRouter.DELETE("/models/:model", admin.RelayNotImplemented)
		publicRelayRouter.POST("/moderations", admin.Relay)
		publicRelayRouter.POST("/assistants", admin.RelayNotImplemented)
		publicRelayRouter.GET("/assistants/:id", admin.RelayNotImplemented)
		publicRelayRouter.POST("/assistants/:id", admin.RelayNotImplemented)
		publicRelayRouter.DELETE("/assistants/:id", admin.RelayNotImplemented)
		publicRelayRouter.GET("/assistants", admin.RelayNotImplemented)
		publicRelayRouter.POST("/assistants/:id/files", admin.RelayNotImplemented)
		publicRelayRouter.GET("/assistants/:id/files/:fileId", admin.RelayNotImplemented)
		publicRelayRouter.DELETE("/assistants/:id/files/:fileId", admin.RelayNotImplemented)
		publicRelayRouter.GET("/assistants/:id/files", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id", admin.RelayNotImplemented)
		publicRelayRouter.DELETE("/threads/:id", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/messages", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/messages/:messageId", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/messages/:messageId", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/messages/:messageId/files/:filesId", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/messages/:messageId/files", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/runs", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/runs/:runsId", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/runs/:runsId", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/runs", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/runs/:runsId/submit_tool_outputs", admin.RelayNotImplemented)
		publicRelayRouter.POST("/threads/:id/runs/:runsId/cancel", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/runs/:runsId/steps/:stepId", admin.RelayNotImplemented)
		publicRelayRouter.GET("/threads/:id/runs/:runsId/steps", admin.RelayNotImplemented)
	}

	adminRouter := engine.Group("/api/v1/admin")
	adminRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	adminRouter.Use(middleware.GlobalAPIRateLimit())
	{
		adminUserRoute := adminRouter.Group("/user")
		adminUserRoute.Use(middleware.AdminAuth())
		{
			adminUserRoute.GET("/", user.GetAllUsers)
			adminUserRoute.GET("/search", user.SearchUsers)
			adminUserRoute.GET("/:id", user.GetUser)
			adminUserRoute.POST("/", user.CreateUser)
			adminUserRoute.POST("/manage", user.ManageUser)
			adminUserRoute.PUT("/", user.UpdateUser)
			adminUserRoute.DELETE("/:id", user.DeleteUser)
		}

		adminOptionRoute := adminRouter.Group("/option")
		adminOptionRoute.Use(middleware.RootAuth())
		{
			adminOptionRoute.GET("/", option.GetOptions)
			adminOptionRoute.PUT("/", option.UpdateOption)
		}

		adminChannelRoute := adminRouter.Group("/channel")
		adminChannelRoute.Use(middleware.AdminAuth())
		{
			adminChannelRoute.GET("/protocols", channel.GetChannelProtocols)
			adminChannelRoute.POST("/create", channel.CreateChannel)
			adminChannelRoute.GET("/:id", channel.GetChannel)
			adminChannelRoute.GET("/:id/models", channel.GetChannelModels)
			adminChannelRoute.GET("/:id/tests", channel.GetChannelTests)
			adminChannelRoute.POST("/:id/refresh", channel.RefreshChannelModels)
			adminChannelRoute.POST("/:id/tests", channel.TestChannelModels)
			adminChannelRoute.GET("/test", channel.TestChannels)
			adminChannelRoute.GET("/test/:id", channel.TestChannel)
			adminChannelRoute.GET("/update_balance", channel.UpdateAllChannelsBalance)
			adminChannelRoute.GET("/update_balance/:id", channel.UpdateChannelBalance)
			adminChannelRoute.POST("/", channel.AddChannel)
			adminChannelRoute.PUT("/", channel.UpdateChannel)
			adminChannelRoute.PUT("/test_model", channel.UpdateChannelTestModel)
			adminChannelRoute.DELETE("/disabled", channel.DeleteDisabledChannel)
			adminChannelRoute.DELETE("/:id", channel.DeleteChannel)
		}
		adminChannelsRoute := adminRouter.Group("/channels")
		adminChannelsRoute.Use(middleware.AdminAuth())
		{
			adminChannelsRoute.GET("/", channel.GetChannels)
		}
		adminGroupsRoute := adminRouter.Group("/groups")
		adminGroupsRoute.Use(middleware.AdminAuth())
		{
			adminGroupsRoute.GET("/", group.GetGroups)
		}

		adminRedemptionRoute := adminRouter.Group("/redemption")
		adminRedemptionRoute.Use(middleware.AdminAuth())
		{
			adminRedemptionRoute.GET("/", admin.GetAllRedemptions)
			adminRedemptionRoute.GET("/search", admin.SearchRedemptions)
			adminRedemptionRoute.GET("/:id", admin.GetRedemption)
			adminRedemptionRoute.POST("/", admin.AddRedemption)
			adminRedemptionRoute.PUT("/", admin.UpdateRedemption)
			adminRedemptionRoute.DELETE("/:id", admin.DeleteRedemption)
		}

		adminLogRoute := adminRouter.Group("/log")
		adminLogRoute.Use(middleware.AdminAuth())
		{
			adminLogRoute.GET("/", log.GetAllLogs)
			adminLogRoute.DELETE("/", log.DeleteHistoryLogs)
			adminLogRoute.GET("/stat", log.GetLogsStat)
			adminLogRoute.GET("/search", log.SearchAllLogs)
		}

		adminGroupRoute := adminRouter.Group("/group")
		adminGroupRoute.Use(middleware.AdminAuth())
		{
			adminGroupRoute.POST("/", group.CreateGroup)
			adminGroupRoute.PUT("/", group.UpdateGroup)
			adminGroupRoute.DELETE("/:id", group.DeleteGroup)
			adminGroupRoute.GET("/:id/channels", group.GetGroupChannels)
			adminGroupRoute.GET("/:id/models", group.GetGroupModels)
			adminGroupRoute.GET("/:id/model-configs", group.GetGroupModelConfigs)
			adminGroupRoute.PUT("/:id/channels", group.UpdateGroupChannels)
			adminGroupRoute.PUT("/:id/model-configs", group.UpdateGroupModelConfigs)
		}

		adminProviderRoute := adminRouter.Group("/providers")
		adminProviderRoute.Use(middleware.AdminAuth())
		{
			adminProviderRoute.GET("/", channel.GetProviders)
			adminProviderRoute.POST("/", channel.CreateProvider)
			adminProviderRoute.POST("/:id/model", channel.AppendProviderModel)
			adminProviderRoute.GET("/:id", channel.GetProvider)
			adminProviderRoute.PUT("/:id", channel.UpdateProvider)
			adminProviderRoute.DELETE("/:id", channel.DeleteProvider)
		}
	}

	internalRouter := engine.Group("/api/v1/internal")
	internalRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	internalRouter.Use(middleware.GlobalAPIRateLimit())
	{
		// reserved for future internal endpoints
	}
}
