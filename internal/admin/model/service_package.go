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

const (
	DefaultServicePackageDurationDays = 30
)

type ServicePackage struct {
	Id                         string `json:"id" gorm:"primaryKey;type:char(36)"`
	Name                       string `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Description                string `json:"description" gorm:"type:varchar(255);default:''"`
	GroupID                    string `json:"group_id" gorm:"type:char(36);not null;index"`
	SalePrice                  float64 `json:"sale_price" gorm:"type:decimal(10,2);not null;default:0"`
	SaleCurrency               string `json:"sale_currency" gorm:"type:varchar(16);not null;default:'CNY'"`
	DailyQuotaLimit            int64  `json:"daily_quota_limit" gorm:"type:bigint;not null;default:0"`
	PackageEmergencyQuotaLimit int64  `json:"package_emergency_quota_limit" gorm:"column:package_emergency_quota_limit;type:bigint;not null;default:0"`
	DurationDays               int    `json:"duration_days" gorm:"type:int;not null;default:30"`
	QuotaResetTimezone         string `json:"quota_reset_timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	Enabled                    bool   `json:"enabled" gorm:"default:true;index"`
	SortOrder                  int    `json:"sort_order" gorm:"default:0;index"`
	Source                     string `json:"source" gorm:"type:varchar(32);default:'manual'"`
	CreatedAt                  int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                  int64  `json:"updated_at" gorm:"bigint;index"`
	GroupName                  string `json:"group_name,omitempty" gorm:"-"`
}

func (ServicePackage) TableName() string {
	return ServicePackagesTableName
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
	maxSortOrder := 0
	if err := db.Model(&ServicePackage{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder).Error; err != nil {
		return ServicePackage{}, err
	}
	now := helper.GetTimestamp()
	row := ServicePackage{
		Id:                         strings.TrimSpace(item.Id),
		Name:                       name,
		Description:                normalizeServicePackageDescription(item.Description),
		GroupID:                    groupID,
		SalePrice:                  normalizeServicePackageSalePrice(item.SalePrice),
		SaleCurrency:               normalizeServicePackageSaleCurrency(item.SaleCurrency),
		DailyQuotaLimit:            normalizeServicePackageDailyQuotaLimit(item.DailyQuotaLimit),
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
	if err := db.Create(&row).Error; err != nil {
		return ServicePackage{}, err
	}
	row.GroupName = resolveServicePackageGroupNameWithDB(db, row.GroupID)
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
	row.Name = nextName
	row.Description = normalizeServicePackageDescription(item.Description)
	row.GroupID = groupID
	row.SalePrice = normalizeServicePackageSalePrice(item.SalePrice)
	row.SaleCurrency = normalizeServicePackageSaleCurrency(item.SaleCurrency)
	row.DailyQuotaLimit = normalizeServicePackageDailyQuotaLimit(item.DailyQuotaLimit)
	row.PackageEmergencyQuotaLimit = normalizeServicePackagePackageEmergencyQuotaLimit(item.PackageEmergencyQuotaLimit)
	row.DurationDays = normalizeServicePackageDurationDays(item.DurationDays)
	row.QuotaResetTimezone = normalizeServicePackageTimezone(item.QuotaResetTimezone)
	row.Enabled = item.Enabled
	if item.SortOrder > 0 {
		row.SortOrder = item.SortOrder
	}
	row.Source = normalizeServicePackageSource(item.Source)
	row.UpdatedAt = helper.GetTimestamp()
	if err := db.Save(&row).Error; err != nil {
		return ServicePackage{}, err
	}
	row.GroupName = resolveServicePackageGroupNameWithDB(db, row.GroupID)
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

func ListEnabledServicePackages() ([]ServicePackage, error) {
	if DB == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]ServicePackage, 0)
	if err := DB.
		Where("enabled = ?", true).
		Order("sort_order asc, name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateServicePackageGroupNamesWithDB(DB, rows); err != nil {
		return nil, err
	}
	return rows, nil
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
