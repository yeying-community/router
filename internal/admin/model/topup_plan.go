package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const (
	TopupPlansTableName = "topup_plans"
)

type TopupPlan struct {
	Id             string  `json:"id" gorm:"primaryKey;type:char(36)"`
	Name           string  `json:"name" gorm:"type:varchar(64);not null"`
	GroupID        string  `json:"group_id" gorm:"type:char(36);not null;index"`
	Amount         float64 `json:"amount" gorm:"type:decimal(10,2);not null;default:0"`
	AmountCurrency string  `json:"amount_currency" gorm:"type:varchar(16);not null;default:'CNY'"`
	QuotaAmount    float64 `json:"quota_amount" gorm:"type:numeric(18,6);not null;default:0"`
	QuotaCurrency  string  `json:"quota_currency" gorm:"type:varchar(16);not null;default:'USD'"`
	ValidityDays   int     `json:"validity_days" gorm:"type:int;not null;default:0"`
	Enabled        bool    `json:"enabled" gorm:"default:true;index"`
	PublicVisible  bool    `json:"public_visible" gorm:"not null;default:true;index"`
	SortOrder      int     `json:"sort_order" gorm:"default:0;index"`
	CreatedAt      int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64   `json:"updated_at" gorm:"bigint;index"`
	GroupName      string  `json:"group_name,omitempty" gorm:"-"`
}

type ResolvedTopupPlan struct {
	TopupPlan
	QuotaYYC int64 `json:"quota_yyc"`
}

func (TopupPlan) TableName() string {
	return TopupPlansTableName
}

func (item *TopupPlan) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

func defaultTopupPlans(defaultGroupID string) []TopupPlan {
	return []TopupPlan{
		{
			Name:           "1 元",
			GroupID:        defaultGroupID,
			Amount:         1,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    20,
			QuotaCurrency:  BillingCurrencyCodeUSD,
			ValidityDays:   0,
			Enabled:        true,
			PublicVisible:  true,
			SortOrder:      1,
		},
		{
			Name:           "10 元",
			GroupID:        defaultGroupID,
			Amount:         10,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    220,
			QuotaCurrency:  BillingCurrencyCodeUSD,
			ValidityDays:   0,
			Enabled:        true,
			PublicVisible:  true,
			SortOrder:      2,
		},
		{
			Name:           "20 元",
			GroupID:        defaultGroupID,
			Amount:         20,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    500,
			QuotaCurrency:  BillingCurrencyCodeUSD,
			ValidityDays:   0,
			Enabled:        true,
			PublicVisible:  true,
			SortOrder:      3,
		},
		{
			Name:           "50 元",
			GroupID:        defaultGroupID,
			Amount:         50,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    1300,
			QuotaCurrency:  BillingCurrencyCodeUSD,
			ValidityDays:   0,
			Enabled:        true,
			PublicVisible:  true,
			SortOrder:      4,
		},
		{
			Name:           "100 元",
			GroupID:        defaultGroupID,
			Amount:         100,
			AmountCurrency: BillingCurrencyCodeCNY,
			QuotaAmount:    2600,
			QuotaCurrency:  BillingCurrencyCodeUSD,
			ValidityDays:   0,
			Enabled:        true,
			PublicVisible:  true,
			SortOrder:      5,
		},
	}
}

func formatTopupPlanNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.0000001 {
		return fmt.Sprintf("%.0f", value)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func normalizeTopupPlanName(value string, amount float64, currency string) string {
	normalized := strings.TrimSpace(value)
	if normalized != "" {
		return normalized
	}
	return fmt.Sprintf("%s %s", formatTopupPlanNumber(amount), strings.ToUpper(strings.TrimSpace(currency)))
}

func normalizeTopupPlanAmount(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*100) / 100
}

func normalizeTopupPlanQuotaAmount(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(value*1000000) / 1000000
}

func resolveDefaultTopupPlanGroupWithDB(db *gorm.DB) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database handle is nil")
	}
	groupRef := strings.TrimSpace(configuredDefaultUserGroupFromDB(db))
	if groupRef == "" {
		groupRef = strings.TrimSpace(config.DefaultUserGroup)
	}
	if groupRef != "" {
		groupID, err := validateDefaultUserGroupOptionValueWithDB(db, groupRef)
		if err == nil && strings.TrimSpace(groupID) != "" {
			return strings.TrimSpace(groupID), nil
		}
	}
	row := GroupCatalog{}
	if err := db.Where("enabled = ?", true).Order("sort_order asc, name asc").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(row.Id), nil
}

func resolveTopupPlanGroupWithDB(db *gorm.DB, groupRef string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database handle is nil")
	}
	normalizedRef := strings.TrimSpace(groupRef)
	if normalizedRef == "" {
		return resolveDefaultTopupPlanGroupWithDB(db)
	}
	row, err := resolveGroupCatalogByReferenceWithDB(db, normalizedRef)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("分组不存在")
		}
		return "", err
	}
	return strings.TrimSpace(row.Id), nil
}

func normalizeTopupPlanRowWithDB(db *gorm.DB, row *TopupPlan) error {
	if row == nil {
		return fmt.Errorf("充值档位不能为空")
	}
	row.EnsureID()
	row.Name = normalizeTopupPlanName(row.Name, row.Amount, row.AmountCurrency)
	row.Amount = normalizeTopupPlanAmount(row.Amount)
	row.AmountCurrency = normalizeBillingCurrencyCode(row.AmountCurrency)
	row.QuotaAmount = normalizeTopupPlanQuotaAmount(row.QuotaAmount)
	row.QuotaCurrency = normalizeBillingCurrencyCode(row.QuotaCurrency)
	row.ValidityDays = normalizeTopupPlanValidityDays(row.ValidityDays)
	row.SortOrder = max(row.SortOrder, 0)
	groupID, err := resolveTopupPlanGroupWithDB(db, row.GroupID)
	if err != nil {
		return err
	}
	row.GroupID = strings.TrimSpace(groupID)
	if row.GroupID == "" {
		return fmt.Errorf("分组不能为空")
	}
	if row.Amount <= 0 {
		return fmt.Errorf("支付金额必须大于 0")
	}
	if row.QuotaAmount <= 0 {
		return fmt.Errorf("到账额度必须大于 0")
	}
	return nil
}

func hydrateTopupPlanGroupNamesWithDB(db *gorm.DB, rows []TopupPlan) error {
	if len(rows) == 0 || db == nil {
		return nil
	}
	groupIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
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
	groups := make([]GroupCatalog, 0, len(groupIDs))
	if err := db.Select("id", "name").Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return err
	}
	nameByID := make(map[string]string, len(groups))
	for _, item := range groups {
		nameByID[strings.TrimSpace(item.Id)] = strings.TrimSpace(item.Name)
	}
	for i := range rows {
		rows[i].GroupName = nameByID[strings.TrimSpace(rows[i].GroupID)]
	}
	return nil
}

func NormalizeTopupPlans(items []TopupPlan) []TopupPlan {
	normalized := make([]TopupPlan, 0, len(items))
	for index, item := range items {
		row := item
		row.Name = normalizeTopupPlanName(row.Name, row.Amount, row.AmountCurrency)
		row.Amount = normalizeTopupPlanAmount(row.Amount)
		row.AmountCurrency = normalizeBillingCurrencyCode(row.AmountCurrency)
		row.QuotaAmount = normalizeTopupPlanQuotaAmount(row.QuotaAmount)
		row.QuotaCurrency = normalizeBillingCurrencyCode(row.QuotaCurrency)
		row.ValidityDays = normalizeTopupPlanValidityDays(row.ValidityDays)
		if row.SortOrder <= 0 {
			row.SortOrder = index + 1
		}
		if row.Amount <= 0 || row.QuotaAmount <= 0 {
			continue
		}
		normalized = append(normalized, row)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder == normalized[j].SortOrder {
			return normalized[i].Name < normalized[j].Name
		}
		return normalized[i].SortOrder < normalized[j].SortOrder
	})
	for index := range normalized {
		normalized[index].SortOrder = index + 1
	}
	return normalized
}

func ListTopupPlans() ([]TopupPlan, error) {
	return listTopupPlansWithDB(DB, false)
}

func ListPublicTopupPlans() ([]TopupPlan, error) {
	return listTopupPlansWithDB(DB, true)
}

func GetTopupPlanByID(id string) (TopupPlan, error) {
	return getTopupPlanByIDWithDB(DB, id)
}

func CreateTopupPlan(item TopupPlan) (TopupPlan, error) {
	return createTopupPlanWithDB(DB, item)
}

func UpdateTopupPlan(item TopupPlan) (TopupPlan, error) {
	return updateTopupPlanWithDB(DB, item)
}

func DeleteTopupPlan(id string) error {
	return deleteTopupPlanWithDB(DB, id)
}

func listTopupPlansWithDB(db *gorm.DB, enabledOnly bool) ([]TopupPlan, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]TopupPlan, 0)
	query := db.Model(&TopupPlan{})
	if enabledOnly {
		query = query.Where("enabled = ? AND public_visible = ?", true, true)
	}
	if err := query.Order("sort_order asc, created_at asc, name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateTopupPlanGroupNamesWithDB(db, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func getTopupPlanByIDWithDB(db *gorm.DB, id string) (TopupPlan, error) {
	if db == nil {
		return TopupPlan{}, fmt.Errorf("database handle is nil")
	}
	row := TopupPlan{}
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		return TopupPlan{}, err
	}
	_ = hydrateTopupPlanGroupNamesWithDB(db, []TopupPlan{row})
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	return row, nil
}

func createTopupPlanWithDB(db *gorm.DB, item TopupPlan) (TopupPlan, error) {
	if db == nil {
		return TopupPlan{}, fmt.Errorf("database handle is nil")
	}
	row := item
	if err := normalizeTopupPlanRowWithDB(db, &row); err != nil {
		return TopupPlan{}, err
	}
	now := helper.GetTimestamp()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if err := db.Create(&row).Error; err != nil {
		return TopupPlan{}, err
	}
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	return row, nil
}

func updateTopupPlanWithDB(db *gorm.DB, item TopupPlan) (TopupPlan, error) {
	if db == nil {
		return TopupPlan{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(item.Id)
	if normalizedID == "" {
		return TopupPlan{}, fmt.Errorf("充值档位 ID 不能为空")
	}
	row := TopupPlan{}
	if err := db.Where("id = ?", normalizedID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TopupPlan{}, fmt.Errorf("充值档位不存在")
		}
		return TopupPlan{}, err
	}
	row.Name = item.Name
	row.GroupID = item.GroupID
	row.Amount = item.Amount
	row.AmountCurrency = item.AmountCurrency
	row.QuotaAmount = item.QuotaAmount
	row.QuotaCurrency = item.QuotaCurrency
	row.ValidityDays = item.ValidityDays
	row.Enabled = item.Enabled
	row.PublicVisible = item.PublicVisible
	row.SortOrder = item.SortOrder
	if err := normalizeTopupPlanRowWithDB(db, &row); err != nil {
		return TopupPlan{}, err
	}
	row.UpdatedAt = helper.GetTimestamp()
	if err := db.Save(&row).Error; err != nil {
		return TopupPlan{}, err
	}
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	return row, nil
}

func deleteTopupPlanWithDB(db *gorm.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return fmt.Errorf("充值档位 ID 不能为空")
	}
	result := db.Delete(&TopupPlan{}, "id = ?", normalizedID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("充值档位不存在")
	}
	return nil
}

func seedDefaultTopupPlansWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	count := int64(0)
	if err := db.Model(&TopupPlan{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaultGroupID, err := resolveDefaultTopupPlanGroupWithDB(db)
	if err != nil {
		return err
	}
	for _, item := range defaultTopupPlans(defaultGroupID) {
		if strings.TrimSpace(item.GroupID) == "" {
			continue
		}
		if _, err := createTopupPlanWithDB(db, item); err != nil {
			return err
		}
	}
	return nil
}

func ResolveTopupPlan(planID string) (ResolvedTopupPlan, error) {
	normalizedPlanID := strings.TrimSpace(planID)
	if normalizedPlanID == "" {
		return ResolvedTopupPlan{}, fmt.Errorf("充值档位不能为空")
	}
	item, err := getTopupPlanByIDWithDB(DB, normalizedPlanID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedTopupPlan{}, fmt.Errorf("充值档位不存在")
		}
		return ResolvedTopupPlan{}, err
	}
	if !item.Enabled {
		return ResolvedTopupPlan{}, fmt.Errorf("充值档位已禁用")
	}
	yycPerUnit, err := GetBillingCurrencyYYCPerUnit(item.QuotaCurrency)
	if err != nil {
		return ResolvedTopupPlan{}, err
	}
	quotaYYC := int64(math.Round(item.QuotaAmount * yycPerUnit))
	if quotaYYC <= 0 {
		return ResolvedTopupPlan{}, fmt.Errorf("充值额度不能为空")
	}
	return ResolvedTopupPlan{
		TopupPlan: item,
		QuotaYYC:  quotaYYC,
	}, nil
}
