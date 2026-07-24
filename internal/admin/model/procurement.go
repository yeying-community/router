package model

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChannelProcurementBatchesTableName      = "channel_procurement_batches"
	RequestProcurementConsumptionsTableName = "request_procurement_consumptions"
)

const (
	ProcurementCostSourceActual    = "actual"
	ProcurementCostSourceEstimated = "estimated"
	ProcurementCostSourceNone      = "none"
	ProcurementCostSourceZeroCost  = "zero_cost"
)

const (
	ProcurementCostStatusActive           = "active"
	ProcurementCostStatusCostUnconfigured = "cost_unconfigured"
	ProcurementCostStatusExhausted        = "exhausted"
	ProcurementCostStatusExpired          = "expired"
	ProcurementCostStatusDisabled         = "disabled"
)

type ChannelProcurementBatch struct {
	Id                   string  `json:"id" gorm:"type:char(36);primaryKey"`
	ChannelId            string  `json:"channel_id" gorm:"type:char(36);not null;index"`
	ResourceType         string  `json:"resource_type" gorm:"type:varchar(32);not null;default:'';index"`
	QuotaType            string  `json:"quota_type" gorm:"type:varchar(32);not null;default:'';index"`
	ScopeType            string  `json:"scope_type" gorm:"type:varchar(32);not null;default:'global';index"`
	ScopeValue           string  `json:"scope_value" gorm:"type:varchar(191);not null;default:'';index"`
	CapacityUnit         string  `json:"capacity_unit" gorm:"type:varchar(32);not null;default:'';index"`
	CapacityTotal        float64 `json:"capacity_total" gorm:"type:double precision;not null;default:0"`
	CapacityEffective    float64 `json:"capacity_effective" gorm:"type:double precision;not null;default:0"`
	CapacityRemaining    float64 `json:"capacity_remaining" gorm:"type:double precision;not null;default:0"`
	PurchaseCurrency     string  `json:"purchase_currency" gorm:"type:varchar(16);not null;default:''"`
	PurchaseAmount       float64 `json:"purchase_amount" gorm:"type:double precision;not null;default:0"`
	PurchaseFXRate       float64 `json:"purchase_fx_rate" gorm:"type:double precision;not null;default:0"`
	PurchaseCostAmount   float64 `json:"purchase_cost_amount" gorm:"type:double precision;not null;default:0"`
	CostPerUnitAmount    float64 `json:"cost_per_unit_amount" gorm:"type:double precision;not null;default:0"`
	CostSource           string  `json:"cost_source" gorm:"type:varchar(32);not null;default:'';index"`
	CostStatus           string  `json:"cost_status" gorm:"type:varchar(32);not null;default:'cost_unconfigured';index"`
	ValidFrom            int64   `json:"valid_from" gorm:"bigint;not null;default:0;index"`
	ExpireAt             int64   `json:"expire_at" gorm:"bigint;not null;default:0;index"`
	ResetCycle           string  `json:"reset_cycle" gorm:"type:varchar(32);not null;default:'none';index"`
	WindowStartedAt      int64   `json:"window_started_at" gorm:"bigint;not null;default:0;index"`
	WindowRemaining      float64 `json:"window_remaining" gorm:"type:double precision;not null;default:0"`
	SourceSnapshotId     string  `json:"source_snapshot_id" gorm:"type:char(36);not null;default:'';index"`
	SourceSnapshotItemId string  `json:"source_snapshot_item_id" gorm:"type:char(36);not null;default:'';index"`
	SourceRef            string  `json:"source_ref" gorm:"type:varchar(191);not null;default:'';index"`
	Metadata             string  `json:"metadata" gorm:"type:text"`
	CreatedAt            int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt            int64   `json:"updated_at" gorm:"bigint;index"`
}

func (ChannelProcurementBatch) TableName() string {
	return ChannelProcurementBatchesTableName
}

type RequestProcurementConsumption struct {
	Id                  string  `json:"id" gorm:"type:char(36);primaryKey"`
	RequestLogId        string  `json:"request_log_id" gorm:"type:char(36);not null;index"`
	ChannelId           string  `json:"channel_id" gorm:"type:char(36);not null;index"`
	ProcurementBatchId  string  `json:"procurement_batch_id" gorm:"type:char(36);not null;index"`
	ResourceType        string  `json:"resource_type" gorm:"type:varchar(32);not null;default:'';index"`
	QuotaType           string  `json:"quota_type" gorm:"type:varchar(32);not null;default:'';index"`
	ScopeType           string  `json:"scope_type" gorm:"type:varchar(32);not null;default:'global';index"`
	ScopeValue          string  `json:"scope_value" gorm:"type:varchar(191);not null;default:'';index"`
	CapacityUnit        string  `json:"capacity_unit" gorm:"type:varchar(32);not null;default:'';index"`
	ConsumedQuantity    float64 `json:"consumed_quantity" gorm:"type:double precision;not null;default:0"`
	UnitCostAmount      float64 `json:"unit_cost_amount" gorm:"type:double precision;not null;default:0"`
	ConsumedCostAmount  float64 `json:"consumed_cost_amount" gorm:"type:double precision;not null;default:0"`
	SettlementTruthMode string  `json:"settlement_truth_mode" gorm:"type:varchar(64);not null;default:'';index"`
	CostSource          string  `json:"cost_source" gorm:"type:varchar(32);not null;default:'';index"`
	CreatedAt           int64   `json:"created_at" gorm:"bigint;index"`
}

func (RequestProcurementConsumption) TableName() string {
	return RequestProcurementConsumptionsTableName
}

type ProcurementConsumeInput struct {
	RequestLogID        string
	ChannelID           string
	ScopeType           string
	ScopeValue          string
	CapacityUnit        string
	Quantity            float64
	SettlementTruthMode string
}

type ProcurementConsumeResult struct {
	Consumptions    []RequestProcurementConsumption
	TotalCostAmount float64
	CostSource      string
}

type ProcurementEstimateResult struct {
	TotalCostAmount float64
	CostSource      string
	CoveredQuantity float64
	MissingQuantity float64
}

// ValidateChannelModelProcurementCostReadyWithDB ensures a model has a
// production-grade cost basis before it can be published. Estimated costs are
// useful for planning, but only actual or explicitly zero-cost batches are
// allowed to support production profitability reporting.
func ValidateChannelModelProcurementCostReadyWithDB(db *gorm.DB, row ChannelModel) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	channelID := strings.TrimSpace(row.ChannelId)
	modelName := strings.TrimSpace(row.Model)
	if channelID == "" || modelName == "" {
		return fmt.Errorf("渠道和模型不能为空")
	}
	pricing := resolvedPricingFromChannelModelRow(row)
	capacityUnits := []string{normalizePricingCapacityUnit(pricing.PriceUnit)}
	if currency := strings.TrimSpace(strings.ToLower(pricing.Currency)); currency != "" {
		capacityUnits = append(capacityUnits, currency+"_equivalent")
	}
	capacityUnits = normalizeTrimmedValuesPreserveOrder(capacityUnits)
	now := helper.GetTimestamp()
	count := int64(0)
	err := db.Model(&ChannelProcurementBatch{}).
		Where("channel_id = ?", channelID).
		Where("capacity_unit IN ?", capacityUnits).
		Where("cost_status = ?", ProcurementCostStatusActive).
		Where("cost_source IN ?", []string{ProcurementCostSourceActual, ProcurementCostSourceZeroCost}).
		Where("capacity_remaining > 0").
		Where("(valid_from = 0 OR valid_from <= ?)", now).
		Where("(expire_at = 0 OR expire_at > ?)", now).
		Where("(scope_type = ? OR (scope_type = ? AND scope_value = ?))", "global", "model", modelName).
		Where("cost_source = ? OR cost_per_unit_amount > 0", ProcurementCostSourceZeroCost).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count <= 0 {
		return fmt.Errorf("模型 %s 缺少正式采购成本：请配置有效的实际成本或明确零成本采购批次，且容量单位必须匹配 %s", modelName, strings.Join(capacityUnits, " / "))
	}
	return nil
}

type ProcurementBatchCostUpdate struct {
	PurchaseCurrency   string
	PurchaseAmount     float64
	PurchaseFXRate     float64
	PurchaseCostAmount float64
	CapacityEffective  float64
	CostSource         string
	CostStatus         string
	ScopeType          string
	ScopeValue         string
}

type ProcurementBatchStatusUpdate struct {
	CostStatus string
}

func procurementBatchCapacityFromSnapshotItem(item ChannelBillingSnapshotItem) (float64, float64) {
	capacityTotal := item.LimitAmount
	if capacityTotal <= 0 {
		capacityTotal = item.Amount
	}
	if capacityTotal <= 0 {
		capacityTotal = item.RemainingAmount
	}
	capacityRemaining := item.RemainingAmount
	if capacityRemaining <= 0 && item.UsedAmount == 0 && item.Amount > 0 {
		capacityRemaining = item.Amount
	}
	if capacityRemaining > capacityTotal && capacityTotal > 0 {
		capacityRemaining = capacityTotal
	}
	return capacityTotal, capacityRemaining
}

func procurementCycleSeconds(quotaType string) int64 {
	switch strings.TrimSpace(strings.ToLower(quotaType)) {
	case "daily":
		return 24 * 60 * 60
	case "weekly":
		return 7 * 24 * 60 * 60
	case "monthly":
		return 30 * 24 * 60 * 60
	default:
		return 0
	}
}

func procurementBatchCapacityFromSnapshot(snapshot ChannelBillingSnapshot, item ChannelBillingSnapshotItem) (float64, float64) {
	capacityTotal, capacityRemaining := procurementBatchCapacityFromSnapshotItem(item)
	if item.ResourceType != ChannelBillingResourceTypeQuota || item.ResetAt > 0 || item.ExpiresAt <= 0 {
		return capacityTotal, capacityRemaining
	}
	cycleSeconds := procurementCycleSeconds(item.QuotaType)
	if cycleSeconds <= 0 || capacityTotal <= 0 {
		return capacityTotal, capacityRemaining
	}
	validFrom := snapshot.PurchaseAt
	if validFrom <= 0 {
		validFrom = snapshot.CreatedAt
	}
	if validFrom <= 0 {
		validFrom = item.CreatedAt
	}
	if validFrom <= 0 || item.ExpiresAt <= validFrom {
		return capacityTotal, capacityRemaining
	}
	cycles := math.Ceil(float64(item.ExpiresAt-validFrom) / float64(cycleSeconds))
	if cycles <= 1 {
		return capacityTotal, capacityRemaining
	}
	effectiveTotal := capacityTotal * cycles
	return effectiveTotal, effectiveTotal
}

func procurementBatchCapacityUnitFromSnapshotItem(item ChannelBillingSnapshotItem) string {
	resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
	currency := strings.TrimSpace(strings.ToLower(item.Currency))
	switch resourceType {
	case ChannelBillingResourceTypeBalance, ChannelBillingResourceTypeCredit:
		if currency != "" {
			return currency + "_equivalent"
		}
		return "currency_equivalent"
	case ChannelBillingResourceTypeQuota:
		if currency != "" {
			return currency + "_equivalent"
		}
		return "quota"
	default:
		return ""
	}
}

func procurementBatchResetCycleFromSnapshotItem(item ChannelBillingSnapshotItem) string {
	switch strings.TrimSpace(strings.ToLower(item.QuotaType)) {
	case "daily":
		return "daily"
	case "weekly":
		return "weekly"
	case "monthly":
		return "monthly"
	default:
		return "none"
	}
}

func procurementBatchExpireAtFromSnapshotItem(item ChannelBillingSnapshotItem) int64 {
	if item.ExpiresAt > 0 {
		return item.ExpiresAt
	}
	if item.ResetAt > 0 {
		return item.ResetAt
	}
	return 0
}

func procurementBatchExpireAtFromSnapshot(snapshot ChannelBillingSnapshot, item ChannelBillingSnapshotItem) int64 {
	if snapshot.ValidUntil > 0 && (item.ExpiresAt <= 0 || snapshot.ValidUntil < item.ExpiresAt) {
		return snapshot.ValidUntil
	}
	if item.ExpiresAt > 0 {
		return item.ExpiresAt
	}
	if item.ResetAt > 0 {
		return item.ResetAt
	}
	return 0
}

func BuildProcurementBatchFromBillingSnapshotItem(snapshot ChannelBillingSnapshot, item ChannelBillingSnapshotItem) (ChannelProcurementBatch, bool) {
	normalizedItems := NormalizeChannelBillingSnapshotItems([]ChannelBillingSnapshotItem{item})
	if len(normalizedItems) == 0 {
		return ChannelProcurementBatch{}, false
	}
	normalizedItem := normalizedItems[0]
	if normalizedItem.ResourceType == ChannelBillingResourceTypePlan {
		return ChannelProcurementBatch{}, false
	}
	if normalizedItem.Status == ChannelBillingItemStatusExpired || normalizedItem.Status == ChannelBillingItemStatusDepleted {
		return ChannelProcurementBatch{}, false
	}
	capacityTotal, capacityRemaining := procurementBatchCapacityFromSnapshot(snapshot, normalizedItem)
	if capacityRemaining <= 0 {
		return ChannelProcurementBatch{}, false
	}
	capacityUnit := procurementBatchCapacityUnitFromSnapshotItem(normalizedItem)
	if capacityUnit == "" {
		return ChannelProcurementBatch{}, false
	}
	sourceSnapshotID := strings.TrimSpace(snapshot.Id)
	if sourceSnapshotID == "" {
		sourceSnapshotID = strings.TrimSpace(normalizedItem.SnapshotId)
	}
	channelID := strings.TrimSpace(snapshot.ChannelId)
	if channelID == "" {
		channelID = strings.TrimSpace(normalizedItem.ChannelId)
	}
	validFrom := snapshot.ValidFrom
	if validFrom == 0 {
		validFrom = snapshot.PurchaseAt
	}
	if validFrom == 0 {
		validFrom = snapshot.CreatedAt
	}
	if validFrom == 0 {
		validFrom = normalizedItem.CreatedAt
	}
	row := ChannelProcurementBatch{
		ChannelId:            channelID,
		ResourceType:         normalizedItem.ResourceType,
		QuotaType:            normalizedItem.QuotaType,
		ScopeType:            "global",
		ScopeValue:           "",
		CapacityUnit:         capacityUnit,
		CapacityTotal:        capacityTotal,
		CapacityEffective:    capacityTotal,
		CapacityRemaining:    capacityRemaining,
		PurchaseCurrency:     "",
		PurchaseAmount:       0,
		PurchaseFXRate:       0,
		PurchaseCostAmount:   0,
		CostPerUnitAmount:    0,
		CostSource:           ProcurementCostSourceNone,
		CostStatus:           ProcurementCostStatusCostUnconfigured,
		ValidFrom:            validFrom,
		ExpireAt:             procurementBatchExpireAtFromSnapshot(snapshot, normalizedItem),
		ResetCycle:           procurementBatchResetCycleFromSnapshotItem(normalizedItem),
		SourceSnapshotId:     sourceSnapshotID,
		SourceSnapshotItemId: strings.TrimSpace(normalizedItem.Id),
		SourceRef:            strings.TrimSpace(normalizedItem.SourceRef),
		Metadata:             strings.TrimSpace(normalizedItem.Metadata),
		CreatedAt:            normalizedItem.CreatedAt,
	}
	if row.ResetCycle != "none" {
		row.WindowStartedAt = procurementWindowStart(row.ResetCycle, validFrom)
		row.WindowRemaining = capacityTotal
	}
	normalizeProcurementBatchRow(&row)
	if row.ChannelId == "" || row.CapacityUnit == "" || row.SourceSnapshotId == "" || row.SourceSnapshotItemId == "" {
		return ChannelProcurementBatch{}, false
	}
	return row, true
}

func CreateProcurementBatchesFromBillingSnapshotItemsWithDB(db *gorm.DB, snapshot ChannelBillingSnapshot, items []ChannelBillingSnapshotItem) ([]ChannelProcurementBatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if len(items) == 0 {
		return []ChannelProcurementBatch{}, nil
	}
	if !db.Migrator().HasTable(&ChannelProcurementBatch{}) {
		if err := ensureProcurementCostTablesWithDB(db); err != nil {
			return nil, err
		}
	}
	rows := make([]ChannelProcurementBatch, 0, len(items))
	now := helper.GetTimestamp()
	for _, item := range items {
		row, ok := BuildProcurementBatchFromBillingSnapshotItem(snapshot, item)
		if !ok {
			continue
		}
		if row.Id == "" {
			row.Id = random.GetUUID()
		}
		if row.CreatedAt == 0 {
			row.CreatedAt = now
		}
		row.UpdatedAt = now
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return []ChannelProcurementBatch{}, nil
	}
	if err := db.Create(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func normalizeProcurementCostSource(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ProcurementCostSourceActual:
		return ProcurementCostSourceActual
	case ProcurementCostSourceEstimated:
		return ProcurementCostSourceEstimated
	case ProcurementCostSourceZeroCost:
		return ProcurementCostSourceZeroCost
	case ProcurementCostSourceNone:
		return ProcurementCostSourceNone
	default:
		return ""
	}
}

func normalizeProcurementCostStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ProcurementCostStatusActive:
		return ProcurementCostStatusActive
	case ProcurementCostStatusCostUnconfigured:
		return ProcurementCostStatusCostUnconfigured
	case ProcurementCostStatusExhausted:
		return ProcurementCostStatusExhausted
	case ProcurementCostStatusExpired:
		return ProcurementCostStatusExpired
	case ProcurementCostStatusDisabled:
		return ProcurementCostStatusDisabled
	default:
		return ProcurementCostStatusCostUnconfigured
	}
}

func normalizeProcurementScopeType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "global":
		return "global"
	case "model":
		return "model"
	default:
		return ""
	}
}

func normalizeProcurementBatchRow(row *ChannelProcurementBatch) {
	if row == nil {
		return
	}
	row.Id = strings.TrimSpace(row.Id)
	if row.Id == "" {
		row.Id = random.GetUUID()
	}
	row.ChannelId = strings.TrimSpace(row.ChannelId)
	row.ResourceType = strings.TrimSpace(strings.ToLower(row.ResourceType))
	row.QuotaType = strings.TrimSpace(strings.ToLower(row.QuotaType))
	row.ScopeType = normalizeProcurementScopeType(row.ScopeType)
	if row.ScopeType == "" {
		row.ScopeType = "global"
	}
	row.ScopeValue = strings.TrimSpace(row.ScopeValue)
	row.CapacityUnit = strings.TrimSpace(strings.ToLower(row.CapacityUnit))
	row.PurchaseCurrency = strings.TrimSpace(strings.ToUpper(row.PurchaseCurrency))
	row.CostSource = normalizeProcurementCostSource(row.CostSource)
	row.CostStatus = normalizeProcurementCostStatus(row.CostStatus)
	row.ResetCycle = strings.TrimSpace(strings.ToLower(row.ResetCycle))
	if row.ResetCycle == "" {
		row.ResetCycle = "none"
	}
	row.SourceSnapshotId = strings.TrimSpace(row.SourceSnapshotId)
	row.SourceSnapshotItemId = strings.TrimSpace(row.SourceSnapshotItemId)
	row.SourceRef = strings.TrimSpace(row.SourceRef)
	if row.CapacityTotal < 0 {
		row.CapacityTotal = 0
	}
	if row.CapacityEffective < 0 {
		row.CapacityEffective = 0
	}
	if row.CapacityRemaining < 0 {
		row.CapacityRemaining = 0
	}
	if row.PurchaseAmount < 0 {
		row.PurchaseAmount = 0
	}
	if row.PurchaseFXRate < 0 {
		row.PurchaseFXRate = 0
	}
	if row.PurchaseCostAmount < 0 {
		row.PurchaseCostAmount = 0
	}
	if row.CostPerUnitAmount < 0 {
		row.CostPerUnitAmount = 0
	}
	if row.WindowRemaining < 0 {
		row.WindowRemaining = 0
	}
}

func procurementWindowStart(cycle string, timestamp int64) int64 {
	if timestamp <= 0 {
		timestamp = helper.GetTimestamp()
	}
	t := time.Unix(timestamp, 0).UTC()
	switch strings.TrimSpace(strings.ToLower(cycle)) {
	case "daily":
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix()
	case "weekly":
		daysSinceMonday := (int(t.Weekday()) + 6) % 7
		monday := t.AddDate(0, 0, -daysSinceMonday)
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC).Unix()
	case "monthly":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Unix()
	default:
		return 0
	}
}

func procurementBatchAvailableAt(row ChannelProcurementBatch, now int64) (float64, int64, float64) {
	available := row.CapacityRemaining
	if row.ResetCycle == "none" || row.ResetCycle == "" {
		return available, 0, 0
	}
	windowStart := procurementWindowStart(row.ResetCycle, now)
	windowRemaining := row.WindowRemaining
	if row.WindowStartedAt != windowStart {
		windowRemaining = row.CapacityTotal
	}
	return math.Min(available, windowRemaining), windowStart, windowRemaining
}

func CreateChannelProcurementBatchWithDB(db *gorm.DB, row ChannelProcurementBatch) (ChannelProcurementBatch, error) {
	if db == nil {
		return ChannelProcurementBatch{}, fmt.Errorf("database handle is nil")
	}
	normalized := row
	normalizeProcurementBatchRow(&normalized)
	if normalized.ChannelId == "" {
		return ChannelProcurementBatch{}, fmt.Errorf("channel_id 不能为空")
	}
	if normalized.CapacityUnit == "" {
		return ChannelProcurementBatch{}, fmt.Errorf("capacity_unit 不能为空")
	}
	now := helper.GetTimestamp()
	if normalized.CreatedAt == 0 {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now
	if err := db.Create(&normalized).Error; err != nil {
		return ChannelProcurementBatch{}, err
	}
	return normalized, nil
}

func ListChannelProcurementBatchesByChannelIDWithDB(db *gorm.DB, channelID string, limit int) ([]ChannelProcurementBatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return []ChannelProcurementBatch{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows := make([]ChannelProcurementBatch, 0, limit)
	if err := db.Where("channel_id = ?", normalizedChannelID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListProcurementBatchesWithDB(db *gorm.DB, limit int) ([]ChannelProcurementBatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if limit <= 0 {
		limit = 1000
	}
	rows := make([]ChannelProcurementBatch, 0, limit)
	if err := db.Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListChannelProcurementBatchesBySourceSnapshotIDWithDB(db *gorm.DB, snapshotID string) ([]ChannelProcurementBatch, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedSnapshotID := strings.TrimSpace(snapshotID)
	if normalizedSnapshotID == "" {
		return []ChannelProcurementBatch{}, nil
	}
	if !db.Migrator().HasTable(&ChannelProcurementBatch{}) {
		if err := ensureProcurementCostTablesWithDB(db); err != nil {
			return nil, err
		}
	}
	rows := make([]ChannelProcurementBatch, 0)
	if err := db.Where("source_snapshot_id = ?", normalizedSnapshotID).
		Order("created_at asc, id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetChannelProcurementBatchByIDWithDB(db *gorm.DB, id string) (ChannelProcurementBatch, error) {
	if db == nil {
		return ChannelProcurementBatch{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return ChannelProcurementBatch{}, gorm.ErrRecordNotFound
	}
	if !db.Migrator().HasTable(&ChannelProcurementBatch{}) {
		if err := ensureProcurementCostTablesWithDB(db); err != nil {
			return ChannelProcurementBatch{}, err
		}
	}
	row := ChannelProcurementBatch{}
	err := db.Where("id = ?", normalizedID).Take(&row).Error
	return row, err
}

func CountRequestProcurementConsumptionsByBatchIDsWithDB(db *gorm.DB, batchIDs []string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	normalizedBatchIDs := normalizeTrimmedValuesPreserveOrder(batchIDs)
	if len(normalizedBatchIDs) == 0 {
		return 0, nil
	}
	var count int64
	if err := db.Model(&RequestProcurementConsumption{}).
		Where("procurement_batch_id IN ?", normalizedBatchIDs).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(db *gorm.DB, snapshotID string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&RequestProcurementConsumption{}) || !db.Migrator().HasTable(&ChannelProcurementBatch{}) {
		if err := ensureProcurementCostTablesWithDB(db); err != nil {
			return 0, err
		}
	}
	rows, err := ListChannelProcurementBatchesBySourceSnapshotIDWithDB(db, snapshotID)
	if err != nil {
		return 0, err
	}
	batchIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Id) != "" {
			batchIDs = append(batchIDs, row.Id)
		}
	}
	return CountRequestProcurementConsumptionsByBatchIDsWithDB(db, batchIDs)
}

func UpdateChannelProcurementBatchCostWithDB(db *gorm.DB, id string, input ProcurementBatchCostUpdate) (ChannelProcurementBatch, error) {
	if db == nil {
		return ChannelProcurementBatch{}, fmt.Errorf("database handle is nil")
	}
	row, err := GetChannelProcurementBatchByIDWithDB(db, id)
	if err != nil {
		return ChannelProcurementBatch{}, err
	}
	purchaseCurrency := strings.TrimSpace(strings.ToUpper(input.PurchaseCurrency))
	purchaseAmount := input.PurchaseAmount
	purchaseFXRate := input.PurchaseFXRate
	purchaseCostAmount := input.PurchaseCostAmount
	capacityEffective := input.CapacityEffective
	costSource := normalizeProcurementCostSource(input.CostSource)
	costStatus := normalizeProcurementCostStatus(input.CostStatus)
	scopeType := normalizeProcurementScopeType(input.ScopeType)
	scopeValue := strings.TrimSpace(input.ScopeValue)
	if costSource == "" || costSource == ProcurementCostSourceNone {
		costSource = ProcurementCostSourceActual
	}
	if costStatus == "" || costStatus == ProcurementCostStatusCostUnconfigured {
		costStatus = ProcurementCostStatusActive
	}
	if purchaseAmount < 0 || purchaseFXRate < 0 || purchaseCostAmount < 0 || capacityEffective < 0 {
		return ChannelProcurementBatch{}, fmt.Errorf("采购成本参数不能小于 0")
	}
	if scopeType == "" {
		return ChannelProcurementBatch{}, fmt.Errorf("采购范围无效")
	}
	if scopeType == "global" {
		scopeValue = ""
	}
	if scopeType == "model" && scopeValue == "" {
		return ChannelProcurementBatch{}, fmt.Errorf("模型范围必须填写模型名称")
	}
	if purchaseCostAmount <= 0 && purchaseAmount > 0 && purchaseFXRate > 0 {
		purchaseCostAmount = purchaseAmount * purchaseFXRate
	}
	if capacityEffective <= 0 {
		capacityEffective = row.CapacityEffective
	}
	if capacityEffective <= 0 {
		capacityEffective = row.CapacityTotal
	}
	if costSource != ProcurementCostSourceZeroCost && purchaseCostAmount <= 0 {
		return ChannelProcurementBatch{}, fmt.Errorf("采购成本必须大于 0")
	}
	if capacityEffective <= 0 {
		return ChannelProcurementBatch{}, fmt.Errorf("有效容量必须大于 0")
	}
	if row.CapacityRemaining > capacityEffective {
		return ChannelProcurementBatch{}, fmt.Errorf("有效容量不能小于当前剩余容量")
	}
	costPerUnitAmount := 0.0
	if costSource != ProcurementCostSourceZeroCost {
		costPerUnitAmount = purchaseCostAmount / capacityEffective
	}
	now := helper.GetTimestamp()
	updates := map[string]any{
		"purchase_currency":    purchaseCurrency,
		"purchase_amount":      purchaseAmount,
		"purchase_fx_rate":     purchaseFXRate,
		"purchase_cost_amount": purchaseCostAmount,
		"capacity_effective":   capacityEffective,
		"cost_per_unit_amount": costPerUnitAmount,
		"cost_source":          costSource,
		"cost_status":          costStatus,
		"scope_type":           scopeType,
		"scope_value":          scopeValue,
		"updated_at":           now,
	}
	if err := db.Model(&ChannelProcurementBatch{}).
		Where("id = ?", strings.TrimSpace(row.Id)).
		Updates(updates).Error; err != nil {
		return ChannelProcurementBatch{}, err
	}
	return GetChannelProcurementBatchByIDWithDB(db, row.Id)
}

func UpdateChannelProcurementBatchStatusWithDB(db *gorm.DB, id string, input ProcurementBatchStatusUpdate) (ChannelProcurementBatch, error) {
	if db == nil {
		return ChannelProcurementBatch{}, fmt.Errorf("database handle is nil")
	}
	row, err := GetChannelProcurementBatchByIDWithDB(db, id)
	if err != nil {
		return ChannelProcurementBatch{}, err
	}
	nextStatus := normalizeProcurementCostStatus(input.CostStatus)
	switch nextStatus {
	case ProcurementCostStatusDisabled:
	case ProcurementCostStatusActive:
		if row.CapacityRemaining <= 0 {
			return ChannelProcurementBatch{}, fmt.Errorf("采购批次剩余容量不足，不能恢复")
		}
		if row.CostSource != ProcurementCostSourceActual && row.CostSource != ProcurementCostSourceEstimated && row.CostSource != ProcurementCostSourceZeroCost {
			return ChannelProcurementBatch{}, fmt.Errorf("采购批次成本未配置，不能恢复")
		}
		if row.CostSource != ProcurementCostSourceZeroCost && row.CostPerUnitAmount <= 0 {
			return ChannelProcurementBatch{}, fmt.Errorf("采购批次单位成本未配置，不能恢复")
		}
	default:
		return ChannelProcurementBatch{}, fmt.Errorf("采购批次状态无效")
	}
	if err := db.Model(&ChannelProcurementBatch{}).
		Where("id = ?", strings.TrimSpace(row.Id)).
		Updates(map[string]any{
			"cost_status": nextStatus,
			"updated_at":  helper.GetTimestamp(),
		}).Error; err != nil {
		return ChannelProcurementBatch{}, err
	}
	return GetChannelProcurementBatchByIDWithDB(db, row.Id)
}

func ListRequestProcurementConsumptionsByBatchIDWithDB(db *gorm.DB, batchID string, limit int) ([]RequestProcurementConsumption, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	if normalizedBatchID == "" {
		return []RequestProcurementConsumption{}, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows := make([]RequestProcurementConsumption, 0, limit)
	if err := db.Where("procurement_batch_id = ?", normalizedBatchID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ConsumeChannelProcurementBatches(input ProcurementConsumeInput) (ProcurementConsumeResult, error) {
	return ConsumeChannelProcurementBatchesWithDB(DB, input)
}

func EstimateChannelProcurementCost(input ProcurementConsumeInput) (ProcurementEstimateResult, error) {
	return EstimateChannelProcurementCostWithDB(DB, input)
}

type procurementConstraintGroup struct {
	Rows      []ChannelProcurementBatch
	Available float64
	CostRow   int
}

func buildProcurementConstraintGroups(rows []ChannelProcurementBatch, now int64) []procurementConstraintGroup {
	groups := make([]procurementConstraintGroup, 0, len(rows))
	groupIndex := make(map[string]int)
	for _, row := range rows {
		key := strings.TrimSpace(row.SourceSnapshotId)
		if key == "" {
			key = "batch:" + row.Id
		}
		key = strings.Join([]string{key, row.ScopeType, row.ScopeValue, row.CapacityUnit}, "|")
		index, ok := groupIndex[key]
		if !ok {
			index = len(groups)
			groupIndex[key] = index
			groups = append(groups, procurementConstraintGroup{Available: math.MaxFloat64, CostRow: -1})
		}
		available, _, _ := procurementBatchAvailableAt(row, now)
		group := &groups[index]
		group.Rows = append(group.Rows, row)
		group.Available = math.Min(group.Available, available)
		rowIndex := len(group.Rows) - 1
		if group.CostRow < 0 || row.QuotaType == "total" || row.QuotaType == "custom" || row.ResetCycle == "none" {
			group.CostRow = rowIndex
		}
	}
	return groups
}

func EstimateChannelProcurementCostWithDB(db *gorm.DB, input ProcurementConsumeInput) (ProcurementEstimateResult, error) {
	if db == nil {
		return ProcurementEstimateResult{}, fmt.Errorf("database handle is nil")
	}
	normalizedChannelID := strings.TrimSpace(input.ChannelID)
	normalizedScopeType := normalizeProcurementScopeType(input.ScopeType)
	normalizedScopeValue := strings.TrimSpace(input.ScopeValue)
	normalizedCapacityUnit := strings.TrimSpace(strings.ToLower(input.CapacityUnit))
	if normalizedChannelID == "" || normalizedCapacityUnit == "" || input.Quantity <= 0 {
		return ProcurementEstimateResult{CostSource: ProcurementCostSourceNone, MissingQuantity: math.Max(input.Quantity, 0)}, nil
	}
	rows := make([]ChannelProcurementBatch, 0)
	now := helper.GetTimestamp()
	query := db.
		Where("channel_id = ?", normalizedChannelID).
		Where("capacity_unit = ?", normalizedCapacityUnit).
		Where("cost_status = ?", ProcurementCostStatusActive).
		Where("cost_source IN ?", []string{ProcurementCostSourceActual, ProcurementCostSourceEstimated, ProcurementCostSourceZeroCost}).
		Where("capacity_remaining > 0").
		Where("(valid_from = 0 OR valid_from <= ?)", now).
		Where("(expire_at = 0 OR expire_at > ?)", now)
	query = query.Where("(scope_type = ? OR (scope_type = ? AND scope_value = ?))", "global", normalizedScopeType, normalizedScopeValue)
	if err := query.Order(procurementBatchConsumeOrderSQL()).Find(&rows).Error; err != nil {
		return ProcurementEstimateResult{}, err
	}
	result := ProcurementEstimateResult{MissingQuantity: input.Quantity}
	remaining := input.Quantity
	for _, group := range buildProcurementConstraintGroups(rows, now) {
		if remaining <= 0 {
			break
		}
		quantity := math.Min(remaining, group.Available)
		if quantity <= 0 {
			continue
		}
		row := group.Rows[group.CostRow]
		result.CoveredQuantity += quantity
		result.TotalCostAmount += quantity * row.CostPerUnitAmount
		if result.CostSource == "" {
			result.CostSource = row.CostSource
		} else if result.CostSource != row.CostSource {
			result.CostSource = ProcurementCostSourceEstimated
		}
		remaining -= quantity
	}
	if result.CoveredQuantity <= 0 {
		result.CostSource = ProcurementCostSourceNone
		result.MissingQuantity = input.Quantity
		return result, nil
	}
	result.MissingQuantity = math.Max(remaining, 0)
	return result, nil
}

func procurementBatchConsumeOrderSQL() string {
	return "CASE WHEN scope_type = 'model' THEN 0 ELSE 1 END ASC, CASE WHEN expire_at = 0 THEN 1 ELSE 0 END ASC, expire_at ASC, cost_per_unit_amount ASC, created_at ASC"
}

func ConsumeChannelProcurementBatchesWithDB(db *gorm.DB, input ProcurementConsumeInput) (ProcurementConsumeResult, error) {
	if db == nil {
		return ProcurementConsumeResult{}, fmt.Errorf("database handle is nil")
	}
	normalizedRequestLogID := strings.TrimSpace(input.RequestLogID)
	normalizedChannelID := strings.TrimSpace(input.ChannelID)
	normalizedScopeType := normalizeProcurementScopeType(input.ScopeType)
	normalizedScopeValue := strings.TrimSpace(input.ScopeValue)
	normalizedCapacityUnit := strings.TrimSpace(strings.ToLower(input.CapacityUnit))
	if normalizedRequestLogID == "" || normalizedChannelID == "" || normalizedCapacityUnit == "" || input.Quantity <= 0 {
		return ProcurementConsumeResult{}, nil
	}

	result := ProcurementConsumeResult{}
	now := helper.GetTimestamp()
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("channel_id = ?", normalizedChannelID).
			Where("capacity_unit = ?", normalizedCapacityUnit).
			Where("cost_status = ?", ProcurementCostStatusActive).
			Where("cost_source IN ?", []string{ProcurementCostSourceActual, ProcurementCostSourceEstimated, ProcurementCostSourceZeroCost}).
			Where("capacity_remaining > 0").
			Where("(valid_from = 0 OR valid_from <= ?)", now).
			Where("(expire_at = 0 OR expire_at > ?)", now)
		query = query.Where("(scope_type = ? OR (scope_type = ? AND scope_value = ?))", "global", normalizedScopeType, normalizedScopeValue)
		rows := make([]ChannelProcurementBatch, 0)
		if err := query.Order(procurementBatchConsumeOrderSQL()).Find(&rows).Error; err != nil {
			return err
		}
		remaining := input.Quantity
		for _, group := range buildProcurementConstraintGroups(rows, now) {
			if remaining <= 0 {
				break
			}
			consumeQuantity := math.Min(remaining, group.Available)
			if consumeQuantity <= 0 {
				continue
			}
			costRow := group.Rows[group.CostRow]
			for rowIndex, row := range group.Rows {
				_, windowStart, windowRemaining := procurementBatchAvailableAt(row, now)
				nextRemaining := math.Max(row.CapacityRemaining-consumeQuantity, 0)
				nextStatus := row.CostStatus
				if nextRemaining <= 0 {
					nextStatus = ProcurementCostStatusExhausted
				}
				updates := map[string]any{
					"capacity_remaining": nextRemaining,
					"cost_status":        nextStatus,
					"updated_at":         now,
				}
				if row.ResetCycle != "none" && row.ResetCycle != "" {
					updates["window_started_at"] = windowStart
					updates["window_remaining"] = math.Max(windowRemaining-consumeQuantity, 0)
				}
				if err := tx.Model(&ChannelProcurementBatch{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
					return err
				}
				unitCost := 0.0
				if rowIndex == group.CostRow {
					unitCost = costRow.CostPerUnitAmount
				}
				consumption := RequestProcurementConsumption{
					Id:                  random.GetUUID(),
					RequestLogId:        normalizedRequestLogID,
					ChannelId:           normalizedChannelID,
					ProcurementBatchId:  row.Id,
					ResourceType:        row.ResourceType,
					QuotaType:           row.QuotaType,
					ScopeType:           row.ScopeType,
					ScopeValue:          row.ScopeValue,
					CapacityUnit:        row.CapacityUnit,
					ConsumedQuantity:    consumeQuantity,
					UnitCostAmount:      unitCost,
					ConsumedCostAmount:  consumeQuantity * unitCost,
					SettlementTruthMode: strings.TrimSpace(input.SettlementTruthMode),
					CostSource:          row.CostSource,
					CreatedAt:           now,
				}
				if err := tx.Create(&consumption).Error; err != nil {
					return err
				}
				result.Consumptions = append(result.Consumptions, consumption)
			}
			result.TotalCostAmount += consumeQuantity * costRow.CostPerUnitAmount
			if result.CostSource == "" {
				result.CostSource = costRow.CostSource
			} else if result.CostSource != costRow.CostSource {
				result.CostSource = ProcurementCostSourceEstimated
			}
			remaining -= consumeQuantity
		}
		return nil
	})
	if err != nil {
		return ProcurementConsumeResult{}, err
	}
	if len(result.Consumptions) == 0 {
		result.CostSource = ProcurementCostSourceNone
	}
	return result, nil
}

func UpdateLogProcurementCostObservation(logID string, costBaseAmount float64, costSource string, sellBaseAmount float64) error {
	return UpdateLogProcurementCostObservationWithDB(LOG_DB, logID, costBaseAmount, costSource, sellBaseAmount)
}

func UpdateLogProcurementCostObservationWithDB(db *gorm.DB, logID string, costBaseAmount float64, costSource string, sellBaseAmount float64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	normalizedLogID := strings.TrimSpace(logID)
	normalizedCostSource := normalizeProcurementCostSource(costSource)
	if normalizedLogID == "" || normalizedCostSource == "" || normalizedCostSource == ProcurementCostSourceNone {
		return nil
	}
	updates := map[string]any{
		"billing_procurement_cost_base_amount": costBaseAmount,
		"billing_procurement_cost_source":      normalizedCostSource,
	}
	if sellBaseAmount > 0 {
		grossProfit := sellBaseAmount - costBaseAmount
		updates["billing_gross_profit_base_amount"] = grossProfit
		updates["billing_gross_margin"] = grossProfit / sellBaseAmount
	}
	return db.Model(&Log{}).Where("id = ?", normalizedLogID).Updates(updates).Error
}
