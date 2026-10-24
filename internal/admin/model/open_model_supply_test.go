package model

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOpenModelSupplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &ProviderModel{}, &BillingCurrency{}, &Log{}, &UserBalanceLot{}, &UserBalanceLotTransaction{}, &PersonalProviderConnection{}, &Publisher{}, &PublisherService{}, &PublisherServiceModel{}, &ServiceOffer{}, &PublisherAuditLog{}, &PublisherOfferSettlement{}, &PublisherSettlementDelivery{}, &CommunityOfferModelRoute{}, &GroupCatalog{}, &GroupModel{}, &GroupSupplyBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	previousDB := DB
	previousLogDB := LOG_DB
	previousSecret := config.JWTSecret
	previousFallbacks := config.JWTFallbackSecrets
	DB = db
	LOG_DB = db
	config.JWTSecret = "open-model-supply-test-secret"
	config.JWTFallbackSecrets = nil
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		config.JWTSecret = previousSecret
		config.JWTFallbackSecrets = previousFallbacks
	})
	return db
}

func seedOpenModelSupplyUser(t *testing.T, db *gorm.DB, id string, did string) {
	t.Helper()
	user := User{Id: id, Username: id, DisplayName: id, Email: id + "@example.com", Password: "not-used", AccessToken: id + "-token", AffCode: id + "-aff", WalletIdentityDID: &did}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func seedOpenModelSupplyCatalog(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&ProviderModel{Provider: "openai", Model: "gpt-5.4", Status: ProviderModelStatusActive}).Error; err != nil {
		t.Fatalf("create model catalog row: %v", err)
	}
}

func testPublisherServiceInput() PublisherServiceInput {
	return PublisherServiceInput{
		Name:           "trusted service",
		Protocol:       "openai",
		BaseURL:        "https://1.1.1.1/v1",
		APIKey:         "sk-test-secret",
		Region:         "CN",
		DataPolicy:     []byte(`{"retention":"none","training_usage":"prohibited"}`),
		CapacityPolicy: []byte(`{"max_concurrency":8}`),
		Models:         []PublisherServiceModel{{Model: "gpt-5.4", UpstreamModel: "gpt-5.4", Endpoint: ChannelModelEndpointChat}},
	}
}

func TestListSupplyChannelViewsForAdminGroupsOffersByService(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	publisher := Publisher{
		Id: "publisher-1", UserId: "user-1", WalletIdentityDID: "did:yeying:publisher-1",
		DisplayName: "Publisher One", Level: PublisherLevelVerifiedCommunity, Status: PublisherStatusActive,
		CreatedAt: 1, UpdatedAt: 2,
	}
	if err := db.Create(&publisher).Error; err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	services := []PublisherService{
		{Id: "service-1", PublisherId: publisher.Id, Name: "multi-model", Protocol: "openai", CredentialEncrypted: "encrypted", Status: PublisherServiceStatusReady, LastCheckedAt: 20, LastCheckOK: true, CreatedAt: 1, UpdatedAt: 30},
		{Id: "service-2", PublisherId: publisher.Id, Name: "no-offer-yet", Protocol: "openai", CredentialEncrypted: "encrypted", Status: PublisherServiceStatusDraft, CreatedAt: 1, UpdatedAt: 10},
	}
	if err := db.Create(&services).Error; err != nil {
		t.Fatalf("create services: %v", err)
	}
	if err := db.Create(&PublisherServiceModel{Id: "service-model-2", ServiceId: "service-2", Model: "text-embedding-3-small", Status: PublisherServiceModelStatusReady, CreatedAt: 2, UpdatedAt: 2}).Error; err != nil {
		t.Fatalf("create service model: %v", err)
	}
	offers := []ServiceOffer{
		{Id: "offer-1", PublisherId: publisher.Id, ServiceId: "service-1", Model: "gpt-5.4", Scope: ServiceOfferScopePublic, Status: ServiceOfferStatusPublished, CreatedAt: 2, UpdatedAt: 20},
		{Id: "offer-2", PublisherId: publisher.Id, ServiceId: "service-1", Model: "o3", Scope: ServiceOfferScopeInvite, Status: ServiceOfferStatusReviewing, CreatedAt: 3, UpdatedAt: 21},
	}
	if err := db.Create(&offers).Error; err != nil {
		t.Fatalf("create offers: %v", err)
	}

	rows, err := ListSupplyChannelViewsForAdmin(10)
	if err != nil {
		t.Fatalf("ListSupplyChannelViewsForAdmin: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("channel rows = %d, want one per service (2)", len(rows))
	}
	first := rows[0]
	if first.ServiceId != "service-1" || first.OfferCount != 2 || len(first.Offers) != 2 {
		t.Fatalf("first service projection = %#v, want one row with two offers", first)
	}
	if len(first.Models) != 2 || first.Models[0] != "gpt-5.4" || first.Models[1] != "o3" {
		t.Fatalf("first service models = %#v, want stable aggregate", first.Models)
	}
	if rows[1].ServiceId != "service-2" || rows[1].OfferCount != 0 || len(rows[1].Offers) != 0 || len(rows[1].Models) != 1 || rows[1].Models[0] != "text-embedding-3-small" {
		t.Fatalf("service without offers projection = %#v", rows[1])
	}
}

func TestEnsurePublisherProfileRequiresWalletIdentityDID(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyUser(t, db, "user-without-did", "")
	if _, err := EnsurePublisherProfile("user-without-did", PublisherProfileInput{}); err == nil || !strings.Contains(err.Error(), "钱包身份") {
		t.Fatalf("EnsurePublisherProfile error = %v, want wallet identity error", err)
	}
}

func TestPublisherServiceOwnershipAndCredentialBoundary(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyCatalog(t, db)
	seedOpenModelSupplyUser(t, db, "user-a", "did:yeying:wid_user_a")
	seedOpenModelSupplyUser(t, db, "user-b", "did:yeying:wid_user_b")
	if _, err := EnsurePublisherProfile("user-a", PublisherProfileInput{}); err != nil {
		t.Fatalf("create publisher A: %v", err)
	}
	if _, err := EnsurePublisherProfile("user-b", PublisherProfileInput{}); err != nil {
		t.Fatalf("create publisher B: %v", err)
	}
	service, err := CreatePublisherService("user-a", testPublisherServiceInput())
	if err != nil {
		t.Fatalf("CreatePublisherService: %v", err)
	}
	if service.CredentialEncrypted == "sk-test-secret" || service.CredentialEncrypted == "" {
		t.Fatalf("service credential was not encrypted: %q", service.CredentialEncrypted)
	}
	if _, err := UpdatePublisherService("user-b", service.Id, testPublisherServiceInput()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other user update error = %v, want record not found", err)
	}
	rows, err := ListPublisherServices("user-a")
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListPublisherServices = %#v, %v", rows, err)
	}
	if !rows[0].CredentialConfigured || rows[0].CredentialEncrypted != service.CredentialEncrypted {
		t.Fatalf("unexpected service credential state: %#v", rows[0])
	}
	if got, err := decryptPublisherServiceCredential(rows[0].CredentialEncrypted); err != nil || got != "sk-test-secret" {
		t.Fatalf("decrypt credential = %q, %v", got, err)
	}
}

func TestPublisherServiceFromPersonalConnectionRequiresOwnershipAndNewCredential(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyCatalog(t, db)
	seedOpenModelSupplyUser(t, db, "user-a", "did:yeying:wid_user_a")
	seedOpenModelSupplyUser(t, db, "user-b", "did:yeying:wid_user_b")
	if _, err := EnsurePublisherProfile("user-a", PublisherProfileInput{}); err != nil {
		t.Fatalf("create publisher A: %v", err)
	}
	connectionA := &PersonalProviderConnection{UserId: "user-a", Name: "private-a", Protocol: "openai", BaseURL: "https://1.1.1.1/v1", Models: []string{"gpt-5.4"}, Status: PersonalProviderStatusEnabled}
	if err := CreatePersonalProviderConnection(connectionA, "sk-private-a"); err != nil {
		t.Fatalf("create connection A: %v", err)
	}
	connectionB := &PersonalProviderConnection{UserId: "user-b", Name: "private-b", Protocol: "openai", BaseURL: "https://1.1.1.2/v1", Models: []string{"gpt-5.4"}, Status: PersonalProviderStatusEnabled}
	if err := CreatePersonalProviderConnection(connectionB, "sk-private-b"); err != nil {
		t.Fatalf("create connection B: %v", err)
	}
	foreignInput := testPublisherServiceInput()
	foreignInput.SourcePersonalConnectionId = connectionB.Id
	if _, err := CreatePublisherService("user-a", foreignInput); err == nil || !strings.Contains(err.Error(), "无权访问") {
		t.Fatalf("foreign source error = %v, want ownership rejection", err)
	}
	ownInput := testPublisherServiceInput()
	ownInput.SourcePersonalConnectionId = connectionA.Id
	ownInput.APIKey = "sk-separate-supply"
	service, err := CreatePublisherService("user-a", ownInput)
	if err != nil {
		t.Fatalf("create service from own connection: %v", err)
	}
	if service.SourcePersonalConnectionId != connectionA.Id {
		t.Fatalf("source connection = %q, want %q", service.SourcePersonalConnectionId, connectionA.Id)
	}
	if got, err := decryptPublisherServiceCredential(service.CredentialEncrypted); err != nil || got != "sk-separate-supply" {
		t.Fatalf("supply credential = %q, %v", got, err)
	}
	if service.CredentialEncrypted == connectionA.CredentialEncrypted {
		t.Fatal("publisher service reused the personal-channel encrypted credential")
	}
	payload, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("marshal service: %v", err)
	}
	serialized := string(payload)
	if strings.Contains(serialized, "sk-private-a") || strings.Contains(serialized, "sk-separate-supply") || strings.Contains(serialized, service.CredentialEncrypted) {
		t.Fatalf("service JSON leaked credential material: %s", serialized)
	}
}

func TestTrustedPublishingOfferLifecycleRequiresVerifiedServiceAndReview(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyCatalog(t, db)
	seedOpenModelSupplyUser(t, db, "publisher-user", "did:yeying:wid_publisher")
	seedOpenModelSupplyUser(t, db, "consumer-user", "did:yeying:wid_consumer")
	publisher, err := EnsurePublisherProfile("publisher-user", PublisherProfileInput{})
	if err != nil {
		t.Fatalf("EnsurePublisherProfile: %v", err)
	}
	service, err := CreatePublisherService("publisher-user", testPublisherServiceInput())
	if err != nil {
		t.Fatalf("CreatePublisherService: %v", err)
	}
	offer, err := CreateServiceOffer("publisher-user", ServiceOfferInput{ServiceId: service.Id, Model: "gpt-5.4", Scope: ServiceOfferScopePublic, Currency: "USD", InputPriceMicros: 1_200_000, OutputPriceMicros: 4_800_000})
	if err != nil {
		t.Fatalf("CreateServiceOffer: %v", err)
	}
	if _, err := SubmitServiceOffer("publisher-user", offer.Id); err == nil || !strings.Contains(err.Error(), "发布者审核") {
		t.Fatalf("submit before publisher review error = %v", err)
	}
	if _, err := SubmitPublisherApplication("publisher-user"); err != nil {
		t.Fatalf("SubmitPublisherApplication: %v", err)
	}
	if _, err := ReviewPublisher("admin-user", publisher.Id, true, "approved for pilot"); err != nil {
		t.Fatalf("ReviewPublisher: %v", err)
	}
	previousVerifier := doPublisherServiceVerificationRequest
	doPublisherServiceVerificationRequest = func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	}
	t.Cleanup(func() { doPublisherServiceVerificationRequest = previousVerifier })
	if _, err := VerifyPublisherService("publisher-user", service.Id); err != nil {
		t.Fatalf("VerifyPublisherService: %v", err)
	}
	if _, err := SubmitServiceOffer("publisher-user", offer.Id); err != nil {
		t.Fatalf("SubmitServiceOffer: %v", err)
	}
	published, err := ReviewServiceOffer("admin-user", offer.Id, true, "approved for pilot")
	if err != nil {
		t.Fatalf("ReviewServiceOffer: %v", err)
	}
	if published.Status != ServiceOfferStatusPublished {
		t.Fatalf("published offer status = %q", published.Status)
	}
	catalog, err := ListPublishedServiceOfferCatalog("gpt-5.4", 10)
	if err != nil || len(catalog) != 1 {
		t.Fatalf("ListPublishedServiceOfferCatalog = %#v, %v", catalog, err)
	}
	if catalog[0].OfferID != offer.Id || catalog[0].PublisherDisplayName != "publisher-user" || catalog[0].ServiceName != "trusted service" {
		t.Fatalf("catalog item = %#v", catalog[0])
	}
	if _, err := GetPublishedServiceOfferCatalogItem(offer.Id); err != nil {
		t.Fatalf("GetPublishedServiceOfferCatalogItem: %v", err)
	}
	settlementInput := PublisherOfferSettlementInput{RequestLogID: "request-log-1", OfferID: offer.Id, ConsumerUserID: "consumer-user", InputTokens: 1_500_000, OutputTokens: 500_000}
	settlement, err := CreatePublisherOfferSettlement(settlementInput)
	if err != nil {
		t.Fatalf("CreatePublisherOfferSettlement: %v", err)
	}
	if settlement.ConsumerAmountMicros != 4_200_000 || settlement.PlatformFeeAmountMicros != 420_000 || settlement.PublisherPayableMicros != 3_780_000 {
		t.Fatalf("settlement amounts = %#v", settlement)
	}
	idempotent, err := CreatePublisherOfferSettlement(settlementInput)
	if err != nil || idempotent.RequestLogID != settlement.RequestLogID {
		t.Fatalf("idempotent settlement = %#v, %v", idempotent, err)
	}
	changedUsage := settlementInput
	changedUsage.OutputTokens++
	if _, err := CreatePublisherOfferSettlement(changedUsage); err == nil || !strings.Contains(err.Error(), "不同的社区服务结算快照") {
		t.Fatalf("changed settlement write error = %v", err)
	}
	publisherSettlements, err := ListPublisherOfferSettlements("publisher-user", 10)
	if err != nil || len(publisherSettlements) != 1 {
		t.Fatalf("ListPublisherOfferSettlements = %#v, %v", publisherSettlements, err)
	}
	if _, err := ReviewPublisher("admin-user", publisher.Id, false, "temporary restriction"); err != nil {
		t.Fatalf("restrict publisher: %v", err)
	}
	catalog, err = ListPublishedServiceOfferCatalog("gpt-5.4", 10)
	if err != nil || len(catalog) != 0 {
		t.Fatalf("restricted publisher leaked into catalog = %#v, %v", catalog, err)
	}
	var audits int64
	if err := db.Model(&PublisherAuditLog{}).Where("publisher_id = ?", publisher.Id).Count(&audits).Error; err != nil {
		t.Fatalf("count publisher audits: %v", err)
	}
	if audits < 6 {
		t.Fatalf("publisher audit count = %d, want lifecycle audit records", audits)
	}
}

func TestTrustedPublisherServiceRejectsUnknownCatalogModel(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyUser(t, db, "publisher-user", "did:yeying:wid_publisher")
	if _, err := EnsurePublisherProfile("publisher-user", PublisherProfileInput{}); err != nil {
		t.Fatalf("EnsurePublisherProfile: %v", err)
	}
	input := testPublisherServiceInput()
	input.Models[0].Model = "unreviewed-model"
	if _, err := CreatePublisherService("publisher-user", input); err == nil || !strings.Contains(err.Error(), "标准模型目录") {
		t.Fatalf("CreatePublisherService error = %v, want standard catalog error", err)
	}
}

func TestCommunityOfferModelRouteOnlyAcceptsPublishedMatchingOffer(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	seedOpenModelSupplyCatalog(t, db)
	seedOpenModelSupplyUser(t, db, "publisher-user", "did:yeying:wid_publisher")
	publisher, err := EnsurePublisherProfile("publisher-user", PublisherProfileInput{})
	if err != nil {
		t.Fatalf("EnsurePublisherProfile: %v", err)
	}
	service, err := CreatePublisherService("publisher-user", testPublisherServiceInput())
	if err != nil {
		t.Fatalf("CreatePublisherService: %v", err)
	}
	offer, err := CreateServiceOffer("publisher-user", ServiceOfferInput{ServiceId: service.Id, Model: "gpt-5.4", Scope: ServiceOfferScopePublic, Currency: "USD", InputPriceMicros: 1_000_000, OutputPriceMicros: 2_000_000})
	if err != nil {
		t.Fatalf("CreateServiceOffer: %v", err)
	}
	if err := UpsertCommunityOfferModelRoute("consumer-user", "gpt-5.4", offer.Id); err == nil || !strings.Contains(err.Error(), "不可用") {
		t.Fatalf("draft offer route error = %v, want unavailable", err)
	}
	if _, err := SubmitPublisherApplication("publisher-user"); err != nil {
		t.Fatalf("SubmitPublisherApplication: %v", err)
	}
	if _, err := ReviewPublisher("admin-user", publisher.Id, true, "approved"); err != nil {
		t.Fatalf("ReviewPublisher: %v", err)
	}
	previousVerifier := doPublisherServiceVerificationRequest
	doPublisherServiceVerificationRequest = func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	}
	t.Cleanup(func() { doPublisherServiceVerificationRequest = previousVerifier })
	if _, err := VerifyPublisherService("publisher-user", service.Id); err != nil {
		t.Fatalf("VerifyPublisherService: %v", err)
	}
	if _, err := SubmitServiceOffer("publisher-user", offer.Id); err != nil {
		t.Fatalf("SubmitServiceOffer: %v", err)
	}
	if _, err := ReviewServiceOffer("admin-user", offer.Id, true, "approved"); err != nil {
		t.Fatalf("ReviewServiceOffer: %v", err)
	}
	if err := UpsertCommunityOfferModelRoute("consumer-user", "other-model", offer.Id); err == nil || !strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("mismatched model route error = %v", err)
	}
	if err := UpsertCommunityOfferModelRoute("consumer-user", "gpt-5.4", offer.Id); err != nil {
		t.Fatalf("UpsertCommunityOfferModelRoute: %v", err)
	}
	route, err := GetCommunityOfferModelRoute("consumer-user", "gpt-5.4")
	if err != nil || route.OfferID != offer.Id {
		t.Fatalf("GetCommunityOfferModelRoute = %#v, %v", route, err)
	}
	channel, err := ResolveCommunityOfferChannel(offer.Id, "gpt-5.4", ChannelModelEndpointChat)
	if err != nil {
		t.Fatalf("ResolveCommunityOfferChannel: %v", err)
	}
	if channel.Id != CommunityOfferChannelPrefix+offer.Id || channel.Key != "sk-test-secret" || len(channel.GetSelectedChannelModels()) != 1 {
		t.Fatalf("community offer relay channel = %#v", channel)
	}
	if _, err := ResolveCommunityOfferChannel(offer.Id, "gpt-5.4", ChannelModelEndpointResponses); err == nil || !strings.Contains(err.Error(), "端点") {
		t.Fatalf("unexpected endpoint error = %v", err)
	}
	deliveryInput := PublisherOfferSettlementInput{RequestLogID: "delivery-request-1", OfferID: offer.Id, ConsumerUserID: "consumer-user", InputTokens: 1_000, OutputTokens: 500}
	quote, err := QuotePublisherOfferSettlement(deliveryInput)
	if err != nil {
		t.Fatalf("QuotePublisherOfferSettlement: %v", err)
	}
	if err := db.Model(&ServiceOffer{}).Where("id = ?", offer.Id).Update("input_price_micros", 9_000_000).Error; err != nil {
		t.Fatalf("change offer after quote: %v", err)
	}
	prepared, err := PreparePublisherSettlementDeliveryFromQuote(quote)
	if err != nil {
		t.Fatalf("PreparePublisherSettlementDeliveryFromQuote: %v", err)
	}
	if prepared.InputPriceMicros != 1_000_000 || prepared.OutputPriceMicros != 2_000_000 || prepared.ConsumerAmountMicros != 2_000 || prepared.PublisherPayableMicros != 1_800 {
		t.Fatalf("prepared delivery snapshot = %#v", prepared)
	}
	cancellationInput := PublisherOfferSettlementInput{RequestLogID: "cancel-delivery-request-1", OfferID: offer.Id, ConsumerUserID: "consumer-user", InputTokens: 1_000, OutputTokens: 500}
	cancellationQuote, err := QuotePublisherOfferSettlement(cancellationInput)
	if err != nil {
		t.Fatalf("QuotePublisherOfferSettlement for cancellation: %v", err)
	}
	cancellationDelivery, err := PreparePublisherSettlementDeliveryForCharge(cancellationQuote, 777, 1.25)
	if err != nil {
		t.Fatalf("PreparePublisherSettlementDeliveryForCharge: %v", err)
	}
	if cancellationDelivery.ChargeRecorded || cancellationDelivery.PlannedQuota != 777 || cancellationDelivery.ChargedQuota != 0 || cancellationDelivery.ChargeRate != 1.25 {
		t.Fatalf("cancellation delivery charge snapshot = %#v", cancellationDelivery)
	}
	if _, err := RecordPublisherSettlementDeliveryCharge(cancellationInput.RequestLogID, 777); err != nil {
		t.Fatalf("RecordPublisherSettlementDeliveryCharge: %v", err)
	}
	if _, err := RecordPublisherSettlementDeliveryCharge(cancellationInput.RequestLogID, 776); err == nil || !strings.Contains(err.Error(), "不同的实际扣减额度") {
		t.Fatalf("changed actual charge snapshot error = %v", err)
	}
	if err := db.Model(&PublisherSettlementDelivery{}).Where("request_log_id = ?", cancellationInput.RequestLogID).Update("created_at", helper.GetTimestamp()-PublisherSettlementDeliveryCancellationMinimumAgeSeconds-1).Error; err != nil {
		t.Fatalf("age cancellation delivery: %v", err)
	}
	cancelled, err := CancelPublisherSettlementDelivery(cancellationInput.RequestLogID, "admin-user", "consume log was not persisted")
	if err != nil || cancelled.Status != PublisherSettlementDeliveryStatusCancelled || cancelled.RefundedQuota != 777 || cancelled.RefundLotID == "" || cancelled.CancelledBy != "admin-user" {
		t.Fatalf("CancelPublisherSettlementDelivery = %#v, %v", cancelled, err)
	}
	if _, err := CancelPublisherSettlementDelivery(cancellationInput.RequestLogID, "admin-user", "duplicate cancellation"); err != nil {
		t.Fatalf("idempotent cancellation: %v", err)
	}
	var refundLot UserBalanceLot
	if err := db.Where("source_type = ? AND source_id = ?", UserBalanceLotSourceCommunityOfferRefund, cancellationInput.RequestLogID).First(&refundLot).Error; err != nil {
		t.Fatalf("load refund lot: %v", err)
	}
	if refundLot.UserID != "consumer-user" || refundLot.TotalAmount != 777 || refundLot.RemainingAmount != 777 {
		t.Fatalf("refund lot = %#v", refundLot)
	}
	var refundLots int64
	if err := db.Model(&UserBalanceLot{}).Where("source_type = ? AND source_id = ?", UserBalanceLotSourceCommunityOfferRefund, cancellationInput.RequestLogID).Count(&refundLots).Error; err != nil || refundLots != 1 {
		t.Fatalf("refund lot count = %d, %v", refundLots, err)
	}
	if _, err := PreparePublisherSettlementDeliveryForCharge(cancellationQuote, 778, 1.25); err == nil || !strings.Contains(err.Error(), "不同的社区服务结算投递快照") {
		t.Fatalf("changed charge delivery snapshot error = %v", err)
	}
	if err := db.Create(&Log{Id: cancellationInput.RequestLogID, Type: LogTypeConsume, UpstreamSource: "community_offer"}).Error; err != nil {
		t.Fatalf("create delayed cancellation log: %v", err)
	}
	exception, err := ReconcileCancelledPublisherSettlementDelivery(cancellationInput.RequestLogID)
	if err != nil || exception.Status != PublisherSettlementDeliveryStatusException || exception.ExceptionReason == "" {
		t.Fatalf("ReconcileCancelledPublisherSettlementDelivery = %#v, %v", exception, err)
	}
	var cancellationSettlements int64
	if err := db.Model(&PublisherOfferSettlement{}).Where("request_log_id = ?", cancellationInput.RequestLogID).Count(&cancellationSettlements).Error; err != nil || cancellationSettlements != 0 {
		t.Fatalf("cancellation contradiction settlement count = %d, %v", cancellationSettlements, err)
	}
	resolved, err := ResolvePublisherSettlementDeliveryException(cancellationInput.RequestLogID, "admin-user", "已核对退款记录和消费日志，后续账务按人工凭证处理")
	if err != nil || resolved.Status != PublisherSettlementDeliveryStatusResolved || resolved.ResolvedBy != "admin-user" || resolved.Resolution == "" {
		t.Fatalf("ResolvePublisherSettlementDeliveryException = %#v, %v", resolved, err)
	}
	if _, err := MarkPublisherSettlementDeliveryReady(deliveryInput.RequestLogID); err == nil || !strings.Contains(err.Error(), "尚未落库") {
		t.Fatalf("delivery without log error = %v, want pending log", err)
	}
	if err := db.Create(&Log{Id: deliveryInput.RequestLogID, Type: LogTypeConsume, UpstreamSource: "community_offer"}).Error; err != nil {
		t.Fatalf("create community log: %v", err)
	}
	if _, err := MarkPublisherSettlementDeliveryReady(deliveryInput.RequestLogID); err != nil {
		t.Fatalf("MarkPublisherSettlementDeliveryReady: %v", err)
	}
	if err := db.Model(&PublisherService{}).Where("id = ?", service.Id).Updates(map[string]any{
		"status": PublisherServiceStatusSuspended, "last_check_ok": false,
	}).Error; err != nil {
		t.Fatalf("suspend service after completed request: %v", err)
	}
	delivered, err := DeliverPublisherSettlement(deliveryInput.RequestLogID)
	if err != nil || delivered.Status != PublisherSettlementDeliveryStatusDelivered || delivered.Attempts != 1 {
		t.Fatalf("DeliverPublisherSettlement = %#v, %v", delivered, err)
	}
	if _, err := DeliverPublisherSettlement(deliveryInput.RequestLogID); err != nil {
		t.Fatalf("idempotent DeliverPublisherSettlement: %v", err)
	}
	var deliveredSettlements int64
	if err := db.Model(&PublisherOfferSettlement{}).Where("request_log_id = ?", deliveryInput.RequestLogID).Count(&deliveredSettlements).Error; err != nil {
		t.Fatalf("count delivered settlement: %v", err)
	}
	if deliveredSettlements != 1 {
		t.Fatalf("delivered settlement count = %d, want 1", deliveredSettlements)
	}
	var deliveredSettlement PublisherOfferSettlement
	if err := db.Where("request_log_id = ?", deliveryInput.RequestLogID).First(&deliveredSettlement).Error; err != nil {
		t.Fatalf("load delivered settlement: %v", err)
	}
	if deliveredSettlement.PublisherID != publisher.Id || deliveredSettlement.ServiceID != service.Id || deliveredSettlement.ConsumerAmountMicros != 2_000 || deliveredSettlement.PublisherPayableMicros != 1_800 {
		t.Fatalf("delivered historical settlement = %#v", deliveredSettlement)
	}
	if err := DeleteCommunityOfferModelRoute("consumer-user", "gpt-5.4"); err != nil {
		t.Fatalf("DeleteCommunityOfferModelRoute: %v", err)
	}
}

func TestCommunityOfferQuotaForMicrosRoundsUpUSDConversion(t *testing.T) {
	newOpenModelSupplyTestDB(t)
	quota, rate, err := CommunityOfferQuotaForMicros(1)
	if err != nil {
		t.Fatalf("CommunityOfferQuotaForMicros: %v", err)
	}
	if rate <= 0 || quota != 1 {
		t.Fatalf("micro USD quota conversion = quota=%d rate=%v, want rounded one quota", quota, rate)
	}
}

func TestCancelPublisherSettlementDeliveryRejectsUnsafeConditions(t *testing.T) {
	db := newOpenModelSupplyTestDB(t)
	now := helper.GetTimestamp()
	rows := []PublisherSettlementDelivery{
		{RequestLogID: "cancel-no-charge", OfferID: "offer-1", PublisherID: "publisher-1", ServiceID: "service-1", ConsumerUserID: "consumer-1", Model: "gpt-5.4", Currency: "USD", InputTokens: 1, Status: PublisherSettlementDeliveryStatusPrepared, CreatedAt: now - PublisherSettlementDeliveryCancellationMinimumAgeSeconds - 1, UpdatedAt: now},
		{RequestLogID: "cancel-too-young", OfferID: "offer-2", PublisherID: "publisher-2", ServiceID: "service-2", ConsumerUserID: "consumer-2", Model: "gpt-5.4", Currency: "USD", InputTokens: 1, PlannedQuota: 10, ChargedQuota: 10, ChargeRate: 1, ChargeRecorded: true, Status: PublisherSettlementDeliveryStatusPrepared, CreatedAt: now, UpdatedAt: now},
		{RequestLogID: "cancel-with-log", OfferID: "offer-3", PublisherID: "publisher-3", ServiceID: "service-3", ConsumerUserID: "consumer-3", Model: "gpt-5.4", Currency: "USD", InputTokens: 1, PlannedQuota: 10, ChargedQuota: 10, ChargeRate: 1, ChargeRecorded: true, Status: PublisherSettlementDeliveryStatusPrepared, CreatedAt: now - PublisherSettlementDeliveryCancellationMinimumAgeSeconds - 1, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create cancellation rows: %v", err)
	}
	if _, err := CancelPublisherSettlementDelivery("cancel-no-charge", "admin-user", "missing charge snapshot"); err == nil || !strings.Contains(err.Error(), "缺少扣费快照") {
		t.Fatalf("missing charge snapshot cancellation error = %v", err)
	}
	if _, err := CancelPublisherSettlementDelivery("cancel-too-young", "admin-user", "too young"); err == nil || !strings.Contains(err.Error(), "至少等待") {
		t.Fatalf("too-young cancellation error = %v", err)
	}
	if err := db.Create(&Log{Id: "cancel-with-log", Type: LogTypeConsume, UpstreamSource: "community_offer"}).Error; err != nil {
		t.Fatalf("create cancellation log: %v", err)
	}
	if _, err := CancelPublisherSettlementDelivery("cancel-with-log", "admin-user", "log exists"); err == nil || !strings.Contains(err.Error(), "日志已存在") {
		t.Fatalf("logged cancellation error = %v", err)
	}
	var refunds int64
	if err := db.Model(&UserBalanceLot{}).Where("source_type = ?", UserBalanceLotSourceCommunityOfferRefund).Count(&refunds).Error; err != nil || refunds != 0 {
		t.Fatalf("unsafe cancellation refunds = %d, %v", refunds, err)
	}
}
