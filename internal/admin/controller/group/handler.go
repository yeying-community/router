package group

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	groupsvc "github.com/yeying-community/router/internal/admin/service/group"
)

type upsertGroupRequest struct {
	Id                 string                       `json:"id"`
	Name               string                       `json:"name"`
	Description        string                       `json:"description"`
	BillingRatio       *float64                     `json:"billing_ratio"`
	DailyQuotaLimit    *int64                       `json:"daily_quota_limit"`
	QuotaResetTimezone *string                      `json:"quota_reset_timezone"`
	Enabled            *bool                        `json:"enabled"`
	SortOrder          int                          `json:"sort_order"`
	ChannelIDs         []string                     `json:"channel_ids"`
	ModelConfigs       []model.GroupModelConfigItem `json:"model_configs"`
}

type updateGroupChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids"`
}

const maxGroupListPageSize = 100

type groupListPageData struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func parseGroupListPageParams(c *gin.Context) (page int, pageSize int, keyword string) {
	page = 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	pageSize = config.ItemsPerPage
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxGroupListPageSize {
		pageSize = maxGroupListPageSize
	}
	keyword = strings.TrimSpace(c.Query("keyword"))
	return page, pageSize, keyword
}

func listGroupsPage(page int, pageSize int, keyword string) (groupListPageData, error) {
	rows, total, err := groupsvc.ListPage(page, pageSize, keyword)
	if err != nil {
		return groupListPageData{}, err
	}
	return groupListPageData{
		Items:    rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetGroups godoc
// @Summary List groups with pagination (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param page_size query int false "Page size"
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/groups [get]
func GetGroups(c *gin.Context) {
	page, pageSize, keyword := parseGroupListPageParams(c)
	data, err := listGroupsPage(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	rows, _ := data.Items.([]model.GroupCatalog)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": groupListPageData{
			Items:    presenter.NewGroups(rows),
			Total:    data.Total,
			Page:     data.Page,
			PageSize: data.PageSize,
		},
	})
}

// GetGroup godoc
// @Summary Get group detail by ID (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id} [get]
func GetGroup(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	row, err := groupsvc.Get(id)
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
		"data":    presenter.NewGroup(&row),
	})
}

// GetGroupDailyQuota godoc
// @Summary Get group daily quota snapshot by date (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Param user_id query string true "User ID"
// @Param date query string false "Biz date in YYYY-MM-DD, defaults to today in group timezone"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/quota/daily [get]
func GetGroupDailyQuota(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	userID := strings.TrimSpace(c.Query("user_id"))
	if userID == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户 ID 不能为空",
		})
		return
	}
	bizDate := strings.TrimSpace(c.Query("date"))
	data, err := groupsvc.GetDailyQuotaSnapshot(id, userID, bizDate)
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
		"data":    presenter.NewGroupDailyQuotaSnapshot(data, ""),
	})
}

// CreateGroup godoc
// @Summary Create group (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group [post]
func CreateGroup(c *gin.Context) {
	req := upsertGroupRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	billingRatio, err := resolveCreateBillingRatio(req.BillingRatio)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	dailyQuotaLimit, err := resolveCreateDailyQuotaLimit(req.DailyQuotaLimit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	quotaResetTimezone, err := resolveCreateQuotaResetTimezone(req.QuotaResetTimezone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	createItem := model.GroupCatalog{
		Id:                 strings.TrimSpace(req.Id),
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		Source:             "manual",
		BillingRatio:       billingRatio,
		DailyQuotaLimit:    dailyQuotaLimit,
		QuotaResetTimezone: quotaResetTimezone,
	}
	row := model.GroupCatalog{}
	if req.ModelConfigs != nil {
		row, err = groupsvc.CreateWithConfig(createItem, req.ChannelIDs, req.ModelConfigs)
	} else {
		row, err = groupsvc.CreateWithChannelBindings(createItem, req.ChannelIDs)
	}
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
		"data":    row,
	})
}

// UpdateGroup godoc
// @Summary Update group (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group [put]
func UpdateGroup(c *gin.Context) {
	req := upsertGroupRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	current, findErr := groupsvc.Get(strings.TrimSpace(req.Id))
	if findErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": findErr.Error(),
		})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	} else {
		enabled = current.Enabled
	}
	billingRatio, err := resolveUpdateBillingRatio(req.BillingRatio, current.BillingRatio)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	dailyQuotaLimit, err := resolveUpdateDailyQuotaLimit(req.DailyQuotaLimit, current.DailyQuotaLimit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	quotaResetTimezone, err := resolveUpdateQuotaResetTimezone(req.QuotaResetTimezone, current.QuotaResetTimezone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	item := model.GroupCatalog{
		Id:                 strings.TrimSpace(req.Id),
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		BillingRatio:       billingRatio,
		DailyQuotaLimit:    dailyQuotaLimit,
		QuotaResetTimezone: quotaResetTimezone,
		Enabled:            enabled,
		SortOrder:          req.SortOrder,
	}
	row := model.GroupCatalog{}
	if req.ModelConfigs != nil {
		row, err = groupsvc.UpdateWithConfig(item, req.ChannelIDs, req.ModelConfigs, req.ChannelIDs != nil, true)
	} else if req.ChannelIDs != nil {
		row, err = groupsvc.UpdateWithChannelBindings(item, req.ChannelIDs)
	} else {
		row, err = groupsvc.Update(item)
	}
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
		"data":    row,
	})
}

// DeleteGroup godoc
// @Summary Delete group (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id} [delete]
func DeleteGroup(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	if err := groupsvc.Delete(id); err != nil {
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

func resolveCreateBillingRatio(value *float64) (float64, error) {
	if value == nil {
		return 1, nil
	}
	if *value < 0 {
		return 0, errors.New("分组倍率不能小于 0")
	}
	return *value, nil
}

func resolveUpdateBillingRatio(value *float64, fallback float64) (float64, error) {
	if value == nil {
		return fallback, nil
	}
	if *value < 0 {
		return 0, errors.New("分组倍率不能小于 0")
	}
	return *value, nil
}

func resolveCreateDailyQuotaLimit(value *int64) (int64, error) {
	if value == nil {
		return 0, nil
	}
	if *value < 0 {
		return 0, errors.New("分组每日额度上限不能小于 0")
	}
	return *value, nil
}

func resolveUpdateDailyQuotaLimit(value *int64, fallback int64) (int64, error) {
	if value == nil {
		return fallback, nil
	}
	if *value < 0 {
		return 0, errors.New("分组每日额度上限不能小于 0")
	}
	return *value, nil
}

func resolveCreateQuotaResetTimezone(value *string) (string, error) {
	if value == nil {
		return model.DefaultGroupQuotaResetTimezone, nil
	}
	return model.ValidateGroupQuotaResetTimezone(*value)
}

func resolveUpdateQuotaResetTimezone(value *string, fallback string) (string, error) {
	if value == nil {
		return model.ValidateGroupQuotaResetTimezone(fallback)
	}
	return model.ValidateGroupQuotaResetTimezone(*value)
}

// GetGroupChannels godoc
// @Summary List group channel bindings (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/channels [get]
func GetGroupChannels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	rows, err := groupsvc.ListChannelBindings(id)
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
		"data":    rows,
	})
}

// GetGroupModels godoc
// @Summary List group model summaries (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/models [get]
func GetGroupModels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	rows, err := groupsvc.ListModelSummaries(id)
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
		"data":    rows,
	})
}

// GetGroupModelConfigs godoc
// @Summary List group model configs (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/model-configs [get]
func GetGroupModelConfigs(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	payload, err := groupsvc.GetModelConfigPayload(id)
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

type updateGroupModelConfigsRequest struct {
	ChannelIDs   []string                     `json:"channel_ids"`
	ModelConfigs []model.GroupModelConfigItem `json:"model_configs"`
}

// UpdateGroupModelConfigs godoc
// @Summary Update group model configs (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/model-configs [put]
func UpdateGroupModelConfigs(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}

	req := updateGroupModelConfigsRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := groupsvc.ReplaceModelConfigs(id, req.ChannelIDs, req.ModelConfigs, req.ChannelIDs != nil); err != nil {
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

// UpdateGroupChannels godoc
// @Summary Update group channel bindings (admin)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/{id}/channels [put]
func UpdateGroupChannels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}

	req := updateGroupChannelsRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := groupsvc.ReplaceChannelBindings(id, req.ChannelIDs); err != nil {
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
