package channel

import (
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
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
		DeleteChannelByStatus:        DeleteByStatus,
		DeleteDisabledChannel:        DeleteDisabled,
	})
}

func GetAll(startIdx int, num int, status string) ([]*model.Channel, error) {
	var channels []*model.Channel
	var err error
	switch status {
	case "all":
		err = model.DB.Order("id desc").Find(&channels).Error
	case "disabled":
		err = model.DB.Order("id desc").Where("status = ? or status = ?", model.ChannelStatusAutoDisabled, model.ChannelStatusManuallyDisabled).Find(&channels).Error
	default:
		err = model.DB.Order("id desc").Limit(num).Offset(startIdx).Omit("key").Find(&channels).Error
	}
	return channels, err
}

func Search(keyword string) ([]*model.Channel, error) {
	var channels []*model.Channel
	err := model.DB.Omit("key").Where("id = ? or name LIKE ?", helper.String2Int(keyword), keyword+"%").Find(&channels).Error
	return channels, err
}

func GetByID(id int, selectAll bool) (*model.Channel, error) {
	channel := model.Channel{Id: id}
	var err error
	if selectAll {
		err = model.DB.First(&channel, "id = ?", id).Error
	} else {
		err = model.DB.Omit("key").First(&channel, "id = ?", id).Error
	}
	return &channel, err
}

func BatchInsert(channels []model.Channel) error {
	err := model.DB.Create(&channels).Error
	if err != nil {
		return err
	}
	for _, channel := range channels {
		err = channel.AddAbilities()
		if err != nil {
			return err
		}
	}
	return nil
}

func Insert(channel *model.Channel) error {
	err := model.DB.Create(channel).Error
	if err != nil {
		return err
	}
	return channel.AddAbilities()
}

func Update(channel *model.Channel) error {
	err := model.DB.Model(channel).Updates(channel).Error
	if err != nil {
		return err
	}
	model.DB.Model(channel).First(channel, "id = ?", channel.Id)
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
	err := model.DB.Delete(channel).Error
	if err != nil {
		return err
	}
	return channel.DeleteAbilities()
}

func DeleteByID(id int) error {
	channel := model.Channel{Id: id}
	return Delete(&channel)
}

func DeleteDisabled() (int64, error) {
	result := model.DB.Where("status = ? or status = ?", model.ChannelStatusAutoDisabled, model.ChannelStatusManuallyDisabled).Delete(&model.Channel{})
	return result.RowsAffected, result.Error
}

func UpdateStatusByID(id int, status int) {
	err := model.UpdateAbilityStatus(id, status == model.ChannelStatusEnabled)
	if err != nil {
		logger.SysError("failed to update ability status: " + err.Error())
	}
	err = model.DB.Model(&model.Channel{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		logger.SysError("failed to update channel status: " + err.Error())
	}
}

func UpdateUsedQuota(id int, quota int64) {
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeChannelUsedQuota, id, quota)
		return
	}
	UpdateUsedQuotaDirect(id, quota)
}

func UpdateUsedQuotaDirect(id int, quota int64) {
	err := model.DB.Model(&model.Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
	if err != nil {
		logger.SysError("failed to update channel used quota: " + err.Error())
	}
}

func DeleteByStatus(status int64) (int64, error) {
	result := model.DB.Where("status = ?", status).Delete(&model.Channel{})
	return result.RowsAffected, result.Error
}
