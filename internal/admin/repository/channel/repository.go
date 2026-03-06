package channel

import (
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

func init() {
	model.BindChannelRepository(model.ChannelRepository{
		GetAllChannels:               GetAll,
		SearchChannels:               Search,
		GetChannelById:               GetByID,
		BatchInsertChannels:          BatchInsert,
		Insert:                       Insert,
		Update:                       Update,
		UpdateResponseTime:           UpdateResponseTime,
		UpdateBalance:                UpdateBalance,
		Delete:                       Delete,
		UpdateChannelStatusById:      UpdateStatusByID,
		UpdateChannelUsedQuota:       UpdateUsedQuota,
		UpdateChannelUsedQuotaDirect: UpdateUsedQuotaDirect,
		UpdateChannelTestModelByID:   UpdateTestModelByID,
		DeleteChannelByStatus:        DeleteByStatus,
		DeleteDisabledChannel:        DeleteDisabled,
	})
}

func GetAll(startIdx int, num int, status string) ([]*model.Channel, error) {
	var channels []*model.Channel
	var err error
	switch status {
	case "all":
		err = model.DB.Order("created_time desc").Find(&channels).Error
	case "disabled":
		err = model.DB.Order("created_time desc").Where("status = ? or status = ?", model.ChannelStatusAutoDisabled, model.ChannelStatusManuallyDisabled).Find(&channels).Error
	default:
		err = model.DB.Order("created_time desc").Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	}
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelsWithModels(model.DB, channels); err != nil {
		return nil, err
	}
	if err := model.HydrateChannelsWithCapabilityProfiles(model.DB, channels); err != nil {
		return nil, err
	}
	return channels, model.HydrateChannelsWithCapabilityResults(model.DB, channels)
}

func Search(keyword string) ([]*model.Channel, error) {
	var channels []*model.Channel
	trimmed := strings.TrimSpace(keyword)
	if trimmed == "" {
		return channels, nil
	}
	err := model.DB.Omit("key").Where("id = ? or name LIKE ?", trimmed, trimmed+"%").Find(&channels).Error
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelsWithModels(model.DB, channels); err != nil {
		return nil, err
	}
	if err := model.HydrateChannelsWithCapabilityProfiles(model.DB, channels); err != nil {
		return nil, err
	}
	return channels, model.HydrateChannelsWithCapabilityResults(model.DB, channels)
}

func GetByID(id string, selectAll bool) (*model.Channel, error) {
	id = strings.TrimSpace(id)
	channel := model.Channel{Id: id}
	var err error
	if selectAll {
		err = model.DB.First(&channel, "id = ?", id).Error
	} else {
		err = model.DB.Omit("key").First(&channel, "id = ?", id).Error
	}
	if err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithModels(model.DB, &channel); err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithCapabilityProfiles(model.DB, &channel); err != nil {
		return nil, err
	}
	return &channel, model.HydrateChannelWithCapabilityResults(model.DB, &channel)
}

func BatchInsert(channels []model.Channel) error {
	now := helper.GetTimestamp()
	for i := range channels {
		channels[i].NormalizeProtocol()
		if strings.TrimSpace(channels[i].Id) == "" {
			channels[i].Id = random.GetUUID()
		}
		if channels[i].CreatedTime == 0 {
			channels[i].CreatedTime = now
		}
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&channels).Error; err != nil {
			return err
		}
		for i := range channels {
			if err := model.ReplaceChannelSelectedModelsWithDB(tx, channels[i].Id, channels[i].SelectedModelIDs()); err != nil {
				return err
			}
			if err := model.ReplaceChannelCapabilityProfilesWithDB(tx, channels[i].Id, channels[i].CapabilityProfiles); err != nil {
				return err
			}
			if err := model.EnsureChannelTestModelWithDB(tx, channels[i].Id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := range channels {
		if err := model.HydrateChannelWithModels(model.DB, &channels[i]); err != nil {
			return err
		}
		if err := model.HydrateChannelWithCapabilityProfiles(model.DB, &channels[i]); err != nil {
			return err
		}
		if err := model.HydrateChannelWithCapabilityResults(model.DB, &channels[i]); err != nil {
			return err
		}
		if err := channels[i].AddAbilities(); err != nil {
			return err
		}
	}
	return nil
}

func Insert(channel *model.Channel) error {
	channel.NormalizeProtocol()
	if strings.TrimSpace(channel.Id) == "" {
		channel.Id = random.GetUUID()
	}
	if channel.CreatedTime == 0 {
		channel.CreatedTime = helper.GetTimestamp()
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if err := model.ReplaceChannelSelectedModelsWithDB(tx, channel.Id, channel.SelectedModelIDs()); err != nil {
			return err
		}
		if err := model.ReplaceChannelCapabilityProfilesWithDB(tx, channel.Id, channel.CapabilityProfiles); err != nil {
			return err
		}
		return model.EnsureChannelTestModelWithDB(tx, channel.Id)
	})
	if err != nil {
		return err
	}
	if err := model.HydrateChannelWithModels(model.DB, channel); err != nil {
		return err
	}
	if err := model.HydrateChannelWithCapabilityProfiles(model.DB, channel); err != nil {
		return err
	}
	if err := model.HydrateChannelWithCapabilityResults(model.DB, channel); err != nil {
		return err
	}
	return channel.AddAbilities()
}

func sameStringPointerValue(left *string, right *string) bool {
	leftValue := ""
	rightValue := ""
	if left != nil {
		leftValue = strings.TrimSpace(*left)
	}
	if right != nil {
		rightValue = strings.TrimSpace(*right)
	}
	return leftValue == rightValue
}

func shouldResetCapabilityResults(existing *model.Channel, incoming *model.Channel) bool {
	if existing == nil || incoming == nil {
		return false
	}
	if incoming.CapabilityResultsStale {
		return true
	}
	if strings.TrimSpace(incoming.Protocol) != "" && existing.GetProtocol() != incoming.GetProtocol() {
		return true
	}
	if incoming.BaseURL != nil && !sameStringPointerValue(existing.BaseURL, incoming.BaseURL) {
		return true
	}
	if strings.TrimSpace(incoming.Key) != "" && strings.TrimSpace(existing.Key) != strings.TrimSpace(incoming.Key) {
		return true
	}
	if strings.TrimSpace(incoming.Config) != "" && strings.TrimSpace(existing.Config) != strings.TrimSpace(incoming.Config) {
		return true
	}
	if incoming.ModelMapping != nil && !sameStringPointerValue(existing.ModelMapping, incoming.ModelMapping) {
		return true
	}
	if strings.TrimSpace(incoming.TestModel) != "" && strings.TrimSpace(existing.TestModel) != strings.TrimSpace(incoming.TestModel) {
		return true
	}
	if incoming.ModelsProvided &&
		model.JoinChannelModelCSV(existing.SelectedModelIDs()) != model.JoinChannelModelCSV(incoming.SelectedModelIDs()) {
		return true
	}
	if incoming.CapabilityProfilesProvided {
		current := model.NormalizeChannelCapabilityProfileRules(existing.CapabilityProfiles)
		next := model.NormalizeChannelCapabilityProfileRules(incoming.CapabilityProfiles)
		if len(current) != len(next) {
			return true
		}
		for i := range current {
			if current[i] != next[i] {
				return true
			}
		}
	}
	return false
}

func Update(channel *model.Channel) error {
	if strings.TrimSpace(channel.Protocol) != "" {
		channel.NormalizeProtocol()
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing := model.Channel{}
		if err := tx.First(&existing, "id = ?", channel.Id).Error; err != nil {
			return err
		}
		if err := model.HydrateChannelWithModels(tx, &existing); err != nil {
			return err
		}
		if err := model.HydrateChannelWithCapabilityProfiles(tx, &existing); err != nil {
			return err
		}
		resetCapabilityResults := shouldResetCapabilityResults(&existing, channel)
		if err := tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(channel).Error; err != nil {
			return err
		}
		if channel.ModelsProvided {
			if err := model.ReplaceChannelSelectedModelsWithDB(tx, channel.Id, channel.SelectedModelIDs()); err != nil {
				return err
			}
			if err := model.EnsureChannelTestModelWithDB(tx, channel.Id); err != nil {
				return err
			}
		}
		if channel.CapabilityProfilesProvided {
			if err := model.ReplaceChannelCapabilityProfilesWithDB(tx, channel.Id, channel.CapabilityProfiles); err != nil {
				return err
			}
		}
		if resetCapabilityResults {
			if err := model.DeleteChannelCapabilityResultsByChannelIDWithDB(tx, channel.Id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := model.DB.First(channel, "id = ?", channel.Id).Error; err != nil {
		return err
	}
	if err := model.HydrateChannelWithModels(model.DB, channel); err != nil {
		return err
	}
	if err := model.HydrateChannelWithCapabilityProfiles(model.DB, channel); err != nil {
		return err
	}
	if err := model.HydrateChannelWithCapabilityResults(model.DB, channel); err != nil {
		return err
	}
	return channel.UpdateAbilities()
}

func UpdateResponseTime(channel *model.Channel, responseTime int64) {
	err := model.DB.Model(channel).Select("response_time", "test_time").Updates(model.Channel{
		TestTime:     helper.GetTimestamp(),
		ResponseTime: int(responseTime),
	}).Error
	if err != nil {
		logger.SysError("failed to update response time: " + err.Error())
	}
}

func UpdateBalance(channel *model.Channel, balance float64) {
	err := model.DB.Model(channel).Select("balance_updated_time", "balance").Updates(model.Channel{
		BalanceUpdatedTime: helper.GetTimestamp(),
		Balance:            balance,
	}).Error
	if err != nil {
		logger.SysError("failed to update balance: " + err.Error())
	}
}

func Delete(channel *model.Channel) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.DeleteChannelModelsByChannelIDWithDB(tx, channel.Id); err != nil {
			return err
		}
		if err := model.DeleteChannelCapabilityResultsByChannelIDWithDB(tx, channel.Id); err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", strings.TrimSpace(channel.Id)).Delete(&model.ChannelCapabilityProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", strings.TrimSpace(channel.Id)).Delete(&model.Ability{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Channel{}, "id = ?", strings.TrimSpace(channel.Id)).Error
	})
}

func DeleteByID(id string) error {
	id = strings.TrimSpace(id)
	channel := model.Channel{Id: id}
	return Delete(&channel)
}

func DeleteDisabled() (int64, error) {
	return deleteChannelsByQuery(model.DB.Where("status = ? or status = ?", model.ChannelStatusAutoDisabled, model.ChannelStatusManuallyDisabled))
}

func UpdateStatusByID(id string, status int) {
	id = strings.TrimSpace(id)
	err := model.UpdateAbilityStatus(id, status == model.ChannelStatusEnabled)
	if err != nil {
		logger.SysError("failed to update ability status: " + err.Error())
	}
	err = model.DB.Model(&model.Channel{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		logger.SysError("failed to update channel status: " + err.Error())
	}
}

func UpdateUsedQuota(id string, quota int64) {
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeChannelUsedQuota, id, quota)
		return
	}
	UpdateUsedQuotaDirect(id, quota)
}

func UpdateUsedQuotaDirect(id string, quota int64) {
	err := model.DB.Model(&model.Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
	if err != nil {
		logger.SysError("failed to update channel used quota: " + err.Error())
	}
}

func UpdateTestModelByID(id string, testModel string) error {
	id = strings.TrimSpace(id)
	testModel = strings.TrimSpace(testModel)
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Channel{}).Where("id = ?", id).Update("test_model", testModel).Error; err != nil {
			return err
		}
		return model.DeleteChannelCapabilityResultsByChannelIDWithDB(tx, id)
	})
}

func DeleteByStatus(status int64) (int64, error) {
	return deleteChannelsByQuery(model.DB.Where("status = ?", status))
}

func deleteChannelsByQuery(query *gorm.DB) (int64, error) {
	channelIDs := make([]string, 0)
	if err := query.Model(&model.Channel{}).Pluck("id", &channelIDs).Error; err != nil {
		return 0, err
	}
	if len(channelIDs) == 0 {
		return 0, nil
	}
	var rowsAffected int64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.DeleteChannelModelsByChannelIDsWithDB(tx, channelIDs); err != nil {
			return err
		}
		if err := model.DeleteChannelCapabilityResultsByChannelIDsWithDB(tx, channelIDs); err != nil {
			return err
		}
		if err := tx.Where("channel_id IN ?", channelIDs).Delete(&model.Ability{}).Error; err != nil {
			return err
		}
		result := tx.Where("id IN ?", channelIDs).Delete(&model.Channel{})
		rowsAffected = result.RowsAffected
		return result.Error
	})
	return rowsAffected, err
}
