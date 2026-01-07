package router

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/controller"
	auth "github.com/yeying-community/router/controller/auth"
	"github.com/yeying-community/router/middleware"
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

	apiRouter := engine.Group("/api")
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)

		apiRouter.GET("/verification", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), controller.ResetPassword)

		apiRouter.GET("/oauth/github", middleware.CriticalRateLimit(), auth.GitHubOAuth)
		apiRouter.GET("/oauth/github/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.GitHubBind)
		apiRouter.GET("/oauth/lark", middleware.CriticalRateLimit(), auth.LarkOAuth)
		apiRouter.GET("/oauth/lark/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.LarkBind)
		apiRouter.GET("/oauth/oidc", middleware.CriticalRateLimit(), auth.OidcAuth)
		apiRouter.GET("/oauth/oidc/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.OidcBind)
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), auth.WeChatAuth)
		apiRouter.GET("/oauth/wechat/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.WeChatBind)
		apiRouter.GET("/oauth/wallet/nonce", middleware.CriticalRateLimit(), auth.WalletNonce)
		apiRouter.POST("/oauth/wallet/login", middleware.CriticalRateLimit(), auth.WalletLogin)
		apiRouter.POST("/oauth/wallet/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), auth.WalletBind)
		apiRouter.GET("/oauth/email/bind", middleware.CriticalRateLimit(), middleware.UserAuth(), controller.EmailBind)
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), auth.GenerateOAuthCode)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), controller.Login)
			userRoute.GET("/logout", controller.Logout)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/dashboard", controller.GetUserDashboard)
				selfRoute.GET("/available_models", controller.GetUserAvailableModels)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.POST("/topup", controller.TopUp)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
			}
		}

		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
		}

		channelRoute := apiRouter.Group("/channel")
		{
			// 模型列表对所有登录用户开放，方便前端展示供应商/模型；其余仍需管理员权限
			channelRoute.GET("/models", middleware.UserAuth(), controller.DashboardListModels)

			adminChannel := channelRoute.Group("/")
			adminChannel.Use(middleware.AdminAuth())
			{
				adminChannel.GET("/", controller.GetAllChannels)
				adminChannel.GET("/search", controller.SearchChannels)
				adminChannel.GET("/:id", controller.GetChannel)
				adminChannel.GET("/test", controller.TestChannels)
				adminChannel.GET("/test/:id", controller.TestChannel)
				adminChannel.GET("/update_balance", controller.UpdateAllChannelsBalance)
				adminChannel.GET("/update_balance/:id", controller.UpdateChannelBalance)
				adminChannel.POST("/", controller.AddChannel)
				adminChannel.PUT("/", controller.UpdateChannel)
				adminChannel.DELETE("/disabled", controller.DeleteDisabledChannel)
				adminChannel.DELETE("/:id", controller.DeleteChannel)
			}
		}

		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", controller.SearchTokens)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}

		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), controller.SearchUserLogs)

		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		// Models list for authenticated users
		apiRouter.GET("/models", middleware.UserAuth(), controller.ListAllModels)
	}
}
