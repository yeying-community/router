package user

import (
	"context"
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
	"github.com/yeying-community/router/common/helper"
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

const batchGrantUserTopUpPlanLimit = 200

var (
	cacheGetGroupModelsFn = model.CacheGetGroupModels
	getGroupCatalogByIDFn = model.GetGroupCatalogByID
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
	Id                         string                      `json:"id"`
	UserID                     string                      `json:"user_id"`
	PackageID                  string                      `json:"package_id"`
	PackageName                string                      `json:"package_name"`
	GroupID                    string                      `json:"group_id"`
	GroupName                  string                      `json:"group_name,omitempty"`
	PackageType                string                      `json:"package_type"`
	ScopeType                  string                      `json:"scope_type"`
	ScopeProvider              string                      `json:"scope_provider"`
	ScopeModel                 string                      `json:"scope_model"`
	ScopeEndpoint              string                      `json:"scope_endpoint"`
	QuotaMetric                string                      `json:"quota_metric"`
	PeriodType                 string                      `json:"period_type"`
	PeriodLimit                int64                       `json:"period_limit"`
	MaxConcurrencyPerUser      int                         `json:"max_concurrency_per_user"`
	MaxConcurrencyPerPackage   int                         `json:"max_concurrency_per_package"`
	AllowBalanceFallback       bool                        `json:"allow_balance_fallback"`
	DailyQuotaLimit            int64                       `json:"daily_quota_limit"`
	PackageEmergencyQuotaLimit int64                       `json:"package_emergency_quota_limit"`
	QuotaResetTimezone         string                      `json:"quota_reset_timezone"`
	StartedAt                  int64                       `json:"started_at"`
	ExpiresAt                  int64                       `json:"expires_at"`
	Status                     int                         `json:"status"`
	Source                     string                      `json:"source,omitempty"`
	SupportedModels            []string                    `json:"supported_models,omitempty"`
	Usage                      *activeUserPackageUsageView `json:"usage,omitempty"`
}

type activeUserPackageUsageView struct {
	Metric          string `json:"metric"`
	PeriodType      string `json:"period_type"`
	PeriodKey       string `json:"period_key"`
	LimitAmount     int64  `json:"limit_amount"`
	ConsumedAmount  int64  `json:"consumed_amount"`
	ReservedAmount  int64  `json:"reserved_amount"`
	RemainingAmount int64  `json:"remaining_amount"`
	Unlimited       bool   `json:"unlimited"`
	UpdatedAt       int64  `json:"updated_at"`
}

type activeUserPackageSubscriptionPayload struct {
	HasActiveSubscription bool                                 `json:"has_active_subscription"`
	CurrentPackage        *activeUserPackageSubscriptionView   `json:"current_package,omitempty"`
	NextPackage           *activeUserPackageSubscriptionView   `json:"next_package,omitempty"`
	Subscription          *activeUserPackageSubscriptionView   `json:"subscription,omitempty"`
	Subscriptions         []*activeUserPackageSubscriptionView `json:"subscriptions,omitempty"`
}

type userRecentRedemptionsPayload struct {
	Items []*presenter.Redemption `json:"items"`
}

type currentUserTopupRedemptionsData struct {
	Items    []*presenter.Redemption `json:"items"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Total    int64                   `json:"total"`
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

func buildActiveUserPackageUsageView(snapshot model.RequestPackageUsageSnapshot) *activeUserPackageUsageView {
	return &activeUserPackageUsageView{
		Metric:          strings.TrimSpace(snapshot.Metric),
		PeriodType:      strings.TrimSpace(snapshot.PeriodType),
		PeriodKey:       strings.TrimSpace(snapshot.PeriodKey),
		LimitAmount:     snapshot.LimitAmount,
		ConsumedAmount:  snapshot.ConsumedAmount,
		ReservedAmount:  snapshot.ReservedAmount,
		RemainingAmount: snapshot.RemainingAmount,
		Unlimited:       snapshot.Unlimited,
		UpdatedAt:       snapshot.UpdatedAt,
	}
}

func buildActiveUserPackageSubscriptionView(subscription model.UserPackageSubscription) (*activeUserPackageSubscriptionView, error) {
	groupID := strings.TrimSpace(subscription.GroupID)
	groupName := ""
	supportedModels := []string{}
	if groupID != "" {
		if groupCatalog, err := getGroupCatalogByIDFn(groupID); err == nil {
			groupName = strings.TrimSpace(groupCatalog.Name)
		}
		models, err := cacheGetGroupModelsFn(context.Background(), groupID)
		if err != nil {
			return nil, err
		}
		supportedModels = append(supportedModels, models...)
	}
	source := ""
	if packageID := strings.TrimSpace(subscription.PackageID); packageID != "" {
		if servicePackage, err := model.GetServicePackageByID(packageID); err == nil {
			source = strings.TrimSpace(servicePackage.Source)
		}
	}
	var usage *activeUserPackageUsageView
	if strings.TrimSpace(subscription.QuotaMetric) == model.ServicePackageQuotaMetricRequestCount {
		snapshot, err := model.GetRequestPackageUsageSnapshot(subscription)
		if err != nil {
			return nil, err
		}
		usage = buildActiveUserPackageUsageView(snapshot)
	}
	return &activeUserPackageSubscriptionView{
		Id:                         strings.TrimSpace(subscription.Id),
		UserID:                     strings.TrimSpace(subscription.UserID),
		PackageID:                  strings.TrimSpace(subscription.PackageID),
		PackageName:                strings.TrimSpace(subscription.PackageName),
		GroupID:                    groupID,
		GroupName:                  groupName,
		PackageType:                strings.TrimSpace(subscription.PackageType),
		ScopeType:                  strings.TrimSpace(subscription.ScopeType),
		ScopeProvider:              strings.TrimSpace(subscription.ScopeProvider),
		ScopeModel:                 strings.TrimSpace(subscription.ScopeModel),
		ScopeEndpoint:              strings.TrimSpace(subscription.ScopeEndpoint),
		QuotaMetric:                strings.TrimSpace(subscription.QuotaMetric),
		PeriodType:                 strings.TrimSpace(subscription.PeriodType),
		PeriodLimit:                subscription.PeriodLimit,
		MaxConcurrencyPerUser:      subscription.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage:   subscription.MaxConcurrencyPerPackage,
		AllowBalanceFallback:       subscription.AllowBalanceFallback,
		DailyQuotaLimit:            subscription.DailyQuotaLimit,
		PackageEmergencyQuotaLimit: subscription.PackageEmergencyQuotaLimit,
		QuotaResetTimezone:         strings.TrimSpace(subscription.QuotaResetTimezone),
		StartedAt:                  subscription.StartedAt,
		ExpiresAt:                  subscription.ExpiresAt,
		Status:                     subscription.Status,
		Source:                     source,
		SupportedModels:            supportedModels,
		Usage:                      usage,
	}, nil
}

func selectPrimaryActiveUserPackageSubscriptionView(views []*activeUserPackageSubscriptionView) *activeUserPackageSubscriptionView {
	for _, view := range views {
		if view == nil {
			continue
		}
		if strings.TrimSpace(view.PackageType) == model.ServicePackageTypeYYCQuota &&
			strings.TrimSpace(view.QuotaMetric) == model.ServicePackageQuotaMetricYYC {
			return view
		}
	}
	if len(views) == 0 {
		return nil
	}
	return views[0]
}

func loadActiveUserPackageSubscriptionPayload(userID string) (activeUserPackageSubscriptionPayload, error) {
	subscriptions, err := model.ListActiveUserPackageSubscriptions(strings.TrimSpace(userID))
	if err != nil {
		return activeUserPackageSubscriptionPayload{}, err
	}
	var nextPackageView *activeUserPackageSubscriptionView
	nextSubscription, nextErr := model.GetNextUserPackageSubscription(strings.TrimSpace(userID))
	if nextErr == nil {
		nextPackageView, err = buildActiveUserPackageSubscriptionView(nextSubscription)
		if err != nil {
			return activeUserPackageSubscriptionPayload{}, err
		}
	} else if !errors.Is(nextErr, gorm.ErrRecordNotFound) {
		return activeUserPackageSubscriptionPayload{}, nextErr
	}
	if len(subscriptions) == 0 {
		return activeUserPackageSubscriptionPayload{
			HasActiveSubscription: false,
			NextPackage:           nextPackageView,
		}, nil
	}
	views := make([]*activeUserPackageSubscriptionView, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		view, err := buildActiveUserPackageSubscriptionView(subscription)
		if err != nil {
			return activeUserPackageSubscriptionPayload{}, err
		}
		views = append(views, view)
	}
	currentPackage := selectPrimaryActiveUserPackageSubscriptionView(views)
	return activeUserPackageSubscriptionPayload{
		HasActiveSubscription: true,
		CurrentPackage:        currentPackage,
		NextPackage:           nextPackageView,
		Subscription:          currentPackage,
		Subscriptions:         views,
	}, nil
}

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
	payload, err := loadActiveUserPackageSubscriptionPayload(id)
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
		"data":    payload,
	})
}

func GetCurrentUserActivePackageSubscription(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 ID 不能为空",
		})
		return
	}
	payload, err := loadActiveUserPackageSubscriptionPayload(userID)
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
		"data":    payload,
	})
}

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

func GetCurrentUserTopupRedemptions(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 ID 不能为空",
		})
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", strconv.Itoa(config.ItemsPerPage))))
	if pageSize <= 0 {
		pageSize = config.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	baseQuery := model.DB.Model(&model.Redemption{}).
		Where("redeemed_by_user_id = ? AND redeemed_time > 0 AND status = ?", userID, model.RedemptionCodeStatusUsed)
	if err := baseQuery.Count(&total).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	rows := make([]*model.Redemption, 0, pageSize)
	if err := baseQuery.
		Order("redeemed_time desc, id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.RedeemedByUsername = model.GetUsernameById(userID)
		groupID := strings.TrimSpace(row.GroupID)
		if groupID == "" {
			continue
		}
		if groupRow, err := model.GetGroupCatalogByID(groupID); err == nil {
			row.GroupName = strings.TrimSpace(groupRow.Name)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": currentUserTopupRedemptionsData{
			Items:    presenter.NewRedemptions(rows),
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	})
}

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
			"today_cost":               todayCost,
			"today_charge_amount":      todayCost,
			"today_revenue":            todayRevenue,
			"today_revenue_amount":     todayRevenue,
			"today_requests":           todayRequests,
			"today_tokens":             todayTokens,
			"yesterday_cost":           yesterdayCost,
			"yesterday_charge_amount":  yesterdayCost,
			"yesterday_revenue":        yesterdayRevenue,
			"yesterday_revenue_amount": yesterdayRevenue,
			"period_cost":              periodCost,
			"period_charge_amount":     periodCost,
			"period_revenue":           periodRevenue,
			"period_revenue_amount":    periodRevenue,
			"period_requests":          periodRequests,
			"period_tokens":            periodTokens,
			"period_days":              periodDays,
			"period_start":             periodStartUnix,
			"period_end":               periodEndUnix,
			"yesterday_start":          yesterdayStart.Unix(),
			"yesterday_end":            yesterdayEnd.Unix(),
		},
	})
	return
}

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

func GetUserTopUpBalanceLots(c *gin.Context) {
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
	page, pageSize, sourceType, status, positiveOnly, err := parseTopupBalanceLotPageParams(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if _, expireErr := model.ExpireUserBalanceLots(id); expireErr != nil {
		logger.Error(c.Request.Context(), "expire user balance lots failed: "+expireErr.Error())
	}
	items, total, err := model.ListUserBalanceLotsPageWithDB(model.DB, id, sourceType, status, page, pageSize, positiveOnly)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	adminItems, err := buildAdminTopUpBalanceLotListItemsWithSources(model.DB, items)
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
		"data": adminTopUpBalanceLotListData{
			Items:    adminItems,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GetUserTopUpBalanceLotTransactions(c *gin.Context) {
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
	page, pageSize, sourceType, txType, err := parseTopupBalanceLotTransactionPageParams(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if _, expireErr := model.ExpireUserBalanceLots(id); expireErr != nil {
		logger.Error(c.Request.Context(), "expire user balance lots failed: "+expireErr.Error())
	}
	items, total, err := model.ListUserBalanceLotTransactionsPageWithDB(model.DB, id, sourceType, txType, page, pageSize)
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
		"data": topUpBalanceLotTransactionListData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GrantUserTopUpPlan(c *gin.Context) {
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
	req := grantUserTopUpPlanRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	order, err := model.GrantTopupPlanToUserWithDB(
		model.DB,
		strings.TrimSpace(targetUser.Id),
		strings.TrimSpace(targetUser.Username),
		strings.TrimSpace(req.PlanID),
		strings.TrimSpace(c.GetString(ctxkey.Id)),
	)
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

func BatchGrantUserTopUpPlan(c *gin.Context) {
	req := batchGrantUserTopUpPlanRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	userIDs, err := normalizeBatchGrantUserIDs(req.UserIDs, batchGrantUserTopUpPlanLimit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	planID := strings.TrimSpace(req.PlanID)
	if _, err := model.ResolveTopupPlan(planID); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	grantedBy := strings.TrimSpace(c.GetString(ctxkey.Id))
	items := make([]batchGrantUserTopUpResult, 0, len(userIDs))
	succeeded := 0
	for _, userID := range userIDs {
		item := batchGrantUserTopUpResult{
			UserID: userID,
		}
		targetUser, err := usersvc.GetByID(userID, false)
		if err != nil {
			item.Message = err.Error()
			items = append(items, item)
			continue
		}
		item.Username = strings.TrimSpace(targetUser.Username)
		if !requesterCanReadUser(c, targetUser) {
			item.Message = "无权获取同级或更高等级用户的信息"
			items = append(items, item)
			continue
		}
		order, err := model.GrantTopupPlanToUserWithDB(
			model.DB,
			strings.TrimSpace(targetUser.Id),
			strings.TrimSpace(targetUser.Username),
			planID,
			grantedBy,
		)
		if err != nil {
			item.Message = err.Error()
			items = append(items, item)
			continue
		}
		item.Success = true
		item.OrderID = strings.TrimSpace(order.Id)
		succeeded++
		items = append(items, item)
	}
	response := batchGrantUserTopUpResponse{
		Total:     len(userIDs),
		Succeeded: succeeded,
		Failed:    len(userIDs) - succeeded,
		Items:     items,
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

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

type topUpBalanceSummaryData struct {
	TotalBalanceAmount  int64 `json:"total_balance_amount"`
	TopupBalanceAmount  int64 `json:"topup_balance_amount"`
	RedeemBalanceAmount int64 `json:"redeem_balance_amount"`
}

type topUpBalanceLotListData struct {
	Items    []model.UserBalanceLot `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type adminTopUpBalanceLotListItem struct {
	model.UserBalanceLot
	SourceDetail *topUpBalanceLotSourceDetail `json:"source_detail,omitempty"`
}

type adminTopUpBalanceLotListData struct {
	Items    []adminTopUpBalanceLotListItem `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
}

type topUpBalanceLotSourceDetail struct {
	Type         string  `json:"type"`
	ID           string  `json:"id"`
	Title        string  `json:"title,omitempty"`
	Status       string  `json:"status,omitempty"`
	Amount       float64 `json:"amount,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	CreditAmount int64   `json:"credit_amount,omitempty"`
	OccurredAt   int64   `json:"occurred_at,omitempty"`
	DetailPath   string  `json:"detail_path,omitempty"`
}

func buildAdminTopUpBalanceLotListItemsWithSources(db *gorm.DB, lots []model.UserBalanceLot) ([]adminTopUpBalanceLotListItem, error) {
	items := make([]adminTopUpBalanceLotListItem, 0, len(lots))
	if len(lots) == 0 {
		return items, nil
	}
	topupIDs := make([]string, 0)
	redemptionIDs := make([]string, 0)
	for _, lot := range lots {
		items = append(items, adminTopUpBalanceLotListItem{UserBalanceLot: lot})
		sourceID := strings.TrimSpace(lot.SourceID)
		if sourceID == "" {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(lot.SourceType)) {
		case model.UserBalanceLotSourceTopup:
			topupIDs = appendUniqueString(topupIDs, sourceID)
		case model.UserBalanceLotSourceRedeem:
			redemptionIDs = appendUniqueString(redemptionIDs, sourceID)
		}
	}
	if db == nil {
		return items, fmt.Errorf("database handle is nil")
	}
	topupDetails, err := loadTopupBalanceLotTopupSourceDetails(db, topupIDs)
	if err != nil {
		return nil, err
	}
	redemptionDetails, err := loadTopupBalanceLotRedemptionSourceDetails(db, redemptionIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		sourceID := strings.TrimSpace(items[i].SourceID)
		if sourceID == "" {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(items[i].SourceType)) {
		case model.UserBalanceLotSourceTopup:
			if detail, ok := topupDetails[sourceID]; ok {
				items[i].SourceDetail = &detail
			}
		case model.UserBalanceLotSourceRedeem:
			if detail, ok := redemptionDetails[sourceID]; ok {
				items[i].SourceDetail = &detail
			}
		}
	}
	return items, nil
}

func appendUniqueString(items []string, value string) []string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return items
	}
	for _, item := range items {
		if item == normalized {
			return items
		}
	}
	return append(items, normalized)
}

func loadTopupBalanceLotTopupSourceDetails(db *gorm.DB, ids []string) (map[string]topUpBalanceLotSourceDetail, error) {
	details := make(map[string]topUpBalanceLotSourceDetail, len(ids))
	if len(ids) == 0 {
		return details, nil
	}
	orders := make([]model.TopupOrder, 0, len(ids))
	if err := db.Where("id IN ?", ids).Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, order := range orders {
		id := strings.TrimSpace(order.Id)
		if id == "" {
			continue
		}
		occurredAt := order.RedeemedAt
		if occurredAt <= 0 {
			occurredAt = order.PaidAt
		}
		if occurredAt <= 0 {
			occurredAt = order.CreatedAt
		}
		details[id] = topUpBalanceLotSourceDetail{
			Type:         model.UserBalanceLotSourceTopup,
			ID:           id,
			Title:        strings.TrimSpace(order.Title),
			Status:       strings.TrimSpace(order.Status),
			Amount:       order.Amount,
			Currency:     strings.TrimSpace(order.Currency),
			CreditAmount: order.Quota,
			OccurredAt:   occurredAt,
			DetailPath:   "/admin/flow/topup/" + id,
		}
	}
	return details, nil
}

func loadTopupBalanceLotRedemptionSourceDetails(db *gorm.DB, ids []string) (map[string]topUpBalanceLotSourceDetail, error) {
	details := make(map[string]topUpBalanceLotSourceDetail, len(ids))
	if len(ids) == 0 {
		return details, nil
	}
	redemptions := make([]model.Redemption, 0, len(ids))
	if err := db.
		Where("id IN ? AND redeemed_time > 0 AND status = ?", ids, model.RedemptionCodeStatusUsed).
		Find(&redemptions).Error; err != nil {
		return nil, err
	}
	for _, redemption := range redemptions {
		id := strings.TrimSpace(redemption.Id)
		if id == "" {
			continue
		}
		occurredAt := redemption.RedeemedTime
		if occurredAt <= 0 {
			occurredAt = redemption.CreatedTime
		}
		details[id] = topUpBalanceLotSourceDetail{
			Type:         model.UserBalanceLotSourceRedeem,
			ID:           id,
			Title:        strings.TrimSpace(redemption.Name),
			Status:       "used",
			Amount:       redemption.FaceValueAmount,
			Currency:     strings.TrimSpace(redemption.FaceValueUnit),
			CreditAmount: redemption.Quota,
			OccurredAt:   occurredAt,
			DetailPath:   "/admin/flow/redemption/" + id,
		}
	}
	return details, nil
}

func writeTopUpError(c *gin.Context, err error) {
	response := gin.H{
		"success": false,
		"message": err.Error(),
	}
	if code := model.TopupErrorCode(err); code != "" {
		response["data"] = gin.H{"code": code}
	}
	c.JSON(http.StatusOK, response)
}

type topUpBalanceLotTransactionListData struct {
	Items    []model.UserBalanceLotTransaction `json:"items"`
	Total    int64                             `json:"total"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"page_size"`
}

type createTopUpOrderRequest struct {
	BusinessType  string  `json:"business_type"`
	OperationType string  `json:"operation_type"`
	ClientType    string  `json:"client_type"`
	Title         string  `json:"title"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Quota         int64   `json:"quota"`
	PlanID        string  `json:"plan_id"`
	PackageID     string  `json:"package_id"`
	ReturnURL     string  `json:"return_url"`
}

type grantUserTopUpPlanRequest struct {
	PlanID string `json:"plan_id"`
}

type batchGrantUserTopUpPlanRequest struct {
	UserIDs []string `json:"user_ids"`
	PlanID  string   `json:"plan_id"`
}

type batchGrantUserTopUpResult struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Success  bool   `json:"success"`
	Message  string `json:"message,omitempty"`
	OrderID  string `json:"order_id,omitempty"`
}

type batchGrantUserTopUpResponse struct {
	Total     int                         `json:"total"`
	Succeeded int                         `json:"succeeded"`
	Failed    int                         `json:"failed"`
	Items     []batchGrantUserTopUpResult `json:"items"`
}

type previewPackagePurchaseRequest struct {
	PackageID     string `json:"package_id"`
	OperationType string `json:"operation_type"`
}

func normalizeBatchGrantUserIDs(rawUserIDs []string, limit int) ([]string, error) {
	if len(rawUserIDs) == 0 {
		return nil, fmt.Errorf("请选择用户")
	}
	if limit <= 0 {
		limit = batchGrantUserTopUpPlanLimit
	}
	result := make([]string, 0, len(rawUserIDs))
	seen := make(map[string]struct{}, len(rawUserIDs))
	for _, rawUserID := range rawUserIDs {
		userID := strings.TrimSpace(rawUserID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		if len(result) >= limit {
			return nil, fmt.Errorf("单次最多选择 %d 个用户", limit)
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("请选择用户")
	}
	return result, nil
}

func parseTopupOrderPageParams(c *gin.Context) (int, int, string, error) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", strconv.Itoa(config.ItemsPerPage))))
	if pageSize <= 0 {
		pageSize = config.ItemsPerPage
	}
	rawBusinessType := strings.TrimSpace(c.Query("business_type"))
	if rawBusinessType == "" {
		return page, pageSize, "", nil
	}
	normalizedBusinessType := strings.TrimSpace(rawBusinessType)
	switch normalizedBusinessType {
	case model.TopupOrderBusinessBalance, model.TopupOrderBusinessPackage:
		return page, pageSize, normalizedBusinessType, nil
	default:
		return page, pageSize, "", fmt.Errorf("无效的业务类型")
	}
}

func parseTopupBalanceLotPageParams(c *gin.Context) (int, int, string, string, bool, error) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "20")))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	rawSourceType := strings.TrimSpace(strings.ToLower(c.Query("source_type")))
	sourceType := ""
	switch rawSourceType {
	case "":
		sourceType = ""
	case model.UserBalanceLotSourceTopup, model.UserBalanceLotSourceRedeem, model.UserBalanceLotSourceLegacy:
		sourceType = rawSourceType
	default:
		return 0, 0, "", "", false, fmt.Errorf("无效的来源类型")
	}
	rawStatus := strings.TrimSpace(strings.ToLower(c.Query("status")))
	status := ""
	switch rawStatus {
	case "":
		status = ""
	case model.UserBalanceLotStatusActive, model.UserBalanceLotStatusExhaust, model.UserBalanceLotStatusExpired:
		status = rawStatus
	default:
		return 0, 0, "", "", false, fmt.Errorf("无效的状态")
	}
	positiveOnly := false
	rawPositiveOnly := strings.TrimSpace(strings.ToLower(c.Query("positive_only")))
	switch rawPositiveOnly {
	case "":
		positiveOnly = false
	case "1", "true", "yes", "y":
		positiveOnly = true
	case "0", "false", "no", "n":
		positiveOnly = false
	default:
		return 0, 0, "", "", false, fmt.Errorf("无效的 positive_only 参数")
	}
	return page, pageSize, sourceType, status, positiveOnly, nil
}

func parseTopupBalanceLotTransactionPageParams(c *gin.Context) (int, int, string, string, error) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "20")))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	rawSourceType := strings.TrimSpace(strings.ToLower(c.Query("source_type")))
	sourceType := ""
	switch rawSourceType {
	case "":
		sourceType = ""
	case model.UserBalanceLotSourceTopup, model.UserBalanceLotSourceRedeem, model.UserBalanceLotSourceLegacy:
		sourceType = rawSourceType
	default:
		return 0, 0, "", "", fmt.Errorf("无效的来源类型")
	}
	rawTxType := strings.TrimSpace(strings.ToLower(c.Query("tx_type")))
	txType := ""
	switch rawTxType {
	case "":
		txType = ""
	case model.UserBalanceLotTxTypeCredit, model.UserBalanceLotTxTypeConsume, model.UserBalanceLotTxTypeExpire:
		txType = rawTxType
	default:
		return 0, 0, "", "", fmt.Errorf("无效的交易类型")
	}
	return page, pageSize, sourceType, txType, nil
}

func minInt64(a int64, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

func normalizeNonNegativeQuota(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func buildTopUpBalanceSummary(totalBalance int64, topupRemain int64, redeemRemain int64) topUpBalanceSummaryData {
	normalizedTotal := normalizeNonNegativeQuota(totalBalance)
	normalizedTopup := normalizeNonNegativeQuota(topupRemain)
	normalizedRedeem := normalizeNonNegativeQuota(redeemRemain)

	sumRemain := normalizedTopup + normalizedRedeem
	if sumRemain < normalizedTotal {
		// For historical/manual balance adjustments without source information,
		// attribute the residual to top-up balance so the split always matches total.
		normalizedTopup += normalizedTotal - sumRemain
		sumRemain = normalizedTotal
	}
	if sumRemain > normalizedTotal {
		overflow := sumRemain - normalizedTotal
		reduceTopup := minInt64(overflow, normalizedTopup)
		normalizedTopup -= reduceTopup
		overflow -= reduceTopup
		if overflow > 0 {
			normalizedRedeem -= minInt64(overflow, normalizedRedeem)
		}
	}

	return topUpBalanceSummaryData{
		TotalBalanceAmount:  normalizedTotal,
		TopupBalanceAmount:  normalizedTopup,
		RedeemBalanceAmount: normalizedRedeem,
	}
}

func GetCurrentUserTopUpBalanceSummary(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的 user id",
		})
		return
	}
	if _, expireErr := model.ExpireUserBalanceLots(userID); expireErr != nil {
		logger.Error(c.Request.Context(), "expire user balance lots failed: "+expireErr.Error())
	}

	user, err := usersvc.GetByID(userID, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var topupRemain int64
	if err := model.DB.Model(&model.UserBalanceLot{}).
		Select("COALESCE(SUM(remaining_amount), 0)").
		Where("user_id = ? AND source_type = ? AND remaining_amount > 0", userID, model.UserBalanceLotSourceTopup).
		Scan(&topupRemain).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var redeemRemain int64
	if err := model.DB.Model(&model.UserBalanceLot{}).
		Select("COALESCE(SUM(remaining_amount), 0)").
		Where("user_id = ? AND source_type = ? AND remaining_amount > 0", userID, model.UserBalanceLotSourceRedeem).
		Scan(&redeemRemain).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildTopUpBalanceSummary(user.Quota, topupRemain, redeemRemain),
	})
}

func GetCurrentUserTopUpBalanceLots(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的 user id",
		})
		return
	}
	page, pageSize, sourceType, status, positiveOnly, err := parseTopupBalanceLotPageParams(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if _, expireErr := model.ExpireUserBalanceLots(userID); expireErr != nil {
		logger.Error(c.Request.Context(), "expire user balance lots failed: "+expireErr.Error())
	}
	items, total, err := model.ListUserBalanceLotsPageWithDB(model.DB, userID, sourceType, status, page, pageSize, positiveOnly)
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
		"data": topUpBalanceLotListData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GetCurrentUserTopUpBalanceLotTransactions(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的 user id",
		})
		return
	}
	page, pageSize, sourceType, txType, err := parseTopupBalanceLotTransactionPageParams(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if _, expireErr := model.ExpireUserBalanceLots(userID); expireErr != nil {
		logger.Error(c.Request.Context(), "expire user balance lots failed: "+expireErr.Error())
	}
	items, total, err := model.ListUserBalanceLotTransactionsPageWithDB(model.DB, userID, sourceType, txType, page, pageSize)
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
		"data": topUpBalanceLotTransactionListData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

func GetTopUpOrders(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	page, pageSize, businessType, err := parseTopupOrderPageParams(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	items, total, err := model.ListTopupOrdersPageWithDB(model.DB, userID, businessType, page, pageSize)
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

func PreviewPackagePurchase(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的 user id",
		})
		return
	}
	req := previewPackagePurchaseRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	preview, err := model.PreviewPackagePurchaseWithDB(
		model.DB,
		userID,
		strings.TrimSpace(req.PackageID),
		strings.TrimSpace(req.OperationType),
		helper.GetTimestamp(),
	)
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
		"data":    preview,
	})
}

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
		BusinessType:  req.BusinessType,
		OperationType: req.OperationType,
		ClientType:    resolveTopUpClientType(req.ClientType, c.Request.UserAgent()),
		Title:         req.Title,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Quota:         req.Quota,
		PlanID:        req.PlanID,
		PackageID:     req.PackageID,
		ReturnURL:     req.ReturnURL,
	})
	if err != nil {
		writeTopUpError(c, err)
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

func resolveTopUpClientType(rawClientType string, userAgent string) string {
	clientType := strings.TrimSpace(strings.ToLower(rawClientType))
	switch clientType {
	case "pc", "mobile":
		return clientType
	}
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if strings.Contains(ua, "mobile") ||
		strings.Contains(ua, "android") ||
		strings.Contains(ua, "iphone") ||
		strings.Contains(ua, "ipad") {
		return "mobile"
	}
	return "pc"
}

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

func RefreshTopUpOrder(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.RefreshTopupOrderStatusWithDB(model.DB, orderID, userID)
	if err != nil {
		writeTopUpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    order,
	})
}

func CancelTopUpOrder(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(ctxkey.Id))
	orderID := strings.TrimSpace(c.Param("id"))
	order, err := model.CancelTopupOrderWithDB(model.DB, orderID, userID)
	if err != nil {
		writeTopUpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    order,
	})
}

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
