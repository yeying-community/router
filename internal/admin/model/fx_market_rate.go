package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const FXMarketRatesTableName = "fx_market_rates"

type FXMarketRate struct {
	Base      string  `json:"base" gorm:"primaryKey;type:varchar(16)"`
	Quote     string  `json:"quote" gorm:"primaryKey;type:varchar(16)"`
	Provider  string  `json:"provider" gorm:"type:varchar(64);not null;default:''"`
	RateDate  string  `json:"rate_date" gorm:"column:rate_date;type:varchar(32);not null;default:''"`
	Rate      float64 `json:"rate" gorm:"type:double precision;not null;default:0"`
	UpdatedAt int64   `json:"updated_at" gorm:"bigint;not null;default:0"`
}

func (FXMarketRate) TableName() string {
	return FXMarketRatesTableName
}

func normalizeFXMarketRateCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	switch normalized {
	case "RMB":
		return BillingCurrencyCodeCNY
	default:
		return normalized
	}
}

func normalizeFXMarketRateFilterCodes(currencies []string) []string {
	seen := make(map[string]struct{}, len(currencies))
	codes := make([]string, 0, len(currencies))
	for _, raw := range currencies {
		code := normalizeFXMarketRateCode(raw)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func validateFXMarketRateRow(row FXMarketRate) (FXMarketRate, error) {
	row.Base = normalizeFXMarketRateCode(row.Base)
	row.Quote = normalizeFXMarketRateCode(row.Quote)
	row.Provider = strings.TrimSpace(row.Provider)
	row.RateDate = strings.TrimSpace(row.RateDate)
	if row.Base == "" || row.Quote == "" {
		return FXMarketRate{}, fmt.Errorf("fx market rate base/quote cannot be empty")
	}
	if row.Base == row.Quote {
		return FXMarketRate{}, fmt.Errorf("fx market rate base and quote must differ")
	}
	if row.Provider == "" {
		return FXMarketRate{}, fmt.Errorf("fx market rate provider cannot be empty")
	}
	if row.Rate <= 0 {
		return FXMarketRate{}, fmt.Errorf("fx market rate must be greater than 0")
	}
	row.UpdatedAt = helper.GetTimestamp()
	return row, nil
}

func UpsertFXMarketRatesWithDB(db *gorm.DB, provider string, rateDate string, items []FXMarketRate) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&FXMarketRate{}); err != nil {
		return err
	}

	normalizedRows := make([]FXMarketRate, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.Provider = provider
		item.RateDate = rateDate
		row, err := validateFXMarketRateRow(item)
		if err != nil {
			return err
		}
		key := row.Base + ":" + row.Quote
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalizedRows = append(normalizedRows, row)
	}
	if len(normalizedRows) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range normalizedRows {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "base"},
					{Name: "quote"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"provider",
					"rate_date",
					"rate",
					"updated_at",
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ListFXMarketRatesWithDB(db *gorm.DB, currencies []string) ([]FXMarketRate, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	rows := make([]FXMarketRate, 0)
	query := db.Model(&FXMarketRate{})
	codes := normalizeFXMarketRateFilterCodes(currencies)
	if len(codes) > 0 {
		query = query.Where("base IN ? AND quote IN ?", codes, codes)
	}
	if err := query.Order("base asc").Order("quote asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
