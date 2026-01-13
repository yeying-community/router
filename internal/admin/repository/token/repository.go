package token

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindTokenRepository(model.TokenRepository{
		GetAllUserTokens:         GetAll,
		GetFirstAvailableToken:   GetFirstAvailable,
		SearchUserTokens:         Search,
		ValidateUserToken:        ValidateUserToken,
		GetTokenByIds:            GetByIDs,
		GetTokenById:             GetByID,
		Insert:                   Create,
		Update:                   Update,
		SelectUpdate:             SelectUpdate,
		Delete:                   Delete,
		DeleteTokenById:          DeleteByID,
		IncreaseTokenQuota:       IncreaseQuota,
		DecreaseTokenQuota:       DecreaseQuota,
		IncreaseTokenQuotaDirect: IncreaseQuotaDirect,
		DecreaseTokenQuotaDirect: DecreaseQuotaDirect,
	})
}

func GetAll(userId, start, num int, order string) ([]*model.Token, error) {
	var tokens []*model.Token
	query := model.DB.Where("user_id = ?", userId)

	switch order {
	case "remain_quota":
		query = query.Order("unlimited_quota desc, remain_quota desc")
	case "used_quota":
		query = query.Order("used_quota desc")
	default:
		query = query.Order("id desc")
	}

	err := query.Limit(num).Offset(start).Find(&tokens).Error
	return tokens, err
}

func GetFirstAvailable(userId int) (*model.Token, error) {
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}
	var token model.Token
	now := helper.GetTimestamp()
	err := model.DB.Where("user_id = ? AND status = ?", userId, model.TokenStatusEnabled).
		Where("(expired_time = -1 OR expired_time > ?)", now).
		Where("(unlimited_quota <> 0 OR remain_quota > 0)").
		Order("id asc").
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func Search(userId int, keyword string) ([]*model.Token, error) {
	var tokens []*model.Token
	err := model.DB.Where("user_id = ?", userId).Where("name LIKE ?", keyword+"%").Find(&tokens).Error
	return tokens, err
}

func ValidateUserToken(key string) (*model.Token, error) {
	if key == "" {
		return nil, errors.New("未提供令牌")
	}
	token, err := model.CacheGetTokenByKey(key)
	if err != nil {
		logger.SysError("CacheGetTokenByKey failed: " + err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("无效的令牌")
		}
		return nil, errors.New("令牌验证失败")
	}
	if token.Status == model.TokenStatusExhausted {
		return nil, fmt.Errorf("令牌 %s（#%d）额度已用尽", token.Name, token.Id)
	} else if token.Status == model.TokenStatusExpired {
		return nil, errors.New("该令牌已过期")
	}
	if token.Status != model.TokenStatusEnabled {
		return nil, errors.New("该令牌状态不可用")
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < helper.GetTimestamp() {
		if !common.RedisEnabled {
			token.Status = model.TokenStatusExpired
			err := SelectUpdate(token)
			if err != nil {
				logger.SysError("failed to update token status" + err.Error())
			}
		}
		return nil, errors.New("该令牌已过期")
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		if !common.RedisEnabled {
			token.Status = model.TokenStatusExhausted
			err := SelectUpdate(token)
			if err != nil {
				logger.SysError("failed to update token status" + err.Error())
			}
		}
		return nil, errors.New("该令牌额度已用尽")
	}
	return token, nil
}

func GetByIDs(tokenId, userId int) (*model.Token, error) {
	if tokenId == 0 || userId == 0 {
		return nil, errors.New("id 或 userId 为空！")
	}
	token := model.Token{Id: tokenId, UserId: userId}
	err := model.DB.First(&token, "id = ? and user_id = ?", tokenId, userId).Error
	return &token, err
}

func GetByID(tokenId int) (*model.Token, error) {
	if tokenId == 0 {
		return nil, errors.New("id 为空！")
	}
	token := model.Token{Id: tokenId}
	err := model.DB.First(&token, "id = ?", tokenId).Error
	return &token, err
}

func Create(token *model.Token) error {
	return model.DB.Create(token).Error
}

func Update(token *model.Token) error {
	return model.DB.Model(token).Select("name", "status", "expired_time", "remain_quota", "unlimited_quota", "models", "subnet").Updates(token).Error
}

func SelectUpdate(token *model.Token) error {
	return model.DB.Model(token).Select("accessed_time", "status").Updates(token).Error
}

func Delete(token *model.Token) error {
	return model.DB.Delete(token).Error
}

func DeleteByID(tokenId, userId int) error {
	if tokenId == 0 || userId == 0 {
		return errors.New("id 或 userId 为空！")
	}
	token := model.Token{Id: tokenId, UserId: userId}
	err := model.DB.Where(token).First(&token).Error
	if err != nil {
		return err
	}
	return Delete(&token)
}

func IncreaseQuota(id int, quota int64) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeTokenQuota, id, quota)
		return nil
	}
	return IncreaseQuotaDirect(id, quota)
}

func IncreaseQuotaDirect(id int, quota int64) error {
	return model.DB.Model(&model.Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota + ?", quota),
			"used_quota":    gorm.Expr("used_quota - ?", quota),
			"accessed_time": helper.GetTimestamp(),
		},
	).Error
}

func DecreaseQuota(id int, quota int64) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeTokenQuota, id, -quota)
		return nil
	}
	return DecreaseQuotaDirect(id, quota)
}

func DecreaseQuotaDirect(id int, quota int64) error {
	return model.DB.Model(&model.Token{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"remain_quota":  gorm.Expr("remain_quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"accessed_time": helper.GetTimestamp(),
		},
	).Error
}
