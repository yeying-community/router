package model

import (
	"context"
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
)

type GroupModelRoute struct {
	Group         string `json:"group" gorm:"type:varchar(32);primaryKey;autoIncrement:false"`
	Model         string `json:"model" gorm:"primaryKey;autoIncrement:false"`
	ChannelId     string `json:"channel_id" gorm:"type:varchar(64);primaryKey;autoIncrement:false;index"`
	UpstreamModel string `json:"upstream_model" gorm:"type:varchar(255);default:'';index"`
	Provider      string `json:"provider" gorm:"type:varchar(128);default:'';index"`
	Enabled       bool   `json:"enabled"`
	Priority      *int64 `json:"priority" gorm:"bigint;default:0;index"`
}

func (route GroupModelRoute) GetPriority() int64 {
	if route.Priority == nil {
		return 0
	}
	return *route.Priority
}

const (
	GroupModelRoutesTableName = "group_model_routes"
)

// GroupModelRoute is a runtime-expanded row used directly by request-time routing.
// It is derived from group_models, group_channel_bindings, and channel model capability data.
// Admin flows must not treat this table as source-of-truth configuration.
func (GroupModelRoute) TableName() string {
	return GroupModelRoutesTableName
}

func GetRandomSatisfiedChannel(group string, model string, ignoreFirstPriority bool) (*Channel, error) {
	return mustGroupModelRouteRepo().GetRandomSatisfiedChannel(group, model, ignoreFirstPriority)
}

func ListSatisfiedChannels(group string, model string) ([]*Channel, error) {
	return mustGroupModelRouteRepo().ListSatisfiedChannels(group, model)
}

func (channel *Channel) AddGroupModelRoutes() error {
	return mustGroupModelRouteRepo().AddGroupModelRoutes(channel)
}

func (channel *Channel) DeleteGroupModelRoutes() error {
	return mustGroupModelRouteRepo().DeleteGroupModelRoutes(channel)
}

// UpdateGroupModelRoutes updates runtime group-model routes of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateGroupModelRoutes() error {
	return mustGroupModelRouteRepo().UpdateGroupModelRoutes(channel)
}

func UpdateGroupModelRouteStatus(channelId string, status bool) error {
	return mustGroupModelRouteRepo().UpdateGroupModelRouteStatus(channelId, status)
}

// GetTopChannelByModel returns the highest-priority enabled channel for a given group+model.
// Order: priority desc, then channel_id asc (stable for UI usage).
func GetTopChannelByModel(group string, model string) (*Channel, error) {
	return mustGroupModelRouteRepo().GetTopChannelByModel(group, model)
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	return mustGroupModelRouteRepo().GetGroupModels(ctx, group)
}

func NormalizeGroupModelRouteUpstreamModel(modelName string, upstreamModel string) string {
	upstream := strings.TrimSpace(upstreamModel)
	if upstream != "" {
		return upstream
	}
	return strings.TrimSpace(modelName)
}

func NormalizeGroupModelRouteProvider(provider string) string {
	normalized := commonutils.NormalizeProvider(provider)
	if normalized == "custom" {
		return ""
	}
	return normalized
}

func normalizeGroupModelRouteRowsPreserveOrder(rows []GroupModelRoute) []GroupModelRoute {
	if len(rows) == 0 {
		return []GroupModelRoute{}
	}
	result := make([]GroupModelRoute, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		normalized := row
		normalized.Group = strings.TrimSpace(normalized.Group)
		normalized.Model = strings.TrimSpace(normalized.Model)
		normalized.ChannelId = strings.TrimSpace(normalized.ChannelId)
		normalized.UpstreamModel = NormalizeGroupModelRouteUpstreamModel(normalized.Model, normalized.UpstreamModel)
		normalized.Provider = NormalizeGroupModelRouteProvider(normalized.Provider)
		if normalized.Group == "" || normalized.Model == "" || normalized.ChannelId == "" {
			continue
		}
		key := normalized.Group + "::" + normalized.Model + "::" + normalized.ChannelId
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}
