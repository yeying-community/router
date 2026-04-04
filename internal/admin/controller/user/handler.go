package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/i18n"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	logsvc "github.com/yeying-community/router/internal/admin/service/log"
	usersvc "github.com/yeying-community/router/internal/admin/service/user"
	"gorm.io/gorm"
)

type updateSelfPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type activeUserPackageSubscriptionView struct {
	Id                         string `json:"id"`
	UserID                     string `json:"user_id"`
	PackageID                  string `json:"package_id"`
	PackageName                string `json:"package_name"`
	GroupID                    string `json:"group_id"`
	GroupName                  string `json:"group_name,omitempty"`
	DailyQuotaLimit            int64  `json:"daily_quota_limit"`
	PackageEmergencyQuotaLimit int64  `json:"package_emergency_quota_limit"`
	QuotaResetTimezone         string `json:"quota_reset_timezone"`
	StartedAt                  int64  `json:"started_at"`
	ExpiresAt                  int64  `json:"expires_at"`
	Status                     int    `json:"status"`
	Source                     string `json:"source,omitempty"`
}

type activeUserPackageSubscriptionPayload struct {
	HasActiveSubscription bool                               `json:"has_active_subscription"`
	Subscription          *activeUserPackageSubscriptionView `json:"subscription,omitempty"`
}

type userRecentRedemptionsPayload struct {
	Items []*presenter.Redemption `json:"items"`
}

func exposedUser(user *model.User) *presenter.User {
	if user == nil {
		return nil
	}
	clean := *user
	clean.Role = model.ExposedRole(user)
	clean.CanManageUsers = model.CanManageUsers(user)
	return presenter.NewUser(&clean)
}

func exposedUsers(users []*model.User) []*presenter.User {
	items := make([]*presenter.User, 0, len(users))
	for _, user := range users {
		items = append(items, exposedUser(user))
	}
	return items
}

func attachActivePackageNames(items []*presenter.User) error {
	if len(items) == 0 {
		return nil
	}
	userIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil || item.User == nil {
			continue
		}
		userIDs = append(userIDs, strings.TrimSpace(item.Id))
	}
	subscriptions, err := model.ListActiveUserPackageSubscriptionsByUserIDs(userIDs)
	if err != nil {
		return err
	}
	nameByUserID := make(map[string]string, len(subscriptions))
	for _, subscription := range subscriptions {
		userID := strings.TrimSpace(subscription.UserID)
		if userID == "" {
			continue
		}
		packageName := strings.TrimSpace(subscription.PackageName)
		if packageName == "" {
			packageName = strings.TrimSpace(subscription.PackageID)
		}
		nameByUserID[userID] = packageName
	}
	for _, item := range items {
		if item == nil || item.User == nil {
			continue
		}
		item.ActivePackageName = nameByUserID[strings.TrimSpace(item.Id)]
	}
	return nil
}

func requesterIsRootUser(c *gin.Context) bool {
	return c.GetBool(ctxkey.CanManageUsers) || c.GetInt(ctxkey.Role) == model.RoleRootUser
}

func requesterIsSelf(c *gin.Context, targetUserID string) bool {
	return strings.TrimSpace(c.GetString(ctxkey.Id)) == strings.TrimSpace(targetUserID)
}

func requesterCanReadUser(c *gin.Context, targetUser *model.User) bool {
	if targetUser == nil {
		return false
	}
	if requesterIsRootUser(c) || requesterIsSelf(c, targetUser.Id) {
		return true
	}
	return c.GetInt(ctxkey.Role) >= model.EffectiveRole(targetUser)
}

func buildActiveUserPackageSubscriptionView(subscription model.UserPackageSubscription) *activeUserPackageSubscriptionView {
	groupID := strings.TrimSpace(subscription.GroupID)
	groupName := ""
	if groupID != "" {
		if groupCatalog, err := model.GetGroupCatalogByID(groupID); err == nil {
			groupName = strings.TrimSpace(groupCatalog.Name)
		}
	}
	source := ""
	if packageID := strings.TrimSpace(subscription.PackageID); packageID != "" {
		if servicePackage, err := model.GetServicePackageByID(packageID); err == nil {
			source = strings.TrimSpace(servicePackage.Source)
		}
	}
	return &activeUserPackageSubscriptionView{
		Id:                         strings.TrimSpace(subscription.Id),
		UserID:                     strings.TrimSpace(subscription.UserID),
		PackageID:                  strings.TrimSpace(subscription.PackageID),
		PackageName:                strings.TrimSpace(subscription.PackageName),
		GroupID:                    groupID,
		GroupName:                  groupName,
		DailyQuotaLimit:            subscription.DailyQuotaLimit,
		PackageEmergencyQuotaLimit: subscription.PackageEmergencyQuotaLimit,
		QuotaResetTimezone:         strings.TrimSpace(subscription.QuotaResetTimezone),
		StartedAt:                  subscription.StartedAt,
		ExpiresAt:                  subscription.ExpiresAt,
		Status:                     subscription.Status,
		Source:                     source,
	}
}

// Login godoc
// @Summary Password login (session/cookie)
// @Tags public
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/user/login [post]
func Login(c *gin.Context) {
	if !config.PasswordLoginEnabled {
		logger.Loginf(c.Request.Context(), "password login rejected: disabled")
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员关闭了密码登录",
			"success": false,
		})
		return
	}
	var loginRequest LoginRequest
	err := json.NewDecoder(c.Request.Body).Decode(&loginRequest)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": i18n.Translate(c, "invalid_parameter"),
			"success": false,
		})
		return
	}
	username := loginRequest.Username
	password := loginRequest.Password
	if username == "" || password == "" {
		c.JSON(http.StatusOK, gin.H{
			"message": i18n.Translate(c, "invalid_parameter"),
			"success": false,
		})
		return
	}
	user := model.User{
		Username: username,
		Password: password,
	}
	err = usersvc.ValidateLogin(&user)
	if err != nil {
		logger.Loginf(c.Request.Context(), "password login failed username=%s err=%v", username, err)
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	SetupLogin(&user, c)
}

// SetupSession sets session & cookies without writing response
func SetupSession(user *model.User, c *gin.Context) error {
	session := sessions.Default(c)
	effectiveRole := model.EffectiveRole(user)
	session.Set("id", user.Id)
	session.Set("username", user.Username)
	session.Set("role", effectiveRole)
	session.Set("status", user.Status)
	err := session.Save()
	if err != nil {
		logger.LoginErrorf(c.Request.Context(), "setup session failed user=%s err=%v", user.Id, err)
		return err
	}
	logger.Loginf(c.Request.Context(), "setup session ok user=%s role=%d", user.Id, effectiveRole)
	return nil
}

// setup session & cookies and then return user info
func SetupLogin(user *model.User, c *gin.Context) {
	if err := SetupSession(user, c); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "无法保存会话信息，请重试",
			"success": false,
		})
		return
	}
	cleanUser := model.User{
		Id:             user.Id,
		Username:       user.Username,
		DisplayName:    user.DisplayName,
		Role:           model.ExposedRole(user),
		Status:         user.Status,
		WalletAddress:  user.WalletAddress,
		HasPassword:    user.HasPassword,
		CanManageUsers: model.CanManageUsers(user),
	}
	logger.Loginf(c.Request.Context(), "password login success user=%s role=%d", user.Id, model.EffectiveRole(user))
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
		"data":    cleanUser,
	})
}

// Logout godoc
// @Summary Logout (session/cookie)
// @Tags public
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/user/logout [get]
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	err := session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	logger.Loginf(c.Request.Context(), "logout success user=%s", c.GetString("id"))
	c.JSON(http.StatusOK, gin.H{
		"message": "",
		"success": true,
	})
}

// Register godoc
// @Summary Register user (password)
// @Tags public
// @Accept json
// @Produce json
// @Param body body docs.UserRegisterRequest true "Register payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 400 {object} docs.ErrorResponse
// @Router /api/v1/public/user/register [post]
func Register(c *gin.Context) {
	ctx := c.Request.Context()
	if !config.RegisterEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员关闭了新用户注册",
			"success": false,
		})
		return
	}
	if !config.PasswordRegisterEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员关闭了通过密码进行注册",
			"success": false,
		})
		return
	}
	var user model.User
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	if err := common.Validate.Struct(&user); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_input"),
		})
		return
	}
	affCode := user.AffCode // this code is the inviter's code, not the user's own code
	inviterId, _ := usersvc.GetIDByAffCode(affCode)
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		InviterId:   inviterId,
		HasPassword: true,
	}
	cleanUser.Email = user.Email
	if err := usersvc.Create(ctx, &cleanUser, inviterId); err != nil {
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

// GetAllUsers godoc
// @Summary List users (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param order query string false "Order"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user [get]
func GetAllUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}

	order := c.DefaultQuery("order", "")
	users, err := usersvc.GetAll((page-1)*config.ItemsPerPage, config.ItemsPerPage, order)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	var total int64
	if err := model.DB.Model(&model.User{}).
		Where("status != ?", model.UserStatusDeleted).
		Count(&total).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	items := exposedUsers(users)
	if err := attachActivePackageNames(items); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": config.ItemsPerPage,
		},
	})
}

// SearchUsers godoc
// @Summary Search users (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "可按用户名/邮箱/显示名/钱包地址/ID搜索"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/search [get]
func SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	users, err := usersvc.Search(keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	items := exposedUsers(users)
	if err := attachActivePackageNames(items); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
	return
}

// GetUser godoc
// @Summary Get user detail (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/{id} [get]
func GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	user, err := usersvc.GetByID(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !requesterCanReadUser(c, user) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权获取同级或更高等级用户的信息",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    exposedUser(user),
	})
	return
}

// GetUserActivePackageSubscription godoc
// @Summary Get current active package subscription for user (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/{id}/package/subscription [get]
func GetUserActivePackageSubscription(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	targetUser, err := usersvc.GetByID(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !requesterCanReadUser(c, targetUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权获取同级或更高等级用户的信息",
		})
		return
	}
	subscription, err := model.GetActiveUserPackageSubscription(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "",
				"data": activeUserPackageSubscriptionPayload{
					HasActiveSubscription: false,
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": activeUserPackageSubscriptionPayload{
			HasActiveSubscription: true,
			Subscription:          buildActiveUserPackageSubscriptionView(subscription),
		},
	})
}

// GetUserRecentRedemptions godoc
// @Summary Get recent redemptions for user (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Param limit query int false "Max items"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/{id}/redemptions [get]
func GetUserRecentRedemptions(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	targetUser, err := usersvc.GetByID(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !requesterCanReadUser(c, targetUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权获取同级或更高等级用户的信息",
		})
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "5")))
	rows, err := model.ListRedemptionsByRedeemedUserID(id, limit)
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
		"data": userRecentRedemptionsPayload{
			Items: presenter.NewRedemptions(rows),
		},
	})
}

// GetUserDashboard godoc
// @Summary User dashboard stats
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param start_timestamp query int false "Start timestamp (unix)"
// @Param end_timestamp query int false "End timestamp (unix)"
// @Param granularity query string false "hour|day|week|month|year"
// @Param models query string false "Comma-separated model list"
// @Param include_meta query int false "Include meta info (1)"
// @Success 200 {object} docs.UserDashboardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/dashboard [get]
func GetUserDashboard(c *gin.Context) {
	id := c.GetString(ctxkey.Id)
	granularity := strings.ToLower(strings.TrimSpace(c.DefaultQuery("granularity", "day")))
	switch granularity {
	case "hour", "day", "week", "month", "year":
	default:
		granularity = "day"
	}
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	includeMeta := c.Query("include_meta") == "1"

	models := parseModelFilters(c.Query("models"))

	if startTimestamp <= 0 || endTimestamp <= 0 {
		now := time.Now()
		startTimestamp = now.Truncate(24*time.Hour).AddDate(0, 0, -6).Unix()
		endTimestamp = now.Truncate(24 * time.Hour).Add(24*time.Hour - time.Second).Unix()
		granularity = "day"
	}
	if startTimestamp > endTimestamp {
		startTimestamp, endTimestamp = endTimestamp, startTimestamp
	}

	dashboards, err := usersvc.SearchLogsByPeriodAndModel(id, int(startTimestamp), int(endTimestamp), granularity, models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
			"data":    nil,
		})
		return
	}
	response := gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewLogStatistics(dashboards),
	}
	if includeMeta {
		providerSet := make(map[string]map[string]struct{})
		modelNames, modelErr := usersvc.SearchLogModelsByPeriod(id, int(startTimestamp), int(endTimestamp))
		if modelErr != nil {
			for _, item := range dashboards {
				if item.ModelName == "" {
					continue
				}
				modelNames = append(modelNames, item.ModelName)
			}
		}
		for _, name := range modelNames {
			if strings.TrimSpace(name) == "" {
				continue
			}
			provider := utils.ResolveProvider(name)
			if providerSet[provider] == nil {
				providerSet[provider] = make(map[string]struct{})
			}
			providerSet[provider][name] = struct{}{}
		}
		providers := make(map[string][]string)
		for provider, models := range providerSet {
			list := make([]string, 0, len(models))
			for modelName := range models {
				list = append(list, modelName)
			}
			sort.Strings(list)
			providers[provider] = list
		}
		response["meta"] = gin.H{
			"providers":   providers,
			"granularity": granularity,
			"start":       startTimestamp,
			"end":         endTimestamp,
		}
	}
	c.JSON(http.StatusOK, response)
	return
}

func parseModelFilters(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	modelSet := make(map[string]struct{}, len(parts))
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, exists := modelSet[name]; exists {
			continue
		}
		modelSet[name] = struct{}{}
		models = append(models, name)
	}
	return models
}

func summarizeUsage(rows []*model.LogStatistic) (int64, int64) {
	if len(rows) == 0 {
		return 0, 0
	}
	var requests int64
	var tokens int64
	for _, row := range rows {
		if row == nil {
			continue
		}
		requests += int64(row.RequestCount)
		tokens += int64(row.PromptTokens + row.CompletionTokens)
	}
	return requests, tokens
}

func normalizeSpendOverviewPeriod(raw string) string {
	period := strings.ToLower(strings.TrimSpace(raw))
	switch period {
	case "today",
		"last_7_days",
		"last_30_days",
		"this_month",
		"last_month",
		"this_year",
		"last_year",
		"last_12_months",
		"all_time":
		return period
	case "last_week":
		return "last_7_days"
	default:
		return "last_30_days"
	}
}

// GetUserSpendOverview godoc
// @Summary User spend overview
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param period query string false "today|last_7_days|last_30_days|this_month|last_month|this_year|last_year|last_12_months|all_time"
// @Param models query string false "Comma-separated model list"
// @Success 200 {object} docs.UserSpendOverviewResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/spend/overview [get]
func GetUserSpendOverview(c *gin.Context) {
	userId := c.GetString(ctxkey.Id)
	period := normalizeSpendOverviewPeriod(c.DefaultQuery("period", "last_30_days"))
	models := parseModelFilters(c.Query("models"))
	now := time.Now()
	todayStart := startOfDay(now)
	todayEnd := endOfDay(now)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	yesterdayEnd := todayStart.Add(-time.Second)

	var periodStart time.Time
	var periodEnd time.Time
	switch period {
	case "today":
		periodStart = todayStart
		periodEnd = todayEnd
	case "last_7_days":
		periodStart = todayStart.AddDate(0, 0, -6)
		periodEnd = todayEnd
	case "last_30_days":
		periodStart = todayStart.AddDate(0, 0, -29)
		periodEnd = todayEnd
	case "this_month":
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = todayEnd
	case "last_month":
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = monthStart.Add(-time.Second)
		periodStart = time.Date(periodEnd.Year(), periodEnd.Month(), 1, 0, 0, 0, 0, now.Location())
	case "this_year":
		periodStart = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
		periodEnd = todayEnd
	case "last_year":
		periodStart = time.Date(now.Year()-1, time.January, 1, 0, 0, 0, 0, now.Location())
		periodEnd = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
	case "last_12_months":
		periodStart = todayStart.AddDate(-1, 0, 0)
		periodEnd = todayEnd
	case "all_time":
		minTimestamp, err := logsvc.MinLogTimestampByUserId(userId, []int{model.LogTypeConsume, model.LogTypeTopup})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法获取统计信息",
			})
			return
		}
		if minTimestamp > 0 {
			periodStart = startOfDay(time.Unix(minTimestamp, 0).In(now.Location()))
			periodEnd = todayEnd
		}
	default:
		period = "last_30_days"
		periodStart = todayStart.AddDate(0, 0, -29)
		periodEnd = todayEnd
	}

	periodStartUnix := int64(0)
	periodEndUnix := int64(0)
	periodDays := int64(0)
	if !periodStart.IsZero() && !periodEnd.IsZero() {
		if periodStart.After(periodEnd) {
			periodStart, periodEnd = periodEnd, periodStart
		}
		periodStartUnix = periodStart.Unix()
		periodEndUnix = periodEnd.Unix()
		periodDays = (periodEndUnix-periodStartUnix)/86400 + 1
	}

	todayCost, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeConsume, userId, todayStart.Unix(), todayEnd.Unix(), models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	todayRevenue, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeTopup, userId, todayStart.Unix(), todayEnd.Unix(), models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	yesterdayCost, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeConsume, userId, yesterdayStart.Unix(), yesterdayEnd.Unix(), models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	yesterdayRevenue, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeTopup, userId, yesterdayStart.Unix(), yesterdayEnd.Unix(), models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	periodCost, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeConsume, userId, periodStartUnix, periodEndUnix, models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	periodRevenue, err := logsvc.SumUsedQuotaByUserIdWithModels(model.LogTypeTopup, userId, periodStartUnix, periodEndUnix, models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	periodUsageRows, err := usersvc.SearchLogsByPeriodAndModel(userId, int(periodStartUnix), int(periodEndUnix), "day", models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	todayUsageRows, err := usersvc.SearchLogsByPeriodAndModel(userId, int(todayStart.Unix()), int(todayEnd.Unix()), "day", models)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法获取统计信息",
		})
		return
	}
	periodRequests, periodTokens := summarizeUsage(periodUsageRows)
	todayRequests, todayTokens := summarizeUsage(todayUsageRows)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"today_cost":            todayCost,
			"today_yyc_cost":        todayCost,
			"today_revenue":         todayRevenue,
			"today_yyc_revenue":     todayRevenue,
			"today_requests":        todayRequests,
			"today_tokens":          todayTokens,
			"yesterday_cost":        yesterdayCost,
			"yesterday_yyc_cost":    yesterdayCost,
			"yesterday_revenue":     yesterdayRevenue,
			"yesterday_yyc_revenue": yesterdayRevenue,
			"period_cost":           periodCost,
			"period_yyc_cost":       periodCost,
			"period_revenue":        periodRevenue,
			"period_yyc_revenue":    periodRevenue,
			"period_requests":       periodRequests,
			"period_tokens":         periodTokens,
			"period_days":           periodDays,
			"period_start":          periodStartUnix,
			"period_end":            periodEndUnix,
			"yesterday_start":       yesterdayStart.Unix(),
			"yesterday_end":         yesterdayEnd.Unix(),
		},
	})
	return
}

// GenerateAccessToken godoc
// @Summary Generate access token for current user
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.UserAccessTokenResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/token [get]
func GenerateAccessToken(c *gin.Context) {
	id := c.GetString(ctxkey.Id)
	logger.Loginf(c.Request.Context(), "generate access token request user=%s", id)
	user, err := usersvc.GetByID(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.AccessToken = random.GetUUID()

	exists, err := usersvc.AccessTokenExists(user.AccessToken)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if exists {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请重试，系统生成的 UUID 竟然重复了！",
		})
		return
	}

	if err := usersvc.Update(user, false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	logger.Loginf(c.Request.Context(), "generate access token success user=%s token=%s", user.Id, user.AccessToken)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AccessToken,
	})
	return
}

// GetAffCode godoc
// @Summary Get affiliate code
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.UserAffCodeResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/aff [get]
func GetAffCode(c *gin.Context) {
	id := c.GetString(ctxkey.Id)
	user, err := usersvc.GetByID(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if user.AffCode == "" {
		user.AffCode = random.GetRandomString(4)
		if err := usersvc.Update(user, false); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    user.AffCode,
	})
	return
}

// GetSelf godoc
// @Summary Get current user profile
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.UserSelfResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/self [get]
func GetSelf(c *gin.Context) {
	id := c.GetString(ctxkey.Id)
	user, err := usersvc.GetByID(id, false)
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
		"data":    exposedUser(user),
	})
	return
}

func parseGroupReferences(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	items := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func resolveUserDailyQuotaGroupID(user *model.User, requestedGroupRef string) (string, error) {
	trimmedRequested := strings.TrimSpace(requestedGroupRef)
	if trimmedRequested != "" {
		groupCatalog, err := model.ResolveGroupCatalogByReference(trimmedRequested)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", fmt.Errorf("分组不存在")
			}
			return "", err
		}
		return strings.TrimSpace(groupCatalog.Id), nil
	}
	if user == nil {
		return "", fmt.Errorf("用户不存在")
	}
	if subscription, err := model.GetActiveUserPackageSubscription(strings.TrimSpace(user.Id)); err == nil {
		groupID := strings.TrimSpace(subscription.GroupID)
		if groupID != "" {
			return groupID, nil
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	groupRefs := parseGroupReferences(user.Group)
	if len(groupRefs) == 0 {
		return "", fmt.Errorf("当前用户未绑定分组")
	}
	for _, groupRef := range groupRefs {
		groupCatalog, err := model.ResolveGroupCatalogByReference(groupRef)
		if err != nil {
			continue
		}
		if groupID := strings.TrimSpace(groupCatalog.Id); groupID != "" {
			return groupID, nil
		}
	}
	return "", fmt.Errorf("当前用户未绑定有效分组")
}

// GetCurrentUserDailyQuota godoc
// @Summary Get current user's daily package quota snapshot
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param group_id query string false "Group ID (optional, defaults to user's first bound group)"
// @Param date query string false "Biz date in YYYY-MM-DD, defaults to today in group timezone"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/quota/daily [get]
func GetCurrentUserDailyQuota(c *gin.Context) {
	userID := c.GetString(ctxkey.Id)
	if strings.TrimSpace(userID) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 ID 不能为空",
		})
		return
	}
	user, err := usersvc.GetByID(userID, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	groupID, err := resolveUserDailyQuotaGroupID(user, c.Query("group_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	bizDate := strings.TrimSpace(c.Query("date"))
	snapshot, err := model.GetGroupDailyQuotaSnapshot(groupID, userID, bizDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	groupCatalog, _ := model.GetGroupCatalogByID(groupID)
	groupName := strings.TrimSpace(groupCatalog.Name)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewGroupDailyQuotaSnapshot(snapshot, groupName),
	})
}

// GetCurrentUserQuotaSummary godoc
// @Summary Get current user's quota summary
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param date query string false "Biz date in YYYY-MM-DD"
// @Param month query string false "Biz month in YYYY-MM"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/quota/summary [get]
func GetCurrentUserQuotaSummary(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 ID 不能为空",
		})
		return
	}
	summary, err := model.GetUserQuotaSummary(userID, c.Query("date"), c.Query("month"))
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
		"data":    presenter.NewUserQuotaSummary(summary),
	})
}

// GetUserQuotaSummary godoc
// @Summary Get user quota summary (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Param date query string false "Biz date in YYYY-MM-DD"
// @Param month query string false "Biz month in YYYY-MM"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/{id}/quota/summary [get]
func GetUserQuotaSummary(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	targetUser, err := usersvc.GetByID(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !requesterCanReadUser(c, targetUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权获取同级或更高等级用户的信息",
		})
		return
	}
	summary, err := model.GetUserQuotaSummary(id, c.Query("date"), c.Query("month"))
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
		"data":    presenter.NewUserQuotaSummary(summary),
	})
}

// UpdateUser godoc
// @Summary Update user (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.AdminUserUpdateRequest true "User update payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user [put]
func UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	var updatedUser model.User
	err := json.NewDecoder(c.Request.Body).Decode(&updatedUser)
	if err != nil || updatedUser.Id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	if updatedUser.Password == "" {
		updatedUser.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := common.Validate.Struct(&updatedUser); err != nil {
		logger.Loginf(c.Request.Context(), "update user invalid input err=%v id=%s username=%s display=%s role=%d status=%d quota=%d used=%d email=%s",
			err, updatedUser.Id, updatedUser.Username, updatedUser.DisplayName, updatedUser.Role, updatedUser.Status, updatedUser.Quota, updatedUser.UsedQuota, updatedUser.Email)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_input"),
		})
		return
	}
	if updatedUser.DailyQuotaLimit < 0 || updatedUser.PackageEmergencyQuotaLimit < 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_input"),
		})
		return
	}
	quotaResetTimezone, err := model.ValidateUserQuotaResetTimezone(updatedUser.QuotaResetTimezone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	updatedUser.QuotaResetTimezone = quotaResetTimezone
	originUser, err := usersvc.GetByID(updatedUser.Id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	myRole := c.GetInt(ctxkey.Role)
	targetEffectiveRole := model.EffectiveRole(originUser)
	requesterIsRoot := requesterIsRootUser(c)
	requesterSelf := requesterIsSelf(c, originUser.Id)
	updatedUser.UsedQuota = originUser.UsedQuota
	updatedUser.RequestCount = originUser.RequestCount
	updatedUser.HasPassword = originUser.HasPassword
	updatedUser.WalletAddress = originUser.WalletAddress
	roleChanged := updatedUser.Role != model.ExposedRole(originUser)
	statusChanged := updatedUser.Status != originUser.Status
	passwordChanged := updatedUser.Password != "$I_LOVE_U" && strings.TrimSpace(updatedUser.Password) != ""
	normalizedOriginWallet := ""
	if originUser.WalletAddress != nil {
		normalizedOriginWallet = model.NormalizeWalletAddress(*originUser.WalletAddress)
	}
	normalizedUpdatedWallet := ""
	if updatedUser.WalletAddress != nil {
		normalizedUpdatedWallet = model.NormalizeWalletAddress(*updatedUser.WalletAddress)
	}
	walletChanged := normalizedOriginWallet != normalizedUpdatedWallet
	privilegeSensitiveChanged := roleChanged || statusChanged || passwordChanged || walletChanged
	logger.Loginf(
		ctx,
		"update user permission check actor=%s role=%d target=%s target_role=%d role_changed=%t status_changed=%t password_changed=%t wallet_changed=%t",
		c.GetString(ctxkey.Id),
		myRole,
		originUser.Id,
		targetEffectiveRole,
		roleChanged,
		statusChanged,
		passwordChanged,
		walletChanged,
	)
	if targetEffectiveRole > myRole && !requesterIsRoot {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权更新同权限等级或更高权限等级的用户信息",
		})
		return
	}
	if targetEffectiveRole == myRole && privilegeSensitiveChanged && !requesterIsRoot && !requesterSelf {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权修改同权限等级用户的权限相关字段",
		})
		return
	}
	if myRole <= updatedUser.Role && !requesterIsRoot && roleChanged {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权将其他用户权限等级提升到大于等于自己的权限等级",
		})
		return
	}
	if updatedUser.Password == "$I_LOVE_U" {
		updatedUser.Password = "" // rollback to what it should be
	}
	if strings.TrimSpace(updatedUser.DisplayName) == "" {
		updatedUser.DisplayName = strings.TrimSpace(updatedUser.Username)
	}
	if updatedUser.Role > model.RoleAdminUser {
		updatedUser.Role = model.RoleAdminUser
	}
	updatePassword := updatedUser.Password != ""
	if updatePassword {
		updatedUser.HasPassword = true
	}
	if err := usersvc.Update(&updatedUser, updatePassword); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if originUser.Quota != updatedUser.Quota {
		usersvc.RecordLog(ctx, originUser.Id, model.LogTypeManage, fmt.Sprintf("管理员将用户额度从 %s修改为 %s", common.LogQuota(originUser.Quota), common.LogQuota(updatedUser.Quota)))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

// UpdateSelf godoc
// @Summary Update current user
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.UserSelfUpdateRequest true "Profile update payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/self [put]
func UpdateSelf(c *gin.Context) {
	var user model.User
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	if user.Password == "" {
		user.Password = "$I_LOVE_U" // make Validator happy :)
	}
	if err := common.Validate.Struct(&user); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "输入不合法 " + err.Error(),
		})
		return
	}

	originUser, err := usersvc.GetByID(c.GetString(ctxkey.Id), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	cleanUser := *originUser
	cleanUser.Username = user.Username
	cleanUser.Password = user.Password
	cleanUser.DisplayName = user.DisplayName
	cleanUser.Email = strings.TrimSpace(user.Email)
	if user.Password == "$I_LOVE_U" {
		user.Password = "" // rollback to what it should be
		cleanUser.Password = ""
	}
	if strings.TrimSpace(cleanUser.DisplayName) == "" {
		cleanUser.DisplayName = strings.TrimSpace(cleanUser.Username)
	}
	if cleanUser.Email != "" {
		if err := common.Validate.Var(cleanUser.Email, "email,max=50"); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "邮箱格式不正确",
			})
			return
		}
		currentUser, err := usersvc.GetByID(cleanUser.Id, false)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		if !strings.EqualFold(strings.TrimSpace(currentUser.Email), cleanUser.Email) && model.IsEmailAlreadyTaken(cleanUser.Email) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "邮箱已被使用",
			})
			return
		}
	}
	updatePassword := user.Password != ""
	if err := usersvc.Update(&cleanUser, updatePassword); err != nil {
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

// UpdateSelfPassword godoc
// @Summary Update current user password with current password verification
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/self/password [post]
func UpdateSelfPassword(c *gin.Context) {
	var req updateSelfPasswordRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if len(req.CurrentPassword) < 8 || len(req.NewPassword) < 8 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "密码长度不能少于 8 位",
		})
		return
	}
	userID := c.GetString(ctxkey.Id)
	originUser, err := usersvc.GetByID(userID, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !originUser.HasPassword {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前账户尚未设置密码，请先设置密码",
		})
		return
	}
	if !common.ValidatePasswordAndHash(req.CurrentPassword, originUser.Password) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "当前密码错误",
		})
		return
	}
	cleanUser := model.User{
		Id:          userID,
		Username:    originUser.Username,
		DisplayName: originUser.DisplayName,
		Password:    req.NewPassword,
		HasPassword: true,
	}
	if err := usersvc.Update(&cleanUser, true); err != nil {
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
}

// DeleteUser godoc
// @Summary Delete user (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/{id} [delete]
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "id 为空",
		})
		return
	}
	var err error
	originUser, err := usersvc.GetByID(id, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	myRole := c.GetInt("role")
	if model.IsProtectedRootUser(originUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法删除系统级管理员账户",
		})
		return
	}
	if myRole <= model.EffectiveRole(originUser) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权删除同权限等级或更高权限等级的用户",
		})
		return
	}
	err = usersvc.DeleteByID(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
		})
		return
	}
}

// DeleteSelf godoc
// @Summary Delete current user
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/self [delete]
func DeleteSelf(c *gin.Context) {
	id := c.GetString("id")
	user, _ := usersvc.GetByID(id, false)

	if model.IsProtectedRootUser(user) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "不能删除系统级管理员账户",
		})
		return
	}

	err := usersvc.DeleteByID(id)
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

// CreateUser godoc
// @Summary Create user (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.AdminCreateUserRequest true "Create user payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user [post]
func CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	var user model.User
	err := json.NewDecoder(c.Request.Body).Decode(&user)
	if err != nil || user.Username == "" || user.Password == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	if err := common.Validate.Struct(&user); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_input"),
		})
		return
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	myRole := c.GetInt("role")
	if user.Role >= myRole {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无法创建权限大于等于自己的用户",
		})
		return
	}
	// Even for admin users, we cannot fully trust them!
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.DisplayName,
		HasPassword: strings.TrimSpace(user.Password) != "",
	}
	if err := usersvc.Create(ctx, &cleanUser, ""); err != nil {
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

type ManageRequest struct {
	Username string `json:"username"`
	Action   string `json:"action"`
}

// ManageUser godoc
// @Summary Manage user (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.AdminManageUserRequest true "Manage user payload"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/user/manage [post]
// ManageUser Only admin user can do this
func ManageUser(c *gin.Context) {
	var req ManageRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.Translate(c, "invalid_parameter"),
		})
		return
	}
	user := model.User{
		Username: req.Username,
	}
	foundUser, err := usersvc.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "用户不存在",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if foundUser == nil || foundUser.Id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户不存在",
		})
		return
	}
	user = *foundUser
	myRole := c.GetInt("role")
	if myRole <= model.EffectiveRole(&user) && myRole != model.RoleRootUser {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无权更新同权限等级或更高权限等级的用户信息",
		})
		return
	}
	switch req.Action {
	case "disable":
		user.Status = model.UserStatusDisabled
		if model.IsProtectedRootUser(&user) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法禁用系统级管理员用户",
			})
			return
		}
	case "enable":
		user.Status = model.UserStatusEnabled
	case "delete":
		if model.IsProtectedRootUser(&user) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法删除系统级管理员用户",
			})
			return
		}
		if err := usersvc.Delete(&user); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "promote":
		if myRole != model.RoleRootUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "普通管理员用户无法提升其他用户为管理员",
			})
			return
		}
		if user.Role >= model.RoleAdminUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该用户已经是管理员",
			})
			return
		}
		user.Role = model.RoleAdminUser
	case "demote":
		if model.IsProtectedRootUser(&user) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法降级系统级管理员用户",
			})
			return
		}
		if user.Role == model.RoleCommonUser {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该用户已经是普通用户",
			})
			return
		}
		user.Role = model.RoleCommonUser
	}

	if err := usersvc.Update(&user, false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	clearUser := exposedUser(&model.User{Role: user.Role, Status: user.Status, WalletAddress: user.WalletAddress})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    clearUser,
	})
	return
}

func EmailBind(c *gin.Context) {
	email := c.Query("email")
	code := c.Query("code")
	if !common.VerifyCodeWithKey(email, code, common.EmailVerificationPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码错误或已过期",
		})
		return
	}
	id := c.GetString("id")
	user := model.User{
		Id: id,
	}
	err := usersvc.FillByID(&user)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	user.Email = email
	// no need to check if this email already taken, because we have used verification code to check it
	err = usersvc.Update(&user, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if user.Role == model.RoleRootUser {
		config.RootUserEmail = email
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

type topUpRequest struct {
	Code string `json:"code"`
	Key  string `json:"key"`
}

type topUpOrderResponseData struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	RedirectURL   string `json:"redirect_url"`
	CreatedAt     int64  `json:"created_at"`
}

type topUpOrderListData struct {
	Items    []model.TopupOrder `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type createTopUpOrderRequest struct {
	BusinessType string  `json:"business_type"`
	Title        string  `json:"title"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Quota        int64   `json:"quota"`
	PackageID    string  `json:"package_id"`
	ReturnURL    string  `json:"return_url"`
}

func parseTopupOrderPageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", strconv.Itoa(config.ItemsPerPage))))
	if pageSize <= 0 {
		pageSize = config.ItemsPerPage
	}
	return page, pageSize
}

// GetTopUpOrders godoc
// @Summary List current user top up orders
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} docs.UserTopUpOrderListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/topup/orders [get]
func GetTopUpOrders(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	page, pageSize := parseTopupOrderPageParams(c)
	items, total, err := model.ListTopupOrdersPageWithDB(model.DB, userID, page, pageSize)
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
		"data": topUpOrderListData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// CreateTopUpOrder godoc
// @Summary Create user top up order
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.UserCreateTopUpOrderResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/topup/orders [post]
func CreateTopUpOrder(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的 user id",
		})
		return
	}
	username := strings.TrimSpace(c.GetString(ctxkey.Username))
	if username == "" {
		username = strings.TrimSpace(c.GetString("username"))
	}
	if username == "" {
		user, err := usersvc.GetByID(userID, false)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		username = strings.TrimSpace(user.Username)
	}
	req := createTopUpOrderRequest{}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	order, err := model.CreateTopupOrderWithDB(model.DB, userID, username, model.CreateTopupOrderInput{
		BusinessType: req.BusinessType,
		Title:        req.Title,
		Amount:       req.Amount,
		Currency:     req.Currency,
		Quota:        req.Quota,
		PackageID:    req.PackageID,
		ReturnURL:    req.ReturnURL,
	})
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
		"data": topUpOrderResponseData{
			ID:            order.Id,
			TransactionID: order.TransactionID,
			Status:        order.Status,
			RedirectURL:   order.RedirectURL,
			CreatedAt:     order.CreatedAt,
		},
	})
}

// GetTopUpOrder godoc
// @Summary Get current user top up order detail
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} docs.UserTopUpOrderDetailResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/topup/orders/{id} [get]
func GetTopUpOrder(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.GetTopupOrderByIDWithDB(model.DB, orderID, userID)
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
		"data":    order,
	})
}

// TopUp godoc
// @Summary User top up
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.UserTopUpRequest true "Top up payload"
// @Success 200 {object} docs.UserTopUpResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/user/topup [post]
func TopUp(c *gin.Context) {
	ctx := c.Request.Context()
	req := topUpRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	id := c.GetString("id")
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = strings.TrimSpace(req.Key)
	}
	result, err := usersvc.Redeem(ctx, code, id)
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
		"data":    result,
	})
	return
}

type adminTopUpRequest struct {
	UserId string `json:"user_id"`
	Quota  int    `json:"quota"`
	Remark string `json:"remark"`
}

func AdminTopUp(c *gin.Context) {
	ctx := c.Request.Context()
	req := adminTopUpRequest{}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = usersvc.IncreaseQuota(req.UserId, int64(req.Quota))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if req.Remark == "" {
		req.Remark = fmt.Sprintf("通过 API 充值 %s", common.LogQuota(int64(req.Quota)))
	}
	usersvc.RecordTopupLog(ctx, req.UserId, req.Remark, req.Quota)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return startOfDay(t).AddDate(0, 0, 1).Add(-time.Second)
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return startOfDay(t).AddDate(0, 0, -(weekday - 1))
}
