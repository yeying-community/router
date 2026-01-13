package redemption

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindRedemptionRepository(model.RedemptionRepository{
		GetAllRedemptions:    GetAll,
		SearchRedemptions:    Search,
		GetRedemptionById:    GetByID,
		Redeem:               Redeem,
		Insert:               Create,
		SelectUpdate:         SelectUpdate,
		Update:               Update,
		Delete:               Delete,
		DeleteRedemptionById: DeleteByID,
	})
}

func GetAll(startIdx int, num int) ([]*model.Redemption, error) {
	var redemptions []*model.Redemption
	err := model.DB.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	return redemptions, err
}

func Search(keyword string) ([]*model.Redemption, error) {
	var redemptions []*model.Redemption
	err := model.DB.Where("id = ? or name LIKE ?", keyword, keyword+"%").Find(&redemptions).Error
	return redemptions, err
}

func GetByID(id int) (*model.Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := model.Redemption{Id: id}
	err := model.DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(ctx context.Context, key string, userId int) (int64, error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return 0, errors.New("无效的 user id")
	}
	redemption := &model.Redemption{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != model.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		err = tx.Model(&model.User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		if err != nil {
			return err
		}
		redemption.RedeemedTime = helper.GetTimestamp()
		redemption.Status = model.RedemptionCodeStatusUsed
		return tx.Save(redemption).Error
	})
	if err != nil {
		return 0, errors.New("兑换失败，" + err.Error())
	}
	model.RecordLog(ctx, userId, model.LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s", common.LogQuota(redemption.Quota)))
	return redemption.Quota, nil
}

func Create(redemption *model.Redemption) error {
	return model.DB.Create(redemption).Error
}

func SelectUpdate(redemption *model.Redemption) error {
	return model.DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

func Update(redemption *model.Redemption) error {
	return model.DB.Model(redemption).Select("name", "status", "quota", "redeemed_time").Updates(redemption).Error
}

func Delete(redemption *model.Redemption) error {
	return model.DB.Delete(redemption).Error
}

func DeleteByID(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := model.Redemption{Id: id}
	err := model.DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return Delete(&redemption)
}
