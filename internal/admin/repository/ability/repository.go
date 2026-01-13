package ability

import (
	"context"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/utils"
	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindAbilityRepository(model.AbilityRepository{
		GetRandomSatisfiedChannel: GetRandomSatisfiedChannel,
		AddAbilities:              AddAbilities,
		DeleteAbilities:           DeleteAbilities,
		UpdateAbilities:           UpdateAbilities,
		UpdateAbilityStatus:       UpdateAbilityStatus,
		GetTopChannelByModel:      GetTopChannelByModel,
		GetGroupModels:            GetGroupModels,
	})
}

func GetRandomSatisfiedChannel(group string, modelName string, ignoreFirstPriority bool) (*model.Channel, error) {
	ability := model.Ability{}
	groupCol := "`group`"
	trueVal := "1"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
		trueVal = "true"
	}

	var channelQuery *gorm.DB
	if ignoreFirstPriority {
		channelQuery = model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName)
	} else {
		maxPrioritySubQuery := model.DB.Model(&model.Ability{}).Select("MAX(priority)").Where(groupCol+" = ? and model = ? and enabled = "+trueVal, group, modelName)
		channelQuery = model.DB.Where(groupCol+" = ? and model = ? and enabled = "+trueVal+" and priority = (?)", group, modelName, maxPrioritySubQuery)
	}
	if common.UsingSQLite || common.UsingPostgreSQL {
		if err := channelQuery.Order("RANDOM()").First(&ability).Error; err != nil {
			return nil, err
		}
	} else {
		if err := channelQuery.Order("RAND()").First(&ability).Error; err != nil {
			return nil, err
		}
	}
	channel := model.Channel{Id: ability.ChannelId}
	err := model.DB.First(&channel, "id = ?", ability.ChannelId).Error
	return &channel, err
}

func AddAbilities(channel *model.Channel) error {
	models := strings.Split(channel.Models, ",")
	models = utils.DeDuplication(models)
	groups := strings.Split(channel.Group, ",")
	abilities := make([]model.Ability, 0, len(models)*len(groups))
	for _, modelName := range models {
		for _, group := range groups {
			ability := model.Ability{
				Group:     group,
				Model:     modelName,
				ChannelId: channel.Id,
				Enabled:   channel.Status == model.ChannelStatusEnabled,
				Priority:  channel.Priority,
			}
			abilities = append(abilities, ability)
		}
	}
	return model.DB.Create(&abilities).Error
}

func DeleteAbilities(channel *model.Channel) error {
	return model.DB.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error
}

func UpdateAbilities(channel *model.Channel) error {
	err := DeleteAbilities(channel)
	if err != nil {
		return err
	}
	return AddAbilities(channel)
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return model.DB.Model(&model.Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func GetTopChannelByModel(group string, modelName string) (*model.Channel, error) {
	groupCol := "`group`"
	trueVal := "1"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
		trueVal = "true"
	}

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
	return &channel, nil
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	groupCol := "`group`"
	trueVal := "1"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
		trueVal = "true"
	}
	var models []string
	err := model.DB.Model(&model.Ability{}).Distinct("model").Where(groupCol+" = ? and enabled = "+trueVal, group).Pluck("model", &models).Error
	if err != nil {
		return nil, err
	}
	sort.Strings(models)
	return models, nil
}
