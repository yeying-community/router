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
}
