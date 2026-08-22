package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	usercontroller "github.com/yeying-community/router/internal/admin/controller/user"
	"github.com/yeying-community/router/internal/admin/model"
)

// Identity passkey login handles the no-wallet-plugin scenario. The user
// authenticates through Node's wallet identity authorization flow. Node returns
// the wallet identity DID and associated wallet address, and Router resolves the
// local account by wallet address, same as the wallet plugin login path.

const identityPasskeyLoginTTL = 5 * time.Minute

type nodeResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type nodeError struct {
	Status  int
	Message string
}

func (e *nodeError) Error() string {
	return fmt.Sprintf("夜莺身份服务返回 %d: %s", e.Status, e.Message)
}

type identityAuthorizeResult struct {
	RequestID string `json:"requestId"`
	VerifyURL string `json:"verifyUrl"`
	ExpiresAt string `json:"expiresAt"`
}

type identityExchangeResult struct {
	WalletIdentityID string   `json:"walletIdentityId"`
	DID              string   `json:"did"`
	WalletAddress    string   `json:"walletAddress"`
	Scopes           []string `json:"scopes"`
	Credentials      []struct {
		Type         string `json:"type"`
		CredentialID string `json:"credentialId"`
		Credential   string `json:"credential"`
	} `json:"credentials"`
}

func normalizeIdentityVerifyURL(nodeURL string, verifyURL string) string {
	value := strings.TrimSpace(verifyURL)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return strings.TrimRight(strings.TrimSpace(nodeURL), "/") + value
	}
	return strings.TrimRight(strings.TrimSpace(nodeURL), "/") + "/" + value
}

func identityPasskeyConfiguration() (string, string, string, error) {
	nodeURL := strings.TrimRight(strings.TrimSpace(config.IdentityNodeURL), "/")
	appID := strings.TrimSpace(config.IdentityAppID)
	callbackURL := strings.TrimSpace(config.IdentityCallbackURL)
	if callbackURL == "" {
		callbackURL = strings.TrimRight(strings.TrimSpace(config.ServerAddress), "/") + "/api/v1/public/oauth/identity/callback"
	}
	if nodeURL == "" || appID == "" {
		return "", "", "", errors.New("Router 钱包身份登录尚未配置，请配置 identity.node_url 和 identity.app_id")
	}
	return nodeURL, appID, callbackURL, nil
}

func identityPasskeyRandomURLValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func identityPasskeyPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func nodePost[T any](ctx context.Context, nodeURL, path string, payload any, result *nodeResponse[T]) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nodeURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || result.Code != 0 {
		return &nodeError{Status: resp.StatusCode, Message: strings.TrimSpace(result.Message)}
	}
	return nil
}

var identityPasskeyScopes = []string{"identity.basic", "identity.wallet", "identity.email"}

func CreateIdentityPasskeyLoginSession(c *gin.Context) {
	nodeURL, appID, callbackURL, err := identityPasskeyConfiguration()
	if err != nil {
		identityPasskeyLoginError(c, err.Error())
		return
	}
	state, err := identityPasskeyRandomURLValue(32)
	if err != nil {
		identityPasskeyLoginError(c, "无法创建登录会话")
		return
	}
	verifier, err := identityPasskeyRandomURLValue(64)
	if err != nil {
		identityPasskeyLoginError(c, "无法创建登录会话")
		return
	}
	requestResult := nodeResponse[identityAuthorizeResult]{}
	err = nodePost(c.Request.Context(), nodeURL, "/api/v1/public/identity/authorize/request", gin.H{
		"appId": appID, "redirectUri": callbackURL, "state": state,
		"codeChallenge": identityPasskeyPKCEChallenge(verifier), "codeChallengeMethod": "S256",
		"scopes":       identityPasskeyScopes,
		"requestTtlMs": identityPasskeyLoginTTL.Milliseconds(),
	}, &requestResult)
	if err != nil || strings.TrimSpace(requestResult.Data.RequestID) == "" || strings.TrimSpace(requestResult.Data.VerifyURL) == "" {
		logger.LoginErrorf(c.Request.Context(), "identity passkey authorize request failed err=%v message=%s", err, requestResult.Message)
		var nErr *nodeError
		if errors.As(err, &nErr) && nErr.Status == http.StatusForbidden && nErr.Message == "redirectUri is not allowed" {
			identityPasskeyLoginError(c, "夜莺身份服务未授权当前 Router 回调地址，请检查 Node 应用的 redirectUris 配置")
			return
		}
		identityPasskeyLoginError(c, "无法连接夜莺身份服务，请稍后重试")
		return
	}
	expiresAt := time.Now().Add(identityPasskeyLoginTTL).Unix()
	if parsed, parseErr := time.Parse(time.RFC3339, requestResult.Data.ExpiresAt); parseErr == nil {
		expiresAt = parsed.Unix()
	}
	sessionID, err := identityPasskeyRandomURLValue(32)
	if err != nil {
		identityPasskeyLoginError(c, "无法创建登录会话")
		return
	}
	row := &model.IdentityPasskeyLoginSession{SessionID: sessionID, State: state, RequestID: requestResult.Data.RequestID, CodeVerifier: verifier, Status: model.IdentityPasskeyLoginSessionStatusPending, ExpiresAt: expiresAt}
	if err := model.DB.Create(row).Error; err != nil {
		logger.SysError("create identity passkey login session failed: " + err.Error())
		identityPasskeyLoginError(c, "无法保存登录会话")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"session_id": sessionID, "verify_url": normalizeIdentityVerifyURL(nodeURL, requestResult.Data.VerifyURL), "expires_at": expiresAt, "poll_interval": 2}})
}

func IdentityPasskeyLoginStatus(c *gin.Context) {
	row, err := model.FindIdentityPasskeyLoginSessionByID(c.Query("session_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		identityPasskeyLoginError(c, "登录会话不存在或已失效")
		return
	}
	if err != nil {
		identityPasskeyLoginError(c, "无法读取登录会话")
		return
	}
	if time.Now().Unix() > row.ExpiresAt {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "expired"}})
		return
	}
	if row.Status == model.IdentityPasskeyLoginSessionStatusPending {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
		return
	}
	if row.Status == model.IdentityPasskeyLoginSessionStatusFailed {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": row.ErrorMessage}})
		return
	}
	if row.Status == model.IdentityPasskeyLoginSessionStatusApproved {
		completeIdentityPasskeyLogin(c, row)
		return
	}
	if row.Status == model.IdentityPasskeyLoginSessionStatusComplete {
		completeIdentityPasskeySessionResponse(c, row)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
}

func IdentityPasskeyLoginCallback(c *gin.Context) {
	code, state := strings.TrimSpace(c.Query("code")), strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		c.String(http.StatusBadRequest, "Invalid callback")
		return
	}
	row, err := model.FindIdentityPasskeyLoginSessionByState(state)
	if err != nil || time.Now().Unix() > row.ExpiresAt || row.Status != model.IdentityPasskeyLoginSessionStatusPending {
		c.String(http.StatusBadRequest, "Login request is invalid or expired")
		return
	}
	update := model.DB.Model(&model.IdentityPasskeyLoginSession{}).
		Where("session_id = ? AND status = ?", row.SessionID, model.IdentityPasskeyLoginSessionStatusPending).
		Updates(map[string]any{"status": model.IdentityPasskeyLoginSessionStatusApproved, "code": code})
	if update.Error != nil || update.RowsAffected != 1 {
		c.String(http.StatusInternalServerError, "Unable to complete login")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, "<!doctype html><title>Login complete</title><script>try{localStorage.setItem('__router_identity_callback__',String(Date.now()));new BroadcastChannel('router-identity-login').postMessage('approved')}catch(e){}setTimeout(function(){window.close()},300)</script>Login complete. You can return to Router.")
}

func completeIdentityPasskeyLogin(c *gin.Context, row *model.IdentityPasskeyLoginSession) {
	nodeURL, appID, callbackURL, err := identityPasskeyConfiguration()
	if err != nil {
		identityPasskeyLoginError(c, err.Error())
		return
	}
	claim := model.DB.Model(&model.IdentityPasskeyLoginSession{}).
		Where("session_id = ? AND status = ?", row.SessionID, model.IdentityPasskeyLoginSessionStatusApproved).
		Update("status", "exchanging")
	if claim.Error != nil || claim.RowsAffected != 1 {
		identityPasskeyLoginError(c, "无法完成登录")
		return
	}
	exchange := nodeResponse[identityExchangeResult]{}
	err = nodePost(c.Request.Context(), nodeURL, "/api/v1/public/identity/authorize/exchange", gin.H{"code": row.Code, "appId": appID, "redirectUri": callbackURL, "codeVerifier": row.CodeVerifier}, &exchange)
	if err != nil {
		failIdentityPasskeyLogin(row, "夜莺身份授权失败")
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": "夜莺身份授权失败"}})
		return
	}
	if exchange.Data.DID == "" || exchange.Data.WalletIdentityID == "" {
		failIdentityPasskeyLogin(row, "夜莺身份未返回身份信息")
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": "夜莺身份未返回身份信息"}})
		return
	}
	walletAddress := model.NormalizeWalletAddress(exchange.Data.WalletAddress)
	if walletAddress == "" {
		failIdentityPasskeyLogin(row, "夜莺身份未返回钱包地址")
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "failed", "message": "夜莺身份未返回钱包地址"}})
		return
	}
	user, resolveErr := resolveWalletIdentityUser(walletAddress, c.Request.Context())
	if resolveErr != nil {
		failIdentityPasskeyLogin(row, resolveErr.Error())
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "unbound", "message": resolveErr.Error()}})
		return
	}
	// Sync email from credentials if available
	email := extractEmailFromCredentials(exchange.Data.Credentials)
	if email != "" {
		if err := model.SyncIdentityEmail(user.Id, email); err != nil {
			logger.SysError("sync identity email failed: " + err.Error())
		}
	}
	if err := model.DB.Model(row).Updates(map[string]any{"status": model.IdentityPasskeyLoginSessionStatusComplete, "user_id": user.Id, "code": "", "code_verifier": ""}).Error; err != nil {
		identityPasskeyLoginError(c, "无法完成登录")
		return
	}
	completeIdentityPasskeyUserResponse(c, user)
}

func extractEmailFromCredentials(credentials []struct {
	Type         string `json:"type"`
	CredentialID string `json:"credentialId"`
	Credential   string `json:"credential"`
}) string {
	for _, cred := range credentials {
		if cred.Type == "EmailCredential" && cred.Credential != "" {
			// Parse JWT-VC to extract email claim
			parts := strings.Split(cred.Credential, ".")
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
			if email != "" {
				return email
			}
		}
	}
	return ""
}

func completeIdentityPasskeySessionResponse(c *gin.Context, row *model.IdentityPasskeyLoginSession) {
	user := &model.User{Id: row.UserID}
	if err := user.FillUserById(); err != nil || user.Status != model.UserStatusEnabled {
		identityPasskeyLoginError(c, "Router 账户不可用")
		return
	}
	completeIdentityPasskeyUserResponse(c, user)
}

func completeIdentityPasskeyUserResponse(c *gin.Context, user *model.User) {
	if err := usercontroller.SetupSession(user, c); err != nil {
		identityPasskeyLoginError(c, "无法保存会话信息，请重试")
		return
	}
	addr := ""
	if user.WalletAddress != nil {
		addr = model.NormalizeWalletAddress(*user.WalletAddress)
	}
	token, exp, tokenErr := common.GenerateWalletJWT(user.Id, addr)
	if tokenErr != nil {
		identityPasskeyLoginError(c, "生成 token 失败")
		return
	}
	refreshToken, refreshExp, refreshErr := common.GenerateWalletRefreshJWT(user.Id, addr)
	if refreshErr == nil {
		setWalletRefreshCookie(c, refreshToken, refreshExp)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"status": "complete",
			"login": gin.H{
				"accessToken":      token,
				"refreshToken":     refreshToken,
				"expiresAt":        exp.UnixMilli(),
				"refreshExpiresAt": refreshExp.UnixMilli(),
			},
			"user": gin.H{
				"id":               user.Id,
				"username":         user.Username,
				"display_name":     user.DisplayName,
				"role":             model.ExposedRole(user),
				"status":           user.Status,
				"wallet_address":   user.WalletAddress,
				"has_password":     user.HasPassword,
				"can_manage_users": model.CanManageUsers(user),
			},
		},
	})
}

func failIdentityPasskeyLogin(row *model.IdentityPasskeyLoginSession, message string) {
	_ = model.DB.Model(row).Updates(map[string]any{"status": model.IdentityPasskeyLoginSessionStatusFailed, "error_message": message, "code": "", "code_verifier": ""}).Error
}

func identityPasskeyLoginError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"success": false, "message": message})
}
