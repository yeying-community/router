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

		// Legacy /api/models mirror (avoid conflict with OpenAI-compatible /api/v1/public/models)
		publicRouter.GET("/models-all", middleware.UserAuth(), admin.ListAllModels)
	}

	publicModelsRouter := engine.Group("/api/v1/public/models")
	publicModelsRouter.Use(middleware.TokenAuth())
	{
		publicModelsRouter.GET("", admin.ListModels)
		publicModelsRouter.GET("/:model", admin.RetrieveModel)
	}

	publicRelayRouter := engine.Group("/api/v1/public")
	publicRelayRouter.Use(middleware.TokenAuth(), middleware.Distribute())
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

	apiRouter := engine.Group("/api")
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/status", admin.GetStatus)
		apiRouter.GET("/notice", admin.GetNotice)
		apiRouter.GET("/about", admin.GetAbout)
		apiRouter.GET("/home_page_content", admin.GetHomePageContent)

		// 仅保留密码找回，无额外人机验证
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), admin.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), admin.ResetPassword)

		apiRouter.GET("/oauth/wallet/nonce", middleware.CriticalRateLimit(), auth.WalletNonce)
		apiRouter.POST("/oauth/wallet/login", middleware.CriticalRateLimit(), auth.WalletLogin)
		apiRouter.POST("/oauth/wallet/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.WalletBind)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), user.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), user.Login)
			userRoute.GET("/logout", user.Logout)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self", user.GetSelf)
				selfRoute.GET("/dashboard", user.GetUserDashboard)
				selfRoute.GET("/available_models", admin.GetUserAvailableModels)
				selfRoute.PUT("/self", user.UpdateSelf)
				selfRoute.DELETE("/self", user.DeleteSelf)
				selfRoute.GET("/token", user.GenerateAccessToken)
				selfRoute.GET("/aff", user.GetAffCode)
				selfRoute.POST("/topup", user.TopUp)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", user.GetAllUsers)
				adminRoute.GET("/search", user.SearchUsers)
				adminRoute.GET("/:id", user.GetUser)
				adminRoute.POST("/", user.CreateUser)
				adminRoute.POST("/manage", user.ManageUser)
				adminRoute.PUT("/", user.UpdateUser)
				adminRoute.DELETE("/:id", user.DeleteUser)
			}
		}

		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", option.GetOptions)
			optionRoute.PUT("/", option.UpdateOption)
		}

		channelRoute := apiRouter.Group("/channel")
		{
			// 模型列表对所有登录用户开放，方便前端展示供应商/模型；其余仍需管理员权限
			channelRoute.GET("/models", middleware.UserAuth(), admin.DashboardListModels)

			adminChannel := channelRoute.Group("/")
			adminChannel.Use(middleware.AdminAuth())
			{
				adminChannel.GET("/", channel.GetAllChannels)
				adminChannel.GET("/search", channel.SearchChannels)
				adminChannel.GET("/:id", channel.GetChannel)
				adminChannel.GET("/test", channel.TestChannels)
				adminChannel.GET("/test/:id", channel.TestChannel)
				adminChannel.GET("/update_balance", channel.UpdateAllChannelsBalance)
				adminChannel.GET("/update_balance/:id", channel.UpdateChannelBalance)
				adminChannel.POST("/", channel.AddChannel)
				adminChannel.PUT("/", channel.UpdateChannel)
				adminChannel.DELETE("/disabled", channel.DeleteDisabledChannel)
				adminChannel.DELETE("/:id", channel.DeleteChannel)
			}
		}

		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", token.GetAllTokens)
			tokenRoute.GET("/search", token.SearchTokens)
			tokenRoute.GET("/:id", token.GetToken)
			tokenRoute.POST("/", token.AddToken)
			tokenRoute.PUT("/", token.UpdateToken)
			tokenRoute.DELETE("/:id", token.DeleteToken)
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", admin.GetAllRedemptions)
			redemptionRoute.GET("/search", admin.SearchRedemptions)
			redemptionRoute.GET("/:id", admin.GetRedemption)
			redemptionRoute.POST("/", admin.AddRedemption)
			redemptionRoute.PUT("/", admin.UpdateRedemption)
			redemptionRoute.DELETE("/:id", admin.DeleteRedemption)
		}

		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), log.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), log.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), log.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), log.GetLogsSelfStat)
		logRoute.GET("/search", middleware.AdminAuth(), log.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), log.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), log.SearchUserLogs)

		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", group.GetGroups)
		}

		// Models list for authenticated users
		apiRouter.GET("/models", middleware.UserAuth(), admin.ListAllModels)
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
			adminChannelRoute.GET("/", channel.GetAllChannels)
			adminChannelRoute.GET("/search", channel.SearchChannels)
			adminChannelRoute.GET("/:id", channel.GetChannel)
			adminChannelRoute.GET("/test", channel.TestChannels)
			adminChannelRoute.GET("/test/:id", channel.TestChannel)
			adminChannelRoute.GET("/update_balance", channel.UpdateAllChannelsBalance)
			adminChannelRoute.GET("/update_balance/:id", channel.UpdateChannelBalance)
			adminChannelRoute.POST("/preview/models", channel.PreviewChannelModels)
			adminChannelRoute.POST("/", channel.AddChannel)
			adminChannelRoute.PUT("/", channel.UpdateChannel)
			adminChannelRoute.DELETE("/disabled", channel.DeleteDisabledChannel)
			adminChannelRoute.DELETE("/:id", channel.DeleteChannel)
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
			adminGroupRoute.GET("/", group.GetGroups)
		}
	}

	internalRouter := engine.Group("/api/v1/internal")
	internalRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	internalRouter.Use(middleware.GlobalAPIRateLimit())
	{
		// reserved for future internal endpoints
	}
}
