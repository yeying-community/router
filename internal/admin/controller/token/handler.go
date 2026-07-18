package token

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/network"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	tokensvc "github.com/yeying-community/router/internal/admin/service/token"
	"gorm.io/gorm"
)

const tokenNotFoundMessage = "令牌不存在或无权访问"
const tokenNotFoundCode = "token_not_found"

func GetAllTokens(c *gin.Context) {
	userId := c.GetString(ctxkey.Id)
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}

	order := c.Query("order")
	tokens, err := tokensvc.GetAll(userId, (page-1)*config.ItemsPerPage, config.ItemsPerPage, order)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	var total int64
	if err := model.DB.Model(&model.Token{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewTokens(tokens),
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": config.ItemsPerPage,
		},
	})
	return
}

func SearchTokens(c *gin.Context) {
	userId := c.GetString(ctxkey.Id)
	keyword := c.Query("keyword")
	tokens, err := tokensvc.Search(userId, keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewTokens(tokens),
	})
	return
}

func SearchAdminTokens(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	query := model.DB.Model(&model.Token{}).Order("updated_time desc")
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR id LIKE ?", likeKeyword, likeKeyword)
	}
	rows := make([]*model.Token, 0)
	if err := query.Limit(50).Find(&rows).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewTokens(rows),
	})
}

func GetToken(c *gin.Context) {
	id := c.Param("id")
	userId := c.GetString(ctxkey.Id)
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	token, err := tokensvc.GetByIDs(id, userId)
	if err != nil {
		message := err.Error()
		code := ""
		if errors.Is(err, gorm.ErrRecordNotFound) {
			message = tokenNotFoundMessage
			code = tokenNotFoundCode
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
			"code":    code,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewToken(token),
	})
	return
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetString(ctxkey.TokenId)
	userId := c.GetString(ctxkey.Id)
	if tokenId == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前访问凭证未绑定具体令牌",
		})
		return
	}
	token, err := tokensvc.GetByIDs(tokenId, userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	totalGranted := token.RemainQuota + token.UsedQuota
	totalAvailable := token.RemainQuota
	if token.UnlimitedQuota {
		totalGranted = 0
		totalAvailable = 0
	}
	totalRequestGranted := token.RemainRequestCount + token.UsedRequestCount
	totalRequestAvailable := token.RemainRequestCount
	if token.UnlimitedRequestCount {
		totalRequestGranted = 0
		totalRequestAvailable = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"object":                        "token_credit_summary",
			"token_id":                      token.Id,
			"token_name":                    token.Name,
			"status":                        token.Status,
			"unlimited_quota":               token.UnlimitedQuota,
			"total_granted":                 totalGranted,
			"total_used":                    token.UsedQuota,
			"total_available":               totalAvailable,
			"remaining_amount":              token.RemainQuota,
			"used_amount":                   token.UsedQuota,
			"unlimited_request_count":       token.UnlimitedRequestCount,
			"total_request_count_granted":   totalRequestGranted,
			"total_request_count_used":      token.UsedRequestCount,
			"total_request_count_available": totalRequestAvailable,
			"remaining_request_count":       token.RemainRequestCount,
			"used_request_count":            token.UsedRequestCount,
			"created_at":                    token.CreatedTime * 1000,
			"updated_at":                    token.UpdatedTime * 1000,
			"accessed_at":                   token.AccessedTime * 1000,
			"expires_at":                    expiredAt * 1000,
		},
	})
}

func normalizeTokenRequestCountLimit(token *model.Token) {
	if token == nil {
		return
	}
	if token.RemainRequestCount == 0 && !token.UnlimitedRequestCount {
		token.UnlimitedRequestCount = true
	}
}

func validateToken(c *gin.Context, token model.Token) error {
	if len(token.Name) > 30 {
		return fmt.Errorf("令牌名称过长")
	}
	if token.RemainRequestCount < 0 {
		return fmt.Errorf("请求次数不能为负数")
	}
	if token.Subnet != nil && *token.Subnet != "" {
		err := network.IsValidSubnets(*token.Subnet)
		if err != nil {
			return fmt.Errorf("无效的网段：%s", err.Error())
		}
	}
	return nil
}

var buildUserEntitlementModelsFn = model.BuildUserEntitlementModels

func validateTokenModelEntitlement(ctx context.Context, userID string, token model.Token) error {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return fmt.Errorf("用户 ID 为空")
	}
	payload, err := buildUserEntitlementModelsFn(ctx, normalizedUserID)
	if err != nil {
		return err
	}
	availableModels := make(map[string]struct{}, len(payload.Models))
	for _, modelName := range payload.Models {
		normalizedModel := strings.TrimSpace(modelName)
		if normalizedModel == "" {
			continue
		}
		availableModels[normalizedModel] = struct{}{}
	}
	if len(availableModels) == 0 {
		return fmt.Errorf("当前账号暂无可用模型，请先购买套餐或充值后再创建令牌")
	}
	if token.Models == nil || strings.TrimSpace(*token.Models) == "" {
		return nil
	}
	requestedModels := model.NormalizeChannelModelIDsPreserveOrder(strings.Split(*token.Models, ","))
	if len(requestedModels) == 0 {
		return fmt.Errorf("当前账号暂无可用模型，请先购买套餐或充值后再创建令牌")
	}
	missingModels := make([]string, 0)
	for _, modelName := range requestedModels {
		if _, ok := availableModels[modelName]; !ok {
			missingModels = append(missingModels, modelName)
		}
	}
	if len(missingModels) > 0 {
		return fmt.Errorf("模型范围包含当前账号不可用模型：%s", strings.Join(missingModels, ", "))
	}
	return nil
}

func AddToken(c *gin.Context) {
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	normalizeTokenRequestCountLimit(&token)
	err = validateToken(c, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("参数错误：%s", err.Error()),
		})
		return
	}
	userID := c.GetString(ctxkey.Id)
	err = validateTokenModelEntitlement(c.Request.Context(), userID, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	now := helper.GetTimestamp()
	cleanToken := model.Token{
		UserId:                userID,
		Name:                  token.Name,
		Key:                   random.GenerateKey(),
		CreatedTime:           now,
		UpdatedTime:           now,
		AccessedTime:          now,
		ExpiredTime:           token.ExpiredTime,
		RemainQuota:           token.RemainQuota,
		UnlimitedQuota:        token.UnlimitedQuota,
		RemainRequestCount:    token.RemainRequestCount,
		UnlimitedRequestCount: token.UnlimitedRequestCount,
		Models:                token.Models,
		Subnet:                token.Subnet,
	}
	err = tokensvc.Create(&cleanToken)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewCreatedToken(&cleanToken),
	})
	return
}

func DeleteToken(c *gin.Context) {
	id := c.Param("id")
	userId := c.GetString(ctxkey.Id)
	err := tokensvc.DeleteByID(id, userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func UpdateToken(c *gin.Context) {
	userId := c.GetString(ctxkey.Id)
	statusOnly := c.Query("status_only")
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	normalizeTokenRequestCountLimit(&token)
	err = validateToken(c, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("参数错误：%s", err.Error()),
		})
		return
	}
	cleanToken, err := tokensvc.GetByIDs(token.Id, userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if token.Status == model.TokenStatusEnabled {
		if cleanToken.Status == model.TokenStatusExpired && cleanToken.ExpiredTime <= helper.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "令牌已过期，无法启用，请先修改令牌过期时间，或者设置为永不过期",
			})
			return
		}
		if cleanToken.Status == model.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "令牌可用额度已用尽，无法启用，请先修改令牌剩余额度，或者设置为无限额度",
			})
			return
		}
		if cleanToken.Status == model.TokenStatusExhausted && cleanToken.RemainRequestCount <= 0 && !cleanToken.UnlimitedRequestCount {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "令牌请求次数已用尽，无法启用，请先修改令牌剩余请求次数，或者设置为不限次数",
			})
			return
		}
	}
	if statusOnly == "" || token.Status == model.TokenStatusEnabled {
		nextToken := *cleanToken
		if statusOnly != "" {
			nextToken.Status = token.Status
		} else {
			nextToken.Models = token.Models
		}
		err = validateTokenModelEntitlement(c.Request.Context(), userId, nextToken)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.RemainRequestCount = token.RemainRequestCount
		cleanToken.UnlimitedRequestCount = token.UnlimitedRequestCount
		cleanToken.Models = token.Models
		cleanToken.Subnet = token.Subnet
		cleanToken.UpdatedTime = helper.GetTimestamp()
	}
	err = tokensvc.Update(cleanToken)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewToken(cleanToken),
	})
	return
}
