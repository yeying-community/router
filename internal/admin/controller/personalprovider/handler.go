package personalprovider

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

type connectionInput struct {
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	Models   []string `json:"models"`
	Priority int64    `json:"priority"`
	Status   int      `json:"status"`
}

type modelRouteInput struct {
	Model       string `json:"model"`
	RoutePolicy string `json:"route_policy"`
}

func connectionOutput(row *model.PersonalProviderConnection, runtime model.ChannelRuntimeMetrics, window model.ChannelMetricWindow, supply *model.PublisherService) gin.H {
	if row == nil {
		return nil
	}
	var supplyStatus any
	if supply != nil {
		supplyStatus = gin.H{"service_id": supply.Id, "status": supply.Status, "last_checked_at": supply.LastCheckedAt, "last_check_ok": supply.LastCheckOK}
	}
	return gin.H{
		"id": row.Id, "channel_id": model.PersonalProviderChannelPrefix + row.Id,
		"source_kind": "personal", "visibility": "private", "owner_id": row.UserId,
		"name": row.Name, "protocol": row.Protocol, "base_url": row.BaseURL, "models": row.Models,
		"priority": row.Priority, "status": row.Status, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
		"credential_configured": row.CredentialConfigured,
		"verification":          gin.H{"last_checked_at": row.LastCheckedAt, "ok": row.LastCheckOK, "error": row.LastCheckError},
		"last_checked_at":       row.LastCheckedAt, "last_check_ok": row.LastCheckOK, "last_check_error": row.LastCheckError,
		"runtime_metrics": runtime, "metric_window": window,
		"metric_availability": gin.H{"runtime": true, "quota": false, "cost": false, "earnings": supply != nil},
		"supply":              supplyStatus,
	}
}

func ListConnections(c *gin.Context) {
	userID := c.GetString(ctxkey.Id)
	rows, err := model.ListPersonalProviderConnections(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	window := model.ResolveChannelMetricWindow(c.Query("period"), time.Now())
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].Id)
	}
	metrics, err := model.LoadChannelRuntimeMetrics(model.ChannelRuntimeMetricQuery{Window: window, PersonalProviderIDs: ids, UserID: userID})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	supplyByConnection := make(map[string]*model.PublisherService)
	if services, serviceErr := model.ListPublisherServices(userID); serviceErr == nil {
		for i := range services {
			connectionID := strings.TrimSpace(services[i].SourcePersonalConnectionId)
			if connectionID == "" {
				continue
			}
			service := services[i]
			supplyByConnection[connectionID] = &service
		}
	}
	data := make([]gin.H, 0, len(rows))
	for i := range rows {
		data = append(data, connectionOutput(&rows[i], metrics[model.PersonalProviderChannelPrefix+rows[i].Id], window, supplyByConnection[rows[i].Id]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func GetConnection(c *gin.Context) {
	row, err := model.GetPersonalProviderConnection(c.GetString(ctxkey.Id), c.Param("id"))
	if err != nil {
		respondConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": connectionOutput(row, model.ChannelRuntimeMetrics{}, model.ChannelMetricWindow{}, nil)})
}

func CreateConnection(c *gin.Context) {
	input := connectionInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	row := &model.PersonalProviderConnection{UserId: c.GetString(ctxkey.Id), Name: input.Name, Protocol: input.Protocol, BaseURL: input.BaseURL, Models: input.Models, Priority: input.Priority, Status: input.Status}
	if err := model.CreatePersonalProviderConnection(row, input.APIKey); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": connectionOutput(row, model.ChannelRuntimeMetrics{}, model.ChannelMetricWindow{}, nil)})
}

func UpdateConnection(c *gin.Context) {
	input := connectionInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	userID, connectionID := c.GetString(ctxkey.Id), c.Param("id")
	existing, err := model.GetPersonalProviderConnection(userID, connectionID)
	if err != nil {
		respondConnectionError(c, err)
		return
	}
	status := input.Status
	if status == 0 {
		status = existing.Status
	}
	row := &model.PersonalProviderConnection{Id: existing.Id, UserId: userID, Name: input.Name, Protocol: input.Protocol, BaseURL: input.BaseURL, Models: input.Models, Priority: input.Priority, Status: status}
	var credential *string
	if strings.TrimSpace(input.APIKey) != "" {
		credential = &input.APIKey
	}
	if err := model.UpdatePersonalProviderConnection(row, credential); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	updated, err := model.GetPersonalProviderConnection(userID, connectionID)
	if err != nil {
		respondConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": connectionOutput(updated, model.ChannelRuntimeMetrics{}, model.ChannelMetricWindow{}, nil)})
}

func DeleteConnection(c *gin.Context) {
	if err := model.DeletePersonalProviderConnection(c.GetString(ctxkey.Id), c.Param("id")); err != nil {
		respondConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func VerifyConnection(c *gin.Context) {
	row, err := model.VerifyPersonalProviderConnection(c.GetString(ctxkey.Id), c.Param("id"))
	if err != nil {
		if row == nil {
			respondConnectionError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": connectionOutput(row, model.ChannelRuntimeMetrics{}, model.ChannelMetricWindow{}, nil)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": connectionOutput(row, model.ChannelRuntimeMetrics{}, model.ChannelMetricWindow{}, nil)})
}

func ListModelRoutes(c *gin.Context) {
	rows, err := model.ListPersonalModelRoutes(c.GetString(ctxkey.Id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func UpsertModelRoute(c *gin.Context) {
	input := modelRouteInput{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求格式无效"})
		return
	}
	if !model.IsPersonalRoutePolicy(input.RoutePolicy) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "路由策略无效"})
		return
	}
	available, err := listUserRoutableModels(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if _, exists := available[strings.TrimSpace(input.Model)]; !exists {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "模型不在当前账号可用范围内"})
		return
	}
	if err := model.UpsertPersonalModelRoute(c.GetString(ctxkey.Id), input.Model, input.RoutePolicy); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func listUserRoutableModels(c *gin.Context) (map[string]struct{}, error) {
	userID := c.GetString(ctxkey.Id)
	payload, err := model.BuildUserEntitlementModels(c.Request.Context(), userID)
	if err != nil {
		return nil, err
	}
	personalModels, err := model.ListPersonalProviderModels(userID)
	if err != nil {
		return nil, err
	}
	available := make(map[string]struct{}, len(payload.Models)+len(personalModels))
	for _, modelName := range payload.Models {
		if normalized := strings.TrimSpace(modelName); normalized != "" {
			available[normalized] = struct{}{}
		}
	}
	for _, modelName := range personalModels {
		if normalized := strings.TrimSpace(modelName); normalized != "" {
			available[normalized] = struct{}{}
		}
	}
	return available, nil
}

func DeleteModelRoute(c *gin.Context) {
	if err := model.DeletePersonalModelRoute(c.GetString(ctxkey.Id), c.Param("model")); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "模型路由规则不存在或无权访问", "code": "personal_model_route_not_found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func RoutingQuota(c *gin.Context) {
	start := time.Now().UTC()
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	var used int64
	err := model.LOG_DB.Model(&model.Log{}).Where("user_id = ? AND created_at >= ? AND channel_id LIKE ?", c.GetString(ctxkey.Id), start.Unix(), model.PersonalProviderChannelPrefix+"%").Count(&used).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"period": start.Format("2006-01"), "used_requests": used, "included_requests": 0, "unlimited": true, "unit": "request"}})
}

func respondConnectionError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "个人渠道不存在或无权访问", "code": "personal_provider_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
}
