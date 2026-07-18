package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const EntitlementProductsTableName = "entitlement_products"
const EntitlementProductVisibleUsersTableName = "entitlement_product_visible_users"

const (
	EntitlementProductKindBalance      = "balance"
	EntitlementProductKindSubscription = "subscription"
)

type EntitlementProduct struct {
	Id                       string                             `json:"id" gorm:"primaryKey;type:char(36)"`
	Kind                     string                             `json:"kind" gorm:"type:varchar(32);not null;index"`
	Name                     string                             `json:"name" gorm:"type:varchar(64);not null;index"`
	Description              string                             `json:"description" gorm:"type:varchar(255);not null;default:''"`
	GroupID                  string                             `json:"group_id" gorm:"type:char(36);not null;index"`
	SalePrice                float64                            `json:"sale_price" gorm:"type:decimal(10,2);not null;default:0"`
	SaleCurrency             string                             `json:"sale_currency" gorm:"type:varchar(16);not null;default:'CNY'"`
	QuotaMetric              string                             `json:"quota_metric" gorm:"type:varchar(32);not null;default:'yyc';index"`
	QuotaAmount              float64                            `json:"quota_amount" gorm:"type:numeric(18,6);not null;default:0"`
	QuotaCurrency            string                             `json:"quota_currency" gorm:"type:varchar(16);not null;default:'YYC'"`
	PeriodType               string                             `json:"period_type" gorm:"type:varchar(32);not null;default:'none';index"`
	PeriodLimit              int64                              `json:"period_limit" gorm:"type:bigint;not null;default:0"`
	DurationDays             int                                `json:"duration_days" gorm:"type:int;not null;default:0"`
	ValidityDays             int                                `json:"validity_days" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerUser    int                                `json:"max_concurrency_per_user" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerPackage int                                `json:"max_concurrency_per_package" gorm:"type:int;not null;default:0"`
	AllowBalanceFallback     bool                               `json:"allow_balance_fallback" gorm:"not null;default:false"`
	VisibilityScope          string                             `json:"visibility_scope" gorm:"type:varchar(32);not null;default:'all';index"`
	PublicVisible            bool                               `json:"public_visible" gorm:"not null;index"`
	Enabled                  bool                               `json:"enabled" gorm:"not null;default:false;index"`
	SortOrder                int                                `json:"sort_order" gorm:"not null;default:0;index"`
	Source                   string                             `json:"source" gorm:"type:varchar(32);not null;default:'manual'"`
	CreatedAt                int64                              `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                int64                              `json:"updated_at" gorm:"bigint;index"`
	GroupName                string                             `json:"group_name,omitempty" gorm:"-"`
	SupportedModels          []string                           `json:"supported_models,omitempty" gorm:"-"`
	VisibleUserIDs           []string                           `json:"visible_user_ids,omitempty" gorm:"-"`
	VisibleUsers             []ServicePackageVisibleUserSummary `json:"visible_users,omitempty" gorm:"-"`
}

func (EntitlementProduct) TableName() string {
	return EntitlementProductsTableName
}

type EntitlementProductVisibleUser struct {
	ProductID string `json:"product_id" gorm:"primaryKey;type:char(36)"`
	UserID    string `json:"user_id" gorm:"primaryKey;type:char(36);index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

func (EntitlementProductVisibleUser) TableName() string {
	return EntitlementProductVisibleUsersTableName
}

func ensureEntitlementProductTablesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	return db.AutoMigrate(&EntitlementProduct{}, &EntitlementProductVisibleUser{})
}

func (item *EntitlementProduct) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

func normalizeEntitlementProductKind(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case EntitlementProductKindBalance:
		return EntitlementProductKindBalance
	case EntitlementProductKindSubscription:
		return EntitlementProductKindSubscription
	default:
		return ""
	}
}

func normalizeEntitlementProductSource(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "manual"
	}
	return normalized
}

func normalizeEntitlementProductRow(row *EntitlementProduct) error {
	if row == nil {
		return fmt.Errorf("权益商品不能为空")
	}
	row.EnsureID()
	row.Kind = normalizeEntitlementProductKind(row.Kind)
	if row.Kind == "" {
		return fmt.Errorf("权益商品类型不能为空")
	}
	row.Name = strings.TrimSpace(row.Name)
	if row.Name == "" {
		return fmt.Errorf("权益商品名称不能为空")
	}
	row.Description = strings.TrimSpace(row.Description)
	row.GroupID = strings.TrimSpace(row.GroupID)
	if row.GroupID == "" {
		return fmt.Errorf("分组不能为空")
	}
	row.SalePrice = normalizeServicePackageSalePrice(row.SalePrice)
	row.SaleCurrency = normalizeServicePackageSaleCurrency(row.SaleCurrency)
	row.QuotaMetric = normalizeServicePackageQuotaMetric(row.QuotaMetric, "")
	row.QuotaAmount = normalizeTopupPlanQuotaAmount(row.QuotaAmount)
	row.QuotaCurrency = normalizeBillingCurrencyCode(row.QuotaCurrency)
	row.PeriodType = normalizeServicePackagePeriodType(row.PeriodType, row.QuotaMetric)
	if row.Kind == EntitlementProductKindBalance {
		row.PeriodType = ServicePackagePeriodNone
		row.PeriodLimit = 0
		row.DurationDays = normalizeTopupPlanValidityDays(row.DurationDays)
		row.ValidityDays = row.DurationDays
		row.AllowBalanceFallback = false
	} else {
		row.PeriodLimit = normalizeServicePackagePeriodLimit(row.PeriodLimit, row.QuotaMetric, row.PeriodType, 0)
		row.DurationDays = normalizeServicePackageDurationDays(row.DurationDays)
		row.ValidityDays = row.DurationDays
	}
	row.MaxConcurrencyPerUser = normalizeServicePackageConcurrencyLimit(row.MaxConcurrencyPerUser)
	row.MaxConcurrencyPerPackage = normalizeServicePackageConcurrencyLimit(row.MaxConcurrencyPerPackage)
	row.VisibilityScope = normalizeServicePackageVisibilityScope(row.VisibilityScope)
	row.PublicVisible = row.VisibilityScope == ServicePackageVisibilityScopeAll
	row.SortOrder = max(row.SortOrder, 0)
	row.Source = normalizeEntitlementProductSource(row.Source)
	if row.CreatedAt <= 0 {
		row.CreatedAt = helper.GetTimestamp()
	}
	if row.UpdatedAt <= 0 {
		row.UpdatedAt = helper.GetTimestamp()
	}
	return nil
}

func entitlementProductFromTopupPlan(plan TopupPlan) (EntitlementProduct, error) {
	row := EntitlementProduct{
		Id:                       strings.TrimSpace(plan.Id),
		Kind:                     EntitlementProductKindBalance,
		Name:                     normalizeTopupPlanName(plan.Name, plan.Amount, plan.AmountCurrency),
		GroupID:                  strings.TrimSpace(plan.GroupID),
		SalePrice:                normalizeTopupPlanAmount(plan.Amount),
		SaleCurrency:             normalizeBillingCurrencyCode(plan.AmountCurrency),
		QuotaMetric:              ServicePackageQuotaMetricYYC,
		QuotaAmount:              normalizeTopupPlanQuotaAmount(plan.QuotaAmount),
		QuotaCurrency:            normalizeBillingCurrencyCode(plan.QuotaCurrency),
		PeriodType:               ServicePackagePeriodNone,
		PeriodLimit:              0,
		DurationDays:             normalizeTopupPlanValidityDays(plan.ValidityDays),
		ValidityDays:             normalizeTopupPlanValidityDays(plan.ValidityDays),
		MaxConcurrencyPerUser:    normalizeServicePackageConcurrencyLimit(plan.MaxConcurrencyPerUser),
		MaxConcurrencyPerPackage: normalizeServicePackageConcurrencyLimit(plan.MaxConcurrencyPerPackage),
		VisibilityScope:          normalizeServicePackageVisibilityScope(plan.VisibilityScope),
		PublicVisible:            plan.PublicVisible,
		Enabled:                  plan.Enabled,
		SortOrder:                max(plan.SortOrder, 0),
		Source:                   "legacy_topup_plan",
		CreatedAt:                plan.CreatedAt,
		UpdatedAt:                plan.UpdatedAt,
	}
	if err := normalizeEntitlementProductRow(&row); err != nil {
		return EntitlementProduct{}, err
	}
	return row, nil
}

func entitlementProductFromServicePackage(pkg ServicePackage) (EntitlementProduct, error) {
	normalizeServicePackageScopeAndQuota(&pkg)
	row := EntitlementProduct{
		Id:                       strings.TrimSpace(pkg.Id),
		Kind:                     EntitlementProductKindSubscription,
		Name:                     strings.TrimSpace(pkg.Name),
		Description:              strings.TrimSpace(pkg.Description),
		GroupID:                  strings.TrimSpace(pkg.GroupID),
		SalePrice:                normalizeServicePackageSalePrice(pkg.SalePrice),
		SaleCurrency:             normalizeServicePackageSaleCurrency(pkg.SaleCurrency),
		QuotaMetric:              strings.TrimSpace(pkg.QuotaMetric),
		QuotaAmount:              float64(pkg.PeriodLimit),
		QuotaCurrency:            BillingCurrencyCodeYYC,
		PeriodType:               strings.TrimSpace(pkg.PeriodType),
		PeriodLimit:              pkg.PeriodLimit,
		DurationDays:             normalizeServicePackageDurationDays(pkg.DurationDays),
		ValidityDays:             normalizeServicePackageDurationDays(pkg.DurationDays),
		MaxConcurrencyPerUser:    normalizeServicePackageConcurrencyLimit(pkg.MaxConcurrencyPerUser),
		MaxConcurrencyPerPackage: normalizeServicePackageConcurrencyLimit(pkg.MaxConcurrencyPerPackage),
		AllowBalanceFallback:     pkg.AllowBalanceFallback,
		VisibilityScope:          normalizeServicePackageVisibilityScope(pkg.VisibilityScope),
		PublicVisible:            normalizeServicePackageVisibilityScope(pkg.VisibilityScope) == ServicePackageVisibilityScopeAll,
		Enabled:                  pkg.Enabled,
		SortOrder:                max(pkg.SortOrder, 0),
		Source:                   normalizeServicePackageSource(pkg.Source),
		CreatedAt:                pkg.CreatedAt,
		UpdatedAt:                pkg.UpdatedAt,
	}
	if err := normalizeEntitlementProductRow(&row); err != nil {
		return EntitlementProduct{}, err
	}
	return row, nil
}

func upsertEntitlementProductWithDB(tx *gorm.DB, row EntitlementProduct) (EntitlementProduct, error) {
	if tx == nil {
		return EntitlementProduct{}, fmt.Errorf("database handle is nil")
	}
	if err := ensureEntitlementProductTablesWithDB(tx); err != nil {
		return EntitlementProduct{}, err
	}
	if err := normalizeEntitlementProductRow(&row); err != nil {
		return EntitlementProduct{}, err
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"kind",
			"name",
			"description",
			"group_id",
			"sale_price",
			"sale_currency",
			"quota_metric",
			"quota_amount",
			"quota_currency",
			"period_type",
			"period_limit",
			"duration_days",
			"validity_days",
			"max_concurrency_per_user",
			"max_concurrency_per_package",
			"allow_balance_fallback",
			"visibility_scope",
			"public_visible",
			"enabled",
			"sort_order",
			"source",
			"updated_at",
		}),
	}).Create(&row).Error; err != nil {
		return EntitlementProduct{}, err
	}
	stored := EntitlementProduct{}
	if err := tx.
		Where("id = ?", row.Id).
		First(&stored).Error; err != nil {
		return EntitlementProduct{}, err
	}
	return stored, nil
}

func syncEntitlementProductVisibleUsersWithDB(tx *gorm.DB, productID string, userIDs []string) error {
	normalizedProductID := strings.TrimSpace(productID)
	if normalizedProductID == "" {
		return fmt.Errorf("权益商品 ID 不能为空")
	}
	if err := tx.Where("product_id = ?", normalizedProductID).Delete(&EntitlementProductVisibleUser{}).Error; err != nil {
		return err
	}
	normalizedUserIDs := normalizeServicePackageVisibleUserIDs(userIDs)
	if len(normalizedUserIDs) == 0 {
		return nil
	}
	now := helper.GetTimestamp()
	rows := make([]EntitlementProductVisibleUser, 0, len(normalizedUserIDs))
	for _, userID := range normalizedUserIDs {
		rows = append(rows, EntitlementProductVisibleUser{
			ProductID: normalizedProductID,
			UserID:    userID,
			CreatedAt: now,
		})
	}
	return tx.Create(&rows).Error
}

func syncEntitlementProductsFromLegacyWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := ensureEntitlementProductTablesWithDB(tx); err != nil {
		return err
	}
	if err := ensureTopupPlanVisibleUsersTableWithDB(tx); err != nil {
		return err
	}
	topupPlans := make([]TopupPlan, 0)
	if err := tx.Find(&topupPlans).Error; err != nil {
		return err
	}
	for _, plan := range topupPlans {
		product, err := entitlementProductFromTopupPlan(plan)
		if err != nil {
			return err
		}
		stored, err := upsertEntitlementProductWithDB(tx, product)
		if err != nil {
			return err
		}
		if normalizeServicePackageVisibilityScope(plan.VisibilityScope) == ServicePackageVisibilityScopeUser {
			visibleIDs := make([]string, 0)
			if err := tx.Model(&TopupPlanVisibleUser{}).
				Where("plan_id = ?", strings.TrimSpace(plan.Id)).
				Order("created_at asc, user_id asc").
				Pluck("user_id", &visibleIDs).Error; err != nil {
				return err
			}
			if err := syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, visibleIDs); err != nil {
				return err
			}
		} else if err := syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, nil); err != nil {
			return err
		}
	}
	packages := make([]ServicePackage, 0)
	if err := tx.Find(&packages).Error; err != nil {
		return err
	}
	for _, pkg := range packages {
		product, err := entitlementProductFromServicePackage(pkg)
		if err != nil {
			return err
		}
		stored, err := upsertEntitlementProductWithDB(tx, product)
		if err != nil {
			return err
		}
		if normalizeServicePackageVisibilityScope(pkg.VisibilityScope) == ServicePackageVisibilityScopeUser {
			visibleIDs := make([]string, 0)
			if err := tx.Model(&ServicePackageVisibleUser{}).
				Where("package_id = ?", strings.TrimSpace(pkg.Id)).
				Order("created_at asc, user_id asc").
				Pluck("user_id", &visibleIDs).Error; err != nil {
				return err
			}
			if err := syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, visibleIDs); err != nil {
				return err
			}
		} else if err := syncEntitlementProductVisibleUsersWithDB(tx, stored.Id, nil); err != nil {
			return err
		}
	}
	return nil
}

func deleteEntitlementProductByIDWithDB(tx *gorm.DB, id string) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return nil
	}
	if err := ensureEntitlementProductTablesWithDB(tx); err != nil {
		return err
	}
	if err := tx.Where("product_id = ?", normalizedID).
		Delete(&EntitlementProductVisibleUser{}).Error; err != nil {
		return err
	}
	return tx.
		Where("id = ?", normalizedID).
		Delete(&EntitlementProduct{}).Error
}

func cleanupEntitlementProductLegacySourceColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !tx.Migrator().HasTable(EntitlementProductsTableName) {
		return nil
	}
	hasLegacySourceType := tx.Migrator().HasColumn(EntitlementProductsTableName, "legacy_source_type")
	hasLegacySourceID := tx.Migrator().HasColumn(EntitlementProductsTableName, "legacy_source_id")
	if !hasLegacySourceType && !hasLegacySourceID {
		return nil
	}
	if hasLegacySourceID {
		type legacyProductIDRow struct {
			ID             string `gorm:"column:id"`
			LegacySourceID string `gorm:"column:legacy_source_id"`
		}
		rows := make([]legacyProductIDRow, 0)
		if err := tx.Table(EntitlementProductsTableName).
			Select("id", "legacy_source_id").
			Where("COALESCE(TRIM(legacy_source_id), '') <> '' AND id <> legacy_source_id").
			Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			currentID := strings.TrimSpace(row.ID)
			targetID := strings.TrimSpace(row.LegacySourceID)
			if currentID == "" || targetID == "" || currentID == targetID {
				continue
			}
			if tx.Migrator().HasTable(EntitlementProductVisibleUsersTableName) {
				if err := tx.Exec(
					`DELETE FROM entitlement_product_visible_users
					WHERE product_id = ?
					AND user_id IN (
						SELECT user_id FROM entitlement_product_visible_users WHERE product_id = ?
					)`,
					currentID,
					targetID,
				).Error; err != nil {
					return err
				}
				if err := tx.Model(&EntitlementProductVisibleUser{}).
					Where("product_id = ?", currentID).
					Update("product_id", targetID).Error; err != nil {
					return err
				}
			}
			existingCount := int64(0)
			if err := tx.Table(EntitlementProductsTableName).
				Where("id = ?", targetID).
				Count(&existingCount).Error; err != nil {
				return err
			}
			if existingCount > 0 {
				if err := tx.Where("id = ?", currentID).Delete(&EntitlementProduct{}).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Table(EntitlementProductsTableName).
				Where("id = ?", currentID).
				Update("id", targetID).Error; err != nil {
				return err
			}
		}
	}
	if hasLegacySourceType {
		if err := tx.Migrator().DropColumn(EntitlementProductsTableName, "legacy_source_type"); err != nil {
			return err
		}
	}
	if hasLegacySourceID {
		if err := tx.Migrator().DropColumn(EntitlementProductsTableName, "legacy_source_id"); err != nil {
			return err
		}
	}
	return nil
}

func hydrateEntitlementProductGroupNamesWithDB(db *gorm.DB, rows []EntitlementProduct) error {
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
	groupNames := make(map[string]string, len(groups))
	for _, item := range groups {
		groupNames[strings.TrimSpace(item.Id)] = strings.TrimSpace(item.Name)
	}
	for index := range rows {
		rows[index].GroupName = groupNames[strings.TrimSpace(rows[index].GroupID)]
	}
	return nil
}

func hydrateEntitlementProductSupportedModelsWithDB(db *gorm.DB, rows []EntitlementProduct) error {
	if len(rows) == 0 || db == nil {
		return nil
	}
	modelsByGroupID := make(map[string][]string)
	for index := range rows {
		groupID := strings.TrimSpace(rows[index].GroupID)
		if groupID == "" {
			rows[index].SupportedModels = []string{}
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
		rows[index].SupportedModels = models
	}
	return nil
}

func hydrateEntitlementProductVisibilityWithDB(db *gorm.DB, rows []EntitlementProduct) error {
	if len(rows) == 0 || db == nil {
		return nil
	}
	productIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		productID := strings.TrimSpace(row.Id)
		if productID == "" {
			continue
		}
		if _, ok := seen[productID]; ok {
			continue
		}
		seen[productID] = struct{}{}
		productIDs = append(productIDs, productID)
	}
	if len(productIDs) == 0 {
		return nil
	}
	type visibleUserRow struct {
		ProductID     string
		UserID        string
		Username      string
		DisplayName   string
		WalletAddress string
	}
	visibleRows := make([]visibleUserRow, 0)
	if err := db.Table(EntitlementProductVisibleUsersTableName+" AS epu").
		Select("epu.product_id", "epu.user_id", "u.username", "u.display_name", "u.wallet_address").
		Joins("LEFT JOIN users u ON u.id = epu.user_id").
		Where("epu.product_id IN ?", productIDs).
		Order("epu.created_at ASC, epu.user_id ASC").
		Scan(&visibleRows).Error; err != nil {
		return err
	}
	idsByProduct := make(map[string][]string, len(productIDs))
	usersByProduct := make(map[string][]ServicePackageVisibleUserSummary, len(productIDs))
	for _, row := range visibleRows {
		productID := strings.TrimSpace(row.ProductID)
		userID := strings.TrimSpace(row.UserID)
		if productID == "" || userID == "" {
			continue
		}
		idsByProduct[productID] = append(idsByProduct[productID], userID)
		usersByProduct[productID] = append(usersByProduct[productID], ServicePackageVisibleUserSummary{
			ID:            userID,
			Username:      strings.TrimSpace(row.Username),
			DisplayName:   strings.TrimSpace(row.DisplayName),
			WalletAddress: strings.TrimSpace(row.WalletAddress),
		})
	}
	for index := range rows {
		productID := strings.TrimSpace(rows[index].Id)
		rows[index].VisibilityScope = normalizeServicePackageVisibilityScope(rows[index].VisibilityScope)
		rows[index].VisibleUserIDs = idsByProduct[productID]
		rows[index].VisibleUsers = usersByProduct[productID]
	}
	return nil
}

func buildEntitlementProductListQueryWithDB(db *gorm.DB, kind string, keyword string) *gorm.DB {
	query := db.Model(&EntitlementProduct{})
	if normalizedKind := normalizeEntitlementProductKind(kind); normalizedKind != "" {
		query = query.Where("kind = ?", normalizedKind)
	}
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		`LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ? OR LOWER(group_id) LIKE ? OR EXISTS (SELECT 1 FROM groups g WHERE g.id = entitlement_products.group_id AND LOWER(g.name) LIKE ?)`,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func ListEntitlementProductsPageWithDB(db *gorm.DB, kind string, page int, pageSize int, keyword string) ([]EntitlementProduct, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	query := buildEntitlementProductListQueryWithDB(db, kind, keyword)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]EntitlementProduct, 0, pageSize)
	if err := query.
		Order("kind asc, sort_order asc, created_at asc, name asc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if err := hydrateEntitlementProductGroupNamesWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	if err := hydrateEntitlementProductSupportedModelsWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	if err := hydrateEntitlementProductVisibilityWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func GetEntitlementProductByIDWithDB(db *gorm.DB, id string) (EntitlementProduct, error) {
	if db == nil {
		return EntitlementProduct{}, fmt.Errorf("database handle is nil")
	}
	row := EntitlementProduct{}
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		return EntitlementProduct{}, err
	}
	rows := []EntitlementProduct{row}
	_ = hydrateEntitlementProductGroupNamesWithDB(db, rows)
	_ = hydrateEntitlementProductSupportedModelsWithDB(db, rows)
	_ = hydrateEntitlementProductVisibilityWithDB(db, rows)
	if len(rows) > 0 {
		row = rows[0]
	}
	return row, nil
}

func topupPlanFromEntitlementProduct(row EntitlementProduct) TopupPlan {
	return TopupPlan{
		Id:                       strings.TrimSpace(row.Id),
		Name:                     strings.TrimSpace(row.Name),
		GroupID:                  strings.TrimSpace(row.GroupID),
		Amount:                   row.SalePrice,
		AmountCurrency:           row.SaleCurrency,
		QuotaAmount:              row.QuotaAmount,
		QuotaCurrency:            row.QuotaCurrency,
		ValidityDays:             max(row.ValidityDays, row.DurationDays),
		MaxConcurrencyPerUser:    row.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage: row.MaxConcurrencyPerPackage,
		VisibilityScope:          row.VisibilityScope,
		VisibleUserIDs:           row.VisibleUserIDs,
		Enabled:                  row.Enabled,
		PublicVisible:            row.PublicVisible,
		SortOrder:                row.SortOrder,
	}
}

func servicePackageFromEntitlementProduct(row EntitlementProduct) ServicePackage {
	quotaMetric := normalizeServicePackageQuotaMetric(row.QuotaMetric, "")
	packageType := ServicePackageTypeYYCQuota
	if quotaMetric == ServicePackageQuotaMetricRequestCount {
		packageType = ServicePackageTypeRequestQuota
	}
	periodLimit := row.PeriodLimit
	if periodLimit <= 0 {
		periodLimit = int64(row.QuotaAmount)
	}
	return ServicePackage{
		Id:                       strings.TrimSpace(row.Id),
		Name:                     strings.TrimSpace(row.Name),
		Description:              strings.TrimSpace(row.Description),
		GroupID:                  strings.TrimSpace(row.GroupID),
		PackageType:              packageType,
		ScopeType:                ServicePackageScopeAll,
		QuotaMetric:              quotaMetric,
		PeriodType:               strings.TrimSpace(row.PeriodType),
		PeriodLimit:              periodLimit,
		MaxConcurrencyPerUser:    row.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage: row.MaxConcurrencyPerPackage,
		AllowBalanceFallback:     row.AllowBalanceFallback,
		VisibilityScope:          row.VisibilityScope,
		VisibleUserIDs:           row.VisibleUserIDs,
		SalePrice:                row.SalePrice,
		SaleCurrency:             row.SaleCurrency,
		DailyQuotaLimit:          periodLimit,
		DurationDays:             row.DurationDays,
		QuotaResetTimezone:       DefaultGroupQuotaResetTimezone,
		Enabled:                  row.Enabled,
		SortOrder:                row.SortOrder,
		Source:                   normalizeEntitlementProductSource(row.Source),
	}
}

func CreateEntitlementProductWithDB(db *gorm.DB, row EntitlementProduct) (EntitlementProduct, error) {
	if db == nil {
		return EntitlementProduct{}, fmt.Errorf("database handle is nil")
	}
	row.Kind = normalizeEntitlementProductKind(row.Kind)
	switch row.Kind {
	case EntitlementProductKindBalance:
		created, err := createTopupPlanWithDB(db, topupPlanFromEntitlementProduct(row))
		if err != nil {
			return EntitlementProduct{}, err
		}
		return GetEntitlementProductByIDWithDB(db, created.Id)
	case EntitlementProductKindSubscription:
		created, err := createServicePackageWithDB(db, servicePackageFromEntitlementProduct(row))
		if err != nil {
			return EntitlementProduct{}, err
		}
		return GetEntitlementProductByIDWithDB(db, created.Id)
	default:
		return EntitlementProduct{}, fmt.Errorf("权益商品类型不能为空")
	}
}

func UpdateEntitlementProductWithDB(db *gorm.DB, row EntitlementProduct) (EntitlementProduct, error) {
	if db == nil {
		return EntitlementProduct{}, fmt.Errorf("database handle is nil")
	}
	id := strings.TrimSpace(row.Id)
	if id == "" {
		return EntitlementProduct{}, fmt.Errorf("权益商品 ID 不能为空")
	}
	current, err := GetEntitlementProductByIDWithDB(db, id)
	if err != nil {
		return EntitlementProduct{}, err
	}
	if normalizedKind := normalizeEntitlementProductKind(row.Kind); normalizedKind != "" && normalizedKind != current.Kind {
		return EntitlementProduct{}, fmt.Errorf("权益商品类型不能修改")
	}
	row.Kind = current.Kind
	if row.VisibleUserIDs == nil {
		row.VisibleUserIDs = current.VisibleUserIDs
	}
	switch row.Kind {
	case EntitlementProductKindBalance:
		updated, err := updateTopupPlanWithDB(db, topupPlanFromEntitlementProduct(row))
		if err != nil {
			return EntitlementProduct{}, err
		}
		return GetEntitlementProductByIDWithDB(db, updated.Id)
	case EntitlementProductKindSubscription:
		updated, err := updateServicePackageWithDB(db, servicePackageFromEntitlementProduct(row))
		if err != nil {
			return EntitlementProduct{}, err
		}
		return GetEntitlementProductByIDWithDB(db, updated.Id)
	default:
		return EntitlementProduct{}, fmt.Errorf("权益商品类型不能为空")
	}
}

func DeleteEntitlementProductWithDB(db *gorm.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	row, err := GetEntitlementProductByIDWithDB(db, id)
	if err != nil {
		return err
	}
	switch row.Kind {
	case EntitlementProductKindBalance:
		return deleteTopupPlanWithDB(db, row.Id)
	case EntitlementProductKindSubscription:
		return deleteServicePackageWithDB(db, row.Id)
	default:
		return deleteEntitlementProductByIDWithDB(db, row.Id)
	}
}

func ListEntitlementProductsWithDB(db *gorm.DB, kind string) ([]EntitlementProduct, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]EntitlementProduct, 0)
	query := db.Model(&EntitlementProduct{})
	if normalizedKind := normalizeEntitlementProductKind(kind); normalizedKind != "" {
		query = query.Where("kind = ?", normalizedKind)
	}
	if err := query.Order("kind asc, sort_order asc, created_at asc, name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListEntitlementProductsPage(kind string, page int, pageSize int, keyword string) ([]EntitlementProduct, int64, error) {
	return ListEntitlementProductsPageWithDB(DB, kind, page, pageSize, keyword)
}

func GetEntitlementProductByID(id string) (EntitlementProduct, error) {
	return GetEntitlementProductByIDWithDB(DB, id)
}

func CreateEntitlementProduct(row EntitlementProduct) (EntitlementProduct, error) {
	return CreateEntitlementProductWithDB(DB, row)
}

func UpdateEntitlementProduct(row EntitlementProduct) (EntitlementProduct, error) {
	return UpdateEntitlementProductWithDB(DB, row)
}

func DeleteEntitlementProduct(id string) error {
	return DeleteEntitlementProductWithDB(DB, id)
}
