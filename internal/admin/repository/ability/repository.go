package ability

import (
	"context"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindAbilityRepository(model.AbilityRepository{
		GetRandomSatisfiedChannel: GetRandomSatisfiedChannel,
		ListSatisfiedChannels:     ListSatisfiedChannels,
		AddAbilities:              AddAbilities,
		DeleteAbilities:           DeleteAbilities,
		UpdateAbilities:           UpdateAbilities,
		UpdateAbilityStatus:       UpdateAbilityStatus,
		GetTopChannelByModel:      GetTopChannelByModel,
		GetGroupModels:            GetGroupModels,
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
	abilityRows := make([]model.Ability, 0)
	if err := model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName).
		Order("priority desc, channel_id asc").
		Find(&abilityRows).Error; err != nil {
		return nil, err
	}
	channelIDs := make([]string, 0, len(abilityRows))
	seen := make(map[string]struct{}, len(abilityRows))
	for _, row := range abilityRows {
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
	result := make([]*model.Channel, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if channel := channelByID[channelID]; channel != nil {
			result = append(result, channel)
		}
	}
	return result, nil
}

func AddAbilities(channel *model.Channel) error {
	// Channel-group bindings are managed centrally in group management.
	// Channel creation no longer auto-generates abilities.
	if channel == nil {
		return nil
	}
	model.RefreshAbilityCachesForGroups()
	return nil
}

func listBoundGroupsByChannelID(channelID string) ([]string, error) {
	groupCol := `"group"`
	groups := make([]string, 0)
	err := model.DB.Model(&model.Ability{}).
		Distinct(groupCol).
		Where("channel_id = ?", channelID).
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

func buildAbilitiesForChannel(channel *model.Channel, groups []string) []model.Ability {
	if channel == nil || len(groups) == 0 {
		return nil
	}
	selectedConfigs := channel.GetModelConfigs()
	abilities := make([]model.Ability, 0, len(selectedConfigs)*len(groups))
	for _, row := range selectedConfigs {
		if !row.Selected {
			continue
		}
		normalizedModel := strings.TrimSpace(row.Model)
		if normalizedModel == "" {
			continue
		}
		upstream := model.NormalizeAbilityUpstreamModel(normalizedModel, row.UpstreamModel)
		for _, group := range groups {
			normalizedGroup := strings.TrimSpace(group)
			if normalizedGroup == "" {
				continue
			}
			ability := model.Ability{
				Group:         normalizedGroup,
				Model:         normalizedModel,
				ChannelId:     channel.Id,
				UpstreamModel: upstream,
				Enabled:       channel.Status == model.ChannelStatusEnabled,
				Priority:      channel.Priority,
			}
			abilities = append(abilities, ability)
		}
	}
	return abilities
}

func DeleteAbilities(channel *model.Channel) error {
	if channel == nil {
		return nil
	}
	groups, err := listBoundGroupsByChannelID(channel.Id)
	if err != nil {
		return err
	}
	if err := model.DB.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error; err != nil {
		return err
	}
	if err := model.SyncGroupModelProvidersForGroups(groups...); err != nil {
		return err
	}
	model.RefreshAbilityCachesForGroups(groups...)
	return nil
}

func UpdateAbilities(channel *model.Channel) error {
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
			existing := make([]model.Ability, 0)
			groupCol := `"group"`
			if err := tx.Where(groupCol+" = ? AND channel_id = ?", groupID, channel.Id).Find(&existing).Error; err != nil {
				return err
			}
			next := model.SyncGroupAbilitiesForChannel(groupID, channel, existing)
			if err := tx.Where(groupCol+" = ? AND channel_id = ?", groupID, channel.Id).Delete(&model.Ability{}).Error; err != nil {
				return err
			}
			if len(next) == 0 {
				continue
			}
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		}
		return model.SyncGroupModelProvidersForGroupsWithDB(tx, groups...)
	})
	if err != nil {
		return err
	}
	model.RefreshAbilityCachesForGroups(groups...)
	return nil
}

func UpdateAbilityStatus(channelId string, status bool) error {
	groups, err := listBoundGroupsByChannelID(channelId)
	if err != nil {
		return err
	}
	if err := model.DB.Model(&model.Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error; err != nil {
		return err
	}
	if err := model.SyncGroupModelProvidersForGroups(groups...); err != nil {
		return err
	}
	model.RefreshAbilityCachesForGroups(groups...)
	return nil
}

func GetTopChannelByModel(group string, modelName string) (*model.Channel, error) {
	groupCol := `"group"`
	trueVal := "true"

	ability := model.Ability{}
	err := model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName).
		Order("priority desc, channel_id asc").
		First(&ability).Error
	if err != nil {
		return nil, err
	}
	channel := model.Channel{Id: ability.ChannelId}
	err = model.DB.Omit("key").First(&channel, "id = ?", ability.ChannelId).Error
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithModels(model.DB, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	groupCol := `"group"`
	trueVal := "true"
	var models []string
	err := model.DB.Model(&model.Ability{}).Distinct("model").Where(groupCol+" = ? and enabled = "+trueVal, group).Pluck("model", &models).Error
	if err != nil {
		return nil, err
	}
	sort.Strings(models)
	return models, nil
}
