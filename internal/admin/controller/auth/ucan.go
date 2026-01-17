package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

// PublicProfile implements /api/v1/public/profile
func PublicProfile(c *gin.Context) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		writeWeb3ErrorStatus(c, http.StatusUnauthorized, 401, "Missing access token")
		return
	}
	bearer := authHeader
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		bearer = strings.TrimSpace(authHeader[7:])
	}
	if bearer == "" {
		writeWeb3ErrorStatus(c, http.StatusUnauthorized, 401, "Missing access token")
		return
	}

	if claims, err := common.VerifyWalletJWT(bearer); err == nil {
		addr := strings.ToLower(claims.WalletAddress)
		if addr == "" && claims.UserID != 0 {
			user := model.User{Id: claims.UserID}
			if err := user.FillUserById(); err == nil && user.WalletAddress != nil {
				addr = strings.ToLower(*user.WalletAddress)
			}
		}
		if addr == "" {
			writeWeb3ErrorStatus(c, http.StatusUnauthorized, 401, "Invalid access token")
			return
		}
		writeWeb3OK(c, gin.H{
			"address":  addr,
			"issuedAt": time.Now().UnixMilli(),
		})
		return
	}

	if !common.IsUcanToken(bearer) {
		writeWeb3ErrorStatus(c, http.StatusUnauthorized, 401, "Invalid or expired access token")
		return
	}

	required := []common.UcanCapability{{Resource: config.UcanResource, Action: config.UcanAction}}
	address, err := common.VerifyUcanInvocation(bearer, common.ResolveUcanAudience(), required)
	if err != nil {
		logger.Loginf(c.Request.Context(), "UCAN verify failed err=%v", err)
		writeWeb3ErrorStatus(c, http.StatusUnauthorized, 401, err.Error())
		return
	}
	writeWeb3OK(c, gin.H{
		"address":  address,
		"issuedAt": time.Now().UnixMilli(),
	})
}

func writeWeb3ErrorStatus(c *gin.Context, status int, code int, message string) {
	c.JSON(status, gin.H{
		"code":      code,
		"message":   message,
		"data":      nil,
		"timestamp": time.Now().UnixMilli(),
	})
}
