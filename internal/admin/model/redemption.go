package model

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

type Redemption struct {
	Id                    string  `json:"id" gorm:"type:char(36);primaryKey"`
	UserId                string  `json:"user_id" gorm:"type:char(36);index"`
	RedeemedByUserId      string  `json:"redeemed_by_user_id" gorm:"type:char(36);index"`
	RedeemedByUsername    string  `json:"redeemed_by_username,omitempty" gorm:"-"`
	GroupID               string  `json:"group_id" gorm:"column:group_id;type:char(36);index"`
	GroupName             string  `json:"group_name,omitempty" gorm:"-"`
	EntitlementProductID  string  `json:"entitlement_product_id,omitempty" gorm:"type:char(36);index"`
	ProductKind           string  `json:"product_kind,omitempty" gorm:"type:varchar(32);index"`
	ProductNameSnapshot   string  `json:"product_name_snapshot,omitempty" gorm:"type:varchar(64)"`
	QuotaAmountSnapshot   float64 `json:"quota_amount_snapshot,omitempty" gorm:"type:numeric(18,6)"`
	QuotaCurrencySnapshot string  `json:"quota_currency_snapshot,omitempty" gorm:"type:varchar(16)"`
	ValidityDaysSnapshot  int     `json:"validity_days_snapshot,omitempty" gorm:"type:int"`
	GroupIDSnapshot       string  `json:"group_id_snapshot,omitempty" gorm:"type:char(36)"`
	Code                  string  `json:"code" gorm:"column:code;type:char(32);uniqueIndex"`
	Status                int     `json:"status" gorm:"default:1"`
	Name                  string  `json:"name" gorm:"index"`
	CodeValidityDays      int     `json:"code_validity_days" gorm:"type:int;not null;default:0"`
	CodeExpiresAt         int64   `json:"code_expires_at" gorm:"bigint;not null;default:0;index"`
	CreditExpiresAt       int64   `json:"credit_expires_at" gorm:"bigint;not null;default:0;index"`
	CreatedTime           int64   `json:"created_time" gorm:"bigint"`
	RedeemedTime          int64   `json:"redeemed_time" gorm:"bigint"`
	Count                 int     `json:"count" gorm:"-:all"`
}

type RedemptionResult struct {
	RedeemedAmount      int64  `json:"redeemed_amount"`
	BeforeBalanceAmount int64  `json:"before_balance_amount"`
	AfterBalanceAmount  int64  `json:"after_balance_amount"`
	RedemptionID        string `json:"redemption_id"`
	RedemptionName      string `json:"redemption_name"`
	GroupID             string `json:"group_id,omitempty"`
	GroupName           string `json:"group_name,omitempty"`
	RedeemedAt          int64  `json:"redeemed_at"`
	CreditExpiresAt     int64  `json:"credit_expires_at,omitempty"`
}

func normalizeRedemptionValidityDays(value int) int {
	switch {
	case value < 0:
		return 0
	case value > UserBalanceLotMaxValidityDay:
		return UserBalanceLotMaxValidityDay
	default:
		return value
	}
}

func ResolveRedemptionGroupWithDB(db *gorm.DB, groupRef string) (GroupCatalog, error) {
	if db == nil {
		return GroupCatalog{}, fmt.Errorf("database handle is nil")
	}
	if strings.TrimSpace(groupRef) == "" {
		return GroupCatalog{}, fmt.Errorf("分组不能为空")
	}
	resolved, err := resolveGroupCatalogByReferenceWithDB(db, groupRef)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return GroupCatalog{}, fmt.Errorf("分组不存在")
		}
		return GroupCatalog{}, err
	}
	if !resolved.Enabled {
		return GroupCatalog{}, fmt.Errorf("分组已禁用")
	}
	return resolved, nil
}

func backfillRedemptionGroupWithDefaultGroupWithDB(db *gorm.DB) error {
	return nil
}

func GetAllRedemptions(startIdx int, num int) ([]*Redemption, error) {
	return mustRedemptionRepo().GetAllRedemptions(startIdx, num)
}

func SearchRedemptions(keyword string) ([]*Redemption, error) {
	return mustRedemptionRepo().SearchRedemptions(keyword)
}

func GetRedemptionById(id string) (*Redemption, error) {
	return mustRedemptionRepo().GetRedemptionById(id)
}

func ListRedemptionsByRedeemedUserID(userID string, limit int) ([]*Redemption, error) {
	return mustRedemptionRepo().ListRedemptionsByRedeemedUserID(userID, limit)
}

func Redeem(ctx context.Context, code string, userId string) (RedemptionResult, error) {
	return mustRedemptionRepo().Redeem(ctx, code, userId)
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

func DeleteRedemptionById(id string) error {
	return mustRedemptionRepo().DeleteRedemptionById(id)
}
