package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
	usercontroller "github.com/yeying-community/router/internal/admin/controller/user"
	"github.com/yeying-community/router/internal/admin/model"
)

type walletNonceRequest struct {
	Address string `form:"address" json:"address" binding:"required"`
	ChainId string `form:"chain_id" json:"chain_id"`
}

type walletLoginRequest struct {
	Address   string `json:"address"`
	Signature string `json:"signature"`
	Nonce     string `json:"nonce"`
	ChainId   string `json:"chain_id"`
	Message   string `json:"message"`
}

const walletRefreshCookieName = "refresh_token"

// WalletNonce godoc
// @Summary Get wallet nonce
// @Tags public
// @Produce json
// @Param address query string true "Wallet address"
// @Param chain_id query string false "Chain ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/oauth/wallet/nonce [get]
// WalletNonce issues a nonce & message to sign
func WalletNonce(c *gin.Context) {
	var req walletNonceRequest
	if err := c.ShouldBind(&req); err != nil || !common.IsValidEthAddress(req.Address) {
		logger.Loginf(c.Request.Context(), "wallet nonce invalid param addr=%s err=%v", req.Address, err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误，缺少 address",
		})
		return
	}

	nonce, message := common.GenerateWalletNonce(req.Address, "Login to "+config.SystemName, req.ChainId)
	logger.Loginf(c.Request.Context(), "wallet nonce generated addr=%s chain=%s nonce=%s", strings.ToLower(req.Address), req.ChainId, nonce)
	expireAt := time.Now().Add(time.Duration(config.WalletNonceTTLMinutes) * time.Minute)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"nonce":      nonce,
			"message":    message,
			"expires_at": expireAt.UTC().Format(time.RFC3339),
		},
	})
}

// WalletLogin godoc
// @Summary Wallet login (returns JWT)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.WalletLoginRequest true "Wallet login payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/oauth/wallet/login [post]
// WalletLogin verifies signature and logs user in
func WalletLogin(c *gin.Context) {
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Loginf(c.Request.Context(), "wallet login bind json failed err=%v", err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}

	user, err := walletAuthenticate(c, req)
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet login authenticate failed addr=%s err=%v", strings.ToLower(req.Address), err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := usercontroller.SetupSession(user, c); err != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet login setup session failed user=%d err=%v", user.Id, err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法保存会话信息，请重试",
		})
		return
	}
	addr := ""
	if user.WalletAddress != nil {
		addr = strings.ToLower(*user.WalletAddress)
	}
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet jwt generate failed user=%d err=%v", user.Id, tokenErr)
	}
	logger.Loginf(c.Request.Context(), "wallet login success user=%d addr=%s role=%d token=%t exp=%s", user.Id, addr, user.Role, token != "", exp.UTC().Format(time.RFC3339))
	cleanUser := model.User{
		Id:            user.Id,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		Role:          user.Role,
		Status:        user.Status,
		WalletAddress: user.WalletAddress,
	}
	resp := gin.H{
		"message": "",
		"success": true,
		"data":    cleanUser,
	}
	if token != "" {
		resp["token"] = token
		resp["token_expires_at"] = exp.UTC().Format(time.RFC3339)
	}
	common.ConsumeWalletNonce(strings.ToLower(req.Address))
	c.JSON(http.StatusOK, resp)
}

// WalletBind godoc
// @Summary Bind wallet to current user
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.WalletLoginRequest true "Wallet bind payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/oauth/wallet/bind [post]
// WalletBind binds a wallet to logged-in user
func WalletBind(c *gin.Context) {
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	if err := verifyWalletRequest(req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	addr := strings.ToLower(req.Address)
	session := sessions.Default(c)
	id := session.Get("id")
	if id == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "未登录",
		})
		return
	}
	user := model.User{Id: id.(int)}
	if err := user.FillUserById(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if model.IsWalletAddressAlreadyTaken(addr) {
		exist := model.User{WalletAddress: &addr}
		if err := exist.FillUserByWalletAddress(); err == nil {
			if exist.Status == model.UserStatusDeleted {
				_ = model.DB.Model(&exist).Update("wallet_address", nil)
			} else if exist.Id != user.Id && (user.WalletAddress == nil || strings.ToLower(*user.WalletAddress) != addr) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "该钱包已绑定其他账户",
				})
				return
			}
		}
	}
	user.WalletAddress = &addr
	if err := user.Update(false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	common.ConsumeWalletNonce(addr)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "绑定成功",
	})
}

func verifyWalletRequest(req walletLoginRequest) error {
	if !common.IsValidEthAddress(req.Address) {
		err := errors.New("无效的钱包地址")
		logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err)
		return err
	}
	if req.Signature == "" {
		err := errors.New("缺少签名或 nonce")
		logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err)
		return err
	}
	entry, ok := common.GetWalletNonce(req.Address)
	if !ok {
		err := errors.New("nonce 无效或已过期")
		logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err)
		return err
	}
	if req.Nonce != "" && entry.Nonce != req.Nonce {
		err := errors.New("nonce 无效或已过期")
		logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err)
		return err
	}

	message := entry.Message
	if strings.TrimSpace(req.Message) != "" {
		message = req.Message
		nonce := extractNonceFromMessage(message)
		if nonce == "" || nonce != entry.Nonce {
			err := errors.New("nonce 无效或已过期")
			logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err)
			return err
		}
	}

	// verify signature
	recovered, err := recoverAddress(message, req.Signature)
	if err != nil {
		logger.SysError("wallet login verify failed: " + err.Error())
		err2 := errors.New("签名验证失败")
		logger.Loginf(nil, "wallet verify fail addr=%s err=%v", req.Address, err2)
		return err2
	}
	if strings.ToLower(recovered) != strings.ToLower(req.Address) {
		err := errors.New("签名地址与请求地址不一致")
		logger.Loginf(nil, "wallet verify fail addr=%s recovered=%s err=%v", req.Address, recovered, err)
		return err
	}
	return nil
}

func extractNonceFromMessage(message string) string {
	for _, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "nonce:") {
			return strings.TrimSpace(trimmed[len("nonce:"):])
		}
	}
	return ""
}

// walletAuthenticate verifies signature & returns an enabled user (create if allowed)
func walletAuthenticate(c *gin.Context, req walletLoginRequest) (*model.User, error) {
	if err := verifyWalletRequest(req); err != nil {
		return nil, err
	}
	addr := strings.ToLower(req.Address)
	user, err := findOrCreateWalletUser(addr, c.Request.Context())
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet auth find/create failed addr=%s err=%v", addr, err)
		return nil, err
	}
	if user.Status != model.UserStatusEnabled {
		err := errors.New("用户已被封禁")
		logger.Loginf(c.Request.Context(), "wallet auth user disabled addr=%s err=%v", addr, err)
		return nil, err
	}
	common.ConsumeWalletNonce(addr)
	logger.Loginf(c.Request.Context(), "wallet auth success user=%d addr=%s", user.Id, addr)
	return user, nil
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
	}
	if err := user.Insert(ctx, 0); err != nil {
		return nil, err
	}
	return &user, nil
}

func recoverAddress(message, signature string) (string, error) {
	sig := strings.TrimPrefix(signature, "0x")
	raw, err := hex.DecodeString(sig)
	if err != nil {
		return "", err
	}
	if len(raw) != 65 {
		return "", errors.New("签名长度异常")
	}
	// fix v value
	if raw[64] >= 27 {
		raw[64] -= 27
	}
	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, raw)
	if err != nil {
		return "", err
	}
	addr := crypto.PubkeyToAddress(*pub)
	return strings.ToLower(addr.Hex()), nil
}

// --- proto-aligned handlers ---

// WalletChallengeProto godoc
// @Summary Wallet challenge (proto)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.WalletChallengeRequest true "Challenge payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/common/auth/challenge [post]
// WalletChallengeProto implements /api/v1/public/common/auth/challenge
func WalletChallengeProto(c *gin.Context) {
	var req walletNonceRequest
	if err := c.ShouldBindJSON(&req); err != nil || !common.IsValidEthAddress(req.Address) {
		logger.Loginf(c.Request.Context(), "wallet proto challenge bind fail addr=%s err=%v", req.Address, err)
		writeProtoError(c, 2, "参数错误，缺少 address")
		return
	}
	addr := strings.ToLower(req.Address)
	if !model.IsWalletAddressAlreadyTaken(addr) && !config.AutoRegisterEnabled {
		logger.Loginf(c.Request.Context(), "wallet proto challenge reject addr=%s not bound and auto-register disabled", addr)
		writeProtoError(c, 5, "钱包未绑定账户，请先绑定或由管理员开启自动注册")
		return
	}
	nonce, message := common.GenerateWalletNonce(addr, "Login to "+config.SystemName, req.ChainId)
	logger.Loginf(c.Request.Context(), "wallet proto challenge success addr=%s nonce=%s chain=%s", addr, nonce, req.ChainId)
	expireAt := time.Now().Add(time.Duration(config.WalletNonceTTLMinutes) * time.Minute)
	body := gin.H{
		"status":     protoStatus(1, "OK"),
		"nonce":      nonce,
		"message":    message,
		"address":    addr,
		"expires_at": expireAt.UTC().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, gin.H{
		"body":    body,
		"success": true,
		"message": "",
		"data": gin.H{ // 兼容旧格式
			"nonce":      nonce,
			"message":    message,
			"expires_at": expireAt.UTC().Format(time.RFC3339),
		},
	})
}

// WalletVerifyProto godoc
// @Summary Wallet verify (proto)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.WalletLoginRequest true "Verify payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/common/auth/verify [post]
// WalletVerifyProto implements /api/v1/public/common/auth/verify
func WalletVerifyProto(c *gin.Context) {
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Loginf(c.Request.Context(), "wallet proto verify bind fail err=%v", err)
		writeProtoError(c, 2, "参数错误")
		return
	}
	user, err := walletAuthenticate(c, req)
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet proto verify auth fail addr=%s err=%v", req.Address, err)
		writeProtoError(c, 3, err.Error())
		return
	}
	if err := usercontroller.SetupSession(user, c); err != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet proto verify setup session fail user=%d err=%v", user.Id, err)
		writeProtoError(c, 8, "无法保存会话信息，请重试")
		return
	}
	addr := ""
	if user.WalletAddress != nil {
		addr = strings.ToLower(*user.WalletAddress)
	}
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		logger.SysError("wallet jwt generate failed: " + tokenErr.Error())
		writeProtoError(c, 8, "生成 token 失败")
		return
	}
	logger.Loginf(c.Request.Context(), "wallet proto verify success user=%d addr=%s token_exp=%s", user.Id, addr, exp.UTC().Format(time.RFC3339))
	body := gin.H{
		"status":     protoStatus(1, "OK"),
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"user": gin.H{
			"id":             user.Id,
			"username":       user.Username,
			"wallet_address": user.WalletAddress,
			"role":           user.Role,
			"status":         user.Status,
		},
	}
	c.JSON(http.StatusOK, gin.H{
		"body":    body,
		"success": true,
		"message": "",
		"data":    body, // 兼容旧风格
	})
}

// WalletRefreshToken godoc
// @Summary Refresh token (proto)
// @Tags public
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/common/auth/refreshToken [post]
// WalletRefreshToken implements /api/v1/public/common/auth/refreshToken
func WalletRefreshToken(c *gin.Context) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}
	if authHeader == "" {
		logger.Loginf(c.Request.Context(), "wallet refresh missing token")
		writeProtoError(c, 3, "缺少 token")
		return
	}
	claims, err := common.VerifyWalletJWT(authHeader)
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet refresh verify failed err=%v", err)
		writeProtoError(c, 3, "token 无效或已过期")
		return
	}
	user := model.User{Id: claims.UserID}
	if err := user.FillUserById(); err != nil {
		logger.Loginf(c.Request.Context(), "wallet refresh user not found id=%d", claims.UserID)
		writeProtoError(c, 5, "用户不存在")
		return
	}
	userAddr := ""
	if user.WalletAddress != nil {
		userAddr = strings.ToLower(*user.WalletAddress)
	}
	if user.WalletAddress == nil || userAddr != strings.ToLower(claims.WalletAddress) {
		logger.Loginf(c.Request.Context(), "wallet refresh addr mismatch token=%s user=%s", claims.WalletAddress, userAddr)
		writeProtoError(c, 3, "钱包地址不匹配")
		return
	}
	if user.Status != model.UserStatusEnabled {
		logger.Loginf(c.Request.Context(), "wallet refresh user disabled id=%d", user.Id)
		writeProtoError(c, 4, "用户已被封禁")
		return
	}
	if err := usercontroller.SetupSession(&user, c); err != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet refresh setup session failed user=%d err=%v", user.Id, err)
		writeProtoError(c, 8, "无法保存会话信息，请重试")
		return
	}
	addr := strings.ToLower(*user.WalletAddress)
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet refresh generate token failed user=%d err=%v", user.Id, tokenErr)
		writeProtoError(c, 8, "生成 token 失败")
		return
	}
	logger.Loginf(c.Request.Context(), "wallet refresh success user=%d addr=%s exp=%s", user.Id, addr, exp.UTC().Format(time.RFC3339))
	body := gin.H{
		"status":     protoStatus(1, "OK"),
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, gin.H{
		"body":    body,
		"success": true,
		"message": "",
		"data":    body,
	})
}

func protoStatus(code int, message string) gin.H {
	return gin.H{
		"code":    code,
		"message": message,
	}
}

func writeProtoError(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{
		"body": gin.H{
			"status": protoStatus(code, message),
		},
		"success": false,
		"message": message,
	})
}

// --- web3 README-aligned handlers ---

// WalletChallengeWeb3 godoc
// @Summary Wallet challenge (web3)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.WalletChallengeRequest true "Challenge payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/auth/challenge [post]
// WalletChallengeWeb3 implements /api/v1/public/auth/challenge
func WalletChallengeWeb3(c *gin.Context) {
	var req walletNonceRequest
	if err := c.ShouldBindJSON(&req); err != nil || !common.IsValidEthAddress(req.Address) {
		logger.Loginf(c.Request.Context(), "wallet web3 challenge bind fail addr=%s err=%v", req.Address, err)
		writeWeb3Error(c, 2, "参数错误，缺少 address")
		return
	}
	addr := strings.ToLower(req.Address)
	if !model.IsWalletAddressAlreadyTaken(addr) && !config.AutoRegisterEnabled {
		logger.Loginf(c.Request.Context(), "wallet web3 challenge reject addr=%s not bound and auto-register disabled", addr)
		writeWeb3Error(c, 5, "钱包未绑定账户，请先绑定或由管理员开启自动注册")
		return
	}
	now := time.Now()
	nonce, message := common.GenerateWalletNonce(addr, "Login to "+config.SystemName, req.ChainId)
	expiresAt := now.Add(time.Duration(config.WalletNonceTTLMinutes) * time.Minute)
	logger.Loginf(c.Request.Context(), "wallet web3 challenge success addr=%s nonce=%s chain=%s", addr, nonce, req.ChainId)
	writeWeb3OK(c, gin.H{
		"address":   addr,
		"challenge": message,
		"nonce":     nonce,
		"issuedAt":  now.UnixMilli(),
		"expiresAt": expiresAt.UnixMilli(),
	})
}

// WalletVerifyWeb3 godoc
// @Summary Wallet verify (web3)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.WalletLoginRequest true "Verify payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/auth/verify [post]
// WalletVerifyWeb3 implements /api/v1/public/auth/verify
func WalletVerifyWeb3(c *gin.Context) {
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Loginf(c.Request.Context(), "wallet web3 verify bind fail err=%v", err)
		writeWeb3Error(c, 2, "参数错误")
		return
	}
	user, err := walletAuthenticate(c, req)
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet web3 verify auth fail addr=%s err=%v", req.Address, err)
		writeWeb3Error(c, 3, err.Error())
		return
	}
	if err := usercontroller.SetupSession(user, c); err != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet web3 verify setup session failed user=%d err=%v", user.Id, err)
		writeWeb3Error(c, 8, "无法保存会话信息，请重试")
		return
	}
	addr := ""
	if user.WalletAddress != nil {
		addr = strings.ToLower(*user.WalletAddress)
	}
	accessToken, accessExp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		logger.SysError("wallet web3 access token generate failed: " + tokenErr.Error())
		writeWeb3Error(c, 8, "生成 token 失败")
		return
	}
	refreshToken, refreshExp, refreshErr := common.GenerateWalletRefreshJWT(user.Id, addr)
	if refreshErr != nil {
		logger.SysError("wallet web3 refresh token generate failed: " + refreshErr.Error())
		writeWeb3Error(c, 8, "生成 refresh token 失败")
		return
	}
	setWalletRefreshCookie(c, refreshToken, refreshExp)
	logger.Loginf(c.Request.Context(), "wallet web3 verify success user=%d addr=%s exp=%s refresh_exp=%s", user.Id, addr, accessExp.UTC().Format(time.RFC3339), refreshExp.UTC().Format(time.RFC3339))
	writeWeb3OK(c, gin.H{
		"address":          addr,
		"token":            accessToken,
		"expiresAt":        accessExp.UnixMilli(),
		"refreshExpiresAt": refreshExp.UnixMilli(),
	})
}

// WalletRefreshWeb3 godoc
// @Summary Refresh token (web3)
// @Tags public
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/auth/refresh [post]
// WalletRefreshWeb3 implements /api/v1/public/auth/refresh
func WalletRefreshWeb3(c *gin.Context) {
	refreshToken, err := c.Cookie(walletRefreshCookieName)
	if err != nil || strings.TrimSpace(refreshToken) == "" {
		logger.Loginf(c.Request.Context(), "wallet web3 refresh missing token")
		writeWeb3Error(c, 3, "缺少 refresh token")
		return
	}
	claims, err := common.VerifyWalletRefreshJWT(refreshToken)
	if err != nil {
		logger.Loginf(c.Request.Context(), "wallet web3 refresh verify failed err=%v", err)
		writeWeb3Error(c, 3, "refresh token 无效或已过期")
		return
	}
	user := model.User{Id: claims.UserID}
	if err := user.FillUserById(); err != nil {
		logger.Loginf(c.Request.Context(), "wallet web3 refresh user not found id=%d", claims.UserID)
		writeWeb3Error(c, 5, "用户不存在")
		return
	}
	userAddr := ""
	if user.WalletAddress != nil {
		userAddr = strings.ToLower(*user.WalletAddress)
	}
	if user.WalletAddress == nil || userAddr != strings.ToLower(claims.WalletAddress) {
		logger.Loginf(c.Request.Context(), "wallet web3 refresh addr mismatch token=%s user=%s", claims.WalletAddress, userAddr)
		writeWeb3Error(c, 3, "钱包地址不匹配")
		return
	}
	if user.Status != model.UserStatusEnabled {
		logger.Loginf(c.Request.Context(), "wallet web3 refresh user disabled id=%d", user.Id)
		writeWeb3Error(c, 4, "用户已被封禁")
		return
	}
	if err := usercontroller.SetupSession(&user, c); err != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet web3 refresh setup session failed user=%d err=%v", user.Id, err)
		writeWeb3Error(c, 8, "无法保存会话信息，请重试")
		return
	}
	addr := strings.ToLower(*user.WalletAddress)
	accessToken, accessExp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet web3 refresh generate token failed user=%d err=%v", user.Id, tokenErr)
		writeWeb3Error(c, 8, "生成 token 失败")
		return
	}
	newRefreshToken, refreshExp, refreshErr := common.GenerateWalletRefreshJWT(user.Id, addr)
	if refreshErr != nil {
		logger.LoginErrorf(c.Request.Context(), "wallet web3 refresh generate refresh token failed user=%d err=%v", user.Id, refreshErr)
		writeWeb3Error(c, 8, "生成 refresh token 失败")
		return
	}
	setWalletRefreshCookie(c, newRefreshToken, refreshExp)
	logger.Loginf(c.Request.Context(), "wallet web3 refresh success user=%d addr=%s exp=%s refresh_exp=%s", user.Id, addr, accessExp.UTC().Format(time.RFC3339), refreshExp.UTC().Format(time.RFC3339))
	writeWeb3OK(c, gin.H{
		"address":          addr,
		"token":            accessToken,
		"expiresAt":        accessExp.UnixMilli(),
		"refreshExpiresAt": refreshExp.UnixMilli(),
	})
}

// WalletLogoutWeb3 godoc
// @Summary Logout (web3)
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/auth/logout [post]
// WalletLogoutWeb3 implements /api/v1/public/auth/logout
func WalletLogoutWeb3(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	_ = session.Save()
	clearWalletRefreshCookie(c)
	writeWeb3OK(c, gin.H{
		"logout": true,
	})
}

func writeWeb3OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"message":   "ok",
		"data":      data,
		"timestamp": time.Now().UnixMilli(),
	})
}

func writeWeb3Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{
		"code":      code,
		"message":   message,
		"data":      nil,
		"timestamp": time.Now().UnixMilli(),
	})
}

func setWalletRefreshCookie(c *gin.Context, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(parseSameSite(config.WalletRefreshCookieSameSite))
	c.SetCookie(walletRefreshCookieName, token, maxAge, "/", config.WalletRefreshCookieDomain, config.WalletRefreshCookieSecure, true)
}

func clearWalletRefreshCookie(c *gin.Context) {
	c.SetSameSite(parseSameSite(config.WalletRefreshCookieSameSite))
	c.SetCookie(walletRefreshCookieName, "", -1, "/", config.WalletRefreshCookieDomain, config.WalletRefreshCookieSecure, true)
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}
