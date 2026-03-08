package group

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/internal/admin/model"
	groupsvc "github.com/yeying-community/router/internal/admin/service/group"
)

type upsertGroupRequest struct {
	Id           string                       `json:"id"`
	Name         string                       `json:"name"`
	Description  string                       `json:"description"`
	BillingRatio *float64                     `json:"billing_ratio"`
	Enabled      *bool                        `json:"enabled"`
	SortOrder    int                          `json:"sort_order"`
	ChannelIDs   []string                     `json:"channel_ids"`
	ModelConfigs []model.GroupModelConfigItem `json:"model_configs"`
}

type updateGroupChannelsRequest struct {
	ChannelIDs []string `json:"channel_ids"`
}

// GetGroupCatalog godoc
// @Summary List groups catalog (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/catalog [get]
func GetGroupCatalog(c *gin.Context) {
	rows, err := groupsvc.ListCatalog()
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

// GetGroupChannelOptions godoc
// @Summary List group channel candidates (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/group/channel-options [get]
func GetGroupChannelOptions(c *gin.Context) {
	rows, err := groupsvc.ListChannelBindingCandidates()
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
	createItem := model.GroupCatalog{
		Id:           strings.TrimSpace(req.Id),
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		Source:       "manual",
		BillingRatio: billingRatio,
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
	item := model.GroupCatalog{
		Id:           strings.TrimSpace(req.Id),
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		BillingRatio: billingRatio,
		Enabled:      enabled,
		SortOrder:    req.SortOrder,
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
			"message": "分组标识不能为空",
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
			"message": "分组标识不能为空",
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
			"message": "分组标识不能为空",
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
			"message": "分组标识不能为空",
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
			"message": "分组标识不能为空",
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
			"message": "分组标识不能为空",
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
