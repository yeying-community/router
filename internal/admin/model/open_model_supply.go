package model

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PublishersTableName                    = "publishers"
	PublisherServicesTableName             = "publisher_services"
	PublisherServiceModelsTableName        = "publisher_service_models"
	ServiceOffersTableName                 = "service_offers"
	PublisherAuditLogsTableName            = "publisher_audit_logs"
	PublisherOfferSettlementsTableName     = "publisher_offer_settlements"
	PublisherSettlementDeliveriesTableName = "publisher_settlement_deliveries"
	CommunityOfferModelRoutesTableName     = "community_offer_model_routes"

	PublisherLevelVerifiedCommunity = "verified_community"

	PublisherStatusDraft      = "draft"
	PublisherStatusReviewing  = "reviewing"
	PublisherStatusActive     = "active"
	PublisherStatusRestricted = "restricted"
	PublisherStatusSuspended  = "suspended"
	PublisherStatusClosed     = "closed"

	PublisherServiceStatusDraft     = "draft"
	PublisherServiceStatusReady     = "ready"
	PublisherServiceStatusSuspended = "suspended"
	PublisherServiceStatusOffline   = "offline"

	PublisherServiceModelStatusDraft = "draft"
	PublisherServiceModelStatusReady = "ready"

	ServiceOfferScopePublic = "public"
	ServiceOfferScopeInvite = "invite"

	ServiceOfferStatusDraft     = "draft"
	ServiceOfferStatusReviewing = "reviewing"
	ServiceOfferStatusPublished = "published"
	ServiceOfferStatusSuspended = "suspended"
	ServiceOfferStatusOffline   = "offline"
	ServiceOfferStatusRejected  = "rejected"

	ServiceOfferDefaultPlatformFeeBPS = 1000

	PublisherOfferSettlementStatusAccrued  = "accrued"
	PublisherOfferSettlementStatusHeld     = "held"
	PublisherOfferSettlementStatusSettled  = "settled"
	PublisherOfferSettlementStatusReversed = "reversed"

	PublisherSettlementDeliveryStatusPrepared  = "prepared"
	PublisherSettlementDeliveryStatusReady     = "ready"
	PublisherSettlementDeliveryStatusDelivered = "delivered"
	PublisherSettlementDeliveryStatusCancelled = "cancelled"
	PublisherSettlementDeliveryStatusException = "exception"
	PublisherSettlementDeliveryStatusResolved  = "resolved"

	// A consume log is written synchronously, but cancellation remains a manual
	// financial action. This grace period prevents an operator from closing a
	// request that is still in the final log-write window.
	PublisherSettlementDeliveryCancellationMinimumAgeSeconds  int64 = 15 * 60
	PublisherSettlementDeliveryCancellationObservationSeconds int64 = 24 * 60 * 60

	// CommunityOfferChannelPrefix identifies a transient relay channel created
	// from one reviewed public offer. It must never be persisted as a Channel:
	// a community offer has its own operational and commercial lifecycle.
	CommunityOfferChannelPrefix = "community-offer:"

	// PublisherServiceChannelPrefix is the stable product identity of a
	// third-party supply channel. Routing and settlement continue to use the
	// offer-level CommunityOfferChannelPrefix above.
	PublisherServiceChannelPrefix = "publisher-service:"
)

// Publisher is the accountable party that can submit independently operated
// model services. It is deliberately distinct from providers, channels and
// private personal-provider connections.
type Publisher struct {
	Id                string `json:"id" gorm:"type:char(36);primaryKey"`
	UserId            string `json:"user_id" gorm:"type:char(36);not null;uniqueIndex"`
	WalletIdentityDID string `json:"wallet_identity_did" gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName       string `json:"display_name" gorm:"type:varchar(96);not null;default:''"`
	ContactEmail      string `json:"contact_email" gorm:"type:varchar(254);not null;default:''"`
	Level             string `json:"level" gorm:"type:varchar(32);not null;default:'verified_community';index"`
	Status            string `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	ReviewNote        string `json:"review_note,omitempty" gorm:"type:text;not null;default:''"`
	SubmittedAt       int64  `json:"submitted_at" gorm:"bigint;not null;default:0;index"`
	ReviewedAt        int64  `json:"reviewed_at" gorm:"bigint;not null;default:0;index"`
	ReviewedBy        string `json:"reviewed_by,omitempty" gorm:"type:char(36);not null;default:'';index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint;not null;index"`
}

func (Publisher) TableName() string { return PublishersTableName }

// PublisherService stores an independently configured upstream. Credentials
// never have a JSON field and are not shared with personal connections.
type PublisherService struct {
	Id                         string                  `json:"id" gorm:"type:char(36);primaryKey"`
	PublisherId                string                  `json:"publisher_id" gorm:"type:char(36);not null;index:idx_publisher_service_name,priority:1;index"`
	SourcePersonalConnectionId string                  `json:"source_personal_connection_id,omitempty" gorm:"type:char(36);not null;default:'';index"`
	Name                       string                  `json:"name" gorm:"type:varchar(96);not null;index:idx_publisher_service_name,priority:2"`
	Protocol                   string                  `json:"protocol" gorm:"type:varchar(64);not null;default:'openai'"`
	BaseURL                    string                  `json:"base_url" gorm:"type:text;not null;default:''"`
	CredentialEncrypted        string                  `json:"-" gorm:"type:text;not null"`
	Region                     string                  `json:"region" gorm:"type:varchar(64);not null;default:''"`
	DataPolicy                 string                  `json:"data_policy" gorm:"type:text;not null;default:'{}'"`
	CapacityPolicy             string                  `json:"capacity_policy" gorm:"type:text;not null;default:'{}'"`
	Status                     string                  `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	LastCheckedAt              int64                   `json:"last_checked_at" gorm:"bigint;not null;default:0;index"`
	LastCheckOK                bool                    `json:"last_check_ok" gorm:"not null;default:false"`
	LastCheckError             string                  `json:"last_check_error" gorm:"type:text;not null;default:''"`
	CredentialConfigured       bool                    `json:"credential_configured" gorm:"-"`
	Models                     []PublisherServiceModel `json:"models,omitempty" gorm:"-"`
	CreatedAt                  int64                   `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt                  int64                   `json:"updated_at" gorm:"bigint;not null;index"`
}

func (PublisherService) TableName() string { return PublisherServicesTableName }

// PublisherServiceModel is an explicit declaration that a publisher service
// has a verified mapping for a standard Router model and a supported endpoint.
type PublisherServiceModel struct {
	Id            string `json:"id" gorm:"type:char(36);primaryKey"`
	ServiceId     string `json:"service_id" gorm:"type:char(36);not null;uniqueIndex:idx_publisher_service_model,priority:1;index"`
	Model         string `json:"model" gorm:"type:varchar(255);not null;uniqueIndex:idx_publisher_service_model,priority:2;index"`
	UpstreamModel string `json:"upstream_model" gorm:"type:varchar(255);not null;default:''"`
	Endpoint      string `json:"endpoint" gorm:"type:varchar(255);not null;default:''"`
	MeteringUnit  string `json:"metering_unit" gorm:"type:varchar(32);not null;default:'token'"`
	Status        string `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;not null;index"`
}

func (PublisherServiceModel) TableName() string { return PublisherServiceModelsTableName }

// ServiceOffer is the reviewable commercial unit. Price fields are integer
// micro-USD per one million tokens so prices are never represented by floats.
type ServiceOffer struct {
	Id                     string `json:"id" gorm:"type:char(36);primaryKey"`
	PublisherId            string `json:"publisher_id" gorm:"type:char(36);not null;index"`
	ServiceId              string `json:"service_id" gorm:"type:char(36);not null;index"`
	Model                  string `json:"model" gorm:"type:varchar(255);not null;index"`
	Scope                  string `json:"scope" gorm:"type:varchar(32);not null;default:'public';index"`
	Currency               string `json:"currency" gorm:"type:varchar(16);not null;default:'USD'"`
	InputPriceMicros       int64  `json:"input_price_micros" gorm:"type:bigint;not null;default:0"`
	OutputPriceMicros      int64  `json:"output_price_micros" gorm:"type:bigint;not null;default:0"`
	PlatformFeeBPS         int    `json:"platform_fee_bps" gorm:"type:int;not null;default:1000"`
	DataPolicySnapshot     string `json:"data_policy_snapshot" gorm:"type:text;not null;default:'{}'"`
	CapacityPolicySnapshot string `json:"capacity_policy_snapshot" gorm:"type:text;not null;default:'{}'"`
	Status                 string `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	Version                int    `json:"version" gorm:"type:int;not null;default:1"`
	ReviewNote             string `json:"review_note,omitempty" gorm:"type:text;not null;default:''"`
	SubmittedAt            int64  `json:"submitted_at" gorm:"bigint;not null;default:0;index"`
	ReviewedAt             int64  `json:"reviewed_at" gorm:"bigint;not null;default:0;index"`
	ReviewedBy             string `json:"reviewed_by,omitempty" gorm:"type:char(36);not null;default:'';index"`
	CreatedAt              int64  `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt              int64  `json:"updated_at" gorm:"bigint;not null;index"`
}

func (ServiceOffer) TableName() string { return ServiceOffersTableName }

// PublisherOfferSettlement is the immutable commercial snapshot for one
// successful community-service request. It is intentionally separate from
// platform procurement records: a publisher payable is a liability owed to a
// community publisher, not Router's cost of purchasing upstream capacity.
//
// RequestLogID is a logical idempotency anchor. Logs may live in a separate
// database, so no cross-database foreign key is used here.
type PublisherOfferSettlement struct {
	RequestLogID            string `json:"request_log_id" gorm:"type:char(36);primaryKey"`
	OfferID                 string `json:"offer_id" gorm:"type:char(36);not null;index"`
	PublisherID             string `json:"publisher_id" gorm:"type:char(36);not null;index"`
	ServiceID               string `json:"service_id" gorm:"type:char(36);not null;index"`
	ConsumerUserID          string `json:"consumer_user_id" gorm:"type:char(36);not null;index"`
	Model                   string `json:"model" gorm:"type:varchar(255);not null;index"`
	PublisherDisplayName    string `json:"publisher_display_name" gorm:"type:varchar(96);not null;default:''"`
	ServiceName             string `json:"service_name" gorm:"type:varchar(96);not null;default:''"`
	Region                  string `json:"region" gorm:"type:varchar(64);not null;default:''"`
	DataPolicySnapshot      string `json:"data_policy_snapshot" gorm:"type:text;not null;default:'{}'"`
	Currency                string `json:"currency" gorm:"type:varchar(16);not null;default:'USD'"`
	InputPriceMicros        int64  `json:"input_price_micros" gorm:"type:bigint;not null;default:0"`
	OutputPriceMicros       int64  `json:"output_price_micros" gorm:"type:bigint;not null;default:0"`
	InputTokens             int64  `json:"input_tokens" gorm:"type:bigint;not null;default:0"`
	OutputTokens            int64  `json:"output_tokens" gorm:"type:bigint;not null;default:0"`
	ConsumerAmountMicros    int64  `json:"consumer_amount_micros" gorm:"type:bigint;not null;default:0"`
	PlatformFeeBPS          int    `json:"platform_fee_bps" gorm:"type:int;not null;default:0"`
	PlatformFeeAmountMicros int64  `json:"platform_fee_amount_micros" gorm:"type:bigint;not null;default:0"`
	PublisherPayableMicros  int64  `json:"publisher_payable_micros" gorm:"type:bigint;not null;default:0"`
	Status                  string `json:"status" gorm:"type:varchar(32);not null;default:'accrued';index"`
	CreatedAt               int64  `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt               int64  `json:"updated_at" gorm:"bigint;not null;index"`
}

func (PublisherOfferSettlement) TableName() string { return PublisherOfferSettlementsTableName }

// PublisherSettlementDelivery is a durable cross-store handoff. Account
// balance and publisher payables live in the main database, while request
// logs live in LOG_DB. A delivery cannot become ready until its log exists.
// Its request log ID also makes settlement creation idempotent.
type PublisherSettlementDelivery struct {
	RequestLogID            string  `json:"request_log_id" gorm:"type:char(36);primaryKey"`
	OfferID                 string  `json:"offer_id" gorm:"type:char(36);not null;index"`
	PublisherID             string  `json:"publisher_id" gorm:"type:char(36);not null;index"`
	ServiceID               string  `json:"service_id" gorm:"type:char(36);not null;index"`
	ConsumerUserID          string  `json:"consumer_user_id" gorm:"type:char(36);not null;index"`
	Model                   string  `json:"model" gorm:"type:varchar(255);not null;index"`
	PublisherDisplayName    string  `json:"publisher_display_name" gorm:"type:varchar(96);not null;default:''"`
	ServiceName             string  `json:"service_name" gorm:"type:varchar(96);not null;default:''"`
	Region                  string  `json:"region" gorm:"type:varchar(64);not null;default:''"`
	DataPolicySnapshot      string  `json:"data_policy_snapshot" gorm:"type:text;not null;default:'{}'"`
	Currency                string  `json:"currency" gorm:"type:varchar(16);not null;default:'USD'"`
	InputPriceMicros        int64   `json:"input_price_micros" gorm:"type:bigint;not null;default:0"`
	OutputPriceMicros       int64   `json:"output_price_micros" gorm:"type:bigint;not null;default:0"`
	InputTokens             int64   `json:"input_tokens" gorm:"type:bigint;not null;default:0"`
	OutputTokens            int64   `json:"output_tokens" gorm:"type:bigint;not null;default:0"`
	ConsumerAmountMicros    int64   `json:"consumer_amount_micros" gorm:"type:bigint;not null;default:0"`
	PlatformFeeBPS          int     `json:"platform_fee_bps" gorm:"type:int;not null;default:0"`
	PlatformFeeAmountMicros int64   `json:"platform_fee_amount_micros" gorm:"type:bigint;not null;default:0"`
	PublisherPayableMicros  int64   `json:"publisher_payable_micros" gorm:"type:bigint;not null;default:0"`
	PlannedQuota            int64   `json:"planned_quota" gorm:"type:bigint;not null;default:0"`
	ChargedQuota            int64   `json:"charged_quota" gorm:"type:bigint;not null;default:0"`
	ChargeRate              float64 `json:"charge_rate" gorm:"type:double precision;not null;default:0"`
	ChargeRecorded          bool    `json:"charge_recorded" gorm:"not null;default:false"`
	Status                  string  `json:"status" gorm:"type:varchar(32);not null;default:'prepared';index"`
	Attempts                int     `json:"attempts" gorm:"type:int;not null;default:0"`
	LastError               string  `json:"last_error,omitempty" gorm:"type:text;not null;default:''"`
	ReadyAt                 int64   `json:"ready_at" gorm:"type:bigint;not null;default:0;index"`
	DeliveredAt             int64   `json:"delivered_at" gorm:"type:bigint;not null;default:0;index"`
	CancelledAt             int64   `json:"cancelled_at" gorm:"type:bigint;not null;default:0;index"`
	CancelledBy             string  `json:"cancelled_by" gorm:"type:char(36);not null;default:'';index"`
	CancellationReason      string  `json:"cancellation_reason" gorm:"type:text;not null;default:''"`
	RefundedQuota           int64   `json:"refunded_quota" gorm:"type:bigint;not null;default:0"`
	RefundLotID             string  `json:"refund_lot_id" gorm:"type:char(36);not null;default:'';index"`
	ExceptionDetectedAt     int64   `json:"exception_detected_at" gorm:"type:bigint;not null;default:0;index"`
	ExceptionReason         string  `json:"exception_reason" gorm:"type:text;not null;default:''"`
	ResolvedAt              int64   `json:"resolved_at" gorm:"type:bigint;not null;default:0;index"`
	ResolvedBy              string  `json:"resolved_by" gorm:"type:char(36);not null;default:'';index"`
	Resolution              string  `json:"resolution" gorm:"type:text;not null;default:''"`
	CreatedAt               int64   `json:"created_at" gorm:"type:bigint;not null;index"`
	UpdatedAt               int64   `json:"updated_at" gorm:"type:bigint;not null;index"`
}

func (PublisherSettlementDelivery) TableName() string { return PublisherSettlementDeliveriesTableName }

// CommunityOfferModelRoute is an explicit consumer decision. It is separate
// from PersonalModelRoute: the latter controls source preference while this
// record fixes the exact reviewed public offer that may serve one model.
type CommunityOfferModelRoute struct {
	UserID    string `json:"user_id" gorm:"type:char(36);primaryKey"`
	Model     string `json:"model" gorm:"type:varchar(255);primaryKey"`
	OfferID   string `json:"offer_id" gorm:"type:char(36);not null;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;not null"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint;not null"`
}

func (CommunityOfferModelRoute) TableName() string { return CommunityOfferModelRoutesTableName }

type PublisherOfferSettlementInput struct {
	RequestLogID   string
	OfferID        string
	ConsumerUserID string
	InputTokens    int64
	OutputTokens   int64
}

type publisherSettlementDeliveryCharge struct {
	Quota    int64
	Rate     float64
	Recorded bool
}

// QuotePublisherOfferSettlement resolves the immutable commercial facts that
// would be recorded for a successful request without creating any liability.
// Relay uses it to charge the consumer before it records the publisher payable.
func QuotePublisherOfferSettlement(input PublisherOfferSettlementInput) (*PublisherOfferSettlement, error) {
	input.RequestLogID = strings.TrimSpace(input.RequestLogID)
	input.OfferID = strings.TrimSpace(input.OfferID)
	input.ConsumerUserID = strings.TrimSpace(input.ConsumerUserID)
	if input.RequestLogID == "" || input.OfferID == "" || input.ConsumerUserID == "" {
		return nil, errors.New("结算请求、报价和消费者不能为空")
	}
	if input.InputTokens < 0 || input.OutputTokens < 0 || input.InputTokens+input.OutputTokens <= 0 {
		return nil, errors.New("结算 token 用量必须为正数")
	}
	offer := &ServiceOffer{}
	if err := DB.Where("id = ?", input.OfferID).First(offer).Error; err != nil {
		return nil, err
	}
	publisher := &Publisher{}
	if err := DB.Where("id = ?", offer.PublisherId).First(publisher).Error; err != nil {
		return nil, err
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", offer.ServiceId, offer.PublisherId).First(service).Error; err != nil {
		return nil, err
	}
	if offer.Status != ServiceOfferStatusPublished || offer.Scope != ServiceOfferScopePublic || publisher.Status != PublisherStatusActive || service.Status != PublisherServiceStatusReady || !service.LastCheckOK {
		return nil, errors.New("报价当前不满足社区服务结算条件")
	}
	consumerAmount, platformFee, publisherPayable, err := calculatePublisherOfferSettlementAmounts(offer.InputPriceMicros, offer.OutputPriceMicros, input.InputTokens, input.OutputTokens, offer.PlatformFeeBPS)
	if err != nil {
		return nil, err
	}
	now := helper.GetTimestamp()
	return &PublisherOfferSettlement{
		RequestLogID: input.RequestLogID, OfferID: offer.Id, PublisherID: publisher.Id, ServiceID: service.Id, ConsumerUserID: input.ConsumerUserID,
		Model: offer.Model, PublisherDisplayName: publisher.DisplayName, ServiceName: service.Name, Region: service.Region, DataPolicySnapshot: offer.DataPolicySnapshot,
		Currency: offer.Currency, InputPriceMicros: offer.InputPriceMicros, OutputPriceMicros: offer.OutputPriceMicros, InputTokens: input.InputTokens, OutputTokens: input.OutputTokens,
		ConsumerAmountMicros: consumerAmount, PlatformFeeBPS: offer.PlatformFeeBPS, PlatformFeeAmountMicros: platformFee, PublisherPayableMicros: publisherPayable,
		Status: PublisherOfferSettlementStatusAccrued, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// CommunityOfferQuotaForMicros converts the fixed USD micro amount into the
// Router account quota unit. The value is rounded up so successful calls never
// undercharge due to a fractional quota conversion.
func CommunityOfferQuotaForMicros(amountMicros int64) (int64, float64, error) {
	if amountMicros <= 0 {
		return 0, 0, nil
	}
	rate, err := GetBillingCurrencyChargeRate(BillingCurrencyCodeUSD)
	if err != nil {
		return 0, 0, err
	}
	quota := int64(math.Ceil(float64(amountMicros) * rate / 1_000_000))
	if quota < 1 {
		quota = 1
	}
	return quota, rate, nil
}

func validPublisherSettlementDeliveryStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PublisherSettlementDeliveryStatusPrepared, PublisherSettlementDeliveryStatusReady, PublisherSettlementDeliveryStatusDelivered, PublisherSettlementDeliveryStatusCancelled, PublisherSettlementDeliveryStatusException, PublisherSettlementDeliveryStatusResolved:
		return true
	default:
		return false
	}
}

func normalizedPublisherOfferSettlementInput(input PublisherOfferSettlementInput) (PublisherOfferSettlementInput, error) {
	input.RequestLogID = strings.TrimSpace(input.RequestLogID)
	input.OfferID = strings.TrimSpace(input.OfferID)
	input.ConsumerUserID = strings.TrimSpace(input.ConsumerUserID)
	if input.RequestLogID == "" || input.OfferID == "" || input.ConsumerUserID == "" {
		return input, errors.New("结算请求、报价和消费者不能为空")
	}
	if input.InputTokens < 0 || input.OutputTokens < 0 || input.InputTokens+input.OutputTokens <= 0 {
		return input, errors.New("结算 token 用量必须为正数")
	}
	return input, nil
}

func validateExistingPublisherSettlementDelivery(existing *PublisherSettlementDelivery, input PublisherOfferSettlementInput) error {
	if existing == nil {
		return errors.New("结算投递不能为空")
	}
	if existing.OfferID != input.OfferID || existing.ConsumerUserID != input.ConsumerUserID || existing.InputTokens != input.InputTokens || existing.OutputTokens != input.OutputTokens {
		return errors.New("同一请求不能使用不同的社区服务结算投递内容")
	}
	return nil
}

func publisherSettlementDeliveryMatchesSettlement(delivery *PublisherSettlementDelivery, settlement *PublisherOfferSettlement) bool {
	if delivery == nil || settlement == nil {
		return false
	}
	return delivery.RequestLogID == settlement.RequestLogID &&
		delivery.OfferID == settlement.OfferID &&
		delivery.PublisherID == settlement.PublisherID &&
		delivery.ServiceID == settlement.ServiceID &&
		delivery.ConsumerUserID == settlement.ConsumerUserID &&
		delivery.Model == settlement.Model &&
		delivery.PublisherDisplayName == settlement.PublisherDisplayName &&
		delivery.ServiceName == settlement.ServiceName &&
		delivery.Region == settlement.Region &&
		delivery.DataPolicySnapshot == settlement.DataPolicySnapshot &&
		delivery.Currency == settlement.Currency &&
		delivery.InputPriceMicros == settlement.InputPriceMicros &&
		delivery.OutputPriceMicros == settlement.OutputPriceMicros &&
		delivery.InputTokens == settlement.InputTokens &&
		delivery.OutputTokens == settlement.OutputTokens &&
		delivery.ConsumerAmountMicros == settlement.ConsumerAmountMicros &&
		delivery.PlatformFeeBPS == settlement.PlatformFeeBPS &&
		delivery.PlatformFeeAmountMicros == settlement.PlatformFeeAmountMicros &&
		delivery.PublisherPayableMicros == settlement.PublisherPayableMicros
}

func publisherOfferSettlementFromDelivery(delivery *PublisherSettlementDelivery, now int64) *PublisherOfferSettlement {
	return &PublisherOfferSettlement{
		RequestLogID: delivery.RequestLogID, OfferID: delivery.OfferID, PublisherID: delivery.PublisherID, ServiceID: delivery.ServiceID, ConsumerUserID: delivery.ConsumerUserID,
		Model: delivery.Model, PublisherDisplayName: delivery.PublisherDisplayName, ServiceName: delivery.ServiceName, Region: delivery.Region, DataPolicySnapshot: delivery.DataPolicySnapshot,
		Currency: delivery.Currency, InputPriceMicros: delivery.InputPriceMicros, OutputPriceMicros: delivery.OutputPriceMicros, InputTokens: delivery.InputTokens, OutputTokens: delivery.OutputTokens,
		ConsumerAmountMicros: delivery.ConsumerAmountMicros, PlatformFeeBPS: delivery.PlatformFeeBPS, PlatformFeeAmountMicros: delivery.PlatformFeeAmountMicros, PublisherPayableMicros: delivery.PublisherPayableMicros,
		Status: PublisherOfferSettlementStatusAccrued, CreatedAt: now, UpdatedAt: now,
	}
}

func createPublisherOfferSettlementFromSnapshot(snapshot *PublisherOfferSettlement) (*PublisherOfferSettlement, error) {
	if snapshot == nil {
		return nil, errors.New("结算快照不能为空")
	}
	if DB == nil {
		return nil, errors.New("database handle is nil")
	}
	existing := &PublisherOfferSettlement{}
	if err := DB.Where("request_log_id = ?", snapshot.RequestLogID).First(existing).Error; err == nil {
		if !publisherOfferSettlementMatchesSnapshot(existing, snapshot) {
			return nil, errors.New("请求已经存在不同的社区服务结算快照")
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(snapshot)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return snapshot, nil
	}
	if err := DB.Where("request_log_id = ?", snapshot.RequestLogID).First(existing).Error; err != nil {
		return nil, err
	}
	if !publisherOfferSettlementMatchesSnapshot(existing, snapshot) {
		return nil, errors.New("请求已经存在不同的社区服务结算快照")
	}
	return existing, nil
}

func createPublisherOfferSettlementFromDelivery(delivery *PublisherSettlementDelivery) (*PublisherOfferSettlement, error) {
	if delivery == nil {
		return nil, errors.New("结算投递不能为空")
	}
	settledAt := delivery.ReadyAt
	if settledAt == 0 {
		settledAt = helper.GetTimestamp()
	}
	return createPublisherOfferSettlementFromSnapshot(publisherOfferSettlementFromDelivery(delivery, settledAt))
}

// PreparePublisherSettlementDelivery must run before consumer balance is
// consumed. If this durable handoff cannot be stored, Relay must not create a
// charge that could lose its publisher payable after a process interruption.
func PreparePublisherSettlementDelivery(input PublisherOfferSettlementInput) (*PublisherSettlementDelivery, error) {
	input, err := normalizedPublisherOfferSettlementInput(input)
	if err != nil {
		return nil, err
	}
	existing := &PublisherSettlementDelivery{}
	if err := DB.Where("request_log_id = ?", input.RequestLogID).First(existing).Error; err == nil {
		if err := validateExistingPublisherSettlementDelivery(existing, input); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	quote, err := QuotePublisherOfferSettlement(input)
	if err != nil {
		return nil, err
	}
	return PreparePublisherSettlementDeliveryFromQuote(quote)
}

// PreparePublisherSettlementDeliveryFromQuote stores exactly the commercial
// facts used to charge the consumer. Relay must use this after it has quoted a
// completed request, so a concurrent offer change cannot make the charge and
// the publisher payable describe two different prices.
func PreparePublisherSettlementDeliveryFromQuote(quote *PublisherOfferSettlement) (*PublisherSettlementDelivery, error) {
	return preparePublisherSettlementDeliveryFromQuote(quote, publisherSettlementDeliveryCharge{})
}

// PreparePublisherSettlementDeliveryForCharge persists the exact balance
// debit alongside the immutable commercial snapshot. This is the Relay entry
// point: a later cancellation can refund only the amount known to have been
// charged, never a value reconstructed from a changed exchange rate.
func PreparePublisherSettlementDeliveryForCharge(quote *PublisherOfferSettlement, quota int64, chargeRate float64) (*PublisherSettlementDelivery, error) {
	return preparePublisherSettlementDeliveryFromQuote(quote, publisherSettlementDeliveryCharge{Quota: quota, Rate: chargeRate, Recorded: true})
}

func preparePublisherSettlementDeliveryFromQuote(quote *PublisherOfferSettlement, charge publisherSettlementDeliveryCharge) (*PublisherSettlementDelivery, error) {
	if quote == nil {
		return nil, errors.New("结算报价不能为空")
	}
	if charge.Quota < 0 || (charge.Recorded && charge.Quota > 0 && charge.Rate <= 0) {
		return nil, errors.New("结算扣费快照无效")
	}
	input, err := normalizedPublisherOfferSettlementInput(PublisherOfferSettlementInput{
		RequestLogID: quote.RequestLogID, OfferID: quote.OfferID, ConsumerUserID: quote.ConsumerUserID,
		InputTokens: quote.InputTokens, OutputTokens: quote.OutputTokens,
	})
	if err != nil {
		return nil, err
	}
	existing := &PublisherSettlementDelivery{}
	if err := DB.Where("request_log_id = ?", input.RequestLogID).First(existing).Error; err == nil {
		if err := validateExistingPublisherSettlementDelivery(existing, input); err != nil {
			return nil, err
		}
		if !publisherSettlementDeliveryMatchesSettlement(existing, quote) || !publisherSettlementDeliveryMatchesCharge(existing, charge) {
			return nil, errors.New("同一请求不能使用不同的社区服务结算投递快照")
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := helper.GetTimestamp()
	row := &PublisherSettlementDelivery{
		RequestLogID: quote.RequestLogID, OfferID: quote.OfferID, PublisherID: quote.PublisherID, ServiceID: quote.ServiceID, ConsumerUserID: quote.ConsumerUserID,
		Model: quote.Model, PublisherDisplayName: quote.PublisherDisplayName, ServiceName: quote.ServiceName, Region: quote.Region, DataPolicySnapshot: quote.DataPolicySnapshot,
		Currency: quote.Currency, InputPriceMicros: quote.InputPriceMicros, OutputPriceMicros: quote.OutputPriceMicros, InputTokens: quote.InputTokens, OutputTokens: quote.OutputTokens,
		ConsumerAmountMicros: quote.ConsumerAmountMicros, PlatformFeeBPS: quote.PlatformFeeBPS, PlatformFeeAmountMicros: quote.PlatformFeeAmountMicros, PublisherPayableMicros: quote.PublisherPayableMicros,
		PlannedQuota: charge.Quota, ChargeRate: charge.Rate, ChargeRecorded: false,
		Status: PublisherSettlementDeliveryStatusPrepared, CreatedAt: now, UpdatedAt: now,
	}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		return row, nil
	}
	if err := DB.Where("request_log_id = ?", input.RequestLogID).First(existing).Error; err != nil {
		return nil, err
	}
	if err := validateExistingPublisherSettlementDelivery(existing, input); err != nil {
		return nil, errors.New("同一请求不能使用不同的社区服务结算投递内容")
	}
	if !publisherSettlementDeliveryMatchesSettlement(existing, quote) || !publisherSettlementDeliveryMatchesCharge(existing, charge) {
		return nil, errors.New("同一请求不能使用不同的社区服务结算投递快照")
	}
	return existing, nil
}

func publisherSettlementDeliveryMatchesCharge(delivery *PublisherSettlementDelivery, charge publisherSettlementDeliveryCharge) bool {
	if delivery == nil {
		return false
	}
	return delivery.PlannedQuota == charge.Quota && delivery.ChargeRate == charge.Rate
}

// RecordPublisherSettlementDeliveryCharge confirms the amount that the
// balance-lot transaction actually consumed. It is deliberately separate from
// prepare: a planned debit is not a refund-safe financial fact until the lot
// transaction has committed.
func RecordPublisherSettlementDeliveryCharge(requestLogID string, chargedQuota int64) (*PublisherSettlementDelivery, error) {
	requestLogID = strings.TrimSpace(requestLogID)
	if requestLogID == "" || chargedQuota < 0 {
		return nil, errors.New("结算请求和实际扣减额度必须有效")
	}
	row := &PublisherSettlementDelivery{}
	now := helper.GetTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
			return err
		}
		if row.Status != PublisherSettlementDeliveryStatusPrepared {
			return errors.New("当前结算投递不能确认扣费")
		}
		if chargedQuota > row.PlannedQuota {
			return errors.New("实际扣减额度不能超过计划额度")
		}
		if row.ChargeRecorded {
			if row.ChargedQuota != chargedQuota {
				return errors.New("同一结算投递不能记录不同的实际扣减额度")
			}
			return nil
		}
		if err := tx.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", requestLogID).Updates(map[string]any{
			"charged_quota": chargedQuota, "charge_recorded": true, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		row.ChargedQuota, row.ChargeRecorded, row.UpdatedAt = chargedQuota, true, now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func communityOfferConsumeLogExists(requestLogID string) (bool, error) {
	if LOG_DB == nil {
		return false, errors.New("调用日志数据库不可用")
	}
	var count int64
	if err := LOG_DB.Model(&Log{}).Where("id = ? AND type = ? AND upstream_source = ?", strings.TrimSpace(requestLogID), LogTypeConsume, "community_offer").Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func markPublisherSettlementDeliveryReady(requestLogID string) (*PublisherSettlementDelivery, error) {
	requestLogID = strings.TrimSpace(requestLogID)
	if requestLogID == "" {
		return nil, errors.New("结算请求不能为空")
	}
	row := &PublisherSettlementDelivery{}
	if err := DB.Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
		return nil, err
	}
	if row.Status == PublisherSettlementDeliveryStatusDelivered {
		return row, nil
	}
	if row.Status == PublisherSettlementDeliveryStatusCancelled {
		logged, err := communityOfferConsumeLogExists(requestLogID)
		if err != nil {
			return nil, err
		}
		if logged {
			return nil, errors.New("已取消的结算投递出现消费日志，需要人工核对退款与发布者应收")
		}
		return row, nil
	}
	if row.Status == PublisherSettlementDeliveryStatusException || row.Status == PublisherSettlementDeliveryStatusResolved {
		return nil, errors.New("结算投递已进入异常处置流程，不能自动生成发布者应收")
	}
	logged, err := communityOfferConsumeLogExists(requestLogID)
	if err != nil {
		return nil, err
	}
	if !logged {
		return nil, errors.New("社区调用日志尚未落库，结算投递保持待确认")
	}
	now := helper.GetTimestamp()
	if err := DB.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", requestLogID).Updates(map[string]any{
		"status": PublisherSettlementDeliveryStatusReady, "ready_at": now, "updated_at": now, "last_error": "",
	}).Error; err != nil {
		return nil, err
	}
	row.Status, row.ReadyAt, row.UpdatedAt, row.LastError = PublisherSettlementDeliveryStatusReady, now, now, ""
	return row, nil
}

// MarkPublisherSettlementDeliveryReady performs the log existence check that
// separates a charged but interrupted request from a settled publisher claim.
func MarkPublisherSettlementDeliveryReady(requestLogID string) (*PublisherSettlementDelivery, error) {
	return markPublisherSettlementDeliveryReady(requestLogID)
}

// CancelPublisherSettlementDelivery closes a stale prepared delivery only
// after confirming that no consume log exists. It creates an independent,
// idempotent balance lot instead of mutating the original consumed lots,
// because a request can consume several lots and that allocation is not a
// stable refund ownership record.
func CancelPublisherSettlementDelivery(requestLogID string, actorUserID string, reason string) (*PublisherSettlementDelivery, error) {
	requestLogID = strings.TrimSpace(requestLogID)
	actorUserID = strings.TrimSpace(actorUserID)
	reason = strings.TrimSpace(reason)
	if requestLogID == "" || actorUserID == "" {
		return nil, errors.New("结算请求和操作人不能为空")
	}
	if reason == "" || len(reason) > 1024 {
		return nil, errors.New("取消原因不能为空且不能超过 1024 个字符")
	}
	logged, err := communityOfferConsumeLogExists(requestLogID)
	if err != nil {
		return nil, err
	}
	if logged {
		return nil, errors.New("社区调用日志已存在，不能取消结算投递")
	}
	now := helper.GetTimestamp()
	row := &PublisherSettlementDelivery{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
			return err
		}
		if row.Status == PublisherSettlementDeliveryStatusCancelled {
			return nil
		}
		if row.Status != PublisherSettlementDeliveryStatusPrepared {
			return errors.New("只有待确认的结算投递可以取消")
		}
		if now-row.CreatedAt < PublisherSettlementDeliveryCancellationMinimumAgeSeconds {
			return fmt.Errorf("结算投递至少等待 %d 分钟后才能人工取消", PublisherSettlementDeliveryCancellationMinimumAgeSeconds/60)
		}
		if !row.ChargeRecorded {
			return errors.New("该结算投递缺少扣费快照，不能自动退款")
		}
		refundLotID := ""
		if row.ChargedQuota > 0 {
			lot, _, err := CreditUserBalanceLotWithDB(tx, UserBalanceLotCreditInput{
				UserID: row.ConsumerUserID, SourceType: UserBalanceLotSourceCommunityOfferRefund, SourceID: row.RequestLogID,
				TotalAmount: row.ChargedQuota, GrantedAt: now,
			})
			if err != nil {
				return err
			}
			if lot.UserID != row.ConsumerUserID || lot.TotalAmount != row.ChargedQuota {
				return errors.New("退款额度批次与结算投递不一致")
			}
			refundLotID = lot.Id
		}
		updates := map[string]any{
			"status": PublisherSettlementDeliveryStatusCancelled, "cancelled_at": now, "cancelled_by": actorUserID,
			"cancellation_reason": reason, "refunded_quota": row.ChargedQuota, "refund_lot_id": refundLotID,
			"last_error": "", "updated_at": now,
		}
		if err := tx.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", row.RequestLogID).Updates(updates).Error; err != nil {
			return err
		}
		if err := recordPublisherAuditWithDB(tx, row.PublisherID, actorUserID, "settlement_delivery_cancelled", "publisher_settlement_delivery", row.RequestLogID, reason); err != nil {
			return err
		}
		row.Status, row.CancelledAt, row.CancelledBy, row.CancellationReason = PublisherSettlementDeliveryStatusCancelled, now, actorUserID, reason
		row.RefundedQuota, row.RefundLotID, row.LastError, row.UpdatedAt = row.ChargedQuota, refundLotID, "", now
		return nil
	})
	if err != nil {
		return nil, err
	}
	RefreshUserGroupCaches(row.ConsumerUserID)
	return row, nil
}

// ReconcileCancelledPublisherSettlementDelivery detects the only dangerous
// cross-store contradiction in the simplified cancellation flow: a consume
// log materializes after a refund has been issued. Detection never changes
// money or creates a publisher payable; it only moves the record to a visible
// exception state for an operator to decide.
func ReconcileCancelledPublisherSettlementDelivery(requestLogID string) (*PublisherSettlementDelivery, error) {
	requestLogID = strings.TrimSpace(requestLogID)
	if requestLogID == "" {
		return nil, errors.New("结算请求不能为空")
	}
	row := &PublisherSettlementDelivery{}
	if err := DB.Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
		return nil, err
	}
	if row.Status == PublisherSettlementDeliveryStatusException || row.Status == PublisherSettlementDeliveryStatusResolved {
		return row, nil
	}
	if row.Status != PublisherSettlementDeliveryStatusCancelled {
		return nil, errors.New("只有已取消的结算投递可以进行异常核对")
	}
	logged, err := communityOfferConsumeLogExists(requestLogID)
	if err != nil {
		return nil, err
	}
	if !logged {
		return row, nil
	}
	now := helper.GetTimestamp()
	reason := "退款后发现社区调用消费日志，已停止自动结算，等待人工处置"
	if err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ? AND status = ?", requestLogID, PublisherSettlementDeliveryStatusCancelled).Updates(map[string]any{
			"status": PublisherSettlementDeliveryStatusException, "exception_detected_at": now, "exception_reason": reason,
			"last_error": reason, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return recordPublisherAuditWithDB(tx, row.PublisherID, "", "settlement_delivery_exception_detected", "publisher_settlement_delivery", row.RequestLogID, reason)
	}); err != nil {
		return nil, err
	}
	if err := DB.Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// ResolvePublisherSettlementDeliveryException closes the operator workflow
// only. It intentionally does not issue a second charge, reverse a refund, or
// create a publisher payable: each of those decisions requires evidence and a
// separately auditable financial operation.
func ResolvePublisherSettlementDeliveryException(requestLogID string, actorUserID string, resolution string) (*PublisherSettlementDelivery, error) {
	requestLogID = strings.TrimSpace(requestLogID)
	actorUserID = strings.TrimSpace(actorUserID)
	resolution = strings.TrimSpace(resolution)
	if requestLogID == "" || actorUserID == "" {
		return nil, errors.New("结算请求和操作人不能为空")
	}
	if resolution == "" || len(resolution) > 2048 {
		return nil, errors.New("处理结论不能为空且不能超过 2048 个字符")
	}
	now := helper.GetTimestamp()
	row := &PublisherSettlementDelivery{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_log_id = ?", requestLogID).First(row).Error; err != nil {
			return err
		}
		if row.Status == PublisherSettlementDeliveryStatusResolved {
			return nil
		}
		if row.Status != PublisherSettlementDeliveryStatusException {
			return errors.New("只有异常待处置的结算投递可以确认处理")
		}
		updates := map[string]any{
			"status": PublisherSettlementDeliveryStatusResolved, "resolved_at": now, "resolved_by": actorUserID,
			"resolution": resolution, "updated_at": now,
		}
		if err := tx.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", requestLogID).Updates(updates).Error; err != nil {
			return err
		}
		if err := recordPublisherAuditWithDB(tx, row.PublisherID, actorUserID, "settlement_delivery_exception_resolved", "publisher_settlement_delivery", row.RequestLogID, resolution); err != nil {
			return err
		}
		row.Status, row.ResolvedAt, row.ResolvedBy, row.Resolution, row.UpdatedAt = PublisherSettlementDeliveryStatusResolved, now, actorUserID, resolution, now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func DeliverPublisherSettlement(requestLogID string) (*PublisherSettlementDelivery, error) {
	row := &PublisherSettlementDelivery{}
	if err := DB.Where("request_log_id = ?", strings.TrimSpace(requestLogID)).First(row).Error; err != nil {
		return nil, err
	}
	if row.Status == PublisherSettlementDeliveryStatusDelivered || row.Status == PublisherSettlementDeliveryStatusCancelled {
		return row, nil
	}
	if row.Status != PublisherSettlementDeliveryStatusReady {
		return nil, errors.New("结算投递尚未确认调用日志")
	}
	_, err := createPublisherOfferSettlementFromDelivery(row)
	now := helper.GetTimestamp()
	updates := map[string]any{"attempts": row.Attempts + 1, "updated_at": now}
	if err != nil {
		updates["last_error"] = truncatePublisherSettlementDeliveryError(err.Error())
		if updateErr := DB.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", row.RequestLogID).Updates(updates).Error; updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	updates["status"] = PublisherSettlementDeliveryStatusDelivered
	updates["delivered_at"] = now
	updates["last_error"] = ""
	if err := DB.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", row.RequestLogID).Updates(updates).Error; err != nil {
		return nil, err
	}
	row.Status, row.Attempts, row.DeliveredAt, row.UpdatedAt, row.LastError = PublisherSettlementDeliveryStatusDelivered, row.Attempts+1, now, now, ""
	return row, nil
}

func truncatePublisherSettlementDeliveryError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1024 {
		return value
	}
	return value[:1024]
}

func ListPublisherSettlementDeliveryCandidates(limit int) ([]PublisherSettlementDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows := make([]PublisherSettlementDelivery, 0, limit)
	if err := DB.Where("status IN ?", []string{PublisherSettlementDeliveryStatusPrepared, PublisherSettlementDeliveryStatusReady}).Order("updated_at asc, request_log_id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListPublisherSettlementCancellationObservationCandidates(limit int, since int64) ([]PublisherSettlementDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if since <= 0 {
		since = helper.GetTimestamp() - PublisherSettlementDeliveryCancellationObservationSeconds
	}
	rows := make([]PublisherSettlementDelivery, 0, limit)
	if err := DB.Where("status = ? AND cancelled_at >= ?", PublisherSettlementDeliveryStatusCancelled, since).Order("cancelled_at asc, request_log_id asc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListPublisherSettlementDeliveriesForAdmin(status string, limit int) ([]PublisherSettlementDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := DB.Order("updated_at desc, request_log_id desc")
	if normalized := strings.TrimSpace(status); normalized != "" {
		if !validPublisherSettlementDeliveryStatus(normalized) {
			return nil, errors.New("结算投递状态筛选无效")
		}
		query = query.Where("status = ?", normalized)
	}
	rows := make([]PublisherSettlementDelivery, 0)
	if err := query.Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// PublishedServiceOfferCatalogItem is the consumer-safe projection of an
// approved community service. It intentionally excludes endpoints, encrypted
// credentials and the operational identifiers needed to operate the upstream.
type PublishedServiceOfferCatalogItem struct {
	OfferID              string `json:"offer_id"`
	Model                string `json:"model"`
	Scope                string `json:"scope"`
	Currency             string `json:"currency"`
	InputPriceMicros     int64  `json:"input_price_micros"`
	OutputPriceMicros    int64  `json:"output_price_micros"`
	PlatformFeeBPS       int    `json:"platform_fee_bps"`
	PublisherDisplayName string `json:"publisher_display_name"`
	PublisherLevel       string `json:"publisher_level"`
	ServiceName          string `json:"service_name"`
	Region               string `json:"region"`
	DataPolicy           string `json:"data_policy"`
	CapacityPolicy       string `json:"capacity_policy"`
	PublishedAt          int64  `json:"published_at"`
}

type AdminSupplyOfferView struct {
	OfferId     string `json:"offer_id"`
	OfferStatus string `json:"offer_status"`
	OfferScope  string `json:"offer_scope"`
	Model       string `json:"model"`
	UpdatedAt   int64  `json:"updated_at"`
}

type AdminSupplyChannelView struct {
	PublisherId          string                 `json:"publisher_id"`
	PublisherUserId      string                 `json:"publisher_user_id"`
	PublisherDisplayName string                 `json:"publisher_display_name"`
	PublisherStatus      string                 `json:"publisher_status"`
	PublisherLevel       string                 `json:"publisher_level"`
	ServiceId            string                 `json:"service_id"`
	ServiceName          string                 `json:"service_name"`
	ServiceProtocol      string                 `json:"service_protocol"`
	ServiceRegion        string                 `json:"service_region"`
	ServiceStatus        string                 `json:"service_status"`
	LastCheckedAt        int64                  `json:"last_checked_at"`
	LastCheckOK          bool                   `json:"last_check_ok"`
	LastCheckError       string                 `json:"last_check_error"`
	UpdatedAt            int64                  `json:"updated_at"`
	Models               []string               `json:"models" gorm:"-"`
	Offers               []AdminSupplyOfferView `json:"offers" gorm:"-"`
	OfferCount           int                    `json:"offer_count" gorm:"-"`
}

type PublisherAuditLog struct {
	Id          string `json:"id" gorm:"type:char(36);primaryKey"`
	PublisherId string `json:"publisher_id" gorm:"type:char(36);not null;index"`
	ActorUserId string `json:"actor_user_id" gorm:"type:char(36);not null;default:'';index"`
	Action      string `json:"action" gorm:"type:varchar(64);not null;index"`
	SubjectType string `json:"subject_type" gorm:"type:varchar(64);not null;index"`
	SubjectId   string `json:"subject_id" gorm:"type:char(36);not null;default:'';index"`
	Detail      string `json:"detail" gorm:"type:text;not null;default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;not null;index"`
}

func (PublisherAuditLog) TableName() string { return PublisherAuditLogsTableName }

type PublisherProfileInput struct {
	DisplayName  string `json:"display_name"`
	ContactEmail string `json:"contact_email"`
}

type PublisherServiceInput struct {
	SourcePersonalConnectionId string                  `json:"source_personal_connection_id"`
	Name                       string                  `json:"name"`
	Protocol                   string                  `json:"protocol"`
	BaseURL                    string                  `json:"base_url"`
	APIKey                     string                  `json:"api_key"`
	Region                     string                  `json:"region"`
	DataPolicy                 json.RawMessage         `json:"data_policy"`
	CapacityPolicy             json.RawMessage         `json:"capacity_policy"`
	Models                     []PublisherServiceModel `json:"models"`
}

type ServiceOfferInput struct {
	ServiceId         string `json:"service_id"`
	Model             string `json:"model"`
	Scope             string `json:"scope"`
	Currency          string `json:"currency"`
	InputPriceMicros  int64  `json:"input_price_micros"`
	OutputPriceMicros int64  `json:"output_price_micros"`
}

func validPublisherStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PublisherStatusDraft, PublisherStatusReviewing, PublisherStatusActive, PublisherStatusRestricted, PublisherStatusSuspended, PublisherStatusClosed:
		return true
	default:
		return false
	}
}

func canManagePublisher(status string) bool {
	return status != PublisherStatusSuspended && status != PublisherStatusClosed
}

func validPublisherServiceStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PublisherServiceStatusDraft, PublisherServiceStatusReady, PublisherServiceStatusSuspended, PublisherServiceStatusOffline:
		return true
	default:
		return false
	}
}

func validServiceOfferStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case ServiceOfferStatusDraft, ServiceOfferStatusReviewing, ServiceOfferStatusPublished, ServiceOfferStatusSuspended, ServiceOfferStatusOffline, ServiceOfferStatusRejected:
		return true
	default:
		return false
	}
}

func normalizePolicyJSON(value json.RawMessage) (string, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" {
		return "{}", nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", errors.New("数据策略必须是有效 JSON")
	}
	if _, ok := decoded.(map[string]any); !ok {
		return "", errors.New("数据策略必须是 JSON 对象")
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func normalizePublisherServiceModels(rows []PublisherServiceModel) ([]PublisherServiceModel, error) {
	seen := make(map[string]struct{}, len(rows))
	result := make([]PublisherServiceModel, 0, len(rows))
	for _, item := range rows {
		item.Model = strings.TrimSpace(item.Model)
		item.UpstreamModel = strings.TrimSpace(item.UpstreamModel)
		item.Endpoint = NormalizeRequestedChannelModelEndpoint(item.Endpoint)
		item.MeteringUnit = strings.ToLower(strings.TrimSpace(item.MeteringUnit))
		if item.UpstreamModel == "" {
			item.UpstreamModel = item.Model
		}
		if item.MeteringUnit == "" {
			item.MeteringUnit = "token"
		}
		if item.Model == "" || item.UpstreamModel == "" {
			return nil, errors.New("模型和上游模型不能为空")
		}
		if item.Endpoint != ChannelModelEndpointChat && item.Endpoint != ChannelModelEndpointResponses && item.Endpoint != ChannelModelEndpointEmbeddings {
			return nil, errors.New("可信发布试点仅支持对话、Responses 和 Embeddings 端点")
		}
		if item.MeteringUnit != "token" {
			return nil, errors.New("可信发布试点仅支持 token 计量")
		}
		if _, ok := seen[item.Model]; ok {
			return nil, fmt.Errorf("模型 %s 重复", item.Model)
		}
		seen[item.Model] = struct{}{}
		item.Status = PublisherServiceModelStatusDraft
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil, errors.New("至少声明一个模型")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Model < result[j].Model })
	return result, nil
}

func normalizePublisherServiceInput(input PublisherServiceInput, requireCredential bool) (PublisherServiceInput, error) {
	input.SourcePersonalConnectionId = strings.TrimSpace(input.SourcePersonalConnectionId)
	input.Name = strings.TrimSpace(input.Name)
	input.Protocol = relaychannel.NormalizeProtocolName(input.Protocol)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.Region = strings.TrimSpace(input.Region)
	if input.Protocol == "" {
		input.Protocol = "openai"
	}
	if !IsPersonalProviderProtocol(input.Protocol) {
		return input, errors.New("模型服务协议不受支持")
	}
	if input.Name == "" || len(input.Name) > 96 {
		return input, errors.New("模型服务名称不能为空且不能超过 96 个字符")
	}
	if input.BaseURL == "" {
		return input, errors.New("模型服务 Base URL 不能为空")
	}
	if err := client.ValidatePersonalProviderBaseURL(input.BaseURL); err != nil {
		return input, fmt.Errorf("模型服务地址无效: %w", err)
	}
	if len(input.Region) > 64 {
		return input, errors.New("服务区域不能超过 64 个字符")
	}
	if requireCredential && strings.TrimSpace(input.APIKey) == "" {
		return input, errors.New("API Key 不能为空")
	}
	dataPolicy, err := normalizePolicyJSON(input.DataPolicy)
	if err != nil {
		return input, err
	}
	capacityPolicy, err := normalizePolicyJSON(input.CapacityPolicy)
	if err != nil {
		return input, fmt.Errorf("容量策略无效: %w", err)
	}
	models, err := normalizePublisherServiceModels(input.Models)
	if err != nil {
		return input, err
	}
	input.DataPolicy = json.RawMessage(dataPolicy)
	input.CapacityPolicy = json.RawMessage(capacityPolicy)
	input.Models = models
	return input, nil
}

// Trusted publishing starts with Router's reviewed standard model directory.
// Community namespaces are added only with the later public catalog flow.
func validateTrustedPublisherServiceModelsWithDB(db *gorm.DB, rows []PublisherServiceModel) error {
	if db == nil {
		return errors.New("database handle is nil")
	}
	for _, row := range rows {
		var count int64
		if err := db.Model(&ProviderModel{}).
			Where("model = ? AND is_deleted = ? AND status = ?", row.Model, false, ProviderModelStatusActive).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("模型 %s 不在当前标准模型目录中", row.Model)
		}
	}
	return nil
}

func publisherCredentialKey(secret string) []byte {
	sum := sha256.Sum256([]byte("router.publisher-service-credentials.v1:" + secret))
	return sum[:]
}

func encryptPublisherServiceCredential(value string) (string, error) {
	secret := strings.TrimSpace(config.JWTSecret)
	if secret == "" {
		return "", errors.New("auth.jwt_secret 未配置，无法保存模型服务凭据")
	}
	block, err := aes.NewCipher(publisherCredentialKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(strings.TrimSpace(value)), nil)
	return "v1:" + base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func decryptPublisherServiceCredential(value string) (string, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(value), "v1:")
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", errors.New("模型服务凭据格式无效")
	}
	secrets := append([]string{strings.TrimSpace(config.JWTSecret)}, config.JWTFallbackSecrets...)
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		block, blockErr := aes.NewCipher(publisherCredentialKey(secret))
		if blockErr != nil {
			continue
		}
		gcm, gcmErr := cipher.NewGCM(block)
		if gcmErr != nil || len(bytes) < gcm.NonceSize() {
			continue
		}
		plaintext, openErr := gcm.Open(nil, bytes[:gcm.NonceSize()], bytes[gcm.NonceSize():], nil)
		if openErr == nil {
			return string(plaintext), nil
		}
	}
	return "", errors.New("无法解密模型服务凭据，请轮换 API Key")
}

func recordPublisherAuditWithDB(db *gorm.DB, publisherID string, actorUserID string, action string, subjectType string, subjectID string, detail string) error {
	if db == nil {
		return errors.New("database handle is nil")
	}
	return db.Create(&PublisherAuditLog{Id: random.GetUUID(), PublisherId: strings.TrimSpace(publisherID), ActorUserId: strings.TrimSpace(actorUserID), Action: strings.TrimSpace(action), SubjectType: strings.TrimSpace(subjectType), SubjectId: strings.TrimSpace(subjectID), Detail: strings.TrimSpace(detail), CreatedAt: helper.GetTimestamp()}).Error
}

func publisherForUserWithDB(db *gorm.DB, userID string) (*Publisher, error) {
	row := &Publisher{}
	if err := db.Where("user_id = ?", strings.TrimSpace(userID)).First(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func GetPublisherForUser(userID string) (*Publisher, error) {
	return publisherForUserWithDB(DB, userID)
}

func EnsurePublisherProfile(userID string, input PublisherProfileInput) (*Publisher, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("用户不能为空")
	}
	user := User{}
	if err := DB.Select("id", "username", "display_name", "email", "wallet_identity_did").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	did := ""
	if user.WalletIdentityDID != nil {
		did = NormalizeWalletIdentityDID(*user.WalletIdentityDID)
	}
	if did == "" {
		return nil, errors.New("发布模型服务前，请先完成钱包身份验证")
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(user.DisplayName)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(user.Username)
	}
	contactEmail := strings.TrimSpace(input.ContactEmail)
	if contactEmail == "" {
		contactEmail = strings.TrimSpace(user.Email)
	}
	if displayName == "" || len(displayName) > 96 {
		return nil, errors.New("发布者名称不能为空且不能超过 96 个字符")
	}
	if _, err := mail.ParseAddress(contactEmail); err != nil || !strings.Contains(contactEmail, "@") {
		return nil, errors.New("请提供有效联系邮箱")
	}
	now := helper.GetTimestamp()
	row := &Publisher{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("user_id = ?", userID).First(row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			*row = Publisher{Id: random.GetUUID(), UserId: userID, WalletIdentityDID: did, DisplayName: displayName, ContactEmail: contactEmail, Level: PublisherLevelVerifiedCommunity, Status: PublisherStatusDraft, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
			return recordPublisherAuditWithDB(tx, row.Id, userID, "profile_created", "publisher", row.Id, "")
		}
		if err != nil {
			return err
		}
		if !canManagePublisher(row.Status) {
			return errors.New("当前发布者状态不允许修改资料")
		}
		updates := map[string]any{"wallet_identity_did": did, "display_name": displayName, "contact_email": contactEmail, "updated_at": now}
		if err := tx.Model(&Publisher{}).Where("id = ? AND user_id = ?", row.Id, userID).Updates(updates).Error; err != nil {
			return err
		}
		row.WalletIdentityDID, row.DisplayName, row.ContactEmail, row.UpdatedAt = did, displayName, contactEmail, now
		return recordPublisherAuditWithDB(tx, row.Id, userID, "profile_updated", "publisher", row.Id, "")
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func SubmitPublisherApplication(userID string) (*Publisher, error) {
	row, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if row.Status == PublisherStatusActive {
		return nil, errors.New("发布者已通过审核")
	}
	if !canManagePublisher(row.Status) {
		return nil, errors.New("当前发布者状态不允许提交审核")
	}
	if strings.TrimSpace(row.DisplayName) == "" || strings.TrimSpace(row.ContactEmail) == "" {
		return nil, errors.New("请先完善发布者资料")
	}
	now := helper.GetTimestamp()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Publisher{}).Where("id = ? AND user_id = ?", row.Id, strings.TrimSpace(userID)).Updates(map[string]any{"status": PublisherStatusReviewing, "submitted_at": now, "review_note": "", "updated_at": now}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, row.Id, userID, "application_submitted", "publisher", row.Id, "")
	}); err != nil {
		return nil, err
	}
	row.Status, row.SubmittedAt, row.ReviewNote, row.UpdatedAt = PublisherStatusReviewing, now, "", now
	return row, nil
}

func ListPublisherServices(userID string) ([]PublisherService, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	return listPublisherServicesByPublisherWithDB(DB, publisher.Id)
}

func listPublisherServicesByPublisherWithDB(db *gorm.DB, publisherID string) ([]PublisherService, error) {
	rows := make([]PublisherService, 0)
	if err := db.Where("publisher_id = ?", strings.TrimSpace(publisherID)).Order("updated_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].Id)
		rows[i].CredentialConfigured = strings.TrimSpace(rows[i].CredentialEncrypted) != ""
	}
	models := make([]PublisherServiceModel, 0)
	if err := db.Where("service_id IN ?", ids).Order("model asc").Find(&models).Error; err != nil {
		return nil, err
	}
	byService := make(map[string][]PublisherServiceModel, len(rows))
	for _, item := range models {
		byService[item.ServiceId] = append(byService[item.ServiceId], item)
	}
	for i := range rows {
		rows[i].Models = byService[rows[i].Id]
		if rows[i].Models == nil {
			rows[i].Models = []PublisherServiceModel{}
		}
	}
	return rows, nil
}

func getPublisherServiceForUser(userID string, serviceID string) (*Publisher, *PublisherService, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, nil, err
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", strings.TrimSpace(serviceID), publisher.Id).First(service).Error; err != nil {
		return nil, nil, err
	}
	service.CredentialConfigured = strings.TrimSpace(service.CredentialEncrypted) != ""
	return publisher, service, nil
}

func CreatePublisherService(userID string, input PublisherServiceInput) (*PublisherService, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if !canManagePublisher(publisher.Status) {
		return nil, errors.New("当前发布者状态不允许创建模型服务")
	}
	input, err = normalizePublisherServiceInput(input, true)
	if err != nil {
		return nil, err
	}
	if input.SourcePersonalConnectionId != "" {
		if _, err := GetPersonalProviderConnection(userID, input.SourcePersonalConnectionId); err != nil {
			return nil, errors.New("来源个人渠道不存在或无权访问")
		}
	}
	if err := validateTrustedPublisherServiceModelsWithDB(DB, input.Models); err != nil {
		return nil, err
	}
	credential, err := encryptPublisherServiceCredential(input.APIKey)
	if err != nil {
		return nil, err
	}
	now := helper.GetTimestamp()
	service := &PublisherService{Id: random.GetUUID(), PublisherId: publisher.Id, SourcePersonalConnectionId: input.SourcePersonalConnectionId, Name: input.Name, Protocol: input.Protocol, BaseURL: input.BaseURL, CredentialEncrypted: credential, Region: input.Region, DataPolicy: string(input.DataPolicy), CapacityPolicy: string(input.CapacityPolicy), Status: PublisherServiceStatusDraft, CreatedAt: now, UpdatedAt: now}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(service).Error; err != nil {
			return err
		}
		for _, item := range input.Models {
			item.Id, item.ServiceId, item.CreatedAt, item.UpdatedAt = random.GetUUID(), service.Id, now, now
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "service_created", "publisher_service", service.Id, "")
	})
	if err != nil {
		return nil, err
	}
	service.CredentialConfigured = true
	service.Models = input.Models
	for i := range service.Models {
		service.Models[i].ServiceId = service.Id
	}
	return service, nil
}

func UpdatePublisherService(userID string, serviceID string, input PublisherServiceInput) (*PublisherService, error) {
	publisher, existing, err := getPublisherServiceForUser(userID, serviceID)
	if err != nil {
		return nil, err
	}
	if !canManagePublisher(publisher.Status) {
		return nil, errors.New("当前发布者状态不允许修改模型服务")
	}
	input, err = normalizePublisherServiceInput(input, false)
	if err != nil {
		return nil, err
	}
	if input.SourcePersonalConnectionId != "" && input.SourcePersonalConnectionId != existing.SourcePersonalConnectionId {
		if _, err := GetPersonalProviderConnection(userID, input.SourcePersonalConnectionId); err != nil {
			return nil, errors.New("来源个人渠道不存在或无权访问")
		}
	}
	if err := validateTrustedPublisherServiceModelsWithDB(DB, input.Models); err != nil {
		return nil, err
	}
	now := helper.GetTimestamp()
	endpointChanged := existing.Protocol != input.Protocol || existing.BaseURL != input.BaseURL || strings.TrimSpace(input.APIKey) != ""
	updates := map[string]any{"name": input.Name, "protocol": input.Protocol, "base_url": input.BaseURL, "region": input.Region, "data_policy": string(input.DataPolicy), "capacity_policy": string(input.CapacityPolicy), "updated_at": now}
	if input.SourcePersonalConnectionId != "" {
		updates["source_personal_connection_id"] = input.SourcePersonalConnectionId
	}
	if strings.TrimSpace(input.APIKey) != "" {
		credential, encryptErr := encryptPublisherServiceCredential(input.APIKey)
		if encryptErr != nil {
			return nil, encryptErr
		}
		updates["credential_encrypted"] = credential
	}
	if endpointChanged {
		updates["status"] = PublisherServiceStatusDraft
		updates["last_checked_at"] = int64(0)
		updates["last_check_ok"] = false
		updates["last_check_error"] = ""
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PublisherService{}).Where("id = ? AND publisher_id = ?", existing.Id, publisher.Id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("service_id = ?", existing.Id).Delete(&PublisherServiceModel{}).Error; err != nil {
			return err
		}
		for _, item := range input.Models {
			item.Id, item.ServiceId, item.CreatedAt, item.UpdatedAt = random.GetUUID(), existing.Id, now, now
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "service_updated", "publisher_service", existing.Id, "")
	})
	if err != nil {
		return nil, err
	}
	services, err := listPublisherServicesByPublisherWithDB(DB, publisher.Id)
	if err != nil {
		return nil, err
	}
	for i := range services {
		if services[i].Id == existing.Id {
			return &services[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func DeletePublisherService(userID string, serviceID string) error {
	publisher, service, err := getPublisherServiceForUser(userID, serviceID)
	if err != nil {
		return err
	}
	if !canManagePublisher(publisher.Status) {
		return errors.New("当前发布者状态不允许删除模型服务")
	}
	var publishedOffers int64
	if err := DB.Model(&ServiceOffer{}).Where("service_id = ? AND status IN ?", service.Id, []string{ServiceOfferStatusReviewing, ServiceOfferStatusPublished, ServiceOfferStatusSuspended}).Count(&publishedOffers).Error; err != nil {
		return err
	}
	if publishedOffers > 0 {
		return errors.New("服务存在已提交或已发布报价，不能删除")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id = ?", service.Id).Delete(&PublisherServiceModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND publisher_id = ?", service.Id, publisher.Id).Delete(&PublisherService{}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "service_deleted", "publisher_service", service.Id, "")
	})
}

const publisherServiceVerificationTimeout = 12 * time.Second

var doPublisherServiceVerificationRequest = client.DoPersonalProviderRequest

func publisherServiceVerificationEndpoint(service *PublisherService) (string, error) {
	if service == nil {
		return "", errors.New("模型服务不能为空")
	}
	parsed, err := url.Parse(service.BaseURL)
	if err != nil {
		return "", errors.New("模型服务 Base URL 无效")
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch service.Protocol {
	case "openai", "anthropic":
		if !strings.HasSuffix(path, "/v1") {
			path += "/v1"
		}
	case "gemini":
		if !strings.Contains(path, "/v1") {
			path += "/v1beta"
		}
	case "ali":
		if !strings.Contains(path, "/compatible-mode/v1") {
			path += "/compatible-mode/v1"
		}
	case "deepseek":
	default:
		return "", errors.New("模型服务协议不受支持")
	}
	parsed.Path = strings.TrimRight(path, "/") + "/models"
	parsed.RawQuery, parsed.Fragment = "", ""
	return parsed.String(), nil
}

func verifyPublisherServiceCredential(service *PublisherService, credential string) error {
	endpoint, err := publisherServiceVerificationEndpoint(service)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), publisherServiceVerificationTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("无法创建模型服务验证请求: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	switch service.Protocol {
	case "anthropic":
		req.Header.Set("x-api-key", credential)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", credential)
	default:
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	resp, err := doPublisherServiceVerificationRequest(req)
	if err != nil {
		return fmt.Errorf("无法连接模型服务: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return errors.New("模型服务拒绝了 API Key，请检查凭据权限或轮换 API Key")
		}
		return fmt.Errorf("模型服务验证失败：上游返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func truncatePublisherCheckError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func VerifyPublisherService(userID string, serviceID string) (*PublisherService, error) {
	publisher, service, err := getPublisherServiceForUser(userID, serviceID)
	if err != nil {
		return nil, err
	}
	if !canManagePublisher(publisher.Status) {
		return nil, errors.New("当前发布者状态不允许验证模型服务")
	}
	credential, err := decryptPublisherServiceCredential(service.CredentialEncrypted)
	if err == nil {
		err = verifyPublisherServiceCredential(service, credential)
	}
	now := helper.GetTimestamp()
	updates := map[string]any{"last_checked_at": now, "last_check_ok": err == nil, "last_check_error": ""}
	if err == nil {
		updates["status"] = PublisherServiceStatusReady
	} else {
		updates["status"] = PublisherServiceStatusDraft
		updates["last_check_error"] = truncatePublisherCheckError(err.Error())
	}
	if updateErr := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PublisherService{}).Where("id = ? AND publisher_id = ?", service.Id, publisher.Id).Updates(updates).Error; err != nil {
			return err
		}
		if err == nil {
			if err := tx.Model(&PublisherServiceModel{}).Where("service_id = ?", service.Id).Update("status", PublisherServiceModelStatusReady).Error; err != nil {
				return err
			}
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "service_verified", "publisher_service", service.Id, fmt.Sprint(updates["last_check_error"]))
	}); updateErr != nil {
		return nil, updateErr
	}
	service.LastCheckedAt, service.LastCheckOK = now, err == nil
	service.LastCheckError = fmt.Sprint(updates["last_check_error"])
	if err == nil {
		service.Status = PublisherServiceStatusReady
	} else {
		service.Status = PublisherServiceStatusDraft
	}
	return service, err
}

func ListServiceOffers(userID string) ([]ServiceOffer, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	return listServiceOffersByPublisherWithDB(DB, publisher.Id)
}

func listServiceOffersByPublisherWithDB(db *gorm.DB, publisherID string) ([]ServiceOffer, error) {
	rows := make([]ServiceOffer, 0)
	if err := db.Where("publisher_id = ?", strings.TrimSpace(publisherID)).Order("updated_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListPublishedServiceOfferCatalog returns only community offers that are
// independently eligible for discovery. An approved offer alone is
// insufficient: its publisher must remain active and the bound service must
// still be verified and ready. Invite-only offers are deliberately absent
// until their audience-grant model exists.
func publishedServiceOfferCatalogQuery(db *gorm.DB) *gorm.DB {
	return db.Table(ServiceOffersTableName+" AS o").
		Select(`o.id AS offer_id, o.model, o.scope, o.currency, o.input_price_micros, o.output_price_micros,
			o.platform_fee_bps, p.display_name AS publisher_display_name, p.level AS publisher_level,
			s.name AS service_name, s.region, o.data_policy_snapshot AS data_policy,
			o.capacity_policy_snapshot AS capacity_policy, o.reviewed_at AS published_at`).
		Joins("JOIN "+PublishersTableName+" AS p ON p.id = o.publisher_id").
		Joins("JOIN "+PublisherServicesTableName+" AS s ON s.id = o.service_id AND s.publisher_id = o.publisher_id").
		Where("o.status = ? AND o.scope = ? AND p.status = ? AND s.status = ? AND s.last_check_ok = ?",
			ServiceOfferStatusPublished,
			ServiceOfferScopePublic,
			PublisherStatusActive,
			PublisherServiceStatusReady,
			true)
}

func ListPublishedServiceOfferCatalog(modelName string, limit int) ([]PublishedServiceOfferCatalogItem, error) {
	if DB == nil {
		return nil, errors.New("database handle is nil")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := publishedServiceOfferCatalogQuery(DB)
	if normalizedModel := strings.TrimSpace(modelName); normalizedModel != "" {
		query = query.Where("o.model = ?", normalizedModel)
	}
	rows := make([]PublishedServiceOfferCatalogItem, 0)
	if err := query.Order("o.model asc, o.input_price_micros asc, o.output_price_micros asc, o.reviewed_at desc, o.id asc").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListSupplyChannelViewsForAdmin(limit int) ([]AdminSupplyChannelView, error) {
	if DB == nil || !DB.Migrator().HasTable(ServiceOffersTableName) ||
		!DB.Migrator().HasTable(PublishersTableName) ||
		!DB.Migrator().HasTable(PublisherServicesTableName) {
		return []AdminSupplyChannelView{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows := make([]AdminSupplyChannelView, 0)
	if err := DB.Table(PublisherServicesTableName + " AS s").
		Select(`s.publisher_id, p.user_id AS publisher_user_id, p.display_name AS publisher_display_name,
			p.status AS publisher_status, p.level AS publisher_level,
			s.id AS service_id, s.name AS service_name, s.protocol AS service_protocol, s.region AS service_region,
			s.status AS service_status, s.last_checked_at, s.last_check_ok, s.last_check_error,
			s.updated_at`).
		Joins("JOIN " + PublishersTableName + " AS p ON p.id = s.publisher_id").
		Order("s.updated_at desc, s.id asc").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}
	serviceIDs := make([]string, 0, len(rows))
	indexByServiceID := make(map[string]int, len(rows))
	modelSets := make([]map[string]struct{}, len(rows))
	for i := range rows {
		rows[i].Models = []string{}
		rows[i].Offers = []AdminSupplyOfferView{}
		serviceID := strings.TrimSpace(rows[i].ServiceId)
		serviceIDs = append(serviceIDs, serviceID)
		indexByServiceID[serviceID] = i
		modelSets[i] = make(map[string]struct{})
	}
	if DB.Migrator().HasTable(PublisherServiceModelsTableName) {
		type adminSupplyServiceModelRow struct {
			ServiceId string `gorm:"column:service_id"`
			Model     string `gorm:"column:model"`
		}
		serviceModelRows := make([]adminSupplyServiceModelRow, 0)
		if err := DB.Table(PublisherServiceModelsTableName).
			Select("service_id, model").Where("service_id IN ?", serviceIDs).
			Order("service_id asc, model asc").Scan(&serviceModelRows).Error; err != nil {
			return nil, err
		}
		for _, serviceModel := range serviceModelRows {
			index, ok := indexByServiceID[strings.TrimSpace(serviceModel.ServiceId)]
			modelName := strings.TrimSpace(serviceModel.Model)
			if ok && modelName != "" {
				modelSets[index][modelName] = struct{}{}
			}
		}
	}
	type adminSupplyOfferRow struct {
		ServiceId   string `gorm:"column:service_id"`
		OfferId     string `gorm:"column:offer_id"`
		OfferStatus string `gorm:"column:offer_status"`
		OfferScope  string `gorm:"column:offer_scope"`
		Model       string `gorm:"column:model"`
		UpdatedAt   int64  `gorm:"column:updated_at"`
	}
	offerRows := make([]adminSupplyOfferRow, 0)
	if err := DB.Table(ServiceOffersTableName+" AS o").
		Select("o.service_id, o.id AS offer_id, o.status AS offer_status, o.scope AS offer_scope, o.model, o.updated_at").
		Joins("JOIN "+PublisherServicesTableName+" AS s ON s.id = o.service_id AND s.publisher_id = o.publisher_id").
		Where("o.service_id IN ?", serviceIDs).
		Order("o.service_id asc, o.updated_at desc, o.id asc").Scan(&offerRows).Error; err != nil {
		return nil, err
	}
	for _, offer := range offerRows {
		index, ok := indexByServiceID[strings.TrimSpace(offer.ServiceId)]
		if !ok {
			continue
		}
		rows[index].Offers = append(rows[index].Offers, AdminSupplyOfferView{
			OfferId: offer.OfferId, OfferStatus: offer.OfferStatus, OfferScope: offer.OfferScope,
			Model: offer.Model, UpdatedAt: offer.UpdatedAt,
		})
		modelName := strings.TrimSpace(offer.Model)
		if modelName != "" {
			modelSets[index][modelName] = struct{}{}
		}
	}
	for i := range rows {
		rows[i].OfferCount = len(rows[i].Offers)
		for modelName := range modelSets[i] {
			rows[i].Models = append(rows[i].Models, modelName)
		}
		sort.Strings(rows[i].Models)
	}
	return rows, nil
}

func SumPublisherEarningsByOfferIDs(offerIDs []string, startAt int64, endAt int64) (map[string]int64, error) {
	result := make(map[string]int64)
	ids := normalizeTrimmedValuesPreserveOrder(offerIDs)
	if len(ids) == 0 {
		return result, nil
	}
	if DB == nil || !DB.Migrator().HasTable(&PublisherOfferSettlement{}) {
		return result, nil
	}
	type row struct {
		OfferID string `gorm:"column:offer_id"`
		Amount  int64  `gorm:"column:amount"`
	}
	rows := make([]row, 0)
	query := DB.Model(&PublisherOfferSettlement{}).
		Select("offer_id, COALESCE(SUM(publisher_payable_micros), 0) AS amount").
		Where("offer_id IN ?", ids)
	if startAt > 0 && endAt >= startAt {
		query = query.Where("created_at BETWEEN ? AND ?", startAt, endAt)
	}
	if err := query.Group("offer_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.OfferID] = item.Amount
	}
	return result, nil
}

func GetPublishedServiceOfferCatalogItem(offerID string) (*PublishedServiceOfferCatalogItem, error) {
	if DB == nil {
		return nil, errors.New("database handle is nil")
	}
	item := &PublishedServiceOfferCatalogItem{}
	err := publishedServiceOfferCatalogQuery(DB).
		Where("o.id = ?", strings.TrimSpace(offerID)).
		Take(item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func ListCommunityOfferModelRoutes(userID string) ([]CommunityOfferModelRoute, error) {
	rows := make([]CommunityOfferModelRoute, 0)
	if err := DB.Where("user_id = ?", strings.TrimSpace(userID)).Order("model asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetCommunityOfferModelRoute(userID string, modelName string) (*CommunityOfferModelRoute, error) {
	row := &CommunityOfferModelRoute{}
	err := DB.Where("user_id = ? AND model = ?", strings.TrimSpace(userID), strings.TrimSpace(modelName)).First(row).Error
	if err != nil {
		return nil, err
	}
	return row, nil
}

// UpsertCommunityOfferModelRoute accepts only a currently visible, reviewed
// offer for the exact requested model. A caller cannot turn an invite-only,
// suspended, or another model's offer into a relay target by guessing its ID.
func UpsertCommunityOfferModelRoute(userID string, modelName string, offerID string) error {
	userID = strings.TrimSpace(userID)
	modelName = strings.TrimSpace(modelName)
	offerID = strings.TrimSpace(offerID)
	if userID == "" || modelName == "" || offerID == "" {
		return errors.New("用户、模型和社区报价不能为空")
	}
	item, err := GetPublishedServiceOfferCatalogItem(offerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("社区报价不存在或当前不可用")
		}
		return err
	}
	if item.Model != modelName {
		return errors.New("社区报价与模型不匹配")
	}
	now := helper.GetTimestamp()
	row := CommunityOfferModelRoute{UserID: userID, Model: modelName, OfferID: offerID, CreatedAt: now, UpdatedAt: now}
	return DB.Where(CommunityOfferModelRoute{UserID: userID, Model: modelName}).
		Assign(map[string]any{"offer_id": offerID, "updated_at": now}).FirstOrCreate(&row).Error
}

func DeleteCommunityOfferModelRoute(userID string, modelName string) error {
	result := DB.Where("user_id = ? AND model = ?", strings.TrimSpace(userID), strings.TrimSpace(modelName)).Delete(&CommunityOfferModelRoute{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func IsCommunityOfferChannelID(channelID string) bool {
	return strings.HasPrefix(strings.TrimSpace(channelID), CommunityOfferChannelPrefix)
}

func CommunityOfferIDFromChannelID(channelID string) string {
	return strings.TrimPrefix(strings.TrimSpace(channelID), CommunityOfferChannelPrefix)
}

// ResolveCommunityOfferChannel materializes a short-lived relay channel from
// a reviewed offer. The encrypted publisher credential is only used in memory
// by relay setup and is never exposed through catalog or route APIs.
func ResolveCommunityOfferChannel(offerID string, modelName string, requestPath string) (*Channel, error) {
	offerID = strings.TrimSpace(offerID)
	modelName = strings.TrimSpace(modelName)
	if offerID == "" || modelName == "" {
		return nil, gorm.ErrRecordNotFound
	}
	offer := &ServiceOffer{}
	if err := DB.Where("id = ? AND model = ?", offerID, modelName).First(offer).Error; err != nil {
		return nil, err
	}
	publisher := &Publisher{}
	if err := DB.Where("id = ?", offer.PublisherId).First(publisher).Error; err != nil {
		return nil, err
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", offer.ServiceId, offer.PublisherId).First(service).Error; err != nil {
		return nil, err
	}
	if offer.Status != ServiceOfferStatusPublished || offer.Scope != ServiceOfferScopePublic || publisher.Status != PublisherStatusActive || service.Status != PublisherServiceStatusReady || !service.LastCheckOK {
		return nil, errors.New("社区报价当前不可用于调用")
	}
	serviceModel := &PublisherServiceModel{}
	if err := DB.Where("service_id = ? AND model = ?", service.Id, modelName).First(serviceModel).Error; err != nil {
		return nil, err
	}
	endpoint := NormalizeRequestedChannelModelEndpoint(serviceModel.Endpoint)
	if serviceModel.Status != PublisherServiceModelStatusReady || endpoint == "" || endpoint != NormalizeRequestedChannelModelEndpoint(requestPath) {
		return nil, errors.New("社区报价不支持当前调用端点")
	}
	credential, err := decryptPublisherServiceCredential(service.CredentialEncrypted)
	if err != nil {
		return nil, fmt.Errorf("社区模型服务凭据不可用: %w", err)
	}
	baseURL := service.BaseURL
	channel := &Channel{
		Id:       CommunityOfferChannelPrefix + offer.Id,
		Name:     "community-offer-" + offer.Id,
		Protocol: service.Protocol,
		Key:      credential,
		BaseURL:  &baseURL,
		Status:   ChannelStatusEnabled,
	}
	channel.ChannelModels = []ChannelModel{{
		ChannelId: channel.Id, Model: modelName, UpstreamModel: serviceModel.UpstreamModel,
		Provider: "community_offer", Type: "text", Endpoint: endpoint, Endpoints: []string{endpoint},
		Selected: true, PublishEnabled: true, PublishStatus: ChannelModelPublishStatusPublished,
	}}
	return channel, nil
}

func validPublisherOfferSettlementStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PublisherOfferSettlementStatusAccrued, PublisherOfferSettlementStatusHeld, PublisherOfferSettlementStatusSettled, PublisherOfferSettlementStatusReversed:
		return true
	default:
		return false
	}
}

const microsPerMillionTokens int64 = 1_000_000

func ceilMicrosForTokens(priceMicrosPerMillion int64, tokens int64) (int64, error) {
	if priceMicrosPerMillion < 0 || tokens < 0 {
		return 0, errors.New("价格和 token 数量不能为负数")
	}
	if priceMicrosPerMillion == 0 || tokens == 0 {
		return 0, nil
	}
	numerator := new(big.Int).Mul(big.NewInt(priceMicrosPerMillion), big.NewInt(tokens))
	numerator.Add(numerator, big.NewInt(microsPerMillionTokens-1))
	numerator.Quo(numerator, big.NewInt(microsPerMillionTokens))
	if !numerator.IsInt64() {
		return 0, errors.New("结算金额超出可记录范围")
	}
	return numerator.Int64(), nil
}

func addSettlementMicros(left, right int64) (int64, error) {
	value := new(big.Int).Add(big.NewInt(left), big.NewInt(right))
	if !value.IsInt64() {
		return 0, errors.New("结算金额超出可记录范围")
	}
	return value.Int64(), nil
}

func calculatePublisherOfferSettlementAmounts(inputPriceMicros int64, outputPriceMicros int64, inputTokens int64, outputTokens int64, platformFeeBPS int) (consumerAmountMicros int64, platformFeeAmountMicros int64, publisherPayableMicros int64, err error) {
	if platformFeeBPS < 0 || platformFeeBPS > 10_000 {
		err = errors.New("平台服务费比例无效")
		return
	}
	inputAmount, calculateErr := ceilMicrosForTokens(inputPriceMicros, inputTokens)
	if calculateErr != nil {
		err = calculateErr
		return
	}
	outputAmount, calculateErr := ceilMicrosForTokens(outputPriceMicros, outputTokens)
	if calculateErr != nil {
		err = calculateErr
		return
	}
	consumerAmountMicros, err = addSettlementMicros(inputAmount, outputAmount)
	if err != nil {
		return
	}
	feeNumerator := new(big.Int).Mul(big.NewInt(consumerAmountMicros), big.NewInt(int64(platformFeeBPS)))
	if feeNumerator.Sign() > 0 {
		feeNumerator.Add(feeNumerator, big.NewInt(9_999))
	}
	feeNumerator.Quo(feeNumerator, big.NewInt(10_000))
	if !feeNumerator.IsInt64() {
		err = errors.New("平台服务费超出可记录范围")
		return
	}
	platformFeeAmountMicros = feeNumerator.Int64()
	publisherPayableMicros = consumerAmountMicros - platformFeeAmountMicros
	return
}

func validateExistingPublisherOfferSettlement(existing *PublisherOfferSettlement, input PublisherOfferSettlementInput) error {
	if existing == nil {
		return errors.New("结算快照不能为空")
	}
	if existing.OfferID != strings.TrimSpace(input.OfferID) || existing.ConsumerUserID != strings.TrimSpace(input.ConsumerUserID) || existing.InputTokens != input.InputTokens || existing.OutputTokens != input.OutputTokens {
		return errors.New("请求已经存在不同的社区服务结算快照")
	}
	return nil
}

func publisherOfferSettlementMatchesSnapshot(existing *PublisherOfferSettlement, snapshot *PublisherOfferSettlement) bool {
	if existing == nil || snapshot == nil {
		return false
	}
	return existing.RequestLogID == snapshot.RequestLogID &&
		existing.OfferID == snapshot.OfferID &&
		existing.PublisherID == snapshot.PublisherID &&
		existing.ServiceID == snapshot.ServiceID &&
		existing.ConsumerUserID == snapshot.ConsumerUserID &&
		existing.Model == snapshot.Model &&
		existing.PublisherDisplayName == snapshot.PublisherDisplayName &&
		existing.ServiceName == snapshot.ServiceName &&
		existing.Region == snapshot.Region &&
		existing.DataPolicySnapshot == snapshot.DataPolicySnapshot &&
		existing.Currency == snapshot.Currency &&
		existing.InputPriceMicros == snapshot.InputPriceMicros &&
		existing.OutputPriceMicros == snapshot.OutputPriceMicros &&
		existing.InputTokens == snapshot.InputTokens &&
		existing.OutputTokens == snapshot.OutputTokens &&
		existing.ConsumerAmountMicros == snapshot.ConsumerAmountMicros &&
		existing.PlatformFeeBPS == snapshot.PlatformFeeBPS &&
		existing.PlatformFeeAmountMicros == snapshot.PlatformFeeAmountMicros &&
		existing.PublisherPayableMicros == snapshot.PublisherPayableMicros
}

// CreatePublisherOfferSettlement records the commercial facts of a completed
// request. It is idempotent by request log ID: a retry can return the original
// snapshot, while a different offer, consumer, or usage is rejected.
func CreatePublisherOfferSettlement(input PublisherOfferSettlementInput) (*PublisherOfferSettlement, error) {
	if DB == nil {
		return nil, errors.New("database handle is nil")
	}
	input, err := normalizedPublisherOfferSettlementInput(input)
	if err != nil {
		return nil, err
	}
	existing := &PublisherOfferSettlement{}
	if err := DB.Where("request_log_id = ?", input.RequestLogID).First(existing).Error; err == nil {
		if err := validateExistingPublisherOfferSettlement(existing, input); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	quote, err := QuotePublisherOfferSettlement(input)
	if err != nil {
		return nil, err
	}
	return createPublisherOfferSettlementFromSnapshot(quote)
}

func ListPublisherOfferSettlements(userID string, limit int) ([]PublisherOfferSettlement, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	rows := make([]PublisherOfferSettlement, 0)
	if err := DB.Where("publisher_id = ?", publisher.Id).Order("created_at desc, request_log_id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListPublisherOfferSettlementsForAdmin(status string, limit int) ([]PublisherOfferSettlement, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	query := DB.Order("created_at desc, request_log_id desc")
	if normalized := strings.TrimSpace(status); normalized != "" {
		if !validPublisherOfferSettlementStatus(normalized) {
			return nil, errors.New("发布者结算状态筛选无效")
		}
		query = query.Where("status = ?", normalized)
	}
	rows := make([]PublisherOfferSettlement, 0)
	if err := query.Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func validateOfferInput(input ServiceOfferInput) (ServiceOfferInput, error) {
	input.ServiceId = strings.TrimSpace(input.ServiceId)
	input.Model = strings.TrimSpace(input.Model)
	input.Scope = strings.ToLower(strings.TrimSpace(input.Scope))
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Scope == "" {
		input.Scope = ServiceOfferScopePublic
	}
	if input.Currency == "" {
		input.Currency = "USD"
	}
	if input.ServiceId == "" || input.Model == "" {
		return input, errors.New("模型服务和模型不能为空")
	}
	if input.Scope != ServiceOfferScopePublic && input.Scope != ServiceOfferScopeInvite {
		return input, errors.New("报价范围无效")
	}
	if input.Currency != "USD" {
		return input, errors.New("可信发布试点仅支持 USD 报价")
	}
	if input.InputPriceMicros < 0 || input.OutputPriceMicros < 0 || (input.InputPriceMicros == 0 && input.OutputPriceMicros == 0) {
		return input, errors.New("报价必须包含有效的非负 token 价格")
	}
	return input, nil
}

func serviceModelExistsWithDB(db *gorm.DB, serviceID string, modelName string) bool {
	var count int64
	return db.Model(&PublisherServiceModel{}).Where("service_id = ? AND model = ?", strings.TrimSpace(serviceID), strings.TrimSpace(modelName)).Count(&count).Error == nil && count > 0
}

func CreateServiceOffer(userID string, input ServiceOfferInput) (*ServiceOffer, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if !canManagePublisher(publisher.Status) {
		return nil, errors.New("当前发布者状态不允许创建报价")
	}
	input, err = validateOfferInput(input)
	if err != nil {
		return nil, err
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", input.ServiceId, publisher.Id).First(service).Error; err != nil {
		return nil, err
	}
	if !serviceModelExistsWithDB(DB, service.Id, input.Model) {
		return nil, errors.New("报价模型不属于该模型服务")
	}
	now := helper.GetTimestamp()
	offer := &ServiceOffer{Id: random.GetUUID(), PublisherId: publisher.Id, ServiceId: service.Id, Model: input.Model, Scope: input.Scope, Currency: input.Currency, InputPriceMicros: input.InputPriceMicros, OutputPriceMicros: input.OutputPriceMicros, PlatformFeeBPS: ServiceOfferDefaultPlatformFeeBPS, DataPolicySnapshot: service.DataPolicy, CapacityPolicySnapshot: service.CapacityPolicy, Status: ServiceOfferStatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(offer).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "offer_created", "service_offer", offer.Id, "")
	}); err != nil {
		return nil, err
	}
	return offer, nil
}

func UpdateServiceOffer(userID string, offerID string, input ServiceOfferInput) (*ServiceOffer, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if !canManagePublisher(publisher.Status) {
		return nil, errors.New("当前发布者状态不允许修改报价")
	}
	input, err = validateOfferInput(input)
	if err != nil {
		return nil, err
	}
	offer := &ServiceOffer{}
	if err := DB.Where("id = ? AND publisher_id = ?", strings.TrimSpace(offerID), publisher.Id).First(offer).Error; err != nil {
		return nil, err
	}
	if offer.Status == ServiceOfferStatusReviewing || offer.Status == ServiceOfferStatusPublished {
		return nil, errors.New("已提交或已发布报价不能直接修改")
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", input.ServiceId, publisher.Id).First(service).Error; err != nil {
		return nil, err
	}
	if !serviceModelExistsWithDB(DB, service.Id, input.Model) {
		return nil, errors.New("报价模型不属于该模型服务")
	}
	now := helper.GetTimestamp()
	updates := map[string]any{"service_id": service.Id, "model": input.Model, "scope": input.Scope, "currency": input.Currency, "input_price_micros": input.InputPriceMicros, "output_price_micros": input.OutputPriceMicros, "data_policy_snapshot": service.DataPolicy, "capacity_policy_snapshot": service.CapacityPolicy, "status": ServiceOfferStatusDraft, "review_note": "", "version": offer.Version + 1, "updated_at": now}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ServiceOffer{}).Where("id = ? AND publisher_id = ?", offer.Id, publisher.Id).Updates(updates).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "offer_updated", "service_offer", offer.Id, "")
	}); err != nil {
		return nil, err
	}
	for key, value := range updates {
		switch key {
		case "service_id":
			offer.ServiceId = value.(string)
		case "model":
			offer.Model = value.(string)
		case "scope":
			offer.Scope = value.(string)
		case "currency":
			offer.Currency = value.(string)
		case "input_price_micros":
			offer.InputPriceMicros = value.(int64)
		case "output_price_micros":
			offer.OutputPriceMicros = value.(int64)
		case "status":
			offer.Status = value.(string)
		case "version":
			offer.Version = value.(int)
		case "updated_at":
			offer.UpdatedAt = value.(int64)
		}
	}
	return offer, nil
}

func DeleteServiceOffer(userID string, offerID string) error {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return err
	}
	offer := &ServiceOffer{}
	if err := DB.Where("id = ? AND publisher_id = ?", strings.TrimSpace(offerID), publisher.Id).First(offer).Error; err != nil {
		return err
	}
	if offer.Status == ServiceOfferStatusReviewing || offer.Status == ServiceOfferStatusPublished || offer.Status == ServiceOfferStatusSuspended {
		return errors.New("已提交或已发布报价不能删除")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND publisher_id = ?", offer.Id, publisher.Id).Delete(&ServiceOffer{}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "offer_deleted", "service_offer", offer.Id, "")
	})
}

func SubmitServiceOffer(userID string, offerID string) (*ServiceOffer, error) {
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	if publisher.Status != PublisherStatusActive {
		return nil, errors.New("发布者审核通过后才能提交报价")
	}
	offer := &ServiceOffer{}
	if err := DB.Where("id = ? AND publisher_id = ?", strings.TrimSpace(offerID), publisher.Id).First(offer).Error; err != nil {
		return nil, err
	}
	if offer.Status != ServiceOfferStatusDraft && offer.Status != ServiceOfferStatusRejected {
		return nil, errors.New("当前报价状态不能提交审核")
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", offer.ServiceId, publisher.Id).First(service).Error; err != nil {
		return nil, err
	}
	if service.Status != PublisherServiceStatusReady || !service.LastCheckOK {
		return nil, errors.New("模型服务验证通过后才能提交报价")
	}
	if !serviceModelExistsWithDB(DB, service.Id, offer.Model) {
		return nil, errors.New("报价模型不属于该模型服务")
	}
	var readyModelCount int64
	if err := DB.Model(&PublisherServiceModel{}).
		Where("service_id = ? AND model = ? AND status = ?", service.Id, offer.Model, PublisherServiceModelStatusReady).
		Count(&readyModelCount).Error; err != nil {
		return nil, err
	}
	if readyModelCount == 0 {
		return nil, errors.New("报价模型尚未通过服务验证")
	}
	now := helper.GetTimestamp()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ServiceOffer{}).Where("id = ? AND publisher_id = ?", offer.Id, publisher.Id).Updates(map[string]any{"status": ServiceOfferStatusReviewing, "submitted_at": now, "review_note": "", "updated_at": now}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, userID, "offer_submitted", "service_offer", offer.Id, "")
	}); err != nil {
		return nil, err
	}
	offer.Status, offer.SubmittedAt, offer.ReviewNote, offer.UpdatedAt = ServiceOfferStatusReviewing, now, "", now
	return offer, nil
}

func ListPublishersForAdmin(status string) ([]Publisher, error) {
	query := DB.Order("submitted_at desc, updated_at desc, id asc")
	if normalized := strings.TrimSpace(status); normalized != "" {
		if !validPublisherStatus(normalized) {
			return nil, errors.New("发布者状态筛选无效")
		}
		query = query.Where("status = ?", normalized)
	}
	rows := make([]Publisher, 0)
	return rows, query.Find(&rows).Error
}

func ReviewPublisher(adminUserID string, publisherID string, approve bool, note string) (*Publisher, error) {
	publisher := &Publisher{}
	if err := DB.Where("id = ?", strings.TrimSpace(publisherID)).First(publisher).Error; err != nil {
		return nil, err
	}
	if publisher.Status != PublisherStatusReviewing && publisher.Status != PublisherStatusActive && publisher.Status != PublisherStatusRestricted {
		return nil, errors.New("当前发布者状态不能审核")
	}
	note = strings.TrimSpace(note)
	if !approve && note == "" {
		return nil, errors.New("限制发布者时必须说明原因")
	}
	now := helper.GetTimestamp()
	status := PublisherStatusRestricted
	action := "publisher_restricted"
	if approve {
		status, action = PublisherStatusActive, "publisher_approved"
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Publisher{}).Where("id = ?", publisher.Id).Updates(map[string]any{"status": status, "review_note": note, "reviewed_at": now, "reviewed_by": strings.TrimSpace(adminUserID), "updated_at": now}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, publisher.Id, adminUserID, action, "publisher", publisher.Id, note)
	}); err != nil {
		return nil, err
	}
	publisher.Status, publisher.ReviewNote, publisher.ReviewedAt, publisher.ReviewedBy, publisher.UpdatedAt = status, note, now, strings.TrimSpace(adminUserID), now
	return publisher, nil
}

func ListPublisherServicesForAdmin(status string) ([]PublisherService, error) {
	query := DB.Order("updated_at desc, id asc")
	if normalized := strings.TrimSpace(status); normalized != "" {
		if !validPublisherServiceStatus(normalized) {
			return nil, errors.New("模型服务状态筛选无效")
		}
		query = query.Where("status = ?", normalized)
	}
	rows := make([]PublisherService, 0)
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].CredentialConfigured = strings.TrimSpace(rows[i].CredentialEncrypted) != ""
	}
	return rows, nil
}

func SuspendPublisherService(adminUserID string, serviceID string, note string) (*PublisherService, error) {
	service := &PublisherService{}
	if err := DB.Where("id = ?", strings.TrimSpace(serviceID)).First(service).Error; err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, errors.New("暂停模型服务时必须说明原因")
	}
	now := helper.GetTimestamp()
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PublisherService{}).Where("id = ?", service.Id).Updates(map[string]any{"status": PublisherServiceStatusSuspended, "last_check_error": note, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&ServiceOffer{}).Where("service_id = ? AND status = ?", service.Id, ServiceOfferStatusPublished).Updates(map[string]any{"status": ServiceOfferStatusSuspended, "review_note": note, "updated_at": now}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, service.PublisherId, adminUserID, "service_suspended", "publisher_service", service.Id, note)
	}); err != nil {
		return nil, err
	}
	service.Status, service.LastCheckError, service.UpdatedAt = PublisherServiceStatusSuspended, note, now
	return service, nil
}

func ListServiceOffersForAdmin(status string) ([]ServiceOffer, error) {
	query := DB.Order("submitted_at desc, updated_at desc, id asc")
	if normalized := strings.TrimSpace(status); normalized != "" {
		if !validServiceOfferStatus(normalized) {
			return nil, errors.New("报价状态筛选无效")
		}
		query = query.Where("status = ?", normalized)
	}
	rows := make([]ServiceOffer, 0)
	return rows, query.Find(&rows).Error
}

func ReviewServiceOffer(adminUserID string, offerID string, approve bool, note string) (*ServiceOffer, error) {
	offer := &ServiceOffer{}
	if err := DB.Where("id = ?", strings.TrimSpace(offerID)).First(offer).Error; err != nil {
		return nil, err
	}
	if offer.Status != ServiceOfferStatusReviewing {
		return nil, errors.New("当前报价状态不能审核")
	}
	note = strings.TrimSpace(note)
	if !approve && note == "" {
		return nil, errors.New("驳回报价时必须说明原因")
	}
	service := &PublisherService{}
	if err := DB.Where("id = ? AND publisher_id = ?", offer.ServiceId, offer.PublisherId).First(service).Error; err != nil {
		return nil, err
	}
	if approve && (service.Status != PublisherServiceStatusReady || !service.LastCheckOK) {
		return nil, errors.New("模型服务未通过验证，不能发布报价")
	}
	now, status, action := helper.GetTimestamp(), ServiceOfferStatusRejected, "offer_rejected"
	if approve {
		status, action = ServiceOfferStatusPublished, "offer_approved"
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ServiceOffer{}).Where("id = ?", offer.Id).Updates(map[string]any{"status": status, "review_note": note, "reviewed_at": now, "reviewed_by": strings.TrimSpace(adminUserID), "updated_at": now}).Error; err != nil {
			return err
		}
		return recordPublisherAuditWithDB(tx, offer.PublisherId, adminUserID, action, "service_offer", offer.Id, note)
	}); err != nil {
		return nil, err
	}
	offer.Status, offer.ReviewNote, offer.ReviewedAt, offer.ReviewedBy, offer.UpdatedAt = status, note, now, strings.TrimSpace(adminUserID), now
	return offer, nil
}
