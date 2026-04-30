package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/blacklist"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/network"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
)

func computeEffectiveAuthRole(user *model.User) (int, bool) {
	if user == nil {
		return model.RoleGuestUser, false
	}
	return model.EffectiveRole(user), model.CanManageUsers(user)
}

func hydrateVideoTaskRelayContext(c *gin.Context, requestModel string) (string, error) {
	path := normalizeRelayPath(c.Request.URL.Path)
	if !strings.HasPrefix(path, "/v1/videos/") || c.Request.Method != http.MethodGet {
		return requestModel, nil
	}
	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		return requestModel, nil
	}
	taskRow, err := model.GetUserTaskByTaskIDWithDB(model.DB, taskID)
	if err != nil {
		if requestModel != "" {
			return requestModel, nil
		}
		return "", err
	}
	if requestModel == "" {
		requestModel = taskRow.Model
	}
	if c.GetString(ctxkey.SpecificChannelId) == "" && strings.TrimSpace(taskRow.ChannelID) != "" {
		c.Set(ctxkey.SpecificChannelId, strings.TrimSpace(taskRow.ChannelID))
	}
	return requestModel, nil
}

func authHelper(c *gin.Context, minRole int) {
	session := sessions.Default(c)
	username := session.Get("username")
	role := session.Get("role")
	id := session.Get("id")
	status := session.Get("status")
	if username == nil {
		// Check access token
		authHeader := strings.TrimSpace(c.Request.Header.Get("Authorization"))
		if authHeader == "" {
			logger.Loginf(c.Request.Context(), "auth failed: no session and no Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，未登录且未提供 access token",
			})
			c.Abort()
			return
		}
		bearer := authHeader
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			bearer = strings.TrimSpace(authHeader[7:])
		}
		logger.Loginf(c.Request.Context(), "auth header parsed raw_len=%d bearer_len=%d", len(authHeader), len(bearer))

		// Try wallet JWT first
		if bearer != "" {
			if claims, err := common.VerifyWalletJWT(bearer); err == nil {
				logger.Loginf(c.Request.Context(), "auth wallet jwt verified uid=%s addr=%s", claims.UserID, claims.WalletAddress)
				user := model.User{Id: claims.UserID}
				foundById := false
				if strings.TrimSpace(claims.UserID) != "" {
					if err := user.FillUserById(); err == nil {
						foundById = true
					} else {
						logger.Loginf(c.Request.Context(), "auth wallet jwt FillUserById fail uid=%s err=%v", claims.UserID, err)
					}
				}

				if !foundById && claims.WalletAddress != "" {
					addr := strings.ToLower(claims.WalletAddress)
					user = model.User{WalletAddress: &addr}
					if err := user.FillUserByWalletAddress(); err == nil {
						logger.Loginf(c.Request.Context(), "auth wallet jwt fallback by address success addr=%s uid=%s", claims.WalletAddress, user.Id)
						foundById = true
					} else {
						logger.Loginf(c.Request.Context(), "auth wallet jwt fallback by address fail addr=%s err=%v", claims.WalletAddress, err)
					}
				}

				if foundById {
					matched := user.WalletAddress != nil && strings.ToLower(*user.WalletAddress) == strings.ToLower(claims.WalletAddress)
					enabled := user.Status == model.UserStatusEnabled
					notBanned := !blacklist.IsUserBanned(user.Id)
					if matched && enabled && notBanned {
						effectiveRole, _ := computeEffectiveAuthRole(&user)
						username = user.Username
						role = effectiveRole
						id = user.Id
						status = user.Status
						logger.Loginf(c.Request.Context(), "auth via wallet jwt success user=%s addr=%s", user.Id, claims.WalletAddress)
					} else {
						logger.Loginf(c.Request.Context(), "auth wallet jwt reject uid=%s matched=%t enabled=%t notBanned=%t db_addr=%v token_addr=%s status=%d", user.Id, matched, enabled, notBanned, user.WalletAddress, claims.WalletAddress, user.Status)
					}
				}
			} else {
				logger.Loginf(c.Request.Context(), "auth wallet jwt verify failed err=%v token_len=%d", err, len(bearer))
			}
		}

		if username == nil {
			user := model.ValidateAccessToken(bearer)
			if user != nil && user.Username != "" {
				// Token is valid
				effectiveRole, _ := computeEffectiveAuthRole(user)
				username = user.Username
				role = effectiveRole
				id = user.Id
				status = user.Status
				logger.Loginf(c.Request.Context(), "auth via access token success user=%s", user.Id)
			} else {
				logger.Loginf(c.Request.Context(), "auth failed: invalid access token")
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "无权进行此操作，access token 无效",
				})
				c.Abort()
				return
			}
		}
	}
	userID := normalizeSessionUserID(id)
	if userID != "" {
		if freshUser, err := model.GetUserById(userID, false); err == nil && freshUser != nil {
			effectiveRole, canManageUsers := computeEffectiveAuthRole(freshUser)
			username = freshUser.Username
			role = effectiveRole
			status = freshUser.Status
			c.Set(ctxkey.CanManageUsers, canManageUsers)
		} else {
			c.Set(ctxkey.CanManageUsers, false)
		}
	} else {
		c.Set(ctxkey.CanManageUsers, false)
	}
	if status.(int) == model.UserStatusDisabled || blacklist.IsUserBanned(userID) {
		logger.Loginf(c.Request.Context(), "auth failed: user banned/disabled id=%s", userID)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被封禁",
		})
		session := sessions.Default(c)
		session.Clear()
		_ = session.Save()
		c.Abort()
		return
	}
	if role.(int) < minRole {
		logger.Loginf(c.Request.Context(), "auth failed: role too low id=%s role=%d need=%d", userID, role.(int), minRole)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权进行此操作，权限不足",
		})
		c.Abort()
		return
	}
	c.Set("username", username)
	c.Set("role", role)
	c.Set("id", userID)
	c.Next()
}

func UserAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, model.RoleCommonUser)
	}
}

func AdminAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, model.RoleAdminUser)
	}
}

func RootAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, model.RoleRootUser)
	}
}

func TokenAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		rawAuth := strings.TrimSpace(c.GetHeader("Authorization"))
		if rawAuth == "" {
			abortWithMessage(c, http.StatusUnauthorized, "未提供令牌")
			return
		}
		auth := rawAuth
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			auth = strings.TrimSpace(auth[7:])
		}

		// 1) 尝试钱包 JWT
		if claims, err := common.VerifyWalletJWT(auth); err == nil {
			user := model.User{Id: claims.UserID}
			found := false
			if strings.TrimSpace(claims.UserID) != "" {
				if err := user.FillUserById(); err == nil {
					found = true
				} else {
					logger.Loginf(ctx, "token auth wallet jwt FillUserById fail uid=%s err=%v", claims.UserID, err)
				}
			}
			if !found && claims.WalletAddress != "" {
				addr := strings.ToLower(claims.WalletAddress)
				user = model.User{WalletAddress: &addr}
				if err := user.FillUserByWalletAddress(); err == nil {
					found = true
					logger.Loginf(ctx, "token auth wallet jwt fallback by address success addr=%s uid=%s", claims.WalletAddress, user.Id)
				} else {
					logger.Loginf(ctx, "token auth wallet jwt fallback by address fail addr=%s err=%v", claims.WalletAddress, err)
				}
			}
			if !found {
				abortWithMessage(c, http.StatusUnauthorized, "token 对应的用户不存在")
				return
			}
			if user.Status != model.UserStatusEnabled || blacklist.IsUserBanned(user.Id) {
				logger.Loginf(ctx, "token auth wallet jwt banned/disabled uid=%s status=%d", user.Id, user.Status)
				abortWithMessage(c, http.StatusForbidden, "用户已被封禁")
				return
			}
			requestModel, err := getRequestModel(c)
			if err != nil && shouldCheckModel(c) {
				abortWithMessage(c, http.StatusBadRequest, err.Error())
				return
			}
			requestModel, err = hydrateVideoTaskRelayContext(c, requestModel)
			if err != nil && shouldCheckModel(c) {
				abortWithMessage(c, http.StatusBadRequest, err.Error())
				return
			}
			c.Set(ctxkey.RequestModel, requestModel)
			c.Set(ctxkey.Id, user.Id)

			// 自动选择该用户的第一个可用 sk 作为默认 key（便于 JWT 直连）
			if token, terr := model.GetFirstAvailableToken(user.Id); terr == nil {
				// subnet 检查
				if token.Subnet != nil && *token.Subnet != "" {
					if !network.IsIpInSubnets(ctx, c.ClientIP(), *token.Subnet) {
						logger.Loginf(ctx, "token auth wallet jwt subnet deny user=%s ip=%s subnet=%s", token.UserId, c.ClientIP(), *token.Subnet)
						abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌只能在指定网段使用：%s，当前 ip：%s", *token.Subnet, c.ClientIP()))
						return
					}
				}
				if token.Models != nil && *token.Models != "" {
					c.Set(ctxkey.AvailableModels, *token.Models)
					if requestModel != "" && !isModelInList(requestModel, *token.Models) {
						abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌无权使用模型：%s", requestModel))
						return
					}
				}
				c.Set(ctxkey.TokenId, token.Id)
				c.Set(ctxkey.TokenName, token.Name)
				logger.Loginf(ctx, "token auth via wallet jwt success user=%s addr=%s use_token=%s", user.Id, claims.WalletAddress, token.Id)
			} else {
				c.Set(ctxkey.TokenId, "")
				c.Set(ctxkey.TokenName, "wallet_jwt")
				logger.Loginf(ctx, "token auth via wallet jwt success user=%s addr=%s no_token_found", user.Id, claims.WalletAddress)
			}
			c.Next()
			return
		}

		// 2) UCAN
		if common.IsUcanToken(auth) {
			requiredSets := common.ResolveUcanRequiredCapabilitySets()
			address, err := common.VerifyUcanInvocationAny(auth, common.ResolveUcanAudience(), requiredSets)
			if err != nil {
				logger.Loginf(ctx, "token auth ucan verify failed err=%v", err)
				abortWithMessage(c, http.StatusUnauthorized, err.Error())
				return
			}
			addr := strings.ToLower(address)
			user, err := findOrCreateWalletUser(addr, ctx)
			if err != nil {
				logger.Loginf(ctx, "token auth ucan resolve user failed addr=%s err=%v", addr, err)
				abortWithMessage(c, http.StatusUnauthorized, err.Error())
				return
			}
			if user.Status != model.UserStatusEnabled || blacklist.IsUserBanned(user.Id) {
				logger.Loginf(ctx, "token auth ucan banned/disabled uid=%s status=%d", user.Id, user.Status)
				abortWithMessage(c, http.StatusForbidden, "用户已被封禁")
				return
			}
			requestModel, err := getRequestModel(c)
			if err != nil && shouldCheckModel(c) {
				abortWithMessage(c, http.StatusBadRequest, err.Error())
				return
			}
			requestModel, err = hydrateVideoTaskRelayContext(c, requestModel)
			if err != nil && shouldCheckModel(c) {
				abortWithMessage(c, http.StatusBadRequest, err.Error())
				return
			}
			c.Set(ctxkey.RequestModel, requestModel)
			c.Set(ctxkey.Id, user.Id)

			if token, terr := model.GetFirstAvailableToken(user.Id); terr == nil {
				if token.Subnet != nil && *token.Subnet != "" {
					if !network.IsIpInSubnets(ctx, c.ClientIP(), *token.Subnet) {
						logger.Loginf(ctx, "token auth ucan subnet deny user=%s ip=%s subnet=%s", token.UserId, c.ClientIP(), *token.Subnet)
						abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌只能在指定网段使用：%s，当前 ip：%s", *token.Subnet, c.ClientIP()))
						return
					}
				}
				if token.Models != nil && *token.Models != "" {
					c.Set(ctxkey.AvailableModels, *token.Models)
					if requestModel != "" && !isModelInList(requestModel, *token.Models) {
						abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌无权使用模型：%s", requestModel))
						return
					}
				}
				c.Set(ctxkey.TokenId, token.Id)
				c.Set(ctxkey.TokenName, token.Name)
				logger.Loginf(ctx, "token auth via ucan success user=%s addr=%s use_token=%s", user.Id, addr, token.Id)
			} else {
				c.Set(ctxkey.TokenId, "")
				c.Set(ctxkey.TokenName, "ucan")
				logger.Loginf(ctx, "token auth via ucan success user=%s addr=%s no_token_found", user.Id, addr)
			}
			c.Next()
			return
		}

		// 3) 回退到 sk- 令牌
		key := auth
		key = strings.TrimPrefix(key, "sk-")
		parts := strings.Split(key, "-")
		key = parts[0]
		token, err := model.ValidateUserToken(key)
		if token != nil {
			c.Set(ctxkey.Id, token.UserId)
			c.Set(ctxkey.TokenId, token.Id)
			c.Set(ctxkey.TokenName, token.Name)
		}
		if err != nil {
			logger.Loginf(c.Request.Context(), "token auth failed: %v", err)
			abortWithMessage(c, http.StatusUnauthorized, err.Error())
			return
		}
		if token.Subnet != nil && *token.Subnet != "" {
			if !network.IsIpInSubnets(ctx, c.ClientIP(), *token.Subnet) {
				logger.Loginf(c.Request.Context(), "token auth subnet deny user=%s ip=%s subnet=%s", token.UserId, c.ClientIP(), *token.Subnet)
				abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌只能在指定网段使用：%s，当前 ip：%s", *token.Subnet, c.ClientIP()))
				return
			}
		}
		userEnabled, err := model.CacheIsUserEnabled(token.UserId)
		if err != nil {
			abortWithMessage(c, http.StatusInternalServerError, err.Error())
			return
		}
		if !userEnabled || blacklist.IsUserBanned(token.UserId) {
			logger.Loginf(c.Request.Context(), "token auth banned user=%s", token.UserId)
			abortWithMessage(c, http.StatusForbidden, "用户已被封禁")
			return
		}
		requestModel, err := getRequestModel(c)
		if err != nil && shouldCheckModel(c) {
			abortWithMessage(c, http.StatusBadRequest, err.Error())
			return
		}
		requestModel, err = hydrateVideoTaskRelayContext(c, requestModel)
		if err != nil && shouldCheckModel(c) {
			abortWithMessage(c, http.StatusBadRequest, err.Error())
			return
		}
		c.Set(ctxkey.RequestModel, requestModel)
		if token.Models != nil && *token.Models != "" {
			c.Set(ctxkey.AvailableModels, *token.Models)
			if requestModel != "" && !isModelInList(requestModel, *token.Models) {
				abortWithMessage(c, http.StatusForbidden, fmt.Sprintf("该令牌无权使用模型：%s", requestModel))
				return
			}
		}
		if len(parts) > 1 {
			if model.IsAdmin(token.UserId) {
				c.Set(ctxkey.SpecificChannelId, parts[1])
			} else {
				logger.Loginf(c.Request.Context(), "token auth reject specific channel user=%s", token.UserId)
				abortWithMessage(c, http.StatusForbidden, "普通用户不支持指定渠道")
				return
			}
		}

		// set channel id for proxy relay
		if channelId := c.Param("channelid"); channelId != "" {
			c.Set(ctxkey.SpecificChannelId, channelId)
		}

		logger.Debugf(c.Request.Context(), "[login] token auth success user=%s tokenId=%s", token.UserId, token.Id)

		c.Next()
	}
}

func findOrCreateWalletUser(addr string, ctx context.Context) (*model.User, error) {
	user := model.User{WalletAddress: &addr}
	if !model.IsWalletAddressAlreadyTaken(addr) {
		if config.AutoRegisterEnabled {
			return autoCreateWalletUser(addr, ctx)
		}
		return nil, errors.New("未找到钱包绑定的账户，请先绑定或由管理员开启自动注册")
	}

	if err := user.FillUserByWalletAddress(); err != nil {
		return nil, err
	}
	if user.Status == model.UserStatusDeleted {
		_ = model.DB.Model(&user).Update("wallet_address", nil)
		return findOrCreateWalletUser(addr, ctx)
	}
	return &user, nil
}

func autoCreateWalletUser(addr string, ctx context.Context) (*model.User, error) {
	username := "wallet_" + random.GetRandomString(6)
	for model.IsUsernameAlreadyTaken(username) {
		username = "wallet_" + random.GetRandomString(6)
	}
	user := model.User{
		Username:      username,
		Password:      random.GetRandomString(16),
		DisplayName:   username,
		Role:          model.RoleCommonUser,
		Status:        model.UserStatusEnabled,
		WalletAddress: &addr,
		HasPassword:   false,
	}
	if err := user.Insert(ctx, ""); err != nil {
		return nil, err
	}
	return &user, nil
}

func normalizeSessionUserID(id interface{}) string {
	switch v := id.(type) {
	case string:
		return strings.TrimSpace(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

func shouldCheckModel(c *gin.Context) bool {
	path := normalizeRelayPath(c.Request.URL.Path)
	if strings.HasPrefix(path, "/v1/completions") {
		return true
	}
	if strings.HasPrefix(path, "/v1/chat/completions") {
		return true
	}
	if strings.HasPrefix(path, "/v1/images") {
		return true
	}
	if strings.HasPrefix(path, "/v1/audio") {
		return true
	}
	if strings.HasPrefix(path, "/v1/videos") {
		return true
	}
	return false
}
