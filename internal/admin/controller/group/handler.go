package group

import (
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
	Id          string                        `json:"id"`
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Enabled     *bool                         `json:"enabled"`
	SortOrder   int                           `json:"sort_order"`
	ChannelIDs  []string                      `json:"channel_ids"`
	Models      []model.GroupModelBindingItem `json:"models"`
}

type updateGroupChannelsRequest struct {
	ChannelIDs []string                 `json:"channel_ids"`
	Channels   []model.GroupChannelItem `json:"channels"`
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

func CreateGroup(c *gin.Context) {
	req := upsertGroupRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	createItem := model.GroupCatalog{
		Id:          strings.TrimSpace(req.Id),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Source:      "manual",
	}
	row := model.GroupCatalog{}
	var err error
	if req.Models != nil {
		row, err = groupsvc.CreateWithModels(createItem, req.ChannelIDs, req.Models)
	} else {
		row, err = groupsvc.CreateWithChannels(createItem, req.ChannelIDs)
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
	item := model.GroupCatalog{
		Id:          strings.TrimSpace(req.Id),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Enabled:     enabled,
		SortOrder:   req.SortOrder,
	}
	row := model.GroupCatalog{}
	var err error
	if req.Models != nil {
		row, err = groupsvc.UpdateWithModels(item, req.ChannelIDs, req.Models, req.ChannelIDs != nil, true)
	} else if req.ChannelIDs != nil {
		row, err = groupsvc.UpdateWithChannels(item, req.ChannelIDs)
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

func GetGroupChannels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	rows, err := groupsvc.ListChannels(id)
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

func GetGroupModels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	payload, err := groupsvc.ListModels(id)
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

type updateGroupModelsRequest struct {
	ChannelIDs []string                      `json:"channel_ids"`
	Models     []model.GroupModelBindingItem `json:"models"`
}

type updateSingleGroupModelRequest struct {
	Models []model.GroupModelBindingItem `json:"models"`
}

func UpdateGroupModels(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}

	req := updateGroupModelsRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := groupsvc.ReplaceModels(id, req.ChannelIDs, req.Models, req.ChannelIDs != nil); err != nil {
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

func UpdateSingleGroupModel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	modelName := strings.TrimSpace(c.Param("model"))
	if modelName == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "模型不能为空",
		})
		return
	}

	req := updateSingleGroupModelRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if len(req.Models) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组模型至少需要一个渠道映射",
		})
		return
	}
	if err := groupsvc.ReplaceSingleModel(id, modelName, req.Models); err != nil {
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

func DeleteSingleGroupModel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组 ID 不能为空",
		})
		return
	}
	modelName := strings.TrimSpace(c.Param("model"))
	if modelName == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "模型不能为空",
		})
		return
	}
	if err := groupsvc.DeleteSingleModel(id, modelName); err != nil {
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
	if len(req.Channels) > 0 {
		if err := groupsvc.ReplaceChannelsWithItems(id, req.Channels); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else if err := groupsvc.ReplaceChannels(id, req.ChannelIDs); err != nil {
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
