package model

import "context"

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

type Redemption struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Key          string `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int    `json:"status" gorm:"default:1"`
	Name         string `json:"name" gorm:"index"`
	Quota        int64  `json:"quota" gorm:"bigint;default:100"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint"`
	Count        int    `json:"count" gorm:"-:all"`
}

func GetAllRedemptions(startIdx int, num int) ([]*Redemption, error) {
	return mustRedemptionRepo().GetAllRedemptions(startIdx, num)
}

func SearchRedemptions(keyword string) ([]*Redemption, error) {
	return mustRedemptionRepo().SearchRedemptions(keyword)
}

func GetRedemptionById(id int) (*Redemption, error) {
	return mustRedemptionRepo().GetRedemptionById(id)
}

func Redeem(ctx context.Context, key string, userId int) (int64, error) {
	return mustRedemptionRepo().Redeem(ctx, key, userId)
}

func (redemption *Redemption) Insert() error {
	return mustRedemptionRepo().Insert(redemption)
}

func (redemption *Redemption) SelectUpdate() error {
	return mustRedemptionRepo().SelectUpdate(redemption)
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	return mustRedemptionRepo().Update(redemption)
}

func (redemption *Redemption) Delete() error {
	return mustRedemptionRepo().Delete(redemption)
}

func DeleteRedemptionById(id int) error {
	return mustRedemptionRepo().DeleteRedemptionById(id)
}
