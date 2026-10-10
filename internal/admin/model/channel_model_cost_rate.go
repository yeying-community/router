package model

import (
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChannelModelCostRate caches one normalized per-(channel, model, capacity_unit)
// unit cost quote sourced from the billing service. The online cost floor reads
// it (gated + TTL) and only treats a quote with Confidence="actual" as
// production-grade; anything else falls back to the local procurement path.
//
// Persisted in CNY for a single, auditable unit: Currency keeps the original
// service currency for transparency, UnitCost is converted to CNY at sync time
// via BillingCurrency.ChargeRate. This is the local rate cache that powers the
// service quote → online floor wiring (P5 §A.4 step 2).

const (
	ChannelModelCostRateConfidenceActual    = "actual"
	ChannelModelCostRateConfidenceEstimated = "estimated"

	ChannelModelCostRateSourceService = "service"
)

type ChannelModelCostRate struct {
	Id           string `json:"id" gorm:"type:char(36);primaryKey"`
	ChannelId    string `json:"channel_id" gorm:"type:char(36);not null;index;uniqueIndex:idx_channel_model_unit"`
	Model        string `json:"model" gorm:"type:varchar(255);not null;index;uniqueIndex:idx_channel_model_unit"`
	CapacityUnit string `json:"capacity_unit" gorm:"type:varchar(64);not null;uniqueIndex:idx_channel_model_unit"`
	// UnitCostYyc is the service unit cost normalized to CNY at sync time, used
	// directly by the cost floor. It is the source of truth on the read path.
	UnitCostYyc float64 `json:"unit_cost_yyc" gorm:"type:double precision;not null;default:0"`
	// UnitCostOriginal + Currency + FXRate are kept for audit / reconciliation
	// only; never read by the online floor.
	UnitCostOriginal float64 `json:"unit_cost_original" gorm:"type:double precision;not null;default:0"`
	Currency         string  `json:"currency" gorm:"type:varchar(16);not null;default:''"`
	FXRate           float64 `json:"fx_rate" gorm:"type:double precision;not null;default:0"`
	Confidence       string  `json:"confidence" gorm:"type:varchar(16);not null;default:''"`
	Source           string  `json:"source" gorm:"type:varchar(32);not null;default:'service'"`
	AsOf             int64   `json:"as_of" gorm:"bigint;not null;default:0"`
	FetchedAt       int64   `json:"fetched_at" gorm:"bigint;not null;default:0"`
	ValidUntil       int64   `json:"valid_until" gorm:"bigint;not null;default:0"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint;not null;default:0"`
}

func (ChannelModelCostRate) TableName() string { return ChannelModelCostRatesTableName }

// NormalizeChannelModelCostRate trims/lowers the model name so cache lookups are
// stable regardless of source casing.
func NormalizeChannelModelCostRate(row ChannelModelCostRate) ChannelModelCostRate {
	row.Model = strings.TrimSpace(row.Model)
	row.CapacityUnit = strings.TrimSpace(strings.ToLower(row.CapacityUnit))
	row.Confidence = strings.TrimSpace(strings.ToLower(row.Confidence))
	row.Source = strings.TrimSpace(strings.ToLower(row.Source))
	row.Currency = strings.TrimSpace(strings.ToUpper(row.Currency))
	return row
}

// IsChannelModelCostRateFresh reports whether the cached rate is still within the
// TTL window (freshSeconds) at the given now.
func IsChannelModelCostRateFresh(rate ChannelModelCostRate, now time.Time, freshSeconds int64) bool {
	if rate.UnitCostYyc <= 0 {
		return false
	}
	if rate.Confidence != ChannelModelCostRateConfidenceActual {
		return false
	}
	if rate.AsOf <= 0 {
		return false
	}
	if freshSeconds <= 0 {
		return false
	}
	return now.Unix()-rate.AsOf <= freshSeconds
}

// ReplaceChannelModelCostRatesWithDB replaces all cached rates for a channel with
// the supplied set (full refresh per sync). An empty set clears the channel's cache.
func ReplaceChannelModelCostRatesWithDB(db *gorm.DB, channelID string, rows []ChannelModelCostRate) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if db == nil || normalizedChannelID == "" {
		return nil
	}
	now := helper.GetTimestamp()
	prepared := make([]ChannelModelCostRate, 0, len(rows))
	for _, row := range rows {
		row = NormalizeChannelModelCostRate(row)
		row.ChannelId = normalizedChannelID
		if row.Model == "" || row.CapacityUnit == "" {
			continue
		}
		if strings.TrimSpace(row.Id) == "" {
			row.Id = random.GetUUID()
		}
		if row.FetchedAt == 0 {
			row.FetchedAt = now
		}
		row.UpdatedAt = now
		prepared = append(prepared, row)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", normalizedChannelID).Delete(&ChannelModelCostRate{}).Error; err != nil {
			return err
		}
		if len(prepared) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "channel_id"}, {Name: "model"}, {Name: "capacity_unit"}},
			UpdateAll: true,
		}).Create(&prepared).Error
	})
}

// ResolveChannelModelCostRateWithDB returns the cached rate for a channel+model that
// matches one of the given capacity units, newest AsOf first. Returns false when no
// matching row exists.
func ResolveChannelModelCostRateWithDB(db *gorm.DB, channelID string, modelName string, capacityUnits []string) (ChannelModelCostRate, bool) {
	normalizedChannelID := strings.TrimSpace(channelID)
	normalizedModel := strings.TrimSpace(modelName)
	if db == nil || normalizedChannelID == "" || normalizedModel == "" {
		return ChannelModelCostRate{}, false
	}
	units := make([]string, 0, len(capacityUnits))
	for _, unit := range capacityUnits {
		if trimmed := strings.TrimSpace(strings.ToLower(unit)); trimmed != "" {
			units = append(units, trimmed)
		}
	}
	if len(units) == 0 {
		return ChannelModelCostRate{}, false
	}
	row := ChannelModelCostRate{}
	err := db.Where("channel_id = ? AND model = ? AND capacity_unit IN ?", normalizedChannelID, normalizedModel, units).
		Order("as_of DESC").
		Take(&row).Error
	if err != nil {
		return ChannelModelCostRate{}, false
	}
	return row, true
}

