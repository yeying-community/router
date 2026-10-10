package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	channelsvc "github.com/yeying-community/router/internal/admin/service/channel"
	"gorm.io/gorm"
)

type channelBillingSummaryData struct {
	ChannelID             string                             `json:"channel_id"`
	BillingSource         string                             `json:"billing_source"`
	ActionCapabilities    []string                           `json:"action_capabilities"`
	RefreshSupported      bool                               `json:"refresh_supported"`
	LatestSnapshotAt      int64                              `json:"latest_snapshot_at"`
	LatestSnapshotStatus  string                             `json:"latest_snapshot_status"`
	LatestSnapshotMessage string                             `json:"latest_snapshot_message"`
	QuotaItems            []model.ChannelBillingSnapshotItem `json:"quota_items"`
}

type channelBillingProfileData struct {
	ChannelID          string            `json:"channel_id"`
	BillingSource      string            `json:"billing_source"`
	CostTrackingMode   string            `json:"cost_tracking_mode"`
	BillingCredentials map[string]string `json:"billing_credentials"`
	ActionCapabilities []string          `json:"action_capabilities"`
	// CostMissingModelCount is how many of the channel's published models have no
	// covering procurement cost under the current cost tracking mode. It is only
	// ever > 0 in actual mode (untracked resolves to untracked, free carries
	// auto-managed zero-cost batches), surfacing the downstream "cost missing"
	// signal right where cost_tracking_mode is set.
	CostMissingModelCount int `json:"cost_missing_model_count"`
}

type channelBillingListData[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type channelBillingAlertFeedItem struct {
	model.ChannelBillingAlertEvent
	ChannelName string `json:"channel_name"`
}

type channelBillingManualSnapshotRequest struct {
	PurchaseAt          int64                                  `json:"purchase_at"`
	PurchaseCurrency    string                                 `json:"purchase_currency"`
	PurchaseAmount      float64                                `json:"purchase_amount"`
	PurchaseFXRate      float64                                `json:"purchase_fx_rate"`
	PurchaseCostAmount  float64                                `json:"purchase_cost_amount"`
	EntitlementName     string                                 `json:"entitlement_name"`
	EventType           string                                 `json:"event_type"`
	ParentSnapshotID    string                                 `json:"parent_snapshot_id"`
	OldBatchDisposition string                                 `json:"old_batch_disposition"`
	ValidFrom           int64                                  `json:"valid_from"`
	ValidUntil          int64                                  `json:"valid_until"`
	Items               []channelBillingManualQuotaItemRequest `json:"items"`
	Message             string                                 `json:"message"`
}

type channelBillingManualQuotaItemRequest struct {
	Id              string  `json:"id"`
	ResourceType    string  `json:"resource_type"`
	QuotaType       string  `json:"quota_type"`
	QuotaLabel      string  `json:"quota_label"`
	Amount          float64 `json:"amount"`
	LimitAmount     float64 `json:"limit_amount"`
	UsedAmount      float64 `json:"used_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	Currency        string  `json:"currency"`
	ResetAt         int64   `json:"reset_at"`
	ExpiresAt       int64   `json:"expires_at"`
	SourceRef       string  `json:"source_ref"`
}

type channelProcurementBatchCostUpdateRequest struct {
	PurchaseCurrency   string  `json:"purchase_currency"`
	PurchaseAmount     float64 `json:"purchase_amount"`
	PurchaseFXRate     float64 `json:"purchase_fx_rate"`
	PurchaseCostAmount float64 `json:"purchase_cost_amount"`
	CapacityEffective  float64 `json:"capacity_effective"`
	CostSource         string  `json:"cost_source"`
	CostStatus         string  `json:"cost_status"`
	ScopeType          string  `json:"scope_type"`
	ScopeValue         string  `json:"scope_value"`
}

type channelProcurementBatchStatusUpdateRequest struct {
	CostStatus string `json:"cost_status"`
}

func isSupportedBillingResourceType(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case model.ChannelBillingResourceTypeQuota,
		model.ChannelBillingResourceTypeBalance,
		model.ChannelBillingResourceTypeCredit,
		model.ChannelBillingResourceTypePlan:
		return true
	default:
		return false
	}
}

type channelBillingProfileUpdateRequest struct {
	BillingSource      string            `json:"billing_source"`
	CostTrackingMode   string            `json:"cost_tracking_mode"`
	BillingCredentials map[string]string `json:"billing_credentials"`
}

func GetChannelBillingAdapters(c *gin.Context) {
	if !billingServiceConfigured() {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": channelBillingListData[billingServiceAdapterInfo]{
				Items: []billingServiceAdapterInfo{},
				Total: 0,
			},
		})
		return
	}
	items, err := listBillingServiceAdapters(c.Request.Context())
	if err != nil {
		logChannelAdminWarn(c, "list_billing_adapters", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelBillingListData[billingServiceAdapterInfo]{
			Items: items,
			Total: len(items),
		},
	})
}

func buildChannelBillingSummary(channelRow *model.Channel, profile model.ChannelBillingProfile, snapshot model.ChannelBillingSnapshot) channelBillingSummaryData {
	capabilities := profile.ParseActionCapabilities()
	summary := channelBillingSummaryData{
		ChannelID:             strings.TrimSpace(channelRow.Id),
		BillingSource:         normalizeChannelBillingSource(profile.BillingSource),
		ActionCapabilities:    capabilities,
		RefreshSupported:      profile.HasCapability(model.ChannelBillingCapabilityRefreshBilling),
		LatestSnapshotAt:      snapshot.CreatedAt,
		LatestSnapshotStatus:  strings.TrimSpace(snapshot.RawStatus),
		LatestSnapshotMessage: strings.TrimSpace(snapshot.Message),
		QuotaItems:            model.NormalizeChannelBillingSnapshotItems(snapshot.Items),
	}
	return summary
}

func buildChannelBillingProfileData(channelRow *model.Channel, profile model.ChannelBillingProfile) channelBillingProfileData {
	fetchConfig := profile.ParseBillingConfig()
	return channelBillingProfileData{
		ChannelID:          strings.TrimSpace(channelRow.Id),
		BillingSource:      normalizeChannelBillingSource(profile.BillingSource),
		CostTrackingMode:   model.NormalizeChannelCostTrackingMode(profile.CostTrackingMode),
		BillingCredentials: sanitizeBillingCredentialMap(fetchConfig.BillingCredentials),
		ActionCapabilities: profile.ParseActionCapabilities(),
	}
}

// countChannelCostMissingModels counts the channel's published models whose
// procurement readiness is neither ready nor untracked under the given cost
// tracking mode — i.e. models whose cost is unrecorded so margin cannot be
// computed. "Published" uses the persisted PublishEnabled + PublishedAt columns
// (IsChannelModelPublished relies on the transient PublishStatus, which is not
// stored on table rows). Mirrors the readiness resolution in
// buildChannelModelListData (model_list.go).
func countChannelCostMissingModels(channelID string, mode string) (int, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return 0, nil
	}
	// untracked never records cost by design, so nothing is "missing".
	if model.NormalizeChannelCostTrackingMode(mode) == model.ChannelCostTrackingModeUntracked {
		return 0, nil
	}
	rows, err := model.ListChannelModelRowsByChannelIDWithDB(model.DB, normalizedChannelID)
	if err != nil {
		return 0, err
	}
	batches, err := model.ListAllChannelProcurementBatchesByChannelIDWithDB(model.DB, normalizedChannelID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if !row.PublishEnabled || row.PublishedAt <= 0 {
			continue
		}
		readiness := model.ResolveChannelModelProcurementReadinessForMode(row, batches, mode)
		if readiness.Status != model.ProcurementReadinessReady && readiness.Status != model.ProcurementReadinessUntracked {
			count++
		}
	}
	return count, nil
}

func attachManualPurchaseCostToSnapshotBatches(tx *gorm.DB, snapshotID string, purchaseCurrency string, purchaseAmount float64, purchaseFXRate float64, purchaseCostAmount float64) error {
	normalizedSnapshotID := strings.TrimSpace(snapshotID)
	if normalizedSnapshotID == "" || purchaseCostAmount <= 0 {
		return nil
	}
	rows := make([]model.ChannelProcurementBatch, 0)
	if err := tx.Where("source_snapshot_id = ?", normalizedSnapshotID).Find(&rows).Error; err != nil {
		return err
	}
	type costGroup struct {
		rows     []model.ChannelProcurementBatch
		primary  int
		capacity float64
	}
	groups := make(map[string]*costGroup)
	groupOrder := make([]string, 0)
	for _, row := range rows {
		key := strings.Join([]string{row.ScopeType, row.ScopeValue, row.CapacityUnit}, "|")
		group := groups[key]
		if group == nil {
			group = &costGroup{primary: -1}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.rows = append(group.rows, row)
		rowIndex := len(group.rows) - 1
		if group.primary < 0 || row.QuotaType == "total" || row.QuotaType == "custom" || row.ResetCycle == "none" {
			group.primary = rowIndex
			group.capacity = row.CapacityEffective
		}
	}
	totalEffectiveCapacity := 0.0
	for _, key := range groupOrder {
		if groups[key].capacity > 0 {
			totalEffectiveCapacity += groups[key].capacity
		}
	}
	if len(rows) == 0 || totalEffectiveCapacity <= 0 {
		return nil
	}
	now := helper.GetTimestamp()
	for _, key := range groupOrder {
		group := groups[key]
		if group.primary < 0 || group.capacity <= 0 {
			continue
		}
		ratio := group.capacity / totalEffectiveCapacity
		allocatedCostAmount := purchaseCostAmount * ratio
		if allocatedCostAmount <= 0 {
			continue
		}
		costPerUnitAmount := allocatedCostAmount / group.capacity
		for rowIndex, row := range group.rows {
			rowPurchaseAmount := 0.0
			rowCostAmount := 0.0
			rowUnitCost := 0.0
			if rowIndex == group.primary {
				rowPurchaseAmount = purchaseAmount * ratio
				rowCostAmount = allocatedCostAmount
				rowUnitCost = costPerUnitAmount
			}
			if err := tx.Model(&model.ChannelProcurementBatch{}).Where("id = ?", row.Id).Updates(map[string]any{
				"purchase_currency":    strings.TrimSpace(strings.ToUpper(purchaseCurrency)),
				"purchase_amount":      rowPurchaseAmount,
				"purchase_fx_rate":     purchaseFXRate,
				"purchase_cost_amount": rowCostAmount,
				"cost_per_unit_amount": rowUnitCost,
				"cost_source":          model.ProcurementCostSourceActual,
				"cost_status":          model.ProcurementCostStatusActive,
				"updated_at":           now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func getEffectiveChannelBillingProfile(channelID string) (*model.Channel, model.ChannelBillingProfile, error) {
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		return nil, model.ChannelBillingProfile{}, err
	}
	profile, _, err := model.GetEffectiveChannelBillingProfileWithDB(model.DB, channelRow)
	if err != nil {
		return nil, model.ChannelBillingProfile{}, err
	}
	return channelRow, profile, nil
}

func GetChannelBilling(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "get_billing", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Balance/entitlement reflects the upstream account, which only exists for an
	// auto-refresh adapter source. Manual channels have no upstream balance (their
	// manual snapshots are procurement cost records, not balance), so leave the
	// summary balance empty rather than surfacing a manual snapshot as "balance".
	var latestSnapshot model.ChannelBillingSnapshot
	if normalizeChannelBillingSource(profile.BillingSource) != model.ChannelBillingSourceManual {
		latestSnapshot, err = model.GetLatestChannelBillingSnapshotBySourceWithDB(model.DB, channelID, model.ChannelBillingSnapshotSourceAPI)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logChannelAdminWarn(c, "get_billing", stringField("channel_id", channelID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildChannelBillingSummary(channelRow, profile, latestSnapshot),
	})
}

func GetChannelBillingProfile(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "get_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	data := buildChannelBillingProfileData(channelRow, profile)
	if count, countErr := countChannelCostMissingModels(channelID, data.CostTrackingMode); countErr != nil {
		logChannelAdminWarn(c, "get_billing_profile_cost_missing", stringField("channel_id", channelID), stringField("reason", countErr.Error()))
	} else {
		data.CostMissingModelCount = count
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func GetChannelBillingSnapshots(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingSnapshotsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_snapshots", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingSnapshot]{Items: rows, Total: len(rows)}})
}

func GetChannelBillingActions(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingActionsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_actions", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingAction]{Items: rows, Total: len(rows)}})
}

func GetChannelBillingAlerts(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelBillingAlertEventsByChannelIDWithDB(model.DB, channelID, 50)
	if err != nil {
		logChannelAdminWarn(c, "list_billing_alerts", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelBillingAlertEvent]{Items: rows, Total: len(rows)}})
}

// channelCostQuoteReconcileRow pairs a billing-service cost quote (normalized to
// YYC) with the model's local procurement readiness, for read-only comparison.
type channelCostQuoteReconcileRow struct {
	Model              string  `json:"model"`
	CapacityUnit       string  `json:"capacity_unit"`
	ServiceUnitCost    float64 `json:"service_unit_cost"`
	ServiceCurrency    string  `json:"service_currency"`
	ServiceUnitCostYYC float64 `json:"service_unit_cost_yyc"`
	ServiceConfidence  string  `json:"service_confidence"`
	LocalReadiness     string  `json:"local_readiness"`
}

type channelCostQuoteReconcileData struct {
	ChannelID        string                         `json:"channel_id"`
	ServiceAvailable bool                           `json:"service_available"`
	Reason           string                         `json:"reason,omitempty"`
	Rows             []channelCostQuoteReconcileRow `json:"rows"`
}

// GetChannelCostQuoteReconciliation is the P5 step-1 read-only reconciliation view:
// it fetches the billing service's normalized cost quotes and shows them next to each
// model's local procurement readiness. It NEVER changes online charging; it only
// surfaces "service cost vs local cost" so an operator can validate consistency
// before cost quotes are wired into the live cost floor.
func GetChannelCostQuoteReconciliation(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "cost_quote_reconcile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	data := channelCostQuoteReconcileData{ChannelID: channelID, Rows: []channelCostQuoteReconcileRow{}}
	quotes, quoteErr := collectBillingServiceCostQuotes(c.Request.Context(), channelRow, profile, nil)
	if quoteErr != nil {
		// Read-only view: a missing/unsupported service is informational, not a failure.
		data.ServiceAvailable = false
		data.Reason = quoteErr.Error()
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
		return
	}
	data.ServiceAvailable = true
	readiness := resolveChannelLocalReadinessByModel(channelID, profile.CostTrackingMode)
	for _, quote := range quotes.Quotes {
		modelName := strings.TrimSpace(quote.Model)
		row := channelCostQuoteReconcileRow{
			Model:             modelName,
			CapacityUnit:      strings.TrimSpace(quote.CapacityUnit),
			ServiceUnitCost:   quote.UnitCost,
			ServiceCurrency:   strings.TrimSpace(strings.ToUpper(quote.Currency)),
			ServiceConfidence: strings.TrimSpace(strings.ToLower(quote.Confidence)),
			LocalReadiness:    readiness[modelName],
		}
		if rate, rateErr := model.GetBillingCurrencyChargeRate(row.ServiceCurrency); rateErr == nil && rate > 0 {
			row.ServiceUnitCostYYC = quote.UnitCost * rate
		}
		data.Rows = append(data.Rows, row)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// resolveChannelLocalReadinessByModel maps each published model to its local
// procurement readiness status under the channel's cost tracking mode.
func resolveChannelLocalReadinessByModel(channelID string, mode string) map[string]string {
	result := map[string]string{}
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return result
	}
	rows, err := model.ListChannelModelRowsByChannelIDWithDB(model.DB, normalizedChannelID)
	if err != nil {
		return result
	}
	batches, err := model.ListAllChannelProcurementBatchesByChannelIDWithDB(model.DB, normalizedChannelID)
	if err != nil {
		return result
	}
	for _, row := range rows {
		readiness := model.ResolveChannelModelProcurementReadinessForMode(row, batches, mode)
		result[strings.TrimSpace(row.Model)] = readiness.Status
	}
	return result
}

// SyncChannelCostQuotes pulls the billing service's cost quotes for a channel and
// caches the production-grade (actual) ones into channel_model_cost_rates, converting
// each unit cost to CNY via the currency charge rate. This is the operator-triggered
// ingestion that lets the online cost floor use service costs (P5 §A.4 step 2). It
// never changes charging by itself — the floor only reads the cache when enabled.
func SyncChannelCostQuotes(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	cached, skipped, err := SyncChannelCostQuotesForChannel(c.Request.Context(), channelID)
	if err != nil {
		logChannelAdminWarn(c, "sync_cost_quotes", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "sync_cost_quotes", stringField("channel_id", channelID), intField("cached", cached), intField("skipped", skipped))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"cached": cached, "skipped": skipped}})
}

// SyncChannelCostQuotesForChannel fetches the billing service's cost quotes for a
// channel and caches the production-grade (actual) ones into channel_model_cost_rates
// (unit cost normalized to CNY). Shared by the admin endpoint and the scheduler.
// Returns counts; never mutates charging directly.
func SyncChannelCostQuotesForChannel(ctx context.Context, channelID string) (int, int, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return 0, 0, fmt.Errorf("渠道 ID 无效")
	}
	channelRow, profile, err := getEffectiveChannelBillingProfile(normalizedChannelID)
	if err != nil {
		return 0, 0, err
	}
	quotes, err := collectBillingServiceCostQuotes(ctx, channelRow, profile, nil)
	if err != nil {
		return 0, 0, err
	}
	rows := make([]model.ChannelModelCostRate, 0, len(quotes.Quotes))
	cached := 0
	skipped := 0
	for _, quote := range quotes.Quotes {
		confidence := strings.TrimSpace(strings.ToLower(quote.Confidence))
		modelName := strings.TrimSpace(quote.Model)
		unit := strings.TrimSpace(strings.ToLower(quote.CapacityUnit))
		// Only production-grade (actual) quotes may back the online floor.
		if confidence != model.ChannelModelCostRateConfidenceActual || modelName == "" || unit == "" || quote.UnitCost <= 0 {
			skipped++
			continue
		}
		currency := strings.TrimSpace(strings.ToUpper(quote.Currency))
		rate, rateErr := model.GetBillingCurrencyChargeRate(currency)
		if rateErr != nil || rate <= 0 {
			skipped++
			continue
		}
		asOf := quotes.FetchedAt.Unix()
		if quote.AsOf != nil {
			asOf = quote.AsOf.Unix()
		}
		validUntil := int64(0)
		if quote.ValidUntil != nil {
			validUntil = quote.ValidUntil.Unix()
		}
		rows = append(rows, model.ChannelModelCostRate{
			ChannelId:        normalizedChannelID,
			Model:            modelName,
			CapacityUnit:     unit,
			UnitCostYyc:      quote.UnitCost * rate,
			UnitCostOriginal: quote.UnitCost,
			Currency:         currency,
			FXRate:           rate,
			Confidence:       confidence,
			Source:           model.ChannelModelCostRateSourceService,
			AsOf:             asOf,
			ValidUntil:       validUntil,
		})
		cached++
	}
	if err := model.ReplaceChannelModelCostRatesWithDB(model.DB, normalizedChannelID, rows); err != nil {
		return 0, 0, err
	}
	return cached, skipped, nil
}

func GetChannelProcurementBatches(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	rows, err := model.ListChannelProcurementBatchesByChannelIDWithDB(model.DB, channelID, 100)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batches", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.ChannelProcurementBatch]{Items: rows, Total: len(rows)}})
}

func UpdateChannelProcurementBatchCost(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	req := channelProcurementBatchCostUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	row, err := model.UpdateChannelProcurementBatchCostWithDB(model.DB, batchID, model.ProcurementBatchCostUpdate{
		PurchaseCurrency:   req.PurchaseCurrency,
		PurchaseAmount:     req.PurchaseAmount,
		PurchaseFXRate:     req.PurchaseFXRate,
		PurchaseCostAmount: req.PurchaseCostAmount,
		CapacityEffective:  req.CapacityEffective,
		CostSource:         req.CostSource,
		CostStatus:         req.CostStatus,
		ScopeType:          req.ScopeType,
		ScopeValue:         req.ScopeValue,
	})
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_procurement_batch_cost", stringField("channel_id", channelID), stringField("batch_id", batchID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func UpdateChannelProcurementBatchStatus(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	req := channelProcurementBatchStatusUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	row, err := model.UpdateChannelProcurementBatchStatusWithDB(model.DB, batchID, model.ProcurementBatchStatusUpdate{
		CostStatus: req.CostStatus,
	})
	if err != nil {
		logChannelAdminWarn(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_procurement_batch_status", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("cost_status", row.CostStatus))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func GetChannelProcurementBatchConsumptions(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	batchID := strings.TrimSpace(c.Param("batch_id"))
	if channelID == "" || batchID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次 ID 无效"})
		return
	}
	current, err := model.GetChannelProcurementBatchByIDWithDB(model.DB, batchID)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batch_consumptions", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不存在"})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelID {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购批次不属于当前渠道"})
		return
	}
	rows, err := model.ListRequestProcurementConsumptionsByBatchIDWithDB(model.DB, batchID, 100)
	if err != nil {
		logChannelAdminWarn(c, "list_procurement_batch_consumptions", stringField("channel_id", channelID), stringField("batch_id", batchID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelBillingListData[model.RequestProcurementConsumption]{Items: rows, Total: len(rows)}})
}

func GetRecentChannelBillingAlerts(c *gin.Context) {
	rows, err := model.ListRecentChannelBillingAlertEventsWithDB(model.DB, 20)
	if err != nil {
		logChannelAdminWarn(c, "list_recent_billing_alerts", stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelIDs := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		channelIDs = append(channelIDs, channelID)
	}
	channelNameByID := make(map[string]string, len(channelIDs))
	if len(channelIDs) > 0 {
		channels := make([]model.Channel, 0, len(channelIDs))
		if err := model.DB.Select("id", "name").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			logChannelAdminWarn(c, "list_recent_billing_alerts", stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		for _, channelRow := range channels {
			channelNameByID[strings.TrimSpace(channelRow.Id)] = strings.TrimSpace(channelRow.DisplayName())
		}
	}
	items := make([]channelBillingAlertFeedItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, channelBillingAlertFeedItem{
			ChannelBillingAlertEvent: row,
			ChannelName:              channelNameByID[strings.TrimSpace(row.ChannelId)],
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": channelBillingListData[channelBillingAlertFeedItem]{
			Items: items,
			Total: len(items),
		},
	})
}

func UpdateChannelBillingProfile(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := channelBillingProfileUpdateRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, err := channelsvc.GetByID(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	profileRow, err := model.GetChannelBillingProfileByChannelIDWithDB(model.DB, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			initialProfile, ok := model.BuildChannelBillingProfileFromChannelConfig(channelRow)
			if !ok {
				profileRow = model.ChannelBillingProfile{
					ChannelId:          channelID,
					BillingSource:      model.ChannelBillingSourceManual,
					ActionCapabilities: "[]",
				}
			} else {
				profileRow = initialProfile
			}
		} else {
			logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	nextSource := normalizeChannelBillingSource(req.BillingSource)
	credentialFields := []billingServiceCredentialField{}
	if nextSource != model.ChannelBillingSourceManual {
		adapterInfo, exists, err := findBillingServiceAdapter(c.Request.Context(), nextSource)
		if err != nil {
			logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		if !exists {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Billing adapter 无效"})
			return
		}
		credentialFields = adapterInfo.CredentialFields
	}
	billingCredentials := filterBillingCredentialsByFields(req.BillingCredentials, nextSource, credentialFields)
	if missingField := missingRequiredBillingCredentialField(credentialFields, billingCredentials); missingField != "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("账务凭据 %s 未配置", missingField)})
		return
	}
	profileRow.BillingSource = nextSource
	nextMode := model.NormalizeChannelCostTrackingMode(req.CostTrackingMode)
	profileRow.CostTrackingMode = nextMode
	nextConfig := map[string]any{
		"billing_credentials": billingCredentials,
	}
	profileRow.BillingConfig = marshalLogJSON(nextConfig)
	capabilities := []string{model.ChannelBillingCapabilityManualUpdateSnapshot}
	if nextSource != model.ChannelBillingSourceManual {
		capabilities = append(capabilities, model.ChannelBillingCapabilityRefreshBilling)
	}
	profileRow.ActionCapabilities = marshalLogJSON(capabilities)
	savedRow, err := model.SaveChannelBillingProfileWithDB(model.DB, profileRow)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Bring auto-managed zero-cost batches in line with the chosen mode: free
	// ensures global zero-cost coverage; untracked/actual disable it. Idempotent, so
	// it also refreshes coverage when the channel's models changed.
	if err := model.ReconcileChannelCostTrackingModeWithDB(model.DB, channelID, savedRow.CostTrackingMode); err != nil {
		logChannelAdminWarn(c, "update_billing_profile", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_billing_profile", stringField("channel_id", channelID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildChannelBillingProfileData(channelRow, savedRow),
	})
}

func marshalLogJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(body)
}

func sanitizeBillingCredentialMap(credentials map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range credentials {
		normalizedKey := normalizeBillingServiceCredentialFieldName(key)
		normalizedValue := strings.TrimSpace(value)
		if normalizedKey == "" || normalizedValue == "" {
			continue
		}
		result[normalizedKey] = normalizedValue
	}
	return result
}

func filterBillingCredentialsByFields(credentials map[string]string, billingSource string, fields []billingServiceCredentialField) map[string]string {
	if normalizeChannelBillingSource(billingSource) == model.ChannelBillingSourceManual {
		return map[string]string{}
	}
	normalizedCredentials := sanitizeBillingCredentialMap(credentials)
	if len(fields) == 0 {
		return normalizedCredentials
	}
	allowedFields := map[string]bool{}
	for _, field := range normalizeBillingServiceCredentialFields(fields) {
		allowedFields[field.Name] = true
	}
	result := make(map[string]string)
	for key, value := range normalizedCredentials {
		if allowedFields[key] {
			result[key] = value
		}
	}
	return result
}

func maskBillingSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 6 {
		return "***"
	}
	return trimmed[:3] + "***" + trimmed[len(trimmed)-3:]
}

func nearlyEqualFloat(left float64, right float64) bool {
	if left > right {
		return left-right < 0.0000001
	}
	return right-left < 0.0000001
}

func ensureConsumedSnapshotImmutableFields(current model.ChannelBillingSnapshot, next normalizedManualBillingSnapshotRequest) error {
	if current.PurchaseAt != next.PurchaseAt ||
		strings.TrimSpace(strings.ToUpper(current.PurchaseCurrency)) != strings.TrimSpace(strings.ToUpper(next.PurchaseCurrency)) ||
		!nearlyEqualFloat(current.PurchaseAmount, next.PurchaseAmount) ||
		!nearlyEqualFloat(current.PurchaseFXRate, next.PurchaseFXRate) ||
		!nearlyEqualFloat(current.PurchaseCostAmount, next.PurchaseCostAmount) {
		return fmt.Errorf("该采购记录已经被消耗，只能修正权益类型、有效期、名称和备注")
	}
	return nil
}

func sumProcurementConsumedQuantityWithDB(tx *gorm.DB, batchID string) (float64, error) {
	normalizedBatchID := strings.TrimSpace(batchID)
	if normalizedBatchID == "" {
		return 0, nil
	}
	var consumed float64
	err := tx.Model(&model.RequestProcurementConsumption{}).
		Where("procurement_batch_id = ?", normalizedBatchID).
		Select("COALESCE(SUM(consumed_quantity), 0)").
		Scan(&consumed).Error
	return consumed, err
}

func updateConsumedManualBillingSnapshotWithDB(tx *gorm.DB, current model.ChannelBillingSnapshot, channelID string, req normalizedManualBillingSnapshotRequest, operatorUserID string) ([]model.ChannelBillingSnapshotItem, error) {
	if err := ensureConsumedSnapshotImmutableFields(current, req); err != nil {
		return nil, err
	}
	existingItems := model.NormalizeChannelBillingSnapshotItems(current.Items)
	if len(existingItems) != len(req.Items) {
		return nil, fmt.Errorf("该采购记录已经被消耗，不能新增或删除权益项")
	}
	itemsByID := make(map[string]model.ChannelBillingSnapshotItem, len(existingItems))
	for _, item := range existingItems {
		if strings.TrimSpace(item.Id) != "" {
			itemsByID[strings.TrimSpace(item.Id)] = item
		}
	}
	updatedSnapshot, err := model.UpdateChannelBillingSnapshotPurchaseWithDB(tx, model.ChannelBillingSnapshot{
		Id:                 current.Id,
		ChannelId:          channelID,
		PurchaseAt:         req.PurchaseAt,
		PurchaseCurrency:   req.PurchaseCurrency,
		PurchaseAmount:     req.PurchaseAmount,
		PurchaseFXRate:     req.PurchaseFXRate,
		PurchaseCostAmount: req.PurchaseCostAmount,
		EntitlementName:    req.EntitlementName,
		ValidFrom:          req.ValidFrom,
		ValidUntil:         req.ValidUntil,
		Message:            req.Message,
		OperatorUserId:     operatorUserID,
	})
	if err != nil {
		return nil, err
	}
	now := helper.GetTimestamp()
	updatedItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, nextItem := range req.Items {
		currentItem := model.ChannelBillingSnapshotItem{}
		if itemID := strings.TrimSpace(nextItem.Id); itemID != "" {
			matched, ok := itemsByID[itemID]
			if !ok {
				return nil, fmt.Errorf("该采购记录已经被消耗，权益项不存在")
			}
			currentItem = matched
		} else {
			currentItem = existingItems[index]
		}
		if strings.TrimSpace(currentItem.ResourceType) != strings.TrimSpace(nextItem.ResourceType) ||
			strings.TrimSpace(strings.ToUpper(currentItem.Currency)) != strings.TrimSpace(strings.ToUpper(nextItem.Currency)) ||
			!nearlyEqualFloat(currentItem.Amount, nextItem.Amount) ||
			!nearlyEqualFloat(currentItem.LimitAmount, nextItem.LimitAmount) ||
			!nearlyEqualFloat(currentItem.UsedAmount, nextItem.UsedAmount) ||
			!nearlyEqualFloat(currentItem.RemainingAmount, nextItem.RemainingAmount) {
			return nil, fmt.Errorf("该采购记录已经被消耗，不能修改权益容量或币种")
		}
		nextItem.Id = currentItem.Id
		nextItem.SnapshotId = current.Id
		nextItem.ChannelId = channelID
		nextItem.CreatedAt = currentItem.CreatedAt
		nextItem.SortOrder = currentItem.SortOrder
		if nextItem.SortOrder == 0 {
			nextItem.SortOrder = index + 1
		}
		if err := tx.Model(&model.ChannelBillingSnapshotItem{}).
			Where("id = ? AND snapshot_id = ?", currentItem.Id, current.Id).
			Updates(map[string]any{
				"quota_type":       strings.TrimSpace(strings.ToLower(nextItem.QuotaType)),
				"quota_label":      strings.TrimSpace(nextItem.QuotaLabel),
				"reset_at":         nextItem.ResetAt,
				"expires_at":       nextItem.ExpiresAt,
				"source_ref":       strings.TrimSpace(nextItem.SourceRef),
				"sort_order":       nextItem.SortOrder,
				"resource_type":    strings.TrimSpace(strings.ToLower(nextItem.ResourceType)),
				"amount":           nextItem.Amount,
				"limit_amount":     nextItem.LimitAmount,
				"used_amount":      nextItem.UsedAmount,
				"remaining_amount": nextItem.RemainingAmount,
				"currency":         strings.TrimSpace(strings.ToUpper(nextItem.Currency)),
			}).Error; err != nil {
			return nil, err
		}
		batchTemplate, ok := model.BuildProcurementBatchFromBillingSnapshotItem(updatedSnapshot, nextItem)
		if !ok {
			return nil, fmt.Errorf("该采购记录已经被消耗，不能移除可消耗权益项")
		}
		batches, err := model.ListChannelProcurementBatchesBySourceSnapshotIDWithDB(tx, current.Id)
		if err != nil {
			return nil, err
		}
		for _, batch := range batches {
			if strings.TrimSpace(batch.SourceSnapshotItemId) != strings.TrimSpace(currentItem.Id) {
				continue
			}
			consumed, err := sumProcurementConsumedQuantityWithDB(tx, batch.Id)
			if err != nil {
				return nil, err
			}
			capacityRemaining := batchTemplate.CapacityEffective - consumed
			if capacityRemaining < 0 {
				capacityRemaining = 0
			}
			costStatus := batch.CostStatus
			if costStatus != model.ProcurementCostStatusDisabled && costStatus != model.ProcurementCostStatusCostUnconfigured {
				if capacityRemaining <= 0 {
					costStatus = model.ProcurementCostStatusExhausted
				} else {
					costStatus = model.ProcurementCostStatusActive
				}
			}
			costPerUnitAmount := batch.CostPerUnitAmount
			if batch.PurchaseCostAmount > 0 && batchTemplate.CapacityEffective > 0 {
				costPerUnitAmount = batch.PurchaseCostAmount / batchTemplate.CapacityEffective
			}
			windowRemaining := batchTemplate.CapacityTotal - consumed
			if windowRemaining < 0 {
				windowRemaining = 0
			}
			updates := map[string]any{
				"resource_type":        batchTemplate.ResourceType,
				"quota_type":           batchTemplate.QuotaType,
				"capacity_total":       batchTemplate.CapacityTotal,
				"capacity_effective":   batchTemplate.CapacityEffective,
				"capacity_remaining":   capacityRemaining,
				"cost_per_unit_amount": costPerUnitAmount,
				"valid_from":           batchTemplate.ValidFrom,
				"expire_at":            batchTemplate.ExpireAt,
				"reset_cycle":          batchTemplate.ResetCycle,
				"window_started_at":    batchTemplate.WindowStartedAt,
				"window_remaining":     windowRemaining,
				"source_ref":           batchTemplate.SourceRef,
				"metadata":             batchTemplate.Metadata,
				"cost_status":          costStatus,
				"updated_at":           now,
			}
			if batchTemplate.ResetCycle == "none" || batchTemplate.ResetCycle == "" {
				updates["window_started_at"] = int64(0)
				updates["window_remaining"] = float64(0)
			}
			if err := tx.Model(&model.ChannelProcurementBatch{}).Where("id = ?", batch.Id).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
		updatedItems = append(updatedItems, nextItem)
	}
	return model.NormalizeChannelBillingSnapshotItems(updatedItems), nil
}

type normalizedManualBillingSnapshotRequest struct {
	PurchaseAt          int64
	PurchaseCurrency    string
	PurchaseAmount      float64
	PurchaseFXRate      float64
	PurchaseCostAmount  float64
	EntitlementName     string
	EventType           string
	ParentSnapshotID    string
	OldBatchDisposition string
	ValidFrom           int64
	ValidUntil          int64
	Items               []model.ChannelBillingSnapshotItem
	Message             string
}

func normalizeManualBillingSnapshotRequest(req channelBillingManualSnapshotRequest, now int64) (normalizedManualBillingSnapshotRequest, error) {
	purchaseCurrency := strings.TrimSpace(strings.ToUpper(req.PurchaseCurrency))
	purchaseAmount := req.PurchaseAmount
	purchaseFXRate := req.PurchaseFXRate
	purchaseCostAmount := req.PurchaseCostAmount
	if purchaseCurrency == "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购币种不能为空")
	}
	if purchaseAmount <= 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("实付金额必须大于 0")
	}
	if purchaseFXRate < 0 || purchaseCostAmount < 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购汇率和采购成本不能小于 0")
	}
	if purchaseCurrency == "CNY" {
		if purchaseFXRate <= 0 {
			purchaseFXRate = 1
		}
		if purchaseCostAmount <= 0 {
			purchaseCostAmount = purchaseAmount
		}
	} else if purchaseCostAmount <= 0 && purchaseFXRate > 0 {
		purchaseCostAmount = purchaseAmount * purchaseFXRate
	}
	if purchaseCostAmount <= 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购成本 CNY 必须大于 0")
	}
	purchaseAt := req.PurchaseAt
	if purchaseAt <= 0 {
		purchaseAt = now
	}
	validFrom := req.ValidFrom
	validUntil := req.ValidUntil
	if validFrom < 0 || validUntil < 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("有效期无效")
	}
	if validUntil > 0 && validFrom > 0 && validUntil <= validFrom {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("有效期结束时间必须晚于开始时间")
	}
	entitlementName := strings.TrimSpace(req.EntitlementName)
	if entitlementName == "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("权益名称不能为空")
	}
	eventType := strings.TrimSpace(strings.ToLower(req.EventType))
	if eventType == "" {
		eventType = "purchase"
	}
	switch eventType {
	case "purchase", "renewal", "upgrade", "downgrade", "quota_adjustment", "correction":
	default:
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("采购变更类型无效")
	}
	parentSnapshotID := strings.TrimSpace(req.ParentSnapshotID)
	oldBatchDisposition := strings.TrimSpace(strings.ToLower(req.OldBatchDisposition))
	if oldBatchDisposition == "" {
		oldBatchDisposition = "keep"
	}
	if oldBatchDisposition != "keep" && oldBatchDisposition != "disable" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("旧采购批次处理方式无效")
	}
	if (eventType == "upgrade" || eventType == "downgrade") && parentSnapshotID == "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("套餐升级或降级必须选择原采购记录")
	}
	if eventType != "upgrade" && eventType != "downgrade" && parentSnapshotID != "" {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("只有套餐升级或降级可以关联原采购记录")
	}
	quotaItems := make([]model.ChannelBillingSnapshotItem, 0, len(req.Items))
	for index, item := range req.Items {
		resourceType := strings.TrimSpace(strings.ToLower(item.ResourceType))
		if !isSupportedBillingResourceType(resourceType) {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条权益类型无效", index+1)
		}
		quotaType := strings.TrimSpace(strings.ToLower(item.QuotaType))
		itemExpiresAt := item.ExpiresAt
		if itemExpiresAt <= 0 {
			itemExpiresAt = validUntil
		}
		if resourceType == model.ChannelBillingResourceTypePlan {
			quotaType = "plan"
		}
		if resourceType == model.ChannelBillingResourceTypeQuota {
			switch quotaType {
			case "daily", "weekly", "monthly", "total", "custom":
			default:
				return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度类型无效", index+1)
			}
		}
		quotaLabel := strings.TrimSpace(item.QuotaLabel)
		if quotaLabel == "" {
			quotaLabel = resourceType
			if quotaType != "" && quotaType != resourceType {
				quotaLabel += ":" + quotaType
			}
		}
		limitAmount := item.LimitAmount
		if limitAmount <= 0 {
			limitAmount = item.Amount
		}
		remainingAmount := item.RemainingAmount
		if remainingAmount <= 0 && item.UsedAmount == 0 {
			remainingAmount = item.Amount
		}
		amount := item.Amount
		if amount <= 0 {
			amount = remainingAmount
		}
		if amount <= 0 && limitAmount > 0 {
			amount = limitAmount
		}
		if item.Amount < 0 || item.LimitAmount < 0 || item.UsedAmount < 0 || item.RemainingAmount < 0 {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度数值不能小于 0", index+1)
		}
		if resourceType != model.ChannelBillingResourceTypePlan && amount <= 0 && remainingAmount <= 0 && limitAmount <= 0 {
			return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("第 %d 条额度数值不能为空", index+1)
		}
		sourceRef := strings.TrimSpace(item.SourceRef)
		if sourceRef == "" {
			sourceRef = "manual"
		}
		quotaItems = append(quotaItems, model.ChannelBillingSnapshotItem{
			Id:              strings.TrimSpace(item.Id),
			ResourceType:    resourceType,
			QuotaType:       quotaType,
			QuotaLabel:      quotaLabel,
			Amount:          amount,
			LimitAmount:     limitAmount,
			UsedAmount:      item.UsedAmount,
			RemainingAmount: remainingAmount,
			Currency:        strings.TrimSpace(item.Currency),
			ResetAt:         item.ResetAt,
			ExpiresAt:       itemExpiresAt,
			SourceRef:       sourceRef,
			SortOrder:       index + 1,
		})
	}
	if len(quotaItems) == 0 {
		return normalizedManualBillingSnapshotRequest{}, fmt.Errorf("请至少填写一条权益项")
	}
	return normalizedManualBillingSnapshotRequest{
		PurchaseAt:          purchaseAt,
		PurchaseCurrency:    purchaseCurrency,
		PurchaseAmount:      purchaseAmount,
		PurchaseFXRate:      purchaseFXRate,
		PurchaseCostAmount:  purchaseCostAmount,
		EntitlementName:     entitlementName,
		EventType:           eventType,
		ParentSnapshotID:    parentSnapshotID,
		OldBatchDisposition: oldBatchDisposition,
		ValidFrom:           validFrom,
		ValidUntil:          validUntil,
		Items:               quotaItems,
		Message:             strings.TrimSpace(req.Message),
	}, nil
}

func CreateChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	if channelID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "渠道 ID 无效"})
		return
	}
	req := channelBillingManualSnapshotRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "create_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	operatorUserID := strings.TrimSpace(c.GetString(ctxkey.Id))
	now := helper.GetTimestamp()
	normalizedReq, err := normalizeManualBillingSnapshotRequest(req, now)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if normalizedReq.ParentSnapshotID != "" {
			parent, err := model.GetChannelBillingSnapshotByIDWithDB(tx, normalizedReq.ParentSnapshotID)
			if err != nil {
				return fmt.Errorf("原采购记录不存在")
			}
			if strings.TrimSpace(parent.ChannelId) != channelRow.Id {
				return fmt.Errorf("原采购记录不属于当前渠道")
			}
			if normalizedReq.OldBatchDisposition == "disable" {
				if err := tx.Model(&model.ChannelProcurementBatch{}).
					Where("source_snapshot_id = ? AND cost_status = ?", parent.Id, model.ProcurementCostStatusActive).
					Updates(map[string]any{"cost_status": model.ProcurementCostStatusDisabled, "updated_at": now}).Error; err != nil {
					return err
				}
			}
		}
		snapshotRow, err := model.CreateChannelBillingSnapshotWithDB(tx, model.ChannelBillingSnapshot{
			ChannelId:           channelRow.Id,
			SourceType:          model.ChannelBillingSnapshotSourceManual,
			PurchaseAt:          normalizedReq.PurchaseAt,
			PurchaseCurrency:    normalizedReq.PurchaseCurrency,
			PurchaseAmount:      normalizedReq.PurchaseAmount,
			PurchaseFXRate:      normalizedReq.PurchaseFXRate,
			PurchaseCostAmount:  normalizedReq.PurchaseCostAmount,
			EntitlementName:     normalizedReq.EntitlementName,
			EventType:           normalizedReq.EventType,
			ParentSnapshotId:    normalizedReq.ParentSnapshotID,
			OldBatchDisposition: normalizedReq.OldBatchDisposition,
			ValidFrom:           normalizedReq.ValidFrom,
			ValidUntil:          normalizedReq.ValidUntil,
			RawStatus:           "manual",
			Message:             normalizedReq.Message,
			OperatorUserId:      operatorUserID,
			CreatedAt:           now,
		})
		if err != nil {
			return err
		}
		createdItems, err := model.CreateChannelBillingSnapshotItemsWithDB(tx, snapshotRow.Id, channelRow.Id, normalizedReq.Items)
		if err != nil {
			return err
		}
		if err := attachManualPurchaseCostToSnapshotBatches(tx, snapshotRow.Id, normalizedReq.PurchaseCurrency, normalizedReq.PurchaseAmount, normalizedReq.PurchaseFXRate, normalizedReq.PurchaseCostAmount); err != nil {
			return err
		}
		_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
			ChannelId:      channelRow.Id,
			ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
			Status:         model.ChannelBillingActionStatusDone,
			RequestPayload: marshalLogJSON(req),
			ResultPayload:  marshalLogJSON(map[string]any{"items": createdItems}),
			Message:        normalizedReq.Message,
			OperatorUserId: operatorUserID,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		return err
	})
	if err != nil {
		logChannelAdminWarn(c, "create_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "create_billing_snapshot", stringField("channel_id", channelID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID}})
}

func UpdateChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	snapshotID := strings.TrimSpace(c.Param("snapshot_id"))
	if channelID == "" || snapshotID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数无效"})
		return
	}
	req := channelBillingManualSnapshotRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	operatorUserID := strings.TrimSpace(c.GetString(ctxkey.Id))
	now := helper.GetTimestamp()
	normalizedReq, err := normalizeManualBillingSnapshotRequest(req, now)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		current, err := model.GetChannelBillingSnapshotByIDWithDB(tx, snapshotID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(current.ChannelId) != channelRow.Id {
			return gorm.ErrRecordNotFound
		}
		if strings.TrimSpace(current.SourceType) != model.ChannelBillingSnapshotSourceManual {
			return fmt.Errorf("只能修改人工采购记录")
		}
		count, err := model.CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(tx, snapshotID)
		if err != nil {
			return err
		}
		if count > 0 {
			updatedItems, err := updateConsumedManualBillingSnapshotWithDB(tx, current, channelRow.Id, normalizedReq, operatorUserID)
			if err != nil {
				return err
			}
			_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
				ChannelId:      channelRow.Id,
				ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
				Status:         model.ChannelBillingActionStatusDone,
				RequestPayload: marshalLogJSON(req),
				ResultPayload:  marshalLogJSON(map[string]any{"items": updatedItems, "consumed_metadata_update": true}),
				Message:        normalizedReq.Message,
				OperatorUserId: operatorUserID,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
			return err
		}
		updatedSnapshot, err := model.UpdateChannelBillingSnapshotPurchaseWithDB(tx, model.ChannelBillingSnapshot{
			Id:                 current.Id,
			ChannelId:          channelRow.Id,
			PurchaseAt:         normalizedReq.PurchaseAt,
			PurchaseCurrency:   normalizedReq.PurchaseCurrency,
			PurchaseAmount:     normalizedReq.PurchaseAmount,
			PurchaseFXRate:     normalizedReq.PurchaseFXRate,
			PurchaseCostAmount: normalizedReq.PurchaseCostAmount,
			EntitlementName:    normalizedReq.EntitlementName,
			ValidFrom:          normalizedReq.ValidFrom,
			ValidUntil:         normalizedReq.ValidUntil,
			Message:            normalizedReq.Message,
			OperatorUserId:     operatorUserID,
		})
		if err != nil {
			return err
		}
		createdItems, err := model.ReplaceChannelBillingSnapshotItemsWithDB(tx, updatedSnapshot.Id, channelRow.Id, normalizedReq.Items)
		if err != nil {
			return err
		}
		if err := attachManualPurchaseCostToSnapshotBatches(tx, updatedSnapshot.Id, normalizedReq.PurchaseCurrency, normalizedReq.PurchaseAmount, normalizedReq.PurchaseFXRate, normalizedReq.PurchaseCostAmount); err != nil {
			return err
		}
		_, err = model.CreateChannelBillingActionWithDB(tx, model.ChannelBillingAction{
			ChannelId:      channelRow.Id,
			ActionType:     model.ChannelBillingActionTypeManualUpdateSnapshot,
			Status:         model.ChannelBillingActionStatusDone,
			RequestPayload: marshalLogJSON(req),
			ResultPayload:  marshalLogJSON(map[string]any{"items": createdItems}),
			Message:        normalizedReq.Message,
			OperatorUserId: operatorUserID,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		return err
	})
	if err != nil {
		logChannelAdminWarn(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "update_billing_snapshot", stringField("channel_id", channelID), stringField("snapshot_id", snapshotID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID, "snapshot_id": snapshotID}})
}

func DeleteChannelBillingSnapshot(c *gin.Context) {
	channelID := strings.TrimSpace(c.Param("id"))
	snapshotID := strings.TrimSpace(c.Param("snapshot_id"))
	if channelID == "" || snapshotID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "参数无效"})
		return
	}
	channelRow, _, err := getEffectiveChannelBillingProfile(channelID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	current, err := model.GetChannelBillingSnapshotByIDWithDB(model.DB, snapshotID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if strings.TrimSpace(current.ChannelId) != channelRow.Id {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "采购记录不存在"})
		return
	}
	if strings.TrimSpace(current.SourceType) != model.ChannelBillingSnapshotSourceManual {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "只能删除人工采购记录"})
		return
	}
	count, err := model.CountRequestProcurementConsumptionsBySourceSnapshotIDWithDB(model.DB, snapshotID)
	if err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该采购记录已经被消耗，不能删除"})
		return
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return model.DeleteChannelBillingSnapshotPurchaseWithDB(tx, snapshotID, channelRow.Id)
	}); err != nil {
		logChannelAdminWarn(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("reason", err.Error()))
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	logChannelAdminInfo(c, "delete_billing_snapshot", stringField("channel_id", channelID), stringField("snapshot_id", snapshotID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channel_id": channelID, "snapshot_id": snapshotID}})
}
