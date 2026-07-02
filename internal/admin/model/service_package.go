package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

const ServicePackagesTableName = "service_packages"
const ServicePackageVisibleUsersTableName = "service_package_visible_users"

const (
	DefaultServicePackageDurationDays = 30
	ServicePackageVisibilityScopeAll  = "all"
	ServicePackageVisibilityScopeUser = "partial_users"
)

const (
	ServicePackageTypeYYCQuota     = "yyc_quota"
	ServicePackageTypeRequestQuota = "request_quota"

	ServicePackageScopeAll = "all"

	ServicePackageQuotaMetricYYC          = "yyc"
	ServicePackageQuotaMetricRequestCount = "request_count"

	ServicePackagePeriodNone         = "none"
	ServicePackagePeriodDaily        = "daily"
	ServicePackagePeriodWeekly       = "weekly"
	ServicePackagePeriodMonthly      = "monthly"
	ServicePackagePeriodPackageTotal = "package_total"
)

const servicePackageDefaultScopeType = ServicePackageScopeAll

type ServicePackage struct {
	Id                         string                             `json:"id" gorm:"primaryKey;type:char(36)"`
	Name                       string                             `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Description                string                             `json:"description" gorm:"type:varchar(255);default:''"`
	GroupID                    string                             `json:"group_id" gorm:"type:char(36);not null;index"`
	PackageType                string                             `json:"package_type" gorm:"type:varchar(32);not null;default:'yyc_quota';index"`
	ScopeType                  string                             `json:"scope_type" gorm:"type:varchar(32);not null;default:'all';index"`
	ScopeProvider              string                             `json:"scope_provider" gorm:"type:varchar(64);not null;default:'';index"`
	ScopeModel                 string                             `json:"scope_model" gorm:"type:varchar(191);not null;default:'';index"`
	ScopeEndpoint              string                             `json:"scope_endpoint" gorm:"type:varchar(191);not null;default:''"`
	QuotaMetric                string                             `json:"quota_metric" gorm:"type:varchar(32);not null;default:'yyc';index"`
	PeriodType                 string                             `json:"period_type" gorm:"type:varchar(32);not null;default:'daily';index"`
	PeriodLimit                int64                              `json:"period_limit" gorm:"type:bigint;not null;default:0"`
	MaxConcurrencyPerUser      int                                `json:"max_concurrency_per_user" gorm:"type:int;not null;default:0"`
	MaxConcurrencyPerPackage   int                                `json:"max_concurrency_per_package" gorm:"type:int;not null;default:0"`
	AllowBalanceFallback       bool                               `json:"allow_balance_fallback" gorm:"not null;default:false"`
	VisibilityScope            string                             `json:"visibility_scope" gorm:"type:varchar(32);not null;default:'all';index"`
	SalePrice                  float64                            `json:"sale_price" gorm:"type:decimal(10,2);not null;default:0"`
	SaleCurrency               string                             `json:"sale_currency" gorm:"type:varchar(16);not null;default:'CNY'"`
	DailyQuotaLimit            int64                              `json:"daily_quota_limit" gorm:"type:bigint;not null;default:0"`
	PackageEmergencyQuotaLimit int64                              `json:"package_emergency_quota_limit" gorm:"column:package_emergency_quota_limit;type:bigint;not null;default:0"`
	DurationDays               int                                `json:"duration_days" gorm:"type:int;not null;default:30"`
	QuotaResetTimezone         string                             `json:"quota_reset_timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	Enabled                    bool                               `json:"enabled" gorm:"index"`
	SortOrder                  int                                `json:"sort_order" gorm:"default:0;index"`
	Source                     string                             `json:"source" gorm:"type:varchar(32);default:'manual'"`
	CreatedAt                  int64                              `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                  int64                              `json:"updated_at" gorm:"bigint;index"`
	GroupName                  string                             `json:"group_name,omitempty" gorm:"-"`
	VisibleUserIDs             []string                           `json:"visible_user_ids,omitempty" gorm:"-"`
	VisibleUsers               []ServicePackageVisibleUserSummary `json:"visible_users,omitempty" gorm:"-"`
	SupportedModels            []string                           `json:"supported_models,omitempty" gorm:"-"`
}

func (ServicePackage) TableName() string {
	return ServicePackagesTableName
}

type ServicePackageVisibleUser struct {
	PackageID string `json:"package_id" gorm:"primaryKey;type:char(36)"`
	UserID    string `json:"user_id" gorm:"primaryKey;type:char(36);index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

func (ServicePackageVisibleUser) TableName() string {
	return ServicePackageVisibleUsersTableName
}

type ServicePackageVisibleUserSummary struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	WalletAddress string `json:"wallet_address"`
}

func (item *ServicePackage) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

func normalizeServicePackageName(value string) string {
	return strings.TrimSpace(value)
}

func normalizeServicePackageDescription(value string) string {
	return strings.TrimSpace(value)
}

func normalizeServicePackageVisibilityScope(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case ServicePackageVisibilityScopeUser:
		return ServicePackageVisibilityScopeUser
	case "", ServicePackageVisibilityScopeAll:
		return ServicePackageVisibilityScopeAll
	default:
		return ServicePackageVisibilityScopeAll
	}
}

func normalizeServicePackagePackageType(value string, quotaMetric string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case ServicePackageTypeYYCQuota, ServicePackageTypeRequestQuota:
		return normalized
	}
	switch strings.TrimSpace(strings.ToLower(quotaMetric)) {
	case ServicePackageQuotaMetricRequestCount:
		return ServicePackageTypeRequestQuota
	default:
		return ServicePackageTypeYYCQuota
	}
}

func normalizeServicePackageQuotaMetric(value string, packageType string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case ServicePackageQuotaMetricYYC, ServicePackageQuotaMetricRequestCount:
		return normalized
	}
	if packageType == ServicePackageTypeRequestQuota {
		return ServicePackageQuotaMetricRequestCount
	}
	return ServicePackageQuotaMetricYYC
}

func normalizeServicePackageScopeFields(scopeType string, provider string, model string, endpoint string) (string, string, string, string) {
	return servicePackageDefaultScopeType, "", "", ""
}

func normalizeServicePackagePeriodType(value string, quotaMetric string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case ServicePackagePeriodNone, ServicePackagePeriodDaily, ServicePackagePeriodWeekly, ServicePackagePeriodMonthly, ServicePackagePeriodPackageTotal:
		return normalized
	}
	if quotaMetric == ServicePackageQuotaMetricRequestCount {
		return ServicePackagePeriodMonthly
	}
	return ServicePackagePeriodDaily
}

func normalizeServicePackagePeriodLimit(value int64, quotaMetric string, periodType string, dailyQuotaLimit int64) int64 {
	if value > 0 {
		return value
	}
	if quotaMetric == ServicePackageQuotaMetricYYC && periodType == ServicePackagePeriodDaily {
		return normalizeServicePackageDailyQuotaLimit(dailyQuotaLimit)
	}
	return 0
}

func normalizeServicePackageConcurrencyLimit(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func DefaultServicePackageAllowBalanceFallback(packageType string, quotaMetric string) bool {
	return packageType == ServicePackageTypeYYCQuota && quotaMetric == ServicePackageQuotaMetricYYC
}

func normalizeServicePackageAllowBalanceFallback(value bool, rawPackageType string, rawQuotaMetric string, packageType string, quotaMetric string) bool {
	if value {
		return true
	}
	if strings.TrimSpace(rawPackageType) == "" && strings.TrimSpace(rawQuotaMetric) == "" {
		return DefaultServicePackageAllowBalanceFallback(packageType, quotaMetric)
	}
	return false
}

func validateServicePackageScopeFields(scopeType string, provider string, model string, endpoint string) error {
	return nil
}

func validateServicePackageQuotaFields(packageType string, quotaMetric string, periodType string, periodLimit int64) error {
	switch packageType {
	case ServicePackageTypeYYCQuota:
		if quotaMetric != ServicePackageQuotaMetricYYC {
			return fmt.Errorf("YYC 套餐只能使用 yyc 额度指标")
		}
	case ServicePackageTypeRequestQuota:
		if quotaMetric != ServicePackageQuotaMetricRequestCount {
			return fmt.Errorf("请求次数套餐只能使用 request_count 额度指标")
		}
		if periodType == ServicePackagePeriodNone {
			return fmt.Errorf("请求次数套餐必须配置周期类型")
		}
		if periodLimit <= 0 {
			return fmt.Errorf("请求次数套餐必须配置周期额度")
		}
	}
	return nil
}

func normalizeServicePackageScopeAndQuota(item *ServicePackage) {
	if item == nil {
		return
	}
	rawPackageType := item.PackageType
	rawQuotaMetric := item.QuotaMetric
	packageType := normalizeServicePackagePackageType(item.PackageType, item.QuotaMetric)
	quotaMetric := normalizeServicePackageQuotaMetric(item.QuotaMetric, packageType)
	scopeType, scopeProvider, scopeModel, scopeEndpoint := normalizeServicePackageScopeFields(
		item.ScopeType,
		item.ScopeProvider,
		item.ScopeModel,
		item.ScopeEndpoint,
	)
	periodType := normalizeServicePackagePeriodType(item.PeriodType, quotaMetric)
	periodLimit := normalizeServicePackagePeriodLimit(item.PeriodLimit, quotaMetric, periodType, item.DailyQuotaLimit)
	item.PackageType = packageType
	item.ScopeType = scopeType
	item.ScopeProvider = scopeProvider
	item.ScopeModel = scopeModel
	item.ScopeEndpoint = scopeEndpoint
	item.QuotaMetric = quotaMetric
	item.PeriodType = periodType
	item.PeriodLimit = periodLimit
	item.MaxConcurrencyPerUser = normalizeServicePackageConcurrencyLimit(item.MaxConcurrencyPerUser)
	item.MaxConcurrencyPerPackage = normalizeServicePackageConcurrencyLimit(item.MaxConcurrencyPerPackage)
	item.AllowBalanceFallback = normalizeServicePackageAllowBalanceFallback(
		item.AllowBalanceFallback,
		rawPackageType,
		rawQuotaMetric,
		packageType,
		quotaMetric,
	)
}

func validateServicePackageScopeAndQuota(item ServicePackage) error {
	if err := validateServicePackageScopeFields(item.ScopeType, item.ScopeProvider, item.ScopeModel, item.ScopeEndpoint); err != nil {
		return err
	}
	return validateServicePackageQuotaFields(item.PackageType, item.QuotaMetric, item.PeriodType, item.PeriodLimit)
}

func normalizeServicePackageVisibleUserIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, item := range values {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeServicePackageDailyQuotaLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeServicePackageSalePrice(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeServicePackageSaleCurrency(value string) string {
	normalized := strings.TrimSpace(strings.ToUpper(value))
	if normalized == "" {
		return BillingCurrencyCodeCNY
	}
	return normalized
}

func normalizeServicePackagePackageEmergencyQuotaLimit(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeServicePackageDurationDays(value int) int {
	if value <= 0 {
		return DefaultServicePackageDurationDays
	}
	return value
}

func normalizeServicePackageSource(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "manual"
	}
	return normalized
}

func normalizeServicePackageTimezone(value string) string {
	return normalizeGroupQuotaResetTimezone(value)
}

func resolveServicePackageGroupIDWithDB(db *gorm.DB, groupRef string) (string, error) {
	groupCatalog, err := resolveGroupCatalogByReferenceWithDB(db, groupRef)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("分组不存在")
		}
		return "", err
	}
	groupID := strings.TrimSpace(groupCatalog.Id)
	if groupID == "" {
		return "", fmt.Errorf("分组不存在")
	}
	return groupID, nil
}

func resolveServicePackageVisibleUsersWithDB(db *gorm.DB, userIDs []string) ([]ServicePackageVisibleUserSummary, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	type visibleUserRow struct {
		ID            string
		Username      string
		DisplayName   string
		WalletAddress string
	}
	rows := make([]visibleUserRow, 0, len(userIDs))
	if err := db.Model(&User{}).
		Select("id", "username", "display_name", "wallet_address").
		Where("status != ? AND id IN ?", UserStatusDeleted, userIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(userIDs) {
		return nil, fmt.Errorf("部分可见用户不存在")
	}
	index := make(map[string]visibleUserRow, len(rows))
	for _, row := range rows {
		index[strings.TrimSpace(row.ID)] = row
	}
	result := make([]ServicePackageVisibleUserSummary, 0, len(userIDs))
	for _, userID := range userIDs {
		row, ok := index[strings.TrimSpace(userID)]
		if !ok {
			return nil, fmt.Errorf("部分可见用户不存在")
		}
		result = append(result, ServicePackageVisibleUserSummary{
			ID:            strings.TrimSpace(row.ID),
			Username:      strings.TrimSpace(row.Username),
			DisplayName:   strings.TrimSpace(row.DisplayName),
			WalletAddress: strings.TrimSpace(row.WalletAddress),
		})
	}
	return result, nil
}

func syncServicePackageVisibleUsersWithDB(tx *gorm.DB, packageID string, userIDs []string) error {
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return fmt.Errorf("套餐 ID 不能为空")
	}
	if err := tx.Where("package_id = ?", normalizedPackageID).Delete(&ServicePackageVisibleUser{}).Error; err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}
	rows := make([]ServicePackageVisibleUser, 0, len(userIDs))
	now := helper.GetTimestamp()
	for _, userID := range userIDs {
		rows = append(rows, ServicePackageVisibleUser{
			PackageID: normalizedPackageID,
			UserID:    strings.TrimSpace(userID),
			CreatedAt: now,
		})
	}
	return tx.Create(&rows).Error
}

func resolveServicePackageVisibleUsersByPackageIDWithDB(db *gorm.DB, packageID string) ([]ServicePackageVisibleUserSummary, error) {
	normalizedPackageID := strings.TrimSpace(packageID)
	if normalizedPackageID == "" {
		return nil, nil
	}
	type visibleUserRow struct {
		UserID        string
		Username      string
		DisplayName   string
		WalletAddress string
	}
	rows := make([]visibleUserRow, 0)
	if err := db.Table(ServicePackageVisibleUsersTableName+" AS spu").
		Select("spu.user_id", "u.username", "u.display_name", "u.wallet_address").
		Joins("LEFT JOIN users u ON u.id = spu.user_id").
		Where("spu.package_id = ?", normalizedPackageID).
		Order("spu.created_at ASC, spu.user_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ServicePackageVisibleUserSummary, 0, len(rows))
	for _, row := range rows {
		userID := strings.TrimSpace(row.UserID)
		if userID == "" {
			continue
		}
		result = append(result, ServicePackageVisibleUserSummary{
			ID:            userID,
			Username:      strings.TrimSpace(row.Username),
			DisplayName:   strings.TrimSpace(row.DisplayName),
			WalletAddress: strings.TrimSpace(row.WalletAddress),
		})
	}
	return result, nil
}

func hydrateServicePackageVisibilityWithDB(db *gorm.DB, rows []ServicePackage) error {
	if len(rows) == 0 {
		return nil
	}
	packageIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		packageID := strings.TrimSpace(row.Id)
		if packageID == "" {
			continue
		}
		if _, ok := seen[packageID]; ok {
			continue
		}
		seen[packageID] = struct{}{}
		packageIDs = append(packageIDs, packageID)
	}
	if len(packageIDs) == 0 {
		return nil
	}
	type visibleUserRow struct {
		PackageID     string
		UserID        string
		Username      string
		DisplayName   string
		WalletAddress string
	}
	visibleRows := make([]visibleUserRow, 0)
	if err := db.Table(ServicePackageVisibleUsersTableName+" AS spu").
		Select("spu.package_id", "spu.user_id", "u.username", "u.display_name", "u.wallet_address").
		Joins("LEFT JOIN users u ON u.id = spu.user_id").
		Where("spu.package_id IN ?", packageIDs).
		Order("spu.created_at ASC, spu.user_id ASC").
		Scan(&visibleRows).Error; err != nil {
		return err
	}
	idsByPackage := make(map[string][]string, len(packageIDs))
	usersByPackage := make(map[string][]ServicePackageVisibleUserSummary, len(packageIDs))
	for _, row := range visibleRows {
		packageID := strings.TrimSpace(row.PackageID)
		userID := strings.TrimSpace(row.UserID)
		if packageID == "" || userID == "" {
			continue
		}
		idsByPackage[packageID] = append(idsByPackage[packageID], userID)
		usersByPackage[packageID] = append(usersByPackage[packageID], ServicePackageVisibleUserSummary{
			ID:            userID,
			Username:      strings.TrimSpace(row.Username),
			DisplayName:   strings.TrimSpace(row.DisplayName),
			WalletAddress: strings.TrimSpace(row.WalletAddress),
		})
	}
	for index := range rows {
		packageID := strings.TrimSpace(rows[index].Id)
		rows[index].VisibilityScope = normalizeServicePackageVisibilityScope(rows[index].VisibilityScope)
		rows[index].VisibleUserIDs = idsByPackage[packageID]
		rows[index].VisibleUsers = usersByPackage[packageID]
	}
	return nil
}

func listServicePackagesPageWithDB(db *gorm.DB, page int, pageSize int, keyword string) ([]ServicePackage, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	query := buildServicePackageListQueryWithDB(db, keyword)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]ServicePackage, 0, pageSize)
	if err := query.
		Order("sort_order asc, name asc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if err := hydrateServicePackageGroupNamesWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	if err := hydrateServicePackageSupportedModelsWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	if err := hydrateServicePackageVisibilityWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func buildServicePackageListQueryWithDB(db *gorm.DB, keyword string) *gorm.DB {
	query := db.Model(&ServicePackage{})
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		`LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ? OR LOWER(group_id) LIKE ? OR EXISTS (SELECT 1 FROM groups g WHERE g.id = service_packages.group_id AND LOWER(g.name) LIKE ?)`,
		likeKeyword,
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
}

func hydrateServicePackageGroupNamesWithDB(db *gorm.DB, rows []ServicePackage) error {
	if len(rows) == 0 {
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

func hydrateServicePackageSupportedModelsWithDB(db *gorm.DB, rows []ServicePackage) error {
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

func getServicePackageByIDWithDB(db *gorm.DB, id string) (ServicePackage, error) {
	if db == nil {
		return ServicePackage{}, fmt.Errorf("database handle is nil")
	}
	row := ServicePackage{}
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		return ServicePackage{}, err
	}
	_ = hydrateServicePackageGroupNamesWithDB(db, []ServicePackage{row})
	if groupCatalog, err := getGroupCatalogByIDWithDB(db, row.GroupID); err == nil {
		row.GroupName = strings.TrimSpace(groupCatalog.Name)
	}
	rows := []ServicePackage{row}
	if err := hydrateServicePackageSupportedModelsWithDB(db, rows); err == nil && len(rows) > 0 {
		row.SupportedModels = rows[0].SupportedModels
	}
	if visibleUsers, err := resolveServicePackageVisibleUsersByPackageIDWithDB(db, row.Id); err == nil {
		row.VisibilityScope = normalizeServicePackageVisibilityScope(row.VisibilityScope)
		row.VisibleUsers = visibleUsers
		row.VisibleUserIDs = make([]string, 0, len(visibleUsers))
		for _, item := range visibleUsers {
			row.VisibleUserIDs = append(row.VisibleUserIDs, item.ID)
		}
	}
	return row, nil
}

func getServicePackageByNameWithDB(db *gorm.DB, name string) (ServicePackage, error) {
	if db == nil {
		return ServicePackage{}, fmt.Errorf("database handle is nil")
	}
	row := ServicePackage{}
	if err := db.Where("name = ?", strings.TrimSpace(name)).First(&row).Error; err != nil {
		return ServicePackage{}, err
	}
	return row, nil
}

func createServicePackageWithDB(db *gorm.DB, item ServicePackage) (ServicePackage, error) {
	if db == nil {
		return ServicePackage{}, fmt.Errorf("database handle is nil")
	}
	name := normalizeServicePackageName(item.Name)
	if name == "" {
		return ServicePackage{}, fmt.Errorf("套餐名称不能为空")
	}
	if len(name) > 64 {
		return ServicePackage{}, fmt.Errorf("套餐名称长度不能超过 64")
	}
	if _, err := getServicePackageByNameWithDB(db, name); err == nil {
		return ServicePackage{}, fmt.Errorf("套餐已存在")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ServicePackage{}, err
	}
	groupID, err := resolveServicePackageGroupIDWithDB(db, item.GroupID)
	if err != nil {
		return ServicePackage{}, err
	}
	visibilityScope := normalizeServicePackageVisibilityScope(item.VisibilityScope)
	visibleUserIDs := normalizeServicePackageVisibleUserIDs(item.VisibleUserIDs)
	if len(visibleUserIDs) > 0 {
		if _, err := resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs); err != nil {
			return ServicePackage{}, err
		}
	}
	maxSortOrder := 0
	if err := db.Model(&ServicePackage{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder).Error; err != nil {
		return ServicePackage{}, err
	}
	now := helper.GetTimestamp()
	normalizedItem := item
	normalizedItem.DailyQuotaLimit = normalizeServicePackageDailyQuotaLimit(item.DailyQuotaLimit)
	normalizeServicePackageScopeAndQuota(&normalizedItem)
	if err := validateServicePackageScopeAndQuota(normalizedItem); err != nil {
		return ServicePackage{}, err
	}
	row := ServicePackage{
		Id:                         strings.TrimSpace(item.Id),
		Name:                       name,
		Description:                normalizeServicePackageDescription(item.Description),
		GroupID:                    groupID,
		PackageType:                normalizedItem.PackageType,
		ScopeType:                  normalizedItem.ScopeType,
		ScopeProvider:              normalizedItem.ScopeProvider,
		ScopeModel:                 normalizedItem.ScopeModel,
		ScopeEndpoint:              normalizedItem.ScopeEndpoint,
		QuotaMetric:                normalizedItem.QuotaMetric,
		PeriodType:                 normalizedItem.PeriodType,
		PeriodLimit:                normalizedItem.PeriodLimit,
		MaxConcurrencyPerUser:      normalizedItem.MaxConcurrencyPerUser,
		MaxConcurrencyPerPackage:   normalizedItem.MaxConcurrencyPerPackage,
		AllowBalanceFallback:       normalizedItem.AllowBalanceFallback,
		VisibilityScope:            visibilityScope,
		SalePrice:                  normalizeServicePackageSalePrice(item.SalePrice),
		SaleCurrency:               normalizeServicePackageSaleCurrency(item.SaleCurrency),
		DailyQuotaLimit:            normalizedItem.DailyQuotaLimit,
		PackageEmergencyQuotaLimit: normalizeServicePackagePackageEmergencyQuotaLimit(item.PackageEmergencyQuotaLimit),
		DurationDays:               normalizeServicePackageDurationDays(item.DurationDays),
		QuotaResetTimezone:         normalizeServicePackageTimezone(item.QuotaResetTimezone),
		Enabled:                    item.Enabled,
		SortOrder:                  item.SortOrder,
		Source:                     normalizeServicePackageSource(item.Source),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	row.EnsureID()
	if row.SortOrder <= 0 {
		row.SortOrder = maxSortOrder + 1
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return syncServicePackageVisibleUsersWithDB(tx, row.Id, visibleUserIDs)
	}); err != nil {
		return ServicePackage{}, err
	}
	row.GroupName = resolveServicePackageGroupNameWithDB(db, row.GroupID)
	row.VisibleUserIDs = visibleUserIDs
	row.VisibleUsers, _ = resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs)
	return row, nil
}

func updateServicePackageWithDB(db *gorm.DB, item ServicePackage) (ServicePackage, error) {
	if db == nil {
		return ServicePackage{}, fmt.Errorf("database handle is nil")
	}
	id := strings.TrimSpace(item.Id)
	if id == "" {
		return ServicePackage{}, fmt.Errorf("套餐 ID 不能为空")
	}
	row := ServicePackage{}
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return ServicePackage{}, err
	}
	nextName := strings.TrimSpace(row.Name)
	if normalized := normalizeServicePackageName(item.Name); normalized != "" {
		nextName = normalized
	}
	if nextName == "" {
		return ServicePackage{}, fmt.Errorf("套餐名称不能为空")
	}
	if len(nextName) > 64 {
		return ServicePackage{}, fmt.Errorf("套餐名称长度不能超过 64")
	}
	if nextName != row.Name {
		existing, err := getServicePackageByNameWithDB(db, nextName)
		if err == nil && strings.TrimSpace(existing.Id) != row.Id {
			return ServicePackage{}, fmt.Errorf("套餐已存在")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return ServicePackage{}, err
		}
	}
	groupID := strings.TrimSpace(row.GroupID)
	if strings.TrimSpace(item.GroupID) != "" {
		resolvedGroupID, err := resolveServicePackageGroupIDWithDB(db, item.GroupID)
		if err != nil {
			return ServicePackage{}, err
		}
		groupID = resolvedGroupID
	}
	visibilityScope := normalizeServicePackageVisibilityScope(item.VisibilityScope)
	visibleUserIDs := normalizeServicePackageVisibleUserIDs(item.VisibleUserIDs)
	if len(visibleUserIDs) > 0 {
		if _, err := resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs); err != nil {
			return ServicePackage{}, err
		}
	}
	row.Name = nextName
	row.Description = normalizeServicePackageDescription(item.Description)
	row.GroupID = groupID
	row.PackageType = item.PackageType
	row.ScopeType = item.ScopeType
	row.ScopeProvider = item.ScopeProvider
	row.ScopeModel = item.ScopeModel
	row.ScopeEndpoint = item.ScopeEndpoint
	row.QuotaMetric = item.QuotaMetric
	row.PeriodType = item.PeriodType
	row.PeriodLimit = item.PeriodLimit
	row.MaxConcurrencyPerUser = item.MaxConcurrencyPerUser
	row.MaxConcurrencyPerPackage = item.MaxConcurrencyPerPackage
	row.AllowBalanceFallback = item.AllowBalanceFallback
	row.VisibilityScope = visibilityScope
	row.SalePrice = normalizeServicePackageSalePrice(item.SalePrice)
	row.SaleCurrency = normalizeServicePackageSaleCurrency(item.SaleCurrency)
	row.DailyQuotaLimit = normalizeServicePackageDailyQuotaLimit(item.DailyQuotaLimit)
	row.PackageEmergencyQuotaLimit = normalizeServicePackagePackageEmergencyQuotaLimit(item.PackageEmergencyQuotaLimit)
	row.DurationDays = normalizeServicePackageDurationDays(item.DurationDays)
	row.QuotaResetTimezone = normalizeServicePackageTimezone(item.QuotaResetTimezone)
	normalizeServicePackageScopeAndQuota(&row)
	if err := validateServicePackageScopeAndQuota(row); err != nil {
		return ServicePackage{}, err
	}
	row.Enabled = item.Enabled
	if item.SortOrder > 0 {
		row.SortOrder = item.SortOrder
	}
	row.Source = normalizeServicePackageSource(item.Source)
	row.UpdatedAt = helper.GetTimestamp()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		return syncServicePackageVisibleUsersWithDB(tx, row.Id, visibleUserIDs)
	}); err != nil {
		return ServicePackage{}, err
	}
	row.GroupName = resolveServicePackageGroupNameWithDB(db, row.GroupID)
	row.VisibleUserIDs = visibleUserIDs
	row.VisibleUsers, _ = resolveServicePackageVisibleUsersWithDB(db, visibleUserIDs)
	return row, nil
}

func resolveServicePackageGroupNameWithDB(db *gorm.DB, groupID string) string {
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(groupCatalog.Name)
}

func deleteServicePackageWithDB(db *gorm.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return fmt.Errorf("套餐 ID 不能为空")
	}
	activeCount := int64(0)
	if err := db.Model(&UserPackageSubscription{}).
		Where("package_id = ? AND status = ?", normalizedID, UserPackageSubscriptionStatusActive).
		Count(&activeCount).Error; err != nil {
		return err
	}
	if activeCount > 0 {
		return fmt.Errorf("套餐仍有生效订阅，无法删除")
	}
	result := db.Where("id = ?", normalizedID).Delete(&ServicePackage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListServicePackagesPage(page int, pageSize int, keyword string) ([]ServicePackage, int64, error) {
	return listServicePackagesPageWithDB(DB, page, pageSize, keyword)
}

func ListEnabledServicePackagesForUser(userID string) ([]ServicePackage, error) {
	if DB == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]ServicePackage, 0)
	query := DB.Where("enabled = ?", true)
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		query = query.Where(
			"COALESCE(visibility_scope, '') = '' OR visibility_scope = ?",
			ServicePackageVisibilityScopeAll,
		)
	} else {
		query = query.Where(
			`(
				COALESCE(visibility_scope, '') = ''
				OR visibility_scope = ?
				OR EXISTS (
					SELECT 1
					FROM `+ServicePackageVisibleUsersTableName+` spu
					WHERE spu.package_id = service_packages.id AND spu.user_id = ?
				)
			)`,
			ServicePackageVisibilityScopeAll,
			normalizedUserID,
		)
	}
	if err := query.
		Order("sort_order asc, name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateServicePackageGroupNamesWithDB(DB, rows); err != nil {
		return nil, err
	}
	if err := hydrateServicePackageSupportedModelsWithDB(DB, rows); err != nil {
		return nil, err
	}
	if err := hydrateServicePackageVisibilityWithDB(DB, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ListEnabledServicePackages() ([]ServicePackage, error) {
	return ListEnabledServicePackagesForUser("")
}

func GetServicePackageByID(id string) (ServicePackage, error) {
	return getServicePackageByIDWithDB(DB, id)
}

func CreateServicePackage(item ServicePackage) (ServicePackage, error) {
	return createServicePackageWithDB(DB, item)
}

func UpdateServicePackage(item ServicePackage) (ServicePackage, error) {
	return updateServicePackageWithDB(DB, item)
}

func DeleteServicePackage(id string) error {
	return deleteServicePackageWithDB(DB, id)
}
