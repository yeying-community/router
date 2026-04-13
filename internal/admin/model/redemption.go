package model

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/yeying-community/router/common/config"
	"gorm.io/gorm"
)

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

const RedemptionFaceValueUnitYYC = "YYC"

type Redemption struct {
	Id     string `json:"id" gorm:"type:char(36);primaryKey"`
	UserId string `json:"user_id" gorm:"type:char(36);index"`
	// Keep for historical linkage compatibility in storage only; do not expose in query APIs.
	TopupOrderID       string  `json:"-" gorm:"type:char(36);index"`
	RedeemedByUserId   string  `json:"redeemed_by_user_id" gorm:"type:char(36);index"`
	RedeemedByUsername string  `json:"redeemed_by_username,omitempty" gorm:"-"`
	GroupID            string  `json:"group_id" gorm:"column:group_id;type:char(36);index"`
	GroupName          string  `json:"group_name,omitempty" gorm:"-"`
	Code               string  `json:"code" gorm:"column:code;type:char(32);uniqueIndex"`
	Status             int     `json:"status" gorm:"default:1"`
	Name               string  `json:"name" gorm:"index"`
	FaceValueAmount    float64 `json:"face_value_amount" gorm:"type:numeric(30,8);not null;default:0"`
	FaceValueUnit      string  `json:"face_value_unit" gorm:"type:varchar(16);not null;default:'YYC'"`
	Quota              int64   `json:"quota" gorm:"bigint;default:100"`
	CodeValidityDays   int     `json:"code_validity_days" gorm:"type:int;not null;default:0"`
	CodeExpiresAt      int64   `json:"code_expires_at" gorm:"bigint;not null;default:0;index"`
	CreditValidityDays int     `json:"credit_validity_days" gorm:"type:int;not null;default:0"`
	CreditExpiresAt    int64   `json:"credit_expires_at" gorm:"bigint;not null;default:0;index"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	RedeemedTime       int64   `json:"redeemed_time" gorm:"bigint"`
	Count              int     `json:"count" gorm:"-:all"`
}

type RedemptionResult struct {
	RedeemedYYC      int64   `json:"redeemed_yyc"`
	BeforeYYCBalance int64   `json:"before_yyc_balance"`
	AfterYYCBalance  int64   `json:"after_yyc_balance"`
	RedemptionID     string  `json:"redemption_id"`
	RedemptionName   string  `json:"redemption_name"`
	GroupID          string  `json:"group_id,omitempty"`
	GroupName        string  `json:"group_name,omitempty"`
	FaceValueAmount  float64 `json:"face_value_amount,omitempty"`
	FaceValueUnit    string  `json:"face_value_unit,omitempty"`
	RedeemedAt       int64   `json:"redeemed_at"`
	CreditExpiresAt  int64   `json:"credit_expires_at,omitempty"`
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

func normalizeRedemptionFaceValueUnit(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "", RedemptionFaceValueUnitYYC:
		return RedemptionFaceValueUnitYYC
	default:
		return normalizeBillingCurrencyCode(normalized)
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

func ResolveRedemptionFaceValueWithDB(db *gorm.DB, amount float64, unit string) (float64, string, int64, error) {
	normalizedUnit := normalizeRedemptionFaceValueUnit(unit)
	if !isValidRedemptionFaceValueAmount(amount) {
		return 0, "", 0, fmt.Errorf("面值必须大于 0")
	}
	switch normalizedUnit {
	case RedemptionFaceValueUnitYYC:
		quota := int64(math.Round(amount))
		if quota <= 0 {
			return 0, "", 0, fmt.Errorf("YYC 面值必须大于 0")
		}
		return amount, RedemptionFaceValueUnitYYC, quota, nil
	default:
		if db == nil {
			return 0, "", 0, fmt.Errorf("database handle is nil")
		}
		currency := BillingCurrency{}
		if err := db.Where("code = ?", normalizedUnit).Take(&currency).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return 0, "", 0, fmt.Errorf("计费单位不存在: %s", normalizedUnit)
			}
			return 0, "", 0, err
		}
		if currency.Status != BillingCurrencyStatusEnabled {
			return 0, "", 0, fmt.Errorf("计费单位已禁用: %s", currency.Code)
		}
		if currency.YYCPerUnit <= 0 {
			return 0, "", 0, fmt.Errorf("计费单位兑换比率无效: %s", currency.Code)
		}
		quotaFloat := amount * currency.YYCPerUnit
		if !isValidRedemptionFaceValueAmount(quotaFloat) {
			return 0, "", 0, fmt.Errorf("面值换算失败")
		}
		quota := int64(math.Round(quotaFloat))
		if quota <= 0 {
			return 0, "", 0, fmt.Errorf("换算后的 YYC 面值必须大于 0")
		}
		return amount, currency.Code, quota, nil
	}
}

func NormalizeRedemptionFaceValueFieldsWithDB(db *gorm.DB, redemption *Redemption) error {
	if redemption == nil {
		return fmt.Errorf("兑换码不能为空")
	}
	amount := redemption.FaceValueAmount
	unit := redemption.FaceValueUnit
	if !isValidRedemptionFaceValueAmount(amount) {
		if redemption.Quota <= 0 {
			return fmt.Errorf("面值必须大于 0")
		}
		amount = float64(redemption.Quota)
		unit = RedemptionFaceValueUnitYYC
	}
	resolvedAmount, resolvedUnit, resolvedQuota, err := ResolveRedemptionFaceValueWithDB(db, amount, unit)
	if err != nil {
		return err
	}
	redemption.FaceValueAmount = resolvedAmount
	redemption.FaceValueUnit = resolvedUnit
	redemption.Quota = resolvedQuota
	return nil
}

func backfillRedemptionGroupWithDefaultGroupWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupRef := strings.TrimSpace(configuredDefaultUserGroupFromDB(db))
	if groupRef == "" {
		groupRef = strings.TrimSpace(config.DefaultUserGroup)
	}
	if groupRef == "" {
		return nil
	}
	groupID, err := validateDefaultUserGroupOptionValueWithDB(db, groupRef)
	if err != nil {
		return err
	}
	if groupID == "" {
		return nil
	}
	return db.Model(&Redemption{}).
		Where("COALESCE(group_id, '') = ''").
		Update("group_id", groupID).Error
}

func configuredDefaultUserGroupFromDB(db *gorm.DB) string {
	if db == nil {
		return ""
	}
	row := Option{}
	if err := db.Where("key = ?", "DefaultUserGroup").Take(&row).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(row.Value)
}

func isValidRedemptionFaceValueAmount(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
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
