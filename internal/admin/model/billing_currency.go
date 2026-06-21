package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BillingCurrenciesTableName = "billing_currencies"

	BillingCurrencyStatusEnabled  = 1
	BillingCurrencyStatusDisabled = 2

	BillingCurrencyCodeYYC = "YYC"
	BillingCurrencyCodeUSD = "USD"
	BillingCurrencyCodeCNY = "CNY"

	BillingCurrencySourceSystemDefault = "system_default"
	BillingCurrencySourceManual        = "manual"
	BillingCurrencySourceFXAuto        = "fx_auto"

	defaultUSDCNYExchangeRate = 7.0
	defaultUSDChargeRate      = 500 * 1000.0
	defaultCNYChargeRate      = defaultUSDChargeRate / defaultUSDCNYExchangeRate
	defaultYYCChargeRate      = 1.0
	defaultYYCMinorUnit       = 0
	defaultFiatMinorUnit      = 6
)

type BillingCurrency struct {
	Code       string  `json:"code" gorm:"primaryKey;type:varchar(16)"`
	Name       string  `json:"name" gorm:"type:varchar(64);not null;default:''"`
	Symbol     string  `json:"symbol" gorm:"type:varchar(16);not null;default:''"`
	MinorUnit  int     `json:"minor_unit" gorm:"not null;default:6"`
	ChargeRate float64 `json:"charge_rate" gorm:"column:charge_rate;type:double precision;not null;default:0"`
	Status     int     `json:"status" gorm:"not null;default:1"`
	Source     string  `json:"source" gorm:"type:varchar(64);not null;default:'system_default'"`
	CreatedAt  int64   `json:"created_at" gorm:"bigint;not null;default:0"`
	UpdatedAt  int64   `json:"updated_at" gorm:"bigint;not null;default:0"`
}

func (BillingCurrency) TableName() string {
	return BillingCurrenciesTableName
}

type billingCurrencyIndex struct {
	allByCode     map[string]BillingCurrency
	enabledByCode map[string]BillingCurrency
}

var (
	billingCurrencyIndexLock sync.RWMutex
	billingCurrencyCache     = billingCurrencyIndex{
		allByCode:     make(map[string]BillingCurrency),
		enabledByCode: make(map[string]BillingCurrency),
	}
)

func normalizeBillingCurrencyCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	switch normalized {
	case "", BillingCurrencyCodeUSD:
		return BillingCurrencyCodeUSD
	case "RMB":
		return BillingCurrencyCodeCNY
	default:
		return normalized
	}
}

func defaultBillingCurrencies() []BillingCurrency {
	usdChargeRate := config.QuotaPerUnit
	if usdChargeRate <= 0 {
		usdChargeRate = defaultUSDChargeRate
	}
	now := helper.GetTimestamp()
	return []BillingCurrency{
		{
			Code:       BillingCurrencyCodeYYC,
			Name:       "Yeying Coin",
			Symbol:     "Ɏ",
			MinorUnit:  defaultYYCMinorUnit,
			ChargeRate: defaultYYCChargeRate,
			Status:     BillingCurrencyStatusEnabled,
			Source:     BillingCurrencySourceSystemDefault,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			Code:       BillingCurrencyCodeUSD,
			Name:       "US Dollar",
			Symbol:     "$",
			MinorUnit:  defaultFiatMinorUnit,
			ChargeRate: usdChargeRate,
			Status:     BillingCurrencyStatusEnabled,
			Source:     BillingCurrencySourceSystemDefault,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			Code:       BillingCurrencyCodeCNY,
			Name:       "Chinese Yuan",
			Symbol:     "¥",
			MinorUnit:  defaultFiatMinorUnit,
			ChargeRate: defaultCNYChargeRate,
			Status:     BillingCurrencyStatusEnabled,
			Source:     BillingCurrencySourceSystemDefault,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

func syncDefaultBillingCurrenciesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	defaults := defaultBillingCurrencies()
	for _, item := range defaults {
		row := BillingCurrency{}
		err := db.Where("code = ?", item.Code).Take(&row).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error; err != nil {
				return err
			}
			continue
		}
		if strings.TrimSpace(strings.ToLower(row.Source)) != BillingCurrencySourceSystemDefault {
			continue
		}
		updates := map[string]any{
			"name":       item.Name,
			"symbol":     item.Symbol,
			"minor_unit": item.MinorUnit,
			"status":     item.Status,
			"source":     item.Source,
			"updated_at": item.UpdatedAt,
		}
		if row.CreatedAt <= 0 {
			updates["created_at"] = item.CreatedAt
		}
		// Keep non-USD currencies decoupled from QuotaPerUnit-driven default linkage.
		// For USD, preserve QuotaPerUnit compatibility as a system default.
		if item.Code == BillingCurrencyCodeUSD || row.ChargeRate <= 0 {
			updates["charge_rate"] = item.ChargeRate
		}
		if err := db.Model(&BillingCurrency{}).
			Where("code = ?", item.Code).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func billingCurrencyColumnExistsWithDB(db *gorm.DB, columnName string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database handle is nil")
	}
	var count int64
	switch strings.ToLower(strings.TrimSpace(db.Dialector.Name())) {
	case "sqlite":
		if err := db.Raw(
			"SELECT COUNT(1) FROM pragma_table_info(?) WHERE name = ?",
			BillingCurrenciesTableName,
			columnName,
		).Scan(&count).Error; err != nil {
			return false, err
		}
	default:
		if err := db.Raw(
			"SELECT COUNT(1) FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			BillingCurrenciesTableName,
			columnName,
		).Scan(&count).Error; err != nil {
			return false, err
		}
	}
	return count > 0, nil
}

func migrateBillingCurrencyChargeRateWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	hasChargeRate, err := billingCurrencyColumnExistsWithDB(db, "charge_rate")
	if err != nil {
		return err
	}
	if !hasChargeRate {
		addColumnSQL := "ALTER TABLE billing_currencies ADD COLUMN charge_rate DOUBLE PRECISION NOT NULL DEFAULT 0"
		if err := db.Exec(addColumnSQL).Error; err != nil {
			return err
		}
	}
	err = db.Exec(`
		UPDATE billing_currencies
		SET charge_rate = yyc_per_unit
		WHERE COALESCE(charge_rate, 0) <= 0
		  AND COALESCE(yyc_per_unit, 0) > 0
	`).Error
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "yyc_per_unit") {
		return err
	}
	return nil
}

func decoupleCNYYYCFromSystemDefaultWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&BillingCurrency{}); err != nil {
		return err
	}
	if err := syncDefaultBillingCurrenciesWithDB(db); err != nil {
		return err
	}

	row := BillingCurrency{}
	err := db.Where("code = ?", BillingCurrencyCodeCNY).Take(&row).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&BillingCurrency{
			Code:       BillingCurrencyCodeCNY,
			Name:       "Chinese Yuan",
			Symbol:     "¥",
			MinorUnit:  defaultFiatMinorUnit,
			ChargeRate: defaultCNYChargeRate,
			Status:     BillingCurrencyStatusEnabled,
			Source:     "manual",
			CreatedAt:  helper.GetTimestamp(),
			UpdatedAt:  helper.GetTimestamp(),
		}).Error
	}

	updates := map[string]any{
		"updated_at": helper.GetTimestamp(),
	}
	if strings.TrimSpace(strings.ToLower(row.Source)) == "" ||
		strings.TrimSpace(strings.ToLower(row.Source)) == BillingCurrencySourceSystemDefault {
		updates["source"] = "manual"
	}
	if row.ChargeRate <= 0 {
		updates["charge_rate"] = defaultCNYChargeRate
	}
	if len(updates) == 1 {
		return nil
	}
	return db.Model(&BillingCurrency{}).
		Where("code = ?", BillingCurrencyCodeCNY).
		Updates(updates).Error
}

func SyncBillingCurrencyCatalogWithDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	rows := make([]BillingCurrency, 0)
	if err := db.Order("code asc").Find(&rows).Error; err != nil {
		return err
	}

	next := billingCurrencyIndex{
		allByCode:     make(map[string]BillingCurrency, len(rows)),
		enabledByCode: make(map[string]BillingCurrency, len(rows)),
	}
	for _, row := range rows {
		code := normalizeBillingCurrencyCode(row.Code)
		if code == "" {
			continue
		}
		row.Code = code
		next.allByCode[code] = row
		if row.Status == BillingCurrencyStatusEnabled && row.ChargeRate > 0 {
			next.enabledByCode[code] = row
		}
	}

	billingCurrencyIndexLock.Lock()
	billingCurrencyCache = next
	billingCurrencyIndexLock.Unlock()
	return nil
}

func ListBillingCurrencies() ([]BillingCurrency, error) {
	if DB == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if err := SyncBillingCurrencyCatalogWithDB(DB); err != nil {
		return nil, err
	}
	rows := make([]BillingCurrency, 0)
	if err := DB.Order("code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for index := range rows {
		rows[index].Code = normalizeBillingCurrencyCode(rows[index].Code)
	}
	return rows, nil
}

func GetBillingCurrency(code string) (BillingCurrency, error) {
	normalizedCode := normalizeBillingCurrencyCode(code)
	billingCurrencyIndexLock.RLock()
	row, ok := billingCurrencyCache.allByCode[normalizedCode]
	billingCurrencyIndexLock.RUnlock()
	if !ok && DB != nil {
		if err := SyncBillingCurrencyCatalogWithDB(DB); err != nil {
			return BillingCurrency{}, err
		}
		billingCurrencyIndexLock.RLock()
		row, ok = billingCurrencyCache.allByCode[normalizedCode]
		billingCurrencyIndexLock.RUnlock()
	}
	if !ok {
		for _, item := range defaultBillingCurrencies() {
			if item.Code == normalizedCode {
				return item, nil
			}
		}
		return BillingCurrency{}, fmt.Errorf("billing currency not configured: %s", normalizedCode)
	}
	return row, nil
}

func GetBillingCurrencyChargeRate(code string) (float64, error) {
	row, err := GetBillingCurrency(code)
	if err != nil {
		return 0, err
	}
	if row.Status != BillingCurrencyStatusEnabled {
		return 0, fmt.Errorf("billing currency disabled: %s", row.Code)
	}
	if row.ChargeRate <= 0 {
		return 0, fmt.Errorf("billing currency charge_rate invalid: %s", row.Code)
	}
	return row.ChargeRate, nil
}

func normalizeBillingCurrencyStatus(status int) int {
	switch status {
	case BillingCurrencyStatusEnabled, BillingCurrencyStatusDisabled:
		return status
	default:
		return BillingCurrencyStatusEnabled
	}
}

func normalizeBillingCurrencyMinorUnit(value int) int {
	if value < 0 {
		return 0
	}
	if value > 8 {
		return 8
	}
	return value
}

func normalizeBillingCurrencySource(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return BillingCurrencySourceManual
	}
	return normalized
}

func validateBillingCurrencyForWrite(row BillingCurrency, isCreate bool) (BillingCurrency, error) {
	row.Code = normalizeBillingCurrencyCode(row.Code)
	if row.Code == "" {
		return BillingCurrency{}, fmt.Errorf("币种代码不能为空")
	}
	if len(row.Code) > 16 {
		return BillingCurrency{}, fmt.Errorf("币种代码长度不能超过 16")
	}
	row.Name = strings.TrimSpace(row.Name)
	if row.Name == "" {
		return BillingCurrency{}, fmt.Errorf("币种名称不能为空")
	}
	row.Symbol = strings.TrimSpace(row.Symbol)
	row.MinorUnit = normalizeBillingCurrencyMinorUnit(row.MinorUnit)
	row.Status = normalizeBillingCurrencyStatus(row.Status)
	row.Source = normalizeBillingCurrencySource(row.Source)
	if row.ChargeRate < 0 {
		return BillingCurrency{}, fmt.Errorf("扣减比率不能小于 0")
	}
	if row.Status == BillingCurrencyStatusEnabled && row.ChargeRate <= 0 {
		return BillingCurrency{}, fmt.Errorf("启用状态的币种必须配置大于 0 的 扣减比率")
	}
	if isCreate && row.UpdatedAt == 0 {
		now := helper.GetTimestamp()
		row.UpdatedAt = now
		if row.CreatedAt == 0 {
			row.CreatedAt = now
		}
	}
	if !isCreate {
		row.UpdatedAt = helper.GetTimestamp()
	}
	return row, nil
}

func CreateBillingCurrencyWithDB(db *gorm.DB, row BillingCurrency) (BillingCurrency, error) {
	if db == nil {
		return BillingCurrency{}, fmt.Errorf("database handle is nil")
	}
	normalized, err := validateBillingCurrencyForWrite(row, true)
	if err != nil {
		return BillingCurrency{}, err
	}
	if err := db.Create(&normalized).Error; err != nil {
		return BillingCurrency{}, err
	}
	if err := SyncBillingCurrencyCatalogWithDB(db); err != nil {
		return BillingCurrency{}, err
	}
	return normalized, nil
}

func DeleteBillingCurrencyWithDB(db *gorm.DB, code string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedCode := normalizeBillingCurrencyCode(code)
	if normalizedCode == "" {
		return fmt.Errorf("币种代码不能为空")
	}
	if err := db.Where("code = ?", normalizedCode).Delete(&BillingCurrency{}).Error; err != nil {
		return err
	}
	return SyncBillingCurrencyCatalogWithDB(db)
}

func normalizeBillingCurrencyAutoSourcesWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	return db.Model(&BillingCurrency{}).
		Where("lower(trim(source)) = ?", BillingCurrencySourceFXAuto).
		Updates(map[string]any{
			"source":     BillingCurrencySourceManual,
			"updated_at": helper.GetTimestamp(),
		}).Error
}

func UpdateBillingCurrencyWithDB(db *gorm.DB, code string, apply func(current BillingCurrency) (BillingCurrency, error)) (BillingCurrency, error) {
	if db == nil {
		return BillingCurrency{}, fmt.Errorf("database handle is nil")
	}
	normalizedCode := normalizeBillingCurrencyCode(code)
	if normalizedCode == "" {
		return BillingCurrency{}, fmt.Errorf("币种代码不能为空")
	}

	updated := BillingCurrency{}
	err := db.Transaction(func(tx *gorm.DB) error {
		current := BillingCurrency{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", normalizedCode).Take(&current).Error; err != nil {
			return err
		}
		next, err := apply(current)
		if err != nil {
			return err
		}
		next.Code = current.Code
		if next.Source == "" && strings.TrimSpace(strings.ToLower(current.Source)) == BillingCurrencySourceSystemDefault {
			next.Source = BillingCurrencySourceManual
		}
		next, err = validateBillingCurrencyForWrite(next, false)
		if err != nil {
			return err
		}
		if err := tx.Model(&BillingCurrency{}).
			Where("code = ?", normalizedCode).
			Updates(map[string]any{
				"name":        next.Name,
				"symbol":      next.Symbol,
				"minor_unit":  next.MinorUnit,
				"charge_rate": next.ChargeRate,
				"status":      next.Status,
				"source":      next.Source,
				"updated_at":  next.UpdatedAt,
			}).Error; err != nil {
			return err
		}
		updated = next
		return nil
	})
	if err != nil {
		return BillingCurrency{}, err
	}
	if err := SyncBillingCurrencyCatalogWithDB(db); err != nil {
		return BillingCurrency{}, err
	}
	if normalizedCode == BillingCurrencyCodeUSD && updated.ChargeRate > 0 {
		if err := UpdateOption("QuotaPerUnit", strconv.FormatFloat(updated.ChargeRate, 'f', -1, 64)); err != nil {
			return BillingCurrency{}, err
		}
	}
	return updated, nil
}
