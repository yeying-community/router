package redemption

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
)

func hydrateRedemptionGroupNamesWithDB(db *gorm.DB, rows []*model.Redemption) error {
	if db == nil || len(rows) == 0 {
		return nil
	}
	groupIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		groupID := strings.TrimSpace(row.GroupID)
		if groupID == "" {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		groupIDs = append(groupIDs, groupID)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	groups := make([]model.GroupCatalog, 0, len(groupIDs))
	if err := db.Select("id", "name").Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return err
	}
	nameByID := make(map[string]string, len(groups))
	for _, group := range groups {
		groupID := strings.TrimSpace(group.Id)
		if groupID == "" {
			continue
		}
		nameByID[groupID] = strings.TrimSpace(group.Name)
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.GroupName = nameByID[strings.TrimSpace(row.GroupID)]
	}
	return nil
}

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
	err := model.DB.Order("created_time desc, id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		return nil, err
	}
	if err := hydrateRedemptionGroupNamesWithDB(model.DB, redemptions); err != nil {
		return nil, err
	}
	return redemptions, err
}

func Search(keyword string) ([]*model.Redemption, error) {
	var redemptions []*model.Redemption
	trimmed := strings.TrimSpace(keyword)
	if trimmed == "" {
		return redemptions, nil
	}
	err := model.DB.
		Where("code LIKE ? OR name LIKE ?", trimmed+"%", trimmed+"%").
		Order("created_time desc, id desc").
		Find(&redemptions).Error
	if err != nil {
		return nil, err
	}
	if err := hydrateRedemptionGroupNamesWithDB(model.DB, redemptions); err != nil {
		return nil, err
	}
	return redemptions, err
}

func GetByID(id string) (*model.Redemption, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("id 为空！")
	}
	redemption := model.Redemption{Id: id}
	err := model.DB.First(&redemption, "id = ?", id).Error
	if err == nil && strings.TrimSpace(redemption.RedeemedByUserId) != "" {
		redemption.RedeemedByUsername = model.GetUsernameById(redemption.RedeemedByUserId)
	}
	if err == nil {
		_ = hydrateRedemptionGroupNamesWithDB(model.DB, []*model.Redemption{&redemption})
	}
	return &redemption, err
}

func Redeem(ctx context.Context, code string, userId string) (model.RedemptionResult, error) {
	if code == "" {
		return model.RedemptionResult{}, errors.New("未提供兑换码")
	}
	if strings.TrimSpace(userId) == "" {
		return model.RedemptionResult{}, errors.New("无效的 user id")
	}
	redemption := &model.Redemption{}
	user := &model.User{}
	result := model.RedemptionResult{}
	codeCol := `"code"`

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where(codeCol+" = ?", code).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != model.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if strings.TrimSpace(redemption.TopupOrderID) != "" {
			order := model.TopupOrder{}
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ?", strings.TrimSpace(redemption.TopupOrderID)).
				First(&order).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("关联订单不存在")
				}
				return err
			}
			if strings.TrimSpace(order.UserID) != strings.TrimSpace(userId) {
				return errors.New("该兑换码不属于当前用户")
			}
			switch model.NormalizeTopupOrderStatus(order.Status) {
			case model.TopupOrderStatusPaid, model.TopupOrderStatusFulfilled:
				// allowed
			case model.TopupOrderStatusCreated, model.TopupOrderStatusPending:
				return errors.New("当前订单尚未支付")
			case model.TopupOrderStatusFailed, model.TopupOrderStatusCanceled:
				return errors.New("当前订单状态不允许兑换")
			default:
				return errors.New("当前订单状态不允许兑换")
			}
		}
		err = tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", userId).First(user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("用户不存在")
			}
			return err
		}
		resolvedGroupID := strings.TrimSpace(redemption.GroupID)
		if resolvedGroupID != "" {
			groupRow, err := model.ResolveRedemptionGroupWithDB(tx, resolvedGroupID)
			if err != nil {
				return err
			}
			redemption.GroupID = strings.TrimSpace(groupRow.Id)
			redemption.GroupName = strings.TrimSpace(groupRow.Name)
		}
		beforeYYCBalance := user.Quota
		afterYYCBalance := beforeYYCBalance + redemption.Quota
		userUpdates := map[string]any{
			"quota": afterYYCBalance,
		}
		if resolvedGroupID != "" {
			userUpdates["group"] = resolvedGroupID
		}
		err = tx.Model(&model.User{}).Where("id = ?", userId).Updates(userUpdates).Error
		if err != nil {
			return err
		}
		redemption.RedeemedTime = helper.GetTimestamp()
		redemption.Status = model.RedemptionCodeStatusUsed
		redemption.RedeemedByUserId = strings.TrimSpace(userId)
		if err := tx.Save(redemption).Error; err != nil {
			return err
		}
		result = model.RedemptionResult{
			RedeemedYYC:      redemption.Quota,
			BeforeYYCBalance: beforeYYCBalance,
			AfterYYCBalance:  afterYYCBalance,
			RedemptionID:     strings.TrimSpace(redemption.Id),
			RedemptionName:   strings.TrimSpace(redemption.Name),
			GroupID:          strings.TrimSpace(redemption.GroupID),
			GroupName:        strings.TrimSpace(redemption.GroupName),
			FaceValueAmount:  redemption.FaceValueAmount,
			FaceValueUnit:    strings.TrimSpace(redemption.FaceValueUnit),
			RedeemedAt:       redemption.RedeemedTime,
		}
		if strings.TrimSpace(redemption.TopupOrderID) != "" {
			if err := model.MarkTopupOrderRedeemedWithDB(tx, redemption.TopupOrderID, redemption.Id, redemption.RedeemedTime); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return model.RedemptionResult{}, errors.New("兑换失败，" + err.Error())
	}
	model.RecordLog(ctx, userId, model.LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s", common.LogQuota(redemption.Quota)))
	return result, nil
}

func Create(redemption *model.Redemption) error {
	if strings.TrimSpace(redemption.Id) == "" {
		redemption.Id = random.GetUUID()
	}
	return model.DB.Create(redemption).Error
}

func SelectUpdate(redemption *model.Redemption) error {
	return model.DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

func Update(redemption *model.Redemption) error {
	return model.DB.Model(redemption).Select("name", "status", "group_id", "face_value_amount", "face_value_unit", "quota", "redeemed_time").Updates(redemption).Error
}

func Delete(redemption *model.Redemption) error {
	return model.DB.Delete(redemption).Error
}

func DeleteByID(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("id 为空！")
	}
	redemption := model.Redemption{Id: id}
	err := model.DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return Delete(&redemption)
}
