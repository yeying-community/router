package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	GroupChannelsTableName = "group_channels"
)

type GroupChannel struct {
	Group        string  `json:"group" gorm:"column:group;primaryKey;type:varchar(32);autoIncrement:false"`
	ChannelId    string  `json:"channel_id" gorm:"primaryKey;type:varchar(64);autoIncrement:false;index"`
	Enabled      bool    `json:"enabled" gorm:"not null;index"`
	Priority     int64   `json:"priority" gorm:"bigint;not null;default:0;index"`
	BillingRatio float64 `json:"billing_ratio" gorm:"type:numeric(12,6);not null;default:1"`
	CreatedAt    int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt    int64   `json:"updated_at" gorm:"bigint;index"`
}

func (GroupChannel) TableName() string {
	return GroupChannelsTableName
}

func listGroupChannelRowsWithDB(db *gorm.DB, groupID string) ([]GroupChannel, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := resolveGroupCatalogByReferenceWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	groupCol := `"group"`
	rows := make([]GroupChannel, 0)
	if err := db.
		Where(groupCol+" = ?", groupCatalog.Id).
		Order("priority desc, channel_id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}
	channelIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		channelIDs = append(channelIDs, channelID)
	}
	channelIDs = normalizeChannelIDList(channelIDs)
	if len(channelIDs) == 0 {
		return []GroupChannel{}, nil
	}
	channelsByID, err := loadChannelsByIDWithDB(db, channelIDs)
	if err != nil {
		return nil, err
	}
	filtered := make([]GroupChannel, 0, len(rows))
	orphanIDs := make([]string, 0)
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := channelsByID[channelID]; !ok {
			orphanIDs = append(orphanIDs, channelID)
			continue
		}
		filtered = append(filtered, row)
	}
	if len(orphanIDs) > 0 {
		if err := db.Where(groupCol+" = ? AND channel_id IN ?", groupCatalog.Id, orphanIDs).Delete(&GroupChannel{}).Error; err != nil {
			return nil, err
		}
	}
	return filtered, nil
}

func listGroupBoundChannelIDsWithDB(db *gorm.DB, groupID string) ([]string, error) {
	rows, err := listGroupChannelRowsWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		result = append(result, channelID)
	}
	return normalizeChannelIDList(result), nil
}

func listGroupChannelPriorityByChannelWithDB(db *gorm.DB, groupID string) (map[string]*int64, error) {
	result := make(map[string]*int64)
	rows, err := listGroupChannelRowsWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" || !row.Enabled {
			continue
		}
		priority := row.Priority
		result[channelID] = &priority
	}
	return result, nil
}

func ListGroupChannelPriorityByChannelWithDB(db *gorm.DB, groupID string) (map[string]*int64, error) {
	return listGroupChannelPriorityByChannelWithDB(db, groupID)
}

func replaceGroupChannelRowsWithItemsDB(db *gorm.DB, groupID string, items []GroupChannelItem) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := resolveGroupCatalogByReferenceWithDB(db, groupID)
	if err != nil {
		return err
	}
	groupID = groupCatalog.Id

	normalizedItems := normalizeGroupChannelItems(items)
	now := helper.GetTimestamp()
	channelIDs := make([]string, 0, len(normalizedItems))
	for _, item := range normalizedItems {
		if !item.Bound {
			continue
		}
		channelIDs = append(channelIDs, item.Id)
	}
	channelIDs = normalizeChannelIDList(channelIDs)
	channelsByID, err := loadChannelsByIDWithDB(db, channelIDs)
	if err != nil {
		return err
	}
	existingRows, err := listGroupChannelRowsWithDB(db, groupID)
	if err != nil {
		return err
	}
	existingByChannelID := make(map[string]GroupChannel, len(existingRows))
	for _, row := range existingRows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		existingByChannelID[channelID] = row
	}

	rows := make([]GroupChannel, 0, len(channelIDs))
	for _, item := range normalizedItems {
		if !item.Bound {
			continue
		}
		channel, ok := channelsByID[item.Id]
		if !ok {
			return fmt.Errorf("渠道不存在: %s", item.Id)
		}
		if channel.Status != ChannelStatusEnabled {
			return fmt.Errorf("渠道未启用: %s", item.Id)
		}
		existing, hasExisting := existingByChannelID[item.Id]
		priority := resolveGroupChannelPriority(true, item.Priority, channel.Priority)
		createdAt := now
		if hasExisting && existing.CreatedAt > 0 {
			createdAt = existing.CreatedAt
		}
		rows = append(rows, GroupChannel{
			Group:        groupID,
			ChannelId:    item.Id,
			Enabled:      true,
			Priority:     toSafeGroupChannelPriority(priority),
			BillingRatio: normalizeGroupChannelItemBillingRatio(item.BillingRatio),
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		})
	}

	groupCol := `"group"`
	if err := db.Where(groupCol+" = ?", groupID).Delete(&GroupChannel{}).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Create(&rows).Error
}

func syncGroupChannelRowsByChannelIDsDB(db *gorm.DB, groupID string, channelIDs []string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := resolveGroupCatalogByReferenceWithDB(db, groupID)
	if err != nil {
		return err
	}
	groupID = groupCatalog.Id

	normalizedChannelIDs := normalizeChannelIDList(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		groupCol := `"group"`
		return db.Where(groupCol+" = ?", groupID).Delete(&GroupChannel{}).Error
	}
	existingRows, err := listGroupChannelRowsWithDB(db, groupID)
	if err != nil {
		return err
	}
	existingByChannelID := make(map[string]GroupChannel, len(existingRows))
	for _, row := range existingRows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		existingByChannelID[channelID] = row
	}
	channelsByID, err := loadChannelsByIDWithDB(db, normalizedChannelIDs)
	if err != nil {
		return err
	}
	now := helper.GetTimestamp()
	rows := make([]GroupChannel, 0, len(normalizedChannelIDs))
	for _, channelID := range normalizedChannelIDs {
		channel, ok := channelsByID[channelID]
		if !ok {
			return fmt.Errorf("渠道不存在: %s", channelID)
		}
		if channel.Status != ChannelStatusEnabled {
			return fmt.Errorf("渠道未启用: %s", channelID)
		}
		existing, hasExisting := existingByChannelID[channelID]
		priority := resolveGroupChannelPriority(true, nil, channel.Priority)
		if hasExisting {
			priority = helperInt64Pointer(&existing.Priority)
		}
		createdAt := now
		if hasExisting && existing.CreatedAt > 0 {
			createdAt = existing.CreatedAt
		}
		billingRatio := 1.0
		if hasExisting {
			billingRatio = normalizeGroupBillingRatio(existing.BillingRatio)
		}
		rows = append(rows, GroupChannel{
			Group:        groupID,
			ChannelId:    channelID,
			Enabled:      true,
			Priority:     toSafeGroupChannelPriority(priority),
			BillingRatio: billingRatio,
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		})
	}
	groupCol := `"group"`
	if err := db.Where(groupCol+" = ?", groupID).Delete(&GroupChannel{}).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group"}, {Name: "channel_id"}},
		UpdateAll: true,
	}).Create(&rows).Error
}

func backfillGroupChannelBillingRatioWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := db.AutoMigrate(&GroupChannel{}); err != nil {
		return err
	}
	return syncGroupRuntimeCachesWithDB(db)
}

func migrateGroupChannelsWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := migrateGroupModelRoutesTableWithDB(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&GroupChannel{}); err != nil {
		return err
	}
	type sourceRow struct {
		Group     string `gorm:"column:group"`
		ChannelId string `gorm:"column:channel_id"`
		Priority  *int64 `gorm:"column:priority"`
	}
	groupCol := `"group"`
	sourceRows := make([]sourceRow, 0)
	if err := db.Model(&GroupModelChannel{}).
		Select(groupCol + " as \"group\", channel_id, MAX(priority) AS priority").
		Where("channel_id <> ''").
		Group(groupCol + ", channel_id").
		Find(&sourceRows).Error; err != nil {
		return err
	}
	if len(sourceRows) == 0 {
		return nil
	}

	groupOrder := make([]string, 0)
	groupChannelIDs := make(map[string][]string)
	priorityByGroupChannel := make(map[string]*int64, len(sourceRows))
	seenGroup := make(map[string]struct{})
	for _, row := range sourceRows {
		groupID := strings.TrimSpace(row.Group)
		channelID := strings.TrimSpace(row.ChannelId)
		if groupID == "" || channelID == "" {
			continue
		}
		if _, ok := seenGroup[groupID]; !ok {
			seenGroup[groupID] = struct{}{}
			groupOrder = append(groupOrder, groupID)
		}
		groupChannelIDs[groupID] = append(groupChannelIDs[groupID], channelID)
		priorityByGroupChannel[groupID+"::"+channelID] = helperInt64Pointer(row.Priority)
	}
	sort.Strings(groupOrder)
	if len(groupOrder) == 0 {
		return nil
	}

	for _, groupID := range groupOrder {
		channelIDs := normalizeChannelIDList(groupChannelIDs[groupID])
		if err := backfillGroupChannelRowsFromGroupModelRouteDB(db, groupID, channelIDs, priorityByGroupChannel); err != nil {
			return err
		}
	}
	return nil
}

func renameLegacyGroupChannelsTableWithDB(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	migrator := db.Migrator()
	if migrator.HasTable(GroupChannelsTableName) {
		return migrator.AutoMigrate(&GroupChannel{})
	}
	if migrator.HasTable("group_channel_bindings") {
		if err := migrator.RenameTable("group_channel_bindings", GroupChannelsTableName); err != nil {
			return err
		}
	}
	return migrator.AutoMigrate(&GroupChannel{})
}

func backfillGroupChannelRowsFromGroupModelRouteDB(db *gorm.DB, groupID string, channelIDs []string, priorityByGroupChannel map[string]*int64) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := resolveGroupCatalogByReferenceWithDB(db, groupID)
	if err != nil {
		return err
	}
	normalizedChannelIDs := normalizeChannelIDList(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		return nil
	}
	channelsByID, err := loadChannelsByIDWithDB(db, normalizedChannelIDs)
	if err != nil {
		return err
	}
	existingRows, err := listGroupChannelRowsWithDB(db, groupCatalog.Id)
	if err != nil {
		return err
	}
	existingByChannelID := make(map[string]GroupChannel, len(existingRows))
	for _, row := range existingRows {
		channelID := strings.TrimSpace(row.ChannelId)
		if channelID == "" {
			continue
		}
		existingByChannelID[channelID] = row
	}
	now := helper.GetTimestamp()
	rows := make([]GroupChannel, 0, len(normalizedChannelIDs))
	for _, channelID := range normalizedChannelIDs {
		channel, ok := channelsByID[channelID]
		if !ok {
			continue
		}
		existing, hasExisting := existingByChannelID[channelID]
		priority := resolveGroupChannelPriority(true, priorityByGroupChannel[groupID+"::"+channelID], channel.Priority)
		createdAt := now
		if hasExisting && existing.CreatedAt > 0 {
			createdAt = existing.CreatedAt
		}
		billingRatio := 1.0
		if hasExisting {
			billingRatio = normalizeGroupBillingRatio(existing.BillingRatio)
		}
		rows = append(rows, GroupChannel{
			Group:        groupCatalog.Id,
			ChannelId:    channelID,
			Enabled:      channel.Status == ChannelStatusEnabled,
			Priority:     toSafeGroupChannelPriority(priority),
			BillingRatio: billingRatio,
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		})
	}
	groupCol := `"group"`
	if err := db.Where(groupCol+" = ?", groupCatalog.Id).Delete(&GroupChannel{}).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	return db.Create(&rows).Error
}

func loadChannelsByIDWithDB(db *gorm.DB, channelIDs []string) (map[string]*Channel, error) {
	result := make(map[string]*Channel)
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelIDs := normalizeChannelIDList(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		return result, nil
	}
	rows := make([]Channel, 0, len(normalizedChannelIDs))
	if err := db.Where("id IN ?", normalizedChannelIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].NormalizeIdentity()
		channelID := strings.TrimSpace(rows[i].Id)
		if channelID == "" {
			continue
		}
		result[channelID] = &rows[i]
	}
	return result, nil
}

func toSafeGroupChannelPriority(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
