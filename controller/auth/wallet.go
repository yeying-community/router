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
	"github.com/yeying-community/router/controller"
	"github.com/yeying-community/router/model"
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
}

// WalletNonce issues a nonce & message to sign
func WalletNonce(c *gin.Context) {
	if !config.WalletLoginEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启钱包登录",
		})
		return
	}
	var req walletNonceRequest
	if err := c.ShouldBind(&req); err != nil || !common.IsValidEthAddress(req.Address) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误，缺少 address",
		})
		return
	}

	nonce, message := common.GenerateWalletNonce(req.Address, "Login to "+config.SystemName, req.ChainId)
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

// WalletLogin verifies signature and logs user in
func WalletLogin(c *gin.Context) {
	if !config.WalletLoginEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启钱包登录",
		})
		return
	}
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}

	user, err := walletAuthenticate(c, req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := controller.SetupSession(user, c); err != nil {
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
		logger.SysError("wallet jwt generate failed: " + tokenErr.Error())
	}
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

// WalletBind binds a wallet to logged-in user
func WalletBind(c *gin.Context) {
	if !config.WalletLoginEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启钱包登录",
		})
		return
	}
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
		return errors.New("无效的钱包地址")
	}
	if req.Signature == "" {
		return errors.New("缺少签名或 nonce")
	}
	// chainId check
	if len(config.WalletAllowedChains) > 0 && req.ChainId != "" {
		allowed := false
		for _, c := range config.WalletAllowedChains {
			if strings.TrimSpace(c) == req.ChainId {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.New("不允许的链 ID")
		}
	}
	entry, ok := common.GetWalletNonce(req.Address)
	if !ok {
		return errors.New("nonce 无效或已过期")
	}
	if req.Nonce != "" && entry.Nonce != req.Nonce {
		return errors.New("nonce 无效或已过期")
	}
	// verify signature
	recovered, err := recoverAddress(entry.Message, req.Signature)
	if err != nil {
		logger.SysError("wallet login verify failed: " + err.Error())
		return errors.New("签名验证失败")
	}
	if strings.ToLower(recovered) != strings.ToLower(req.Address) {
		return errors.New("签名地址与请求地址不一致")
	}
	return nil
}

// walletAuthenticate verifies signature & returns an enabled user (create if allowed)
func walletAuthenticate(c *gin.Context, req walletLoginRequest) (*model.User, error) {
	if err := verifyWalletRequest(req); err != nil {
		return nil, err
	}
	addr := strings.ToLower(req.Address)
	user, err := findOrCreateWalletUser(addr, c.Request.Context())
	if err != nil {
		return nil, err
	}
	if user.Status != model.UserStatusEnabled {
		return nil, errors.New("用户已被封禁")
	}
	common.ConsumeWalletNonce(addr)
	return user, nil
}

func findOrCreateWalletUser(addr string, ctx context.Context) (*model.User, error) {
	user := model.User{WalletAddress: &addr}
	if !model.IsWalletAddressAlreadyTaken(addr) {
		if isRootAllowed(addr) {
			var root model.User
			if err := model.DB.Select("id").Where("role = ?", model.RoleRootUser).First(&root).Error; err == nil {
				_ = root.FillUserById()
				root.WalletAddress = &addr
				_ = root.Update(false)
				return &root, nil
			}
		}
		if config.WalletAutoRegisterEnabled {
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

func isRootAllowed(addr string) bool {
	for _, a := range config.WalletRootAllowedAddresses {
		if strings.ToLower(a) == addr {
			return true
		}
	}
	return false
}

// --- proto-aligned handlers ---

// WalletChallengeProto implements /api/v1/public/common/auth/challenge
func WalletChallengeProto(c *gin.Context) {
	if !config.WalletLoginEnabled {
		writeProtoError(c, 12, "管理员未开启钱包登录")
		return
	}
	var req walletNonceRequest
	if err := c.ShouldBindJSON(&req); err != nil || !common.IsValidEthAddress(req.Address) {
		writeProtoError(c, 2, "参数错误，缺少 address")
		return
	}
	addr := strings.ToLower(req.Address)
	if !model.IsWalletAddressAlreadyTaken(addr) && !config.WalletAutoRegisterEnabled && !isRootAllowed(addr) {
		writeProtoError(c, 5, "钱包未绑定账户，请先绑定或由管理员开启自动注册")
		return
	}
	nonce, message := common.GenerateWalletNonce(addr, "Login to "+config.SystemName, req.ChainId)
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

// WalletVerifyProto implements /api/v1/public/common/auth/verify
func WalletVerifyProto(c *gin.Context) {
	if !config.WalletLoginEnabled {
		writeProtoError(c, 12, "管理员未开启钱包登录")
		return
	}
	var req walletLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeProtoError(c, 2, "参数错误")
		return
	}
	user, err := walletAuthenticate(c, req)
	if err != nil {
		writeProtoError(c, 3, err.Error())
		return
	}
	if err := controller.SetupSession(user, c); err != nil {
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

// WalletRefreshToken implements /api/v1/public/common/auth/refreshToken
func WalletRefreshToken(c *gin.Context) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}
	if authHeader == "" {
		writeProtoError(c, 3, "缺少 token")
		return
	}
	claims, err := common.VerifyWalletJWT(authHeader)
	if err != nil {
		writeProtoError(c, 3, "token 无效或已过期")
		return
	}
	user := model.User{Id: claims.UserID}
	if err := user.FillUserById(); err != nil {
		writeProtoError(c, 5, "用户不存在")
		return
	}
	if user.WalletAddress == nil || strings.ToLower(*user.WalletAddress) != strings.ToLower(claims.WalletAddress) {
		writeProtoError(c, 3, "钱包地址不匹配")
		return
	}
	if user.Status != model.UserStatusEnabled {
		writeProtoError(c, 4, "用户已被封禁")
		return
	}
	if err := controller.SetupSession(&user, c); err != nil {
		writeProtoError(c, 8, "无法保存会话信息，请重试")
		return
	}
	addr := strings.ToLower(*user.WalletAddress)
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		writeProtoError(c, 8, "生成 token 失败")
		return
	}
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
