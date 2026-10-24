package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const GroupSupplyBindingsTableName = "group_supply_bindings"

type GroupSupplyBinding struct {
	GroupId   string `json:"group_id" gorm:"column:group;type:char(36);primaryKey"`
	Model     string `json:"model" gorm:"type:varchar(255);primaryKey"`
	OfferId   string `json:"offer_id" gorm:"type:char(36);primaryKey;index"`
	Priority  int64  `json:"priority" gorm:"bigint;not null;default:0"`
	Enabled   bool   `json:"enabled" gorm:"not null;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint;not null;index"`
}

func (GroupSupplyBinding) TableName() string { return GroupSupplyBindingsTableName }

type GroupSupplyBindingItem struct {
	GroupId              string `json:"group_id"`
	OfferId              string `json:"offer_id"`
	Model                string `json:"model"`
	PublisherDisplayName string `json:"publisher_display_name"`
	PublisherLevel       string `json:"publisher_level"`
	ServiceName          string `json:"service_name"`
	Region               string `json:"region"`
	Bound                bool   `json:"bound"`
	Priority             int64  `json:"priority"`
	Enabled              bool   `json:"enabled"`
}

type PublisherGroupAssignment struct {
	GroupId   string `json:"group_id"`
	GroupName string `json:"group_name"`
	OfferId   string `json:"offer_id"`
	ServiceId string `json:"service_id"`
	Model     string `json:"model"`
	Priority  int64  `json:"priority"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt int64  `json:"updated_at"`
}

func ListGroupSupplyBindingItems(groupID string) ([]GroupSupplyBindingItem, error) {
	group, err := GetGroupCatalogByID(groupID)
	if err != nil {
		return nil, err
	}
	offers, err := ListPublishedServiceOfferCatalog("", 200)
	if err != nil {
		return nil, err
	}
	bindings := make([]GroupSupplyBinding, 0)
	if DB != nil && DB.Migrator().HasTable(&GroupSupplyBinding{}) {
		if err := DB.Where("\"group\" = ?", group.Id).Find(&bindings).Error; err != nil {
			return nil, err
		}
	}
	byOffer := make(map[string]GroupSupplyBinding, len(bindings))
	for _, binding := range bindings {
		byOffer[strings.TrimSpace(binding.OfferId)] = binding
	}
	items := make([]GroupSupplyBindingItem, 0, len(offers))
	for _, offer := range offers {
		binding, bound := byOffer[strings.TrimSpace(offer.OfferID)]
		items = append(items, GroupSupplyBindingItem{
			GroupId: group.Id, OfferId: offer.OfferID, Model: offer.Model,
			PublisherDisplayName: offer.PublisherDisplayName, PublisherLevel: offer.PublisherLevel,
			ServiceName: offer.ServiceName, Region: offer.Region, Bound: bound,
			Priority: binding.Priority, Enabled: bound && binding.Enabled,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Model != items[j].Model {
			return items[i].Model < items[j].Model
		}
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].OfferId < items[j].OfferId
	})
	return items, nil
}

func ReplaceGroupSupplyBindings(groupID string, items []GroupSupplyBindingItem) error {
	group, err := GetGroupCatalogByID(groupID)
	if err != nil {
		return err
	}
	allowedModels, err := ListGroupModelNamesByDB(DB, group.Id)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(allowedModels))
	for _, modelName := range allowedModels {
		allowed[strings.TrimSpace(modelName)] = struct{}{}
	}
	now := helper.GetTimestamp()
	rows := make([]GroupSupplyBinding, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !item.Bound {
			continue
		}
		offerID := strings.TrimSpace(item.OfferId)
		if offerID == "" {
			return errors.New("社区报价不能为空")
		}
		if _, duplicate := seen[offerID]; duplicate {
			continue
		}
		catalog, err := GetPublishedServiceOfferCatalogItem(offerID)
		if err != nil {
			return errors.New("社区报价不存在或当前不可用")
		}
		if _, ok := allowed[catalog.Model]; !ok {
			return errors.New("社区报价模型不在当前分组模型范围内")
		}
		seen[offerID] = struct{}{}
		rows = append(rows, GroupSupplyBinding{
			GroupId: group.Id, Model: catalog.Model, OfferId: offerID,
			Priority: item.Priority, Enabled: item.Enabled, CreatedAt: now, UpdatedAt: now,
		})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("\"group\" = ?", group.Id).Delete(&GroupSupplyBinding{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func ListGroupSupplyChannelsForRequest(groupID string, modelName string, requestPath string) ([]*Channel, error) {
	if DB == nil || !DB.Migrator().HasTable(&GroupSupplyBinding{}) {
		return []*Channel{}, nil
	}
	bindings := make([]GroupSupplyBinding, 0)
	if err := DB.Where("\"group\" = ? AND model = ? AND enabled = ?", strings.TrimSpace(groupID), strings.TrimSpace(modelName), true).
		Order("priority desc, offer_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	channels := make([]*Channel, 0, len(bindings))
	for _, binding := range bindings {
		channel, err := ResolveCommunityOfferChannel(binding.OfferId, binding.Model, requestPath)
		if err != nil {
			continue
		}
		priority := binding.Priority
		channel.Priority = &priority
		channels = append(channels, channel)
	}
	return channels, nil
}

func ListPublisherGroupAssignments(userID string) ([]PublisherGroupAssignment, error) {
	if DB == nil || !DB.Migrator().HasTable(&GroupSupplyBinding{}) ||
		!DB.Migrator().HasTable(PublishersTableName) ||
		!DB.Migrator().HasTable(ServiceOffersTableName) ||
		!DB.Migrator().HasTable("groups") {
		return []PublisherGroupAssignment{}, nil
	}
	publisher, err := publisherForUserWithDB(DB, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]PublisherGroupAssignment, 0)
	err = DB.Table(GroupSupplyBindingsTableName+" AS b").
		Select("b.\"group\" AS group_id, g.name AS group_name, b.offer_id, o.service_id, b.model, b.priority, b.enabled, b.updated_at").
		Joins("JOIN "+ServiceOffersTableName+" AS o ON o.id = b.offer_id AND o.publisher_id = ?", publisher.Id).
		Joins("JOIN groups AS g ON g.id = b.\"group\"").
		Order("g.sort_order desc, g.name asc, b.model asc, b.priority desc").
		Scan(&rows).Error
	return rows, err
}

func ListGroupNamesByOfferIDs(offerIDs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	ids := normalizeTrimmedValuesPreserveOrder(offerIDs)
	if len(ids) == 0 {
		return result, nil
	}
	if DB == nil || !DB.Migrator().HasTable(&GroupSupplyBinding{}) || !DB.Migrator().HasTable("groups") {
		return result, nil
	}
	type row struct {
		OfferID   string `gorm:"column:offer_id"`
		GroupName string `gorm:"column:group_name"`
	}
	rows := make([]row, 0)
	if err := DB.Table(GroupSupplyBindingsTableName+" AS b").
		Select("b.offer_id, g.name AS group_name").
		Joins("JOIN groups AS g ON g.id = b.\"group\"").
		Where("b.offer_id IN ? AND b.enabled = ?", ids, true).
		Order("g.sort_order desc, g.name asc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.OfferID] = append(result[item.OfferID], item.GroupName)
	}
	return result, nil
}
