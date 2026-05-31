package group

import (
	"context"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindGroupModelChannelRepository(model.GroupModelChannelRepository{
		GetRandomSatisfiedChannel:                GetRandomSatisfiedChannel,
		ListSatisfiedChannels:                    ListSatisfiedChannels,
		AddGroupModelChannels:                    AddGroupModelChannels,
		DeleteGroupModelChannels:                 DeleteGroupModelChannels,
		UpdateGroupModelChannels:                 UpdateGroupModelChannels,
		RefreshGroupModelChannelsByChannelStatus: RefreshGroupModelChannelsByChannelStatus,
		GetTopChannelByModel:                     GetTopChannelByModel,
		GetGroupModels:                           GetGroupModels,
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
	rows := make([]model.GroupModelChannel, 0)
	if err := model.DB.Where(groupCol+" = ? and model = ?", group, modelName).
		Order("priority desc, channel_id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	channelIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
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
		if channel.Status != model.ChannelStatusEnabled && channel.Status != model.ChannelStatusHalfOpen {
			continue
		}
		channelByID[strings.TrimSpace(channel.Id)] = channel
	}
	result := make([]*model.Channel, 0, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channel := channelByID[channelID]; channel != nil {
			priority := row.GetPriority()
			if channel.Status == model.ChannelStatusHalfOpen {
				priority = model.ChannelHalfOpenPriority
			}
			result = append(result, model.CloneChannelWithPriority(channel, priority))
		}
	}
	return result, nil
}

func AddGroupModelChannels(channel *model.Channel) error {
	// Runtime routes are refreshed from group_models + group_channels.
	// Channel creation no longer writes routing truth directly.
	if channel == nil {
		return nil
	}
	model.RefreshGroupModelChannelCachesForGroups()
	return nil
}

func listBoundGroupsByChannelID(channelID string) ([]string, error) {
	groupCol := `"group"`
	groups := make([]string, 0)
	err := model.DB.Model(&model.GroupChannel{}).
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

func buildGroupModelChannelsForChannel(channel *model.Channel, groups []string) []model.GroupModelChannel {
	if channel == nil || len(groups) == 0 {
		return nil
	}
	rows := make([]model.GroupModelChannel, 0)
	for _, group := range groups {
		normalizedGroup := strings.TrimSpace(group)
		if normalizedGroup == "" {
			continue
		}
		groupModels, err := model.ListGroupModelRowsByDB(model.DB, normalizedGroup)
		if err != nil {
			continue
		}
		rows = append(rows, model.BuildGroupModelChannelsForChannel(normalizedGroup, channel, groupModels, channel.Priority)...)
	}
	return rows
}

func DeleteGroupModelChannels(channel *model.Channel) error {
	if channel == nil {
		return nil
	}
	groups, err := listBoundGroupsByChannelID(channel.Id)
	if err != nil {
		return err
	}
	if err := model.DB.Where("channel_id = ?", channel.Id).Delete(&model.GroupModelChannel{}).Error; err != nil {
		return err
	}
	model.RefreshGroupModelChannelCachesForGroups(groups...)
	return nil
}

func UpdateGroupModelChannels(channel *model.Channel) error {
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
			priorityByChannelID, err := model.ListGroupChannelPriorityByChannelWithDB(tx, groupID)
			if err != nil {
				return err
			}
			next := model.BuildGroupModelChannelsForChannel(groupID, channel, groupModels, priorityByChannelID[strings.TrimSpace(channel.Id)])
			groupCol := `"group"`
			if err := tx.Where(groupCol+" = ? AND channel_id = ?", groupID, channel.Id).Delete(&model.GroupModelChannel{}).Error; err != nil {
				return err
			}
			if len(next) == 0 {
				continue
			}
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	model.RefreshGroupModelChannelCachesForGroups(groups...)
	return nil
}

func RefreshGroupModelChannelsByChannelStatus(channelId string, _ bool) error {
	groups, err := listBoundGroupsByChannelID(channelId)
	if err != nil {
		return err
	}
	model.RefreshGroupModelChannelCachesForGroups(groups...)
	return nil
}

func GetTopChannelByModel(group string, modelName string) (*model.Channel, error) {
	groupCol := `"group"`

	row := model.GroupModelChannel{}
	err := model.DB.Where(groupCol+" = ? and model = ?", group, modelName).
		Order("priority desc, channel_id asc").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	channel := model.Channel{Id: row.ChannelId}
	err = model.DB.Omit("key").First(&channel, "id = ?", row.ChannelId).Error
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithModels(model.DB, &channel); err != nil {
		return nil, err
	}
	if channel.Status != model.ChannelStatusEnabled && channel.Status != model.ChannelStatusHalfOpen {
		return nil, gorm.ErrRecordNotFound
	}
	priority := row.GetPriority()
	if channel.Status == model.ChannelStatusHalfOpen {
		priority = model.ChannelHalfOpenPriority
	}
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
