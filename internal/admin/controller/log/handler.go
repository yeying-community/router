package log

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"github.com/yeying-community/router/internal/admin/presenter"
	logsvc "github.com/yeying-community/router/internal/admin/service/log"
)

type logChannelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type logGroupOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type logFilterOptions struct {
	TokenNames []string           `json:"token_names"`
	ModelNames []string           `json:"model_names"`
	Usernames  []string           `json:"usernames,omitempty"`
	Channels   []logChannelOption `json:"channels,omitempty"`
	Groups     []logGroupOption   `json:"groups,omitempty"`
}

func normalizeStatLogType(raw int) int {
	if raw == model.LogTypeAll {
		return model.LogTypeConsume
	}
	return raw
}

func distinctLogValues(queryColumn string, userID string) ([]string, error) {
	rows := make([]string, 0)
	query := model.LOG_DB.Table(model.EventLogsTableName).
		Distinct(queryColumn).
		Where("COALESCE(" + queryColumn + ", '') <> ''")
	if strings.TrimSpace(userID) != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Order(queryColumn+" asc").Limit(200).Pluck(queryColumn, &rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		value := strings.TrimSpace(row)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result, nil
}

func loadChannelOptions(channelIDs []string) ([]logChannelOption, error) {
	normalizedIDs := make([]string, 0, len(channelIDs))
	seen := make(map[string]struct{}, len(channelIDs))
	for _, id := range channelIDs {
		value := strings.TrimSpace(id)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalizedIDs = append(normalizedIDs, value)
	}
	if len(normalizedIDs) == 0 {
		return []logChannelOption{}, nil
	}
	var channels []*model.Channel
	if err := model.DB.Select("id", "name").Where("id IN ?", normalizedIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	nameByID := make(map[string]string, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		nameByID[strings.TrimSpace(channel.Id)] = strings.TrimSpace(channel.DisplayName())
	}
	options := make([]logChannelOption, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		label := strings.TrimSpace(nameByID[id])
		if label == "" {
			label = id
		}
		options = append(options, logChannelOption{
			ID:    id,
			Label: label,
		})
	}
	return options, nil
}

func loadGroupOptions(groupIDs []string) ([]logGroupOption, error) {
	normalizedIDs := make([]string, 0, len(groupIDs))
	seen := make(map[string]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		value := strings.TrimSpace(id)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalizedIDs = append(normalizedIDs, value)
	}
	if len(normalizedIDs) == 0 {
		return []logGroupOption{}, nil
	}
	var groups []model.GroupCatalog
	if err := model.DB.Select("id", "name").Where("id IN ?", normalizedIDs).Find(&groups).Error; err != nil {
		return nil, err
	}
	nameByID := make(map[string]string, len(groups))
	for _, group := range groups {
		id := strings.TrimSpace(group.Id)
		if id == "" {
			continue
		}
		nameByID[id] = strings.TrimSpace(group.Name)
	}
	options := make([]logGroupOption, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		label := strings.TrimSpace(nameByID[id])
		if label == "" {
			label = id
		}
		options = append(options, logGroupOption{
			ID:    id,
			Label: label,
		})
	}
	return options, nil
}

func buildLogFilterOptions(userID string, includeAdmin bool) (logFilterOptions, error) {
	tokenNames, err := distinctLogValues("token_name", userID)
	if err != nil {
		return logFilterOptions{}, err
	}
	modelNames, err := distinctLogValues("model_name", userID)
	if err != nil {
		return logFilterOptions{}, err
	}
	options := logFilterOptions{
		TokenNames: tokenNames,
		ModelNames: modelNames,
	}
	if !includeAdmin {
		return options, nil
	}
	usernames, err := distinctLogValues("username", "")
	if err != nil {
		return logFilterOptions{}, err
	}
	channelIDs, err := distinctLogValues("channel_id", "")
	if err != nil {
		return logFilterOptions{}, err
	}
	groupIDs, err := distinctLogValues("group_id", "")
	if err != nil {
		return logFilterOptions{}, err
	}
	channels, err := loadChannelOptions(channelIDs)
	if err != nil {
		return logFilterOptions{}, err
	}
	groups, err := loadGroupOptions(groupIDs)
	if err != nil {
		return logFilterOptions{}, err
	}
	options.Usernames = usernames
	options.Channels = channels
	options.Groups = groups
	return options, nil
}

func countAdminLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, groupID string, channel string) (int64, error) {
	query := model.LOG_DB.Table(model.EventLogsTableName)
	if logType != model.LogTypeAll {
		query = query.Where("type = ?", logType)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if username != "" {
		query = query.Where("username = ?", username)
	}
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}
	if strings.TrimSpace(groupID) != "" {
		query = query.Where("group_id = ?", strings.TrimSpace(groupID))
	}
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	if channel != "" {
		query = query.Where("channel_id = ?", channel)
	}
	var total int64
	err := query.Count(&total).Error
	return total, err
}

// GetLogFilterOptions godoc
// @Summary List log filter options (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/log/options [get]
func GetLogFilterOptions(c *gin.Context) {
	options, err := buildLogFilterOptions("", true)
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
		"data":    options,
	})
}

// GetUserLogFilterOptions godoc
// @Summary List current user log filter options
// @Tags public
// @Security BearerAuth
// @Produce json
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/log/options [get]
func GetUserLogFilterOptions(c *gin.Context) {
	options, err := buildLogFilterOptions(c.GetString(ctxkey.Id), false)
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
		"data":    options,
	})
}

func countUserLogs(userId string, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string) (int64, error) {
	query := model.LOG_DB.Table(model.EventLogsTableName).Where("user_id = ?", userId)
	if logType != model.LogTypeAll {
		query = query.Where("type = ?", logType)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if tokenName != "" {
		query = query.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		query = query.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		query = query.Where("created_at <= ?", endTimestamp)
	}
	var total int64
	err := query.Count(&total).Error
	return total, err
}

// GetAllLogs godoc
// @Summary List logs (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param type query int false "Log type"
// @Param start_timestamp query int false "Start timestamp (unix)"
// @Param end_timestamp query int false "End timestamp (unix)"
// @Param username query string false "Username"
// @Param token_name query string false "Token name"
// @Param model_name query string false "Model name"
// @Param group_id query string false "Group ID"
// @Param channel query int false "Channel ID"
// @Success 200 {object} docs.UserLogListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/log [get]
func GetAllLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	groupID := c.Query("group_id")
	channel := c.Query("channel")
	logs, err := logsvc.GetAll(logType, startTimestamp, endTimestamp, modelName, username, tokenName, groupID, (page-1)*config.ItemsPerPage, config.ItemsPerPage, channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	total, err := countAdminLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, groupID, channel)
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
		"data":    presenter.NewLogs(logs),
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": config.ItemsPerPage,
		},
	})
	return
}

// GetUserLogs godoc
// @Summary List user logs
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page (1-based)"
// @Param type query int false "Log type"
// @Param start_timestamp query int false "Start timestamp (unix)"
// @Param end_timestamp query int false "End timestamp (unix)"
// @Param token_name query string false "Token name"
// @Param model_name query string false "Model name"
// @Success 200 {object} docs.UserLogListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/log [get]
func GetUserLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	userId := c.GetString(ctxkey.Id)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	logs, err := logsvc.GetUser(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, (page-1)*config.ItemsPerPage, config.ItemsPerPage)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	total, err := countUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName)
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
		"data":    presenter.NewLogs(logs),
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": config.ItemsPerPage,
		},
	})
	return
}

// GetLog godoc
// @Summary Get log by ID (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Router /api/v1/admin/log/{id} [get]
func GetLog(c *gin.Context) {
	logID := c.Param("id")
	logRow, err := logsvc.GetByID(logID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "日志不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewLog(logRow),
	})
	return
}

// GetCurrentUserLog godoc
// @Summary Get current user log by ID
// @Tags public
// @Security BearerAuth
// @Produce json
// @Router /api/v1/public/log/{id} [get]
func GetCurrentUserLog(c *gin.Context) {
	logID := c.Param("id")
	userId := c.GetString(ctxkey.Id)
	logRow, err := logsvc.GetUserByID(userId, logID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "日志不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    presenter.NewLog(logRow),
	})
	return
}

// SearchAllLogs godoc
// @Summary Search logs (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.UserLogListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/log/search [get]
func SearchAllLogs(c *gin.Context) {
	keyword := c.Query("keyword")
	logs, err := logsvc.SearchAll(keyword)
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
		"data":    presenter.NewLogs(logs),
	})
	return
}

// SearchUserLogs godoc
// @Summary Search user logs
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "Keyword"
// @Success 200 {object} docs.UserLogListResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/log/search [get]
func SearchUserLogs(c *gin.Context) {
	keyword := c.Query("keyword")
	userId := c.GetString(ctxkey.Id)
	logs, err := logsvc.SearchUser(userId, keyword)
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
		"data":    presenter.NewLogs(logs),
	})
	return
}

// GetLogsStat godoc
// @Summary Log stats (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param type query int false "Log type"
// @Param start_timestamp query int false "Start timestamp (unix)"
// @Param end_timestamp query int false "End timestamp (unix)"
// @Param token_name query string false "Token name"
// @Param username query string false "Username"
// @Param model_name query string false "Model name"
// @Param channel query int false "Channel ID"
// @Success 200 {object} docs.UserLogStatResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/log/stat [get]
func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel := c.Query("channel")
	quotaNum := logsvc.SumUsedQuota(normalizeStatLogType(logType), startTimestamp, endTimestamp, modelName, username, tokenName, channel)
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":      quotaNum,
			"yyc_amount": quotaNum,
			//"token": tokenNum,
		},
	})
	return
}

// GetLogsSelfStat godoc
// @Summary Log stats for current user
// @Tags public
// @Security BearerAuth
// @Produce json
// @Param type query int false "Log type"
// @Param start_timestamp query int false "Start timestamp (unix)"
// @Param end_timestamp query int false "End timestamp (unix)"
// @Param token_name query string false "Token name"
// @Param model_name query string false "Model name"
// @Param channel query int false "Channel ID"
// @Success 200 {object} docs.UserLogStatResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/public/log/stat [get]
func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString(ctxkey.Username)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel := c.Query("channel")
	quotaNum := logsvc.SumUsedQuota(normalizeStatLogType(logType), startTimestamp, endTimestamp, modelName, username, tokenName, channel)
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":      quotaNum,
			"yyc_amount": quotaNum,
			//"token": tokenNum,
		},
	})
	return
}

// DeleteHistoryLogs godoc
// @Summary Delete history logs (admin)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param target_timestamp query int true "Target timestamp (unix)"
// @Success 200 {object} docs.StandardResponse
// @Failure 401 {object} docs.ErrorResponse
// @Router /api/v1/admin/log [delete]
func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := logsvc.DeleteOld(targetTimestamp)
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
		"data":    count,
	})
	return
}
