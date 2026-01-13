package model

import (
	"errors"
	"fmt"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/message"
)

const (
	TokenStatusEnabled   = 1 // don't use 0, 0 is the default value!
	TokenStatusDisabled  = 2 // also don't use 0
	TokenStatusExpired   = 3
	TokenStatusExhausted = 4
)

type Token struct {
	Id             int     `json:"id"`
	UserId         int     `json:"user_id"`
	Key            string  `json:"key" gorm:"type:char(48);uniqueIndex"`
	Status         int     `json:"status" gorm:"default:1"`
	Name           string  `json:"name" gorm:"index" `
	CreatedTime    int64   `json:"created_time" gorm:"bigint"`
	AccessedTime   int64   `json:"accessed_time" gorm:"bigint"`
	ExpiredTime    int64   `json:"expired_time" gorm:"bigint;default:-1"`
	RemainQuota    int64   `json:"remain_quota" gorm:"bigint;default:0"`
	UnlimitedQuota bool    `json:"unlimited_quota" gorm:"default:false"`
	UsedQuota      int64   `json:"used_quota" gorm:"bigint;default:0"`
	Models         *string `json:"models" gorm:"type:text"`
	Subnet         *string `json:"subnet" gorm:"default:''"`
}

func GetAllUserTokens(userId int, startIdx int, num int, order string) ([]*Token, error) {
	return mustTokenRepo().GetAllUserTokens(userId, startIdx, num, order)
}

// GetFirstAvailableToken returns the earliest created enabled token of a user
// that is not expired and still has quota (or unlimited).
func GetFirstAvailableToken(userId int) (*Token, error) {
	return mustTokenRepo().GetFirstAvailableToken(userId)
}

func SearchUserTokens(userId int, keyword string) ([]*Token, error) {
	return mustTokenRepo().SearchUserTokens(userId, keyword)
}

func ValidateUserToken(key string) (*Token, error) {
	return mustTokenRepo().ValidateUserToken(key)
}

func GetTokenByIds(id int, userId int) (*Token, error) {
	return mustTokenRepo().GetTokenByIds(id, userId)
}

func GetTokenById(id int) (*Token, error) {
	return mustTokenRepo().GetTokenById(id)
}

func (t *Token) Insert() error {
	return mustTokenRepo().Insert(t)
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (t *Token) Update() error {
	return mustTokenRepo().Update(t)
}

func (t *Token) SelectUpdate() error {
	return mustTokenRepo().SelectUpdate(t)
}

func (t *Token) Delete() error {
	return mustTokenRepo().Delete(t)
}

func (t *Token) GetModels() string {
	if t == nil {
		return ""
	}
	if t.Models == nil {
		return ""
	}
	return *t.Models
}

func DeleteTokenById(id int, userId int) error {
	return mustTokenRepo().DeleteTokenById(id, userId)
}

func IncreaseTokenQuota(id int, quota int64) error {
	return mustTokenRepo().IncreaseTokenQuota(id, quota)
}

func increaseTokenQuota(id int, quota int64) error {
	return mustTokenRepo().IncreaseTokenQuotaDirect(id, quota)
}

func DecreaseTokenQuota(id int, quota int64) error {
	return mustTokenRepo().DecreaseTokenQuota(id, quota)
}

func decreaseTokenQuota(id int, quota int64) error {
	return mustTokenRepo().DecreaseTokenQuotaDirect(id, quota)
}

func PreConsumeTokenQuota(tokenId int, quota int64) (err error) {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	token, err := GetTokenById(tokenId)
	if err != nil {
		return err
	}
	if !token.UnlimitedQuota && token.RemainQuota < quota {
		return errors.New("令牌额度不足")
	}
	userQuota, err := GetUserQuota(token.UserId)
	if err != nil {
		return err
	}
	if userQuota < quota {
		return errors.New("用户额度不足")
	}
	quotaTooLow := userQuota >= config.QuotaRemindThreshold && userQuota-quota < config.QuotaRemindThreshold
	noMoreQuota := userQuota-quota <= 0
	if quotaTooLow || noMoreQuota {
		go func() {
			email, err := GetUserEmail(token.UserId)
			if err != nil {
				logger.SysError("failed to fetch user email: " + err.Error())
			}
			prompt := "额度提醒"
			var contentText string
			if noMoreQuota {
				contentText = "您的额度已用尽"
			} else {
				contentText = "您的额度即将用尽"
			}
			if email != "" {
				topUpLink := fmt.Sprintf("%s/topup", config.ServerAddress)
				content := message.EmailTemplate(
					prompt,
					fmt.Sprintf(`
						<p>您好！</p>
						<p>%s，当前剩余额度为 <strong>%d</strong>。</p>
						<p>为了不影响您的使用，请及时充值。</p>
						<p style="text-align: center; margin: 30px 0;">
							<a href="%s" style="background-color: #007bff; color: white; padding: 12px 24px; text-decoration: none; border-radius: 4px; display: inline-block;">立即充值</a>
						</p>
						<p style="color: #666;">如果按钮无法点击，请复制以下链接到浏览器中打开：</p>
						<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px; word-break: break-all;">%s</p>
					`, contentText, userQuota, topUpLink, topUpLink),
				)
				err = message.SendEmail(prompt, email, content)
				if err != nil {
					logger.SysError("failed to send email: " + err.Error())
				}
			}
		}()
	}
	if !token.UnlimitedQuota {
		err = DecreaseTokenQuota(tokenId, quota)
		if err != nil {
			return err
		}
	}
	err = DecreaseUserQuota(token.UserId, quota)
	return err
}

func PostConsumeTokenQuota(tokenId int, quota int64) (err error) {
	token, err := GetTokenById(tokenId)
	if err != nil {
		return err
	}
	if quota > 0 {
		err = DecreaseUserQuota(token.UserId, quota)
	} else {
		err = IncreaseUserQuota(token.UserId, -quota)
	}
	if !token.UnlimitedQuota {
		if quota > 0 {
			err = DecreaseTokenQuota(tokenId, quota)
		} else {
			err = IncreaseTokenQuota(tokenId, -quota)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
