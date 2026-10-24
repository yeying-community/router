package model

import (
	"testing"

	"github.com/yeying-community/router/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGroupSupplyBindingResolvesVirtualCommunityChannel(t *testing.T) {
	previousDB := DB
	previousSecret := config.JWTSecret
	t.Cleanup(func() {
		DB = previousDB
		config.JWTSecret = previousSecret
	})
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	DB = db
	config.JWTSecret = "group-supply-binding-test-secret"
	if err := db.AutoMigrate(&GroupCatalog{}, &GroupModel{}, &Publisher{}, &PublisherService{}, &PublisherServiceModel{}, &ServiceOffer{}, &GroupSupplyBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	credential, err := encryptPublisherServiceCredential("sk-community-supply")
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	rows := []any{
		&GroupCatalog{Id: "group-1", Name: "高质量", Enabled: true},
		&GroupModel{Group: "group-1", Model: "gpt-5.4", Provider: "openai", Enabled: true},
		&Publisher{Id: "publisher-1", UserId: "publisher-user", DisplayName: "社区发布者", Status: PublisherStatusActive},
		&PublisherService{Id: "service-1", PublisherId: "publisher-1", Name: "上海服务", Protocol: "openai", BaseURL: "https://1.1.1.1/v1", CredentialEncrypted: credential, Status: PublisherServiceStatusReady, LastCheckOK: true},
		&PublisherServiceModel{Id: "service-model-1", ServiceId: "service-1", Model: "gpt-5.4", UpstreamModel: "gpt-5.4", Endpoint: ChannelModelEndpointChat, MeteringUnit: "token", Status: PublisherServiceModelStatusReady},
		&ServiceOffer{Id: "offer-1", PublisherId: "publisher-1", ServiceId: "service-1", Model: "gpt-5.4", Scope: ServiceOfferScopePublic, Currency: "USD", Status: ServiceOfferStatusPublished},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}
	if err := ReplaceGroupSupplyBindings("group-1", []GroupSupplyBindingItem{{OfferId: "offer-1", Bound: true, Enabled: true, Priority: 70}}); err != nil {
		t.Fatalf("ReplaceGroupSupplyBindings: %v", err)
	}
	items, err := ListGroupSupplyBindingItems("group-1")
	if err != nil || len(items) != 1 || !items[0].Bound || !items[0].Enabled || items[0].Priority != 70 {
		t.Fatalf("binding items = %#v, %v", items, err)
	}
	channels, err := ListGroupSupplyChannelsForRequest("group-1", "gpt-5.4", ChannelModelEndpointChat)
	if err != nil || len(channels) != 1 {
		t.Fatalf("virtual channels = %#v, %v", channels, err)
	}
	channel := channels[0]
	if channel.Id != CommunityOfferChannelPrefix+"offer-1" || channel.Key != "sk-community-supply" || channel.GetPriority() != 70 {
		t.Fatalf("virtual channel = %#v", channel)
	}
	assignments, err := ListPublisherGroupAssignments("publisher-user")
	if err != nil || len(assignments) != 1 || assignments[0].GroupName != "高质量" || assignments[0].OfferId != "offer-1" {
		t.Fatalf("publisher assignments = %#v, %v", assignments, err)
	}
}
