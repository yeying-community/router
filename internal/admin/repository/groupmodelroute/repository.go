package groupmodelroute

import (
	"context"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindGroupModelRouteRepository(model.GroupModelRouteRepository{
		GetRandomSatisfiedChannel:   GetRandomSatisfiedChannel,
		ListSatisfiedChannels:       ListSatisfiedChannels,
		AddGroupModelRoutes:         AddGroupModelRoutes,
		DeleteGroupModelRoutes:      DeleteGroupModelRoutes,
		UpdateGroupModelRoutes:      UpdateGroupModelRoutes,
		UpdateGroupModelRouteStatus: UpdateGroupModelRouteStatus,
		GetTopChannelByModel:        GetTopChannelByModel,
		GetGroupModels:              GetGroupModels,
	})
}

func GetRandomSatisfiedChannel(group string, modelName string, ignoreFirstPriority bool) (*model.Channel, error) {
	channels, err := ListSatisfiedChannels(group, modelName)
	if err != nil {
		return nil, err
	}
	channel := model.SelectRandomSatisfiedChannel(channels, ignoreFirstPriority, nil)
	if channel == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return channel, nil
}

func ListSatisfiedChannels(group string, modelName string) ([]*model.Channel, error) {
	groupCol := `"group"`
	trueVal := "true"
	routeRows := make([]model.GroupModelRoute, 0)
	if err := model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName).
		Order("priority desc, channel_id asc").
		Find(&routeRows).Error; err != nil {
		return nil, err
	}
	channelIDs := make([]string, 0, len(routeRows))
	seen := make(map[string]struct{}, len(routeRows))
	for _, row := range routeRows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		channelIDs = append(channelIDs, channelID)
	}
	if len(channelIDs) == 0 {
		return []*model.Channel{}, nil
	}
	channels := make([]*model.Channel, 0, len(channelIDs))
	if err := model.DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	if err := model.HydrateChannelsWithModels(model.DB, channels); err != nil {
		return nil, err
	}
	channelByID := make(map[string]*model.Channel, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		channelByID[strings.TrimSpace(channel.Id)] = channel
	}
	result := make([]*model.Channel, 0, len(routeRows))
	for _, route := range routeRows {
		channelID := strings.TrimSpace(route.ChannelId)
		if channel := channelByID[channelID]; channel != nil {
			result = append(result, model.CloneChannelWithPriority(channel, route.GetPriority()))
		}
	}
	return result, nil
}

func AddGroupModelRoutes(channel *model.Channel) error {
	// Runtime routes are rebuilt from group_models + group_channel_bindings.
	// Channel creation no longer writes routing truth directly.
	if channel == nil {
		return nil
	}
	model.RefreshGroupModelRouteCachesForGroups()
	return nil
}

func listBoundGroupsByChannelID(channelID string) ([]string, error) {
	groupCol := `"group"`
	groups := make([]string, 0)
	err := model.DB.Model(&model.GroupChannelBinding{}).
		Distinct(groupCol).
		Where("channel_id = ? AND enabled = ?", channelID, true).
		Pluck(groupCol, &groups).Error
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		normalized := strings.TrimSpace(group)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func buildGroupModelRoutesForChannel(channel *model.Channel, groups []string) []model.GroupModelRoute {
	if channel == nil || len(groups) == 0 {
		return nil
	}
	routes := make([]model.GroupModelRoute, 0)
	for _, group := range groups {
		normalizedGroup := strings.TrimSpace(group)
		if normalizedGroup == "" {
			continue
		}
		groupModels, err := model.ListGroupModelRowsByDB(model.DB, normalizedGroup)
		if err != nil {
			continue
		}
		routes = append(routes, model.SyncGroupModelRoutesForChannel(normalizedGroup, channel, groupModels, channel.Priority)...)
	}
	return routes
}

func DeleteGroupModelRoutes(channel *model.Channel) error {
	if channel == nil {
		return nil
	}
	groups, err := listBoundGroupsByChannelID(channel.Id)
	if err != nil {
		return err
	}
	if err := model.DB.Where("channel_id = ?", channel.Id).Delete(&model.GroupModelRoute{}).Error; err != nil {
		return err
	}
	for _, groupID := range groups {
		if err := model.RebuildGroupModelsFromRoutesWithDB(model.DB, groupID); err != nil {
			return err
		}
	}
	model.RefreshGroupModelRouteCachesForGroups(groups...)
	return nil
}

func UpdateGroupModelRoutes(channel *model.Channel) error {
	if channel == nil {
		return nil
	}
	groups, err := listBoundGroupsByChannelID(channel.Id)
	if err != nil {
		return err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		for _, groupID := range groups {
			groupID = strings.TrimSpace(groupID)
			if groupID == "" {
				continue
			}
			groupModels, err := model.ListGroupModelRowsByDB(tx, groupID)
			if err != nil {
				return err
			}
			priorityByChannelID, err := model.ListGroupChannelBindingPriorityByChannelWithDB(tx, groupID)
			if err != nil {
				return err
			}
			next := model.SyncGroupModelRoutesForChannel(groupID, channel, groupModels, priorityByChannelID[strings.TrimSpace(channel.Id)])
			groupCol := `"group"`
			if err := tx.Where(groupCol+" = ? AND channel_id = ?", groupID, channel.Id).Delete(&model.GroupModelRoute{}).Error; err != nil {
				return err
			}
			if len(next) == 0 {
				if err := model.RebuildGroupModelsFromRoutesWithDB(tx, groupID); err != nil {
					return err
				}
				continue
			}
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
			if err := model.RebuildGroupModelsFromRoutesWithDB(tx, groupID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	model.RefreshGroupModelRouteCachesForGroups(groups...)
	return nil
}

func UpdateGroupModelRouteStatus(channelId string, status bool) error {
	groups, err := listBoundGroupsByChannelID(channelId)
	if err != nil {
		return err
	}
	if err := model.DB.Model(&model.GroupModelRoute{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error; err != nil {
		return err
	}
	for _, groupID := range groups {
		if err := model.RebuildGroupModelsFromRoutesWithDB(model.DB, groupID); err != nil {
			return err
		}
	}
	model.RefreshGroupModelRouteCachesForGroups(groups...)
	return nil
}

func GetTopChannelByModel(group string, modelName string) (*model.Channel, error) {
	groupCol := `"group"`
	trueVal := "true"

	route := model.GroupModelRoute{}
	err := model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName).
		Order("priority desc, channel_id asc").
		First(&route).Error
	if err != nil {
		return nil, err
	}
	channel := model.Channel{Id: route.ChannelId}
	err = model.DB.Omit("key").First(&channel, "id = ?", route.ChannelId).Error
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithModels(model.DB, &channel); err != nil {
		return nil, err
	}
	priority := route.GetPriority()
	channel.Priority = &priority
	return &channel, nil
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	models, err := model.ListGroupModelNamesByDB(model.DB, group)
	if err != nil {
		return nil, err
	}
	sort.Strings(models)
	return models, nil
}
