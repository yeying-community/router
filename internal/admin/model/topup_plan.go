package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const (
	TopupPlansTableName            = "topup_plans"
	TopupPlanVisibleUsersTableName = "topup_plan_visible_users"
)

type TopupPlan struct {
	Id                       string                             `json:"id" gorm:"primaryKey;type:char(36)"`
	Name                     string                             `json:"name" gorm:"type:varchar(64);not null"`
	GroupID                  string                             `json:"group_id" gorm:"type:char(36);not null;index"`
	Amount                   float64                            `json:"amount" gorm:"type:decimal(10,2);not null;default:0"`
	AmountCurrency           string                             `json:"amount_currency" gorm:"type:varchar(16);not null;default:'CNY'"`
	QuotaAmount              float64                            `json:"quota_amount" gorm:"type:numeric(18,6);not null;default:0"`
	QuotaCurrency            string                             `json:"quota_currency" gorm:"type:varchar(16);not null;default:'USD'"`
	ValidityDays             int                                `json:"validity_days" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerUser    int                                `json:"max_concurrency_per_user" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerPackage int                                `json:"max_concurrency_per_package" gorm:"type:int;not null;default:0"`
	VisibilityScope          string                             `json:"visibility_scope" gorm:"type:varchar(32);not null;default:'all';index"`
	Enabled                  bool                               `json:"enabled" gorm:"index"`
	PublicVisible            bool                               `json:"public_visible" gorm:"not null;index"`
	SortOrder                int                                `json:"sort_order" gorm:"default:0;index"`
	CreatedAt                int64                              `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                int64                              `json:"updated_at" gorm:"bigint;index"`
	GroupName                string                             `json:"group_name,omitempty" gorm:"-"`
	SupportedModels          []string                           `json:"supported_models,omitempty" gorm:"-"`
	VisibleUserIDs           []string                           `json:"visible_user_ids,omitempty" gorm:"-"`
	VisibleUsers             []ServicePackageVisibleUserSummary `json:"visible_users,omitempty" gorm:"-"`
}

type ResolvedTopupPlan struct {
	TopupPlan
	ChargeAmount int64 `json:"charge_amount"`
}

func (TopupPlan) TableName() string {
	return TopupPlansTableName
}

type TopupPlanVisibleUser struct {
	PlanID    string `json:"plan_id" gorm:"primaryKey;type:char(36)"`
	UserID    string `json:"user_id" gorm:"primaryKey;type:char(36);index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

func (TopupPlanVisibleUser) TableName() string {
	return TopupPlanVisibleUsersTableName
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
		return fmt.Errorf("充值额度不能为空")
	}
	row.EnsureID()
	row.Name = normalizeTopupPlanName(row.Name, row.Amount, row.AmountCurrency)
	row.Amount = normalizeTopupPlanAmount(row.Amount)
	row.AmountCurrency = normalizeBillingCurrencyCode(row.AmountCurrency)
	row.QuotaAmount = normalizeTopupPlanQuotaAmount(row.QuotaAmount)
	row.QuotaCurrency = normalizeBillingCurrencyCode(row.QuotaCurrency)
	row.ValidityDays = normalizeTopupPlanValidityDays(row.ValidityDays)
	row.MaxConcurrencyPerUser = normalizeServicePackageConcurrencyLimit(row.MaxConcurrencyPerUser)
	row.MaxConcurrencyPerPackage = normalizeServicePackageConcurrencyLimit(row.MaxConcurrencyPerPackage)
	row.VisibilityScope = normalizeServicePackageVisibilityScope(row.VisibilityScope)
	row.PublicVisible = row.VisibilityScope == ServicePackageVisibilityScopeAll
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

func ensureTopupPlanVisibleUsersTableWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	return db.AutoMigrate(&TopupPlanVisibleUser{})
}

func syncTopupPlanVisibleUsersWithDB(tx *gorm.DB, planID string, userIDs []string) error {
	normalizedPlanID := strings.TrimSpace(planID)
	if normalizedPlanID == "" {
		return fmt.Errorf("充值权益 ID 不能为空")
	}
	if err := ensureTopupPlanVisibleUsersTableWithDB(tx); err != nil {
		return err
	}
	if err := tx.Where("plan_id = ?", normalizedPlanID).Delete(&TopupPlanVisibleUser{}).Error; err != nil {
		return err
	}
	normalizedUserIDs := normalizeServicePackageVisibleUserIDs(userIDs)
	if len(normalizedUserIDs) == 0 {
		return nil
	}
	now := helper.GetTimestamp()
	rows := make([]TopupPlanVisibleUser, 0, len(normalizedUserIDs))
	for _, userID := range normalizedUserIDs {
		rows = append(rows, TopupPlanVisibleUser{
			PlanID:    normalizedPlanID,
			UserID:    userID,
			CreatedAt: now,
		})
	}
	return tx.Create(&rows).Error
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

func hydrateTopupPlanSupportedModelsWithDB(db *gorm.DB, rows []TopupPlan) error {
	if len(rows) == 0 || db == nil {
		return nil
	}
	modelsByGroupID := make(map[string][]string)
	for i := range rows {
		groupID := strings.TrimSpace(rows[i].GroupID)
		if groupID == "" {
			rows[i].SupportedModels = []string{}
			continue
		}
		models, ok := modelsByGroupID[groupID]
		if !ok {
			var err error
			models, err = listGroupModelNamesWithDB(db, groupID, true)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					models = []string{}
				} else {
					return err
				}
			}
			if models == nil {
				models = []string{}
			}
			modelsByGroupID[groupID] = models
		}
		rows[i].SupportedModels = models
	}
	return nil
}

func hydrateTopupPlanVisibilityWithDB(db *gorm.DB, rows []TopupPlan) error {
	if len(rows) == 0 || db == nil {
		return nil
	}
	planIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		planID := strings.TrimSpace(row.Id)
		if planID == "" {
			continue
		}
		if _, ok := seen[planID]; ok {
			continue
		}
		seen[planID] = struct{}{}
		planIDs = append(planIDs, planID)
	}
	if len(planIDs) == 0 {
		return nil
	}
	if err := ensureTopupPlanVisibleUsersTableWithDB(db); err != nil {
		return err
	}
	type visibleUserRow struct {
		PlanID        string
		UserID        string
		Username      string
		DisplayName   string
		WalletAddress string
	}
	visibleRows := make([]visibleUserRow, 0)
	if err := db.Table(TopupPlanVisibleUsersTableName+" AS tpu").
		Select("tpu.plan_id", "tpu.user_id", "u.username", "u.display_name", "u.wallet_address").
		Joins("LEFT JOIN users u ON u.id = tpu.user_id").
		Where("tpu.plan_id IN ?", planIDs).
		Order("tpu.created_at ASC, tpu.user_id ASC").
		Scan(&visibleRows).Error; err != nil {
		return err
	}
	idsByPlan := make(map[string][]string, len(planIDs))
	usersByPlan := make(map[string][]ServicePackageVisibleUserSummary, len(planIDs))
	for _, row := range visibleRows {
		planID := strings.TrimSpace(row.PlanID)
		userID := strings.TrimSpace(row.UserID)
		if planID == "" || userID == "" {
			continue
		}
		idsByPlan[planID] = append(idsByPlan[planID], userID)
		usersByPlan[planID] = append(usersByPlan[planID], ServicePackageVisibleUserSummary{
			ID:            userID,
			Username:      strings.TrimSpace(row.Username),
			DisplayName:   strings.TrimSpace(row.DisplayName),
			WalletAddress: strings.TrimSpace(row.WalletAddress),
		})
	}
	for index := range rows {
		planID := strings.TrimSpace(rows[index].Id)
		rows[index].VisibilityScope = normalizeServicePackageVisibilityScope(rows[index].VisibilityScope)
		rows[index].VisibleUserIDs = idsByPlan[planID]
		rows[index].VisibleUsers = usersByPlan[planID]
		rows[index].PublicVisible = rows[index].VisibilityScope == ServicePackageVisibilityScopeAll
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
	return listTopupPlansWithDB(DB, false, "")
}

func ListPublicTopupPlans() ([]TopupPlan, error) {
	return listTopupPlansWithDB(DB, true, "")
}

func ListPublicTopupPlansForUser(userID string) ([]TopupPlan, error) {
	return listTopupPlansWithDB(DB, true, userID)
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

func listTopupPlansWithDB(db *gorm.DB, enabledOnly bool, userID string) ([]TopupPlan, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]TopupPlan, 0)
	query := db.Model(&TopupPlan{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
		normalizedUserID := strings.TrimSpace(userID)
		if normalizedUserID == "" {
			query = query.Where("COALESCE(visibility_scope, '') = '' OR visibility_scope = ?", ServicePackageVisibilityScopeAll)
		} else {
			query = query.Where(
				`(
					COALESCE(visibility_scope, '') = ''
					OR visibility_scope = ?
					OR EXISTS (
						SELECT 1
						FROM `+TopupPlanVisibleUsersTableName+` tpu
						WHERE tpu.plan_id = topup_plans.id AND tpu.user_id = ?
					)
				)`,
				ServicePackageVisibilityScopeAll,
				normalizedUserID,
			)
		}
	}
	if err := query.Order("sort_order asc, created_at asc, name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateTopupPlanGroupNamesWithDB(db, rows); err != nil {
		return nil, err
	}
	if err := hydrateTopupPlanSupportedModelsWithDB(db, rows); err != nil {
		return nil, err
	}
	if err := hydrateTopupPlanVisibilityWithDB(db, rows); err != nil {
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
	rows := []TopupPlan{row}
	if err := hydrateTopupPlanSupportedModelsWithDB(db, rows); err == nil && len(rows) > 0 {
		row.SupportedModels = rows[0].SupportedModels
	}
	if err := hydrateTopupPlanVisibilityWithDB(db, rows); err == nil && len(rows) > 0 {
		row.VisibleUserIDs = rows[0].VisibleUserIDs
		row.VisibleUsers = rows[0].VisibleUsers
		row.VisibilityScope = rows[0].VisibilityScope
		row.PublicVisible = rows[0].PublicVisible
	}
	return row, nil
}

func createTopupPlanWithDB(db *gorm.DB, item TopupPlan) (TopupPlan, error) {
	if db == nil {
		return TopupPlan{}, fmt.Errorf("database handle is nil")
	}
	row := item
	row.VisibilityScope = normalizeServicePackageVisibilityScope(row.VisibilityScope)
	visibleUserIDs := normalizeServicePackageVisibleUserIDs(row.VisibleUserIDs)
	if row.VisibilityScope == ServicePackageVisibilityScopeUser && len(visibleUserIDs) > 0 {
		if _, err := resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs); err != nil {
			return TopupPlan{}, err
		}
	}
	if err := normalizeTopupPlanRowWithDB(db, &row); err != nil {
		return TopupPlan{}, err
	}
	now := helper.GetTimestamp()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if row.VisibilityScope == ServicePackageVisibilityScopeUser {
			if err := syncTopupPlanVisibleUsersWithDB(tx, row.Id, visibleUserIDs); err != nil {
				return err
			}
		} else if err := syncTopupPlanVisibleUsersWithDB(tx, row.Id, nil); err != nil {
			return err
		}
		product, err := entitlementProductFromTopupPlan(row)
		if err != nil {
			return err
		}
		product.VisibleUserIDs = visibleUserIDs
		stored, err := upsertEntitlementProductWithDB(tx, product)
		if err != nil {
			return err
		}
		if row.VisibilityScope == ServicePackageVisibilityScopeUser {
			return syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, visibleUserIDs)
		}
		return syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, nil)
	}); err != nil {
		return TopupPlan{}, err
	}
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	row.VisibleUserIDs = visibleUserIDs
	if row.VisibilityScope == ServicePackageVisibilityScopeUser {
		row.VisibleUsers, _ = resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs)
	}
	return row, nil
}

func updateTopupPlanWithDB(db *gorm.DB, item TopupPlan) (TopupPlan, error) {
	if db == nil {
		return TopupPlan{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(item.Id)
	if normalizedID == "" {
		return TopupPlan{}, fmt.Errorf("充值额度 ID 不能为空")
	}
	row := TopupPlan{}
	if err := db.Where("id = ?", normalizedID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TopupPlan{}, fmt.Errorf("充值额度不存在")
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
	row.VisibilityScope = normalizeServicePackageVisibilityScope(item.VisibilityScope)
	visibleUserIDs := normalizeServicePackageVisibleUserIDs(item.VisibleUserIDs)
	if row.VisibilityScope == ServicePackageVisibilityScopeUser && len(visibleUserIDs) > 0 {
		if _, err := resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs); err != nil {
			return TopupPlan{}, err
		}
	}
	if err := normalizeTopupPlanRowWithDB(db, &row); err != nil {
		return TopupPlan{}, err
	}
	row.UpdatedAt = helper.GetTimestamp()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if row.VisibilityScope == ServicePackageVisibilityScopeUser {
			if err := syncTopupPlanVisibleUsersWithDB(tx, row.Id, visibleUserIDs); err != nil {
				return err
			}
		} else if err := syncTopupPlanVisibleUsersWithDB(tx, row.Id, nil); err != nil {
			return err
		}
		product, err := entitlementProductFromTopupPlan(row)
		if err != nil {
			return err
		}
		product.VisibleUserIDs = visibleUserIDs
		stored, err := upsertEntitlementProductWithDB(tx, product)
		if err != nil {
			return err
		}
		if row.VisibilityScope == ServicePackageVisibilityScopeUser {
			return syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, visibleUserIDs)
		}
		return syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, nil)
	}); err != nil {
		return TopupPlan{}, err
	}
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	row.VisibleUserIDs = visibleUserIDs
	if row.VisibilityScope == ServicePackageVisibilityScopeUser {
		row.VisibleUsers, _ = resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs)
	}
	return row, nil
}

func deleteTopupPlanWithDB(db *gorm.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return fmt.Errorf("充值额度 ID 不能为空")
	}
	if err := ensureTopupPlanVisibleUsersTableWithDB(db); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", normalizedID).Delete(&TopupPlanVisibleUser{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&TopupPlan{}, "id = ?", normalizedID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("充值额度不存在")
		}
		return deleteEntitlementProductByIDWithDB(tx, normalizedID)
	})
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
	return resolveTopupPlanForUserWithDB(DB, planID, "", false)
}

func ResolveTopupPlanForUser(planID string, userID string) (ResolvedTopupPlan, error) {
	return resolveTopupPlanForUserWithDB(DB, planID, userID, true)
}

func resolveTopupPlanForUserWithDB(db *gorm.DB, planID string, userID string, enforceVisibility bool) (ResolvedTopupPlan, error) {
	normalizedPlanID := strings.TrimSpace(planID)
	if normalizedPlanID == "" {
		return ResolvedTopupPlan{}, fmt.Errorf("充值额度不能为空")
	}
	item, err := getTopupPlanByIDWithDB(db, normalizedPlanID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ResolvedTopupPlan{}, fmt.Errorf("充值额度不存在")
		}
		return ResolvedTopupPlan{}, err
	}
	if !item.Enabled {
		return ResolvedTopupPlan{}, fmt.Errorf("充值额度已禁用")
	}
	if enforceVisibility && normalizeServicePackageVisibilityScope(item.VisibilityScope) == ServicePackageVisibilityScopeUser {
		normalizedUserID := strings.TrimSpace(userID)
		if normalizedUserID == "" {
			return ResolvedTopupPlan{}, fmt.Errorf("充值额度不可用")
		}
		visible := false
		for _, visibleUserID := range item.VisibleUserIDs {
			if strings.TrimSpace(visibleUserID) == normalizedUserID {
				visible = true
				break
			}
		}
		if !visible {
			return ResolvedTopupPlan{}, fmt.Errorf("充值额度不可用")
		}
	}
	chargeRate, err := GetBillingCurrencyChargeRate(item.QuotaCurrency)
	if err != nil {
		return ResolvedTopupPlan{}, err
	}
	quotaChargeAmount := int64(math.Round(item.QuotaAmount * chargeRate))
	if quotaChargeAmount <= 0 {
		return ResolvedTopupPlan{}, fmt.Errorf("充值额度不能为空")
	}
	return ResolvedTopupPlan{
		TopupPlan:    item,
		ChargeAmount: quotaChargeAmount,
	}, nil
}
