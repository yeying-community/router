package channel

import (
	"errors"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

func init() {
	model.BindChannelRepository(model.ChannelRepository{
		GetChannelById:               GetByID,
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

func buildChannelListQuery(db *gorm.DB, keyword string) *gorm.DB {
	query := db.Model(&model.Channel{})
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		"LOWER(name) LIKE ? OR LOWER(protocol) LIKE ? OR LOWER(COALESCE(base_url, '')) LIKE ?",
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func ListPage(page int, pageSize int, keyword string) ([]*model.Channel, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = config.ItemsPerPage
	}
	total := int64(0)
	query := buildChannelListQuery(model.DB, keyword)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	channels := make([]*model.Channel, 0, pageSize)
	if err := query.
		Order("created_time desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Omit("key").
		Find(&channels).Error; err != nil {
		return nil, 0, err
	}
	if err := model.HydrateChannelsWithModels(model.DB, channels); err != nil {
		return nil, 0, err
	}
	return channels, total, nil
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
	return channels, nil
}

func GetByID(id string) (*model.Channel, error) {
	id = strings.TrimSpace(id)
	channel := model.Channel{Id: id}
	if err := model.DB.First(&channel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := model.HydrateChannelWithModels(model.DB, &channel); err != nil {
		return nil, err
	}
	return &channel, model.HydrateChannelWithTests(model.DB, &channel)
}

func GetBasicByID(id string) (*model.Channel, error) {
	id = strings.TrimSpace(id)
	channel := model.Channel{Id: id}
	if err := model.DB.Omit("key").First(&channel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func GetAllBasic(startIdx int, num int, status string, selectAll bool) ([]*model.Channel, error) {
	var channels []*model.Channel
	query := model.DB.Order("created_time desc")
	if !selectAll {
		query = query.Omit("key")
	}
	switch status {
	case "all":
	case "disabled":
		query = query.Where("status = ? or status = ?", model.ChannelStatusAutoDisabled, model.ChannelStatusManuallyDisabled)
	default:
		if num > 0 {
			query = query.Limit(num).Offset(startIdx)
		}
		if !selectAll {
			query = query.Omit("key")
		}
	}
	if err := query.Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func prepareChannelForCreate(channel *model.Channel) error {
	if channel == nil {
		return gorm.ErrInvalidData
	}
	channel.NormalizeIdentity()
	channel.EnsureID()
	if err := channel.ValidateIdentifier(); err != nil {
		return err
	}
	channel.NormalizeProtocol()
	channel.NormalizeModelConfigState()
	if channel.CreatedTime == 0 {
		channel.CreatedTime = helper.GetTimestamp()
	}
	return nil
}

func ensureChannelIdentifierUniqueWithDB(db *gorm.DB, channel *model.Channel) error {
	if db == nil || channel == nil {
		return nil
	}
	identifier := model.NormalizeChannelIdentifier(channel.Name)
	if identifier == "" {
		return nil
	}
	existing := model.Channel{}
	if err := db.Select("id", "name").Where("name = ?", identifier).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(existing.Id) == strings.TrimSpace(channel.Id) {
		return nil
	}
	return errors.New("渠道标识已存在")
}

func Insert(channel *model.Channel) error {
	if err := prepareChannelForCreate(channel); err != nil {
		return err
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := ensureChannelIdentifierUniqueWithDB(tx, channel); err != nil {
			return err
		}
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		if err := model.ReplaceChannelModelConfigsWithDB(tx, channel.Id, channel.GetModelConfigs()); err != nil {
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
	if err := model.HydrateChannelWithTests(model.DB, channel); err != nil {
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

func buildSelectedModelTestSignature(channel *model.Channel) string {
	if channel == nil {
		return ""
	}
	rows := channel.GetModelConfigs()
	if len(rows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if !row.Selected {
			continue
		}
		modelID := strings.TrimSpace(row.Model)
		upstreamModel := strings.TrimSpace(row.UpstreamModel)
		modelType := strings.TrimSpace(row.Type)
		if modelID == "" {
			continue
		}
		if upstreamModel == "" {
			upstreamModel = modelID
		}
		endpoint := strings.TrimSpace(row.Endpoint)
		parts = append(parts, modelID+"=>"+upstreamModel+"#"+modelType+"@"+endpoint)
	}
	return strings.Join(parts, "|")
}

func shouldResetChannelTests(existing *model.Channel, incoming *model.Channel) bool {
	if existing == nil || incoming == nil {
		return false
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
	if strings.TrimSpace(incoming.TestModel) != "" && strings.TrimSpace(existing.TestModel) != strings.TrimSpace(incoming.TestModel) {
		return true
	}
	if incoming.ModelConfigsProvided &&
		buildSelectedModelTestSignature(existing) != buildSelectedModelTestSignature(incoming) {
		return true
	}
	if incoming.ModelsProvided &&
		model.JoinChannelModelCSV(existing.SelectedModelIDs()) != model.JoinChannelModelCSV(incoming.SelectedModelIDs()) {
		return true
	}
	return false
}

func Update(channel *model.Channel) error {
	channel.NormalizeIdentity()
	channel.Id = strings.TrimSpace(channel.Id)
	if channel.Id == "" {
		return errors.New("渠道 ID 不能为空")
	}
	if strings.TrimSpace(channel.Protocol) != "" {
		channel.NormalizeProtocol()
	}
	channel.NormalizeModelConfigState()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		existing := model.Channel{}
		if err := tx.First(&existing, "id = ?", channel.Id).Error; err != nil {
			return err
		}
		if err := model.HydrateChannelWithModels(tx, &existing); err != nil {
			return err
		}
		if !channel.NameProvided || strings.TrimSpace(channel.Name) == "" {
			channel.Name = existing.Name
		}
		if err := channel.ValidateIdentifier(); err != nil {
			return err
		}
		if err := ensureChannelIdentifierUniqueWithDB(tx, channel); err != nil {
			return err
		}
		resetChannelTests := shouldResetChannelTests(&existing, channel)
		if channel.NameProvided {
			if err := tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("name", model.NormalizeChannelIdentifier(channel.Name)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Channel{}).Where("id = ?", channel.Id).Omit("name").Updates(channel).Error; err != nil {
			return err
		}
		if channel.ModelConfigsProvided {
			if err := model.ReplaceChannelModelConfigsWithDB(tx, channel.Id, channel.GetModelConfigs()); err != nil {
				return err
			}
			if err := model.EnsureChannelTestModelWithDB(tx, channel.Id); err != nil {
				return err
			}
		} else if channel.ModelsProvided {
			if err := model.ReplaceChannelSelectedModelsWithDB(tx, channel.Id, channel.SelectedModelIDs()); err != nil {
				return err
			}
			if err := model.EnsureChannelTestModelWithDB(tx, channel.Id); err != nil {
				return err
			}
		}
		if resetChannelTests {
			if err := model.DeleteChannelTestsByChannelIDWithDB(tx, channel.Id); err != nil {
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
	if err := model.HydrateChannelWithTests(model.DB, channel); err != nil {
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
		if err := model.DeleteChannelTestsByChannelIDWithDB(tx, channel.Id); err != nil {
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
		return model.DeleteChannelTestsByChannelIDWithDB(tx, id)
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
		if err := model.DeleteChannelTestsByChannelIDsWithDB(tx, channelIDs); err != nil {
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
