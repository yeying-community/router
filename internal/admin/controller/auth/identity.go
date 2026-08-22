package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	usercontroller "github.com/yeying-community/router/internal/admin/controller/user"
	"github.com/yeying-community/router/internal/admin/model"
)

// Identity login session is created and verified locally without forwarding
// the presentation to Node. Router verifies the Ed25519 presentation signature
// itself; Node is only the credential issuer, not the login authority.

type identityPresentationRequest struct {
	SessionID    string          `json:"session_id"`
	RequestID    string          `json:"request_id"`
	Address      string          `json:"address"`
	Presentation json.RawMessage `json:"presentation"`
}

var routerIdentityScopes = []string{"identity.basic", "identity.wallet", "identity.email"}

func identityRandomURLValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func identityServerAudience() string {
	return strings.TrimRight(strings.TrimSpace(config.ServerAddress), "/")
}

func identityAppID() string {
	return strings.TrimSpace(config.IdentityAppID)
}

func CreateIdentityLoginSession(c *gin.Context) {
	appID := identityAppID()
	if appID == "" {
		identityError(c, "Router 钱包身份登录尚未配置，请配置 identity.app_id")
		return
	}
	nonce, err := identityRandomURLValue(32)
	if err != nil {
		identityError(c, "无法创建登录会话")
		return
	}
	sessionID, err := identityRandomURLValue(32)
	if err != nil {
		identityError(c, "无法创建登录会话")
		return
	}
	expiresAt := time.Now().Add(5 * time.Minute).Unix()
	audience := identityServerAudience()
	row := &model.IdentityLoginSession{
		SessionID: sessionID,
		Nonce:     nonce,
		Audience:  audience,
		AppID:     appID,
		Scopes:    strings.Join(routerIdentityScopes, " "),
		Status:    model.IdentityLoginSessionStatusPending,
		ExpiresAt: expiresAt,
	}
	if err := model.DB.Create(row).Error; err != nil {
		logger.SysError("create identity login session failed: " + err.Error())
		identityError(c, "无法保存登录会话")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"session_id": sessionID,
			"app_id":     appID,
			"nonce":      nonce,
			"audience":   audience,
			"scopes":     routerIdentityScopes,
			"expires_at": expiresAt,
		},
	})
}

func VerifyIdentityWalletLogin(c *gin.Context) {
	var req identityPresentationRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" || len(req.Presentation) == 0 {
		identityError(c, "参数错误")
		return
	}
	row, err := model.FindIdentityLoginSessionByID(req.SessionID)
	if err != nil || row.Status != model.IdentityLoginSessionStatusPending || time.Now().Unix() > row.ExpiresAt {
		identityError(c, "登录会话无效或已过期")
		return
	}

	// Verify the presentation signature locally
	pres, err := common.VerifyIdentityPresentation(req.Presentation, row.Audience, row.Nonce)
	if err != nil {
		identityError(c, "夜莺身份验证失败: "+err.Error())
		return
	}
	if !identityPresentationHasScope(pres.Scopes, "identity.email") || !identityPresentationHasCredential(pres.Credentials, "EmailCredential") {
		identityError(c, "Router 需要已验证邮箱，请先在夜莺钱包插件中完成钱包身份验证和邮箱验证")
		return
	}

	// Verify wallet address from the presentation matches the request
	walletProofAddr := extractWalletProofAddress(req.Presentation)
	if walletProofAddr == "" {
		walletProofAddr = req.Address
	}
	addr := model.NormalizeWalletAddress(walletProofAddr)
	if addr == "" {
		identityError(c, "钱包地址无效")
		return
	}

	// Resolve or create user by wallet address
	user, err := resolveWalletIdentityUser(addr, c.Request.Context())
	if err != nil {
		identityError(c, err.Error())
		return
	}
	if email := identityPresentationEmail(pres.Credentials); email != "" {
		if err := model.SyncIdentityEmail(user.Id, email); err != nil {
			logger.SysError("sync wallet identity email failed: " + err.Error())
			identityError(c, "无法同步钱包身份邮箱")
			return
		}
	}

	// Mark session complete
	if err := model.DB.Model(row).Updates(map[string]any{
		"status":  model.IdentityLoginSessionStatusComplete,
		"user_id": user.Id,
	}).Error; err != nil {
		logger.SysError("complete identity login session failed: " + err.Error())
		identityError(c, "无法保存登录会话")
		return
	}

	if err := usercontroller.SetupSession(user, c); err != nil {
		identityError(c, "无法保存会话信息，请重试")
		return
	}

	userAddr := ""
	if user.WalletAddress != nil {
		userAddr = model.NormalizeWalletAddress(*user.WalletAddress)
	}
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, userAddr)
	if tokenErr != nil {
		identityError(c, "生成 token 失败")
		return
	}

	// Also generate refresh token
	refreshToken, refreshExp, refreshErr := common.GenerateWalletRefreshJWT(user.Id, userAddr)
	if refreshErr == nil {
		setWalletRefreshCookie(c, refreshToken, refreshExp)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"token":              token,
			"expires_at":         exp.UnixMilli(),
			"refresh_expires_at": refreshExp.UnixMilli(),
			"wallet_identity_id": pres.Holder,
			"did":                pres.Holder,
			"user":               user,
		},
	})
}

// extractWalletProofAddress reads the walletProof.address from the presentation JSON.
func extractWalletProofAddress(presentation json.RawMessage) string {
	var raw map[string]any
	if err := json.Unmarshal(presentation, &raw); err != nil {
		return ""
	}
	proof, ok := raw["walletProof"].(map[string]any)
	if !ok {
		return ""
	}
	addr, _ := proof["address"].(string)
	return addr
}

func identityPresentationHasScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.EqualFold(strings.TrimSpace(scope), target) {
			return true
		}
	}
	return false
}

func identityPresentationHasCredential(credentials []string, credentialType string) bool {
	for _, token := range credentials {
		if identityCredentialType(token) == credentialType {
			return true
		}
	}
	return false
}

func identityCredentialType(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	vc, ok := claims["vc"].(map[string]any)
	if !ok {
		return ""
	}
	rawTypes, ok := vc["type"].([]any)
	if !ok {
		return ""
	}
	for _, item := range rawTypes {
		if value, ok := item.(string); ok && value != "VerifiableCredential" {
			return value
		}
	}
	return ""
}

func identityPresentationEmail(credentials []string) string {
	for _, token := range credentials {
		if identityCredentialType(token) != "EmailCredential" {
			continue
		}
		parts := strings.Split(strings.TrimSpace(token), ".")
		if len(parts) != 3 {
			continue
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			continue
		}
		var claims map[string]any
		if err := json.Unmarshal(payload, &claims); err != nil {
			continue
		}
		vc, ok := claims["vc"].(map[string]any)
		if !ok {
			continue
		}
		subject, ok := vc["credentialSubject"].(map[string]any)
		if !ok {
			continue
		}
		email, _ := subject["email"].(string)
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			return email
		}
	}
	return ""
}

func resolveWalletIdentityUser(walletAddress string, ctx context.Context) (*model.User, error) {
	addr := model.NormalizeWalletAddress(walletAddress)
	if addr == "" {
		return nil, errors.New("钱包地址无效")
	}
	user, err := findOrCreateWalletUser(addr, ctx)
	if err != nil || user.Status != model.UserStatusEnabled {
		return nil, errors.New("此钱包尚未关联 Router 账户")
	}
	return user, nil
}

func identityError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"code": 1, "message": message})
}
