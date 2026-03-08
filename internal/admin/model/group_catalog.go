package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

type GroupCatalog struct {
	Id           string                    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name         string                    `json:"name" gorm:"type:varchar(64);default:''"`
	Description  string                    `json:"description" gorm:"type:varchar(255);default:''"`
	Source       string                    `json:"source" gorm:"type:varchar(32);default:'system'"`
	BillingRatio float64                   `json:"billing_ratio" gorm:"type:numeric(12,6);not null;default:1"`
	Enabled      bool                      `json:"enabled" gorm:"default:true;index"`
	SortOrder    int                       `json:"sort_order" gorm:"default:0;index"`
	UpdatedAt    int64                     `json:"updated_at" gorm:"bigint;index"`
	Channels     []GroupChannelBindingItem `json:"channels,omitempty" gorm:"-"`
}

func (GroupCatalog) TableName() string {
	return "groups"
}

func ListGroupCatalog() ([]GroupCatalog, error) {
	return listGroupCatalogWithDB(DB)
}

func GetGroupCatalogByID(id string) (GroupCatalog, error) {
	return getGroupCatalogByIDWithDB(DB, id)
}

func CreateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	row, err := createGroupCatalogWithDB(DB, item)
	if err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func CreateGroupCatalogWithChannelBindings(item GroupCatalog, channelIDs []string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		created, err := createGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		if err := replaceGroupChannelBindingsWithDB(tx, created.Id, channelIDs); err != nil {
			return err
		}
		row = created
		return nil
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshAbilityCachesForGroups(row.Id)
	return row, nil
}

func UpdateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	row, err := updateGroupCatalogWithDB(DB, item)
	if err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func UpdateGroupCatalogWithChannelBindings(item GroupCatalog, channelIDs []string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		updated, err := updateGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		if err := replaceGroupChannelBindingsWithDB(tx, updated.Id, channelIDs); err != nil {
			return err
		}
		row = updated
		return nil
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshAbilityCachesForGroups(row.Id)
	return row, nil
}

func DeleteGroupCatalog(id string) error {
	if err := deleteGroupCatalogWithDB(DB, id); err != nil {
		return err
	}
	RefreshAbilityCachesForGroups(id)
	return syncGroupBillingRatiosRuntimeWithDB(DB)
}

func CreateGroupCatalogWithConfig(item GroupCatalog, channelIDs []string, modelConfigs []GroupModelConfigItem) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		created, err := createGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		row = created
		explicitChannels := channelIDs != nil
		explicitModels := modelConfigs != nil
		switch {
		case explicitModels:
			return replaceGroupModelConfigsWithDB(tx, created.Id, channelIDs, modelConfigs, explicitChannels)
		case explicitChannels:
			return replaceGroupChannelBindingsWithDB(tx, created.Id, channelIDs)
		default:
			return nil
		}
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshAbilityCachesForGroups(row.Id)
	return row, nil
}

func UpdateGroupCatalogWithConfig(item GroupCatalog, channelIDs []string, modelConfigs []GroupModelConfigItem, updateChannels bool, updateModels bool) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		updated, err := updateGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		row = updated
		switch {
		case updateModels:
			return replaceGroupModelConfigsWithDB(tx, updated.Id, channelIDs, modelConfigs, updateChannels)
		case updateChannels:
			return replaceGroupChannelBindingsWithDB(tx, updated.Id, channelIDs)
		default:
			return nil
		}
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupBillingRatiosRuntimeWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshAbilityCachesForGroups(row.Id)
	return row, nil
}

func listGroupCatalogWithDB(db *gorm.DB) ([]GroupCatalog, error) {
	rows := make([]GroupCatalog, 0)
	if err := db.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateGroupCatalogChannelsWithDB(db, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func hydrateGroupCatalogChannelsWithDB(db *gorm.DB, rows []GroupCatalog) error {
	if len(rows) == 0 {
		return nil
	}

	groupIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		groupID := strings.TrimSpace(row.Id)
		if groupID == "" {
			continue
		}
		groupIDs = append(groupIDs, groupID)
	}
	if len(groupIDs) == 0 {
		return nil
	}

	type abilityRow struct {
		Group     string `gorm:"column:group"`
		ChannelId string `gorm:"column:channel_id"`
	}

	abilityRows := make([]abilityRow, 0)
	groupCol := `"group"`
	if err := db.Model(&Ability{}).
		Select(groupCol+" as \"group\", channel_id").
		Distinct().
		Where(groupCol+" IN ?", groupIDs).
		Where("channel_id <> ''").
		Find(&abilityRows).Error; err != nil {
		return err
	}

	groupChannelIDs := make(map[string][]string, len(groupIDs))
	channelIDSet := make(map[string]struct{})
	for _, item := range abilityRows {
		groupID := strings.TrimSpace(item.Group)
		channelID := strings.TrimSpace(item.ChannelId)
		if groupID == "" || channelID == "" {
			continue
		}
		groupChannelIDs[groupID] = append(groupChannelIDs[groupID], channelID)
		channelIDSet[channelID] = struct{}{}
	}
	if len(channelIDSet) == 0 {
		return nil
	}

	channelIDs := make([]string, 0, len(channelIDSet))
	for channelID := range channelIDSet {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Strings(channelIDs)

	channels := make([]Channel, 0, len(channelIDs))
	if err := db.
		Select("id", "name", "protocol", "status", "created_time").
		Where("id IN ?", channelIDs).
		Find(&channels).Error; err != nil {
		return err
	}

	channelsByID := make(map[string]GroupChannelBindingItem, len(channels))
	for _, channel := range channels {
		channelsByID[channel.Id] = GroupChannelBindingItem{
			Id:       channel.Id,
			Name:     channel.DisplayName(),
			Protocol: channel.GetProtocol(),
			Status:   channel.Status,
			Updated:  channel.CreatedTime,
			Bound:    true,
		}
	}

	for index := range rows {
		channelIDs := normalizeChannelIDList(groupChannelIDs[rows[index].Id])
		items := make([]GroupChannelBindingItem, 0, len(channelIDs))
		for _, channelID := range channelIDs {
			item, ok := channelsByID[channelID]
			if !ok {
				continue
			}
			items = append(items, item)
		}
		rows[index].Channels = items
	}
	return nil
}

func getGroupCatalogByIDWithDB(db *gorm.DB, id string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func createGroupCatalogWithDB(db *gorm.DB, item GroupCatalog) (GroupCatalog, error) {
	id := strings.TrimSpace(item.Id)
	if id == "" {
		return GroupCatalog{}, fmt.Errorf("分组标识不能为空")
	}
	if len(id) > 64 {
		return GroupCatalog{}, fmt.Errorf("分组标识长度不能超过 64")
	}
	existing := GroupCatalog{}
	if err := db.Where("id = ?", id).First(&existing).Error; err == nil {
		return GroupCatalog{}, fmt.Errorf("分组已存在")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return GroupCatalog{}, err
	}

	maxSortOrder := 0
	if err := db.Model(&GroupCatalog{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder).Error; err != nil {
		return GroupCatalog{}, err
	}
	now := helper.GetTimestamp()
	row := GroupCatalog{
		Id:           id,
		Name:         strings.TrimSpace(item.Name),
		Description:  strings.TrimSpace(item.Description),
		Source:       strings.TrimSpace(item.Source),
		BillingRatio: normalizeGroupBillingRatio(item.BillingRatio),
		Enabled:      true,
		SortOrder:    maxSortOrder + 1,
		UpdatedAt:    now,
	}
	if row.Source == "" {
		row.Source = "manual"
	}
	if err := db.Create(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func updateGroupCatalogWithDB(db *gorm.DB, item GroupCatalog) (GroupCatalog, error) {
	id := strings.TrimSpace(item.Id)
	if id == "" {
		return GroupCatalog{}, fmt.Errorf("分组标识不能为空")
	}
	row := GroupCatalog{}
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	row.Name = strings.TrimSpace(item.Name)
	row.Description = strings.TrimSpace(item.Description)
	row.BillingRatio = normalizeGroupBillingRatio(item.BillingRatio)
	row.Enabled = item.Enabled
	if item.SortOrder > 0 {
		row.SortOrder = item.SortOrder
	}
	row.UpdatedAt = helper.GetTimestamp()
	if err := db.Save(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func deleteGroupCatalogWithDB(db *gorm.DB, id string) error {
	groupID := strings.TrimSpace(id)
	if groupID == "" {
		return fmt.Errorf("分组标识不能为空")
	}
	inUse, err := isGroupInUseWithDB(db, groupID)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("分组正在被用户或渠道使用，无法删除")
	}
	return db.Where("id = ?", groupID).Delete(&GroupCatalog{}).Error
}

func isGroupInUseWithDB(db *gorm.DB, id string) (bool, error) {
	users := make([]User, 0)
	if err := db.Select("group").Find(&users).Error; err != nil {
		return false, err
	}
	for _, user := range users {
		for _, groupID := range parseGroupNamesFromCSV(user.Group) {
			if groupID == id {
				return true, nil
			}
		}
	}

	abilities := make([]Ability, 0)
	if err := db.Select("group").Find(&abilities).Error; err != nil {
		return false, err
	}
	for _, ability := range abilities {
		if strings.TrimSpace(ability.Group) == id {
			return true, nil
		}
	}
	return false, nil
}

func parseGroupNamesFromCSV(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	return normalizeGroupNames(parts)
}

func normalizeGroupNames(names []string) []string {
	unique := make(map[string]struct{}, len(names))
	for _, name := range names {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			continue
		}
		unique[normalized] = struct{}{}
	}
	result := make([]string, 0, len(unique))
	for name := range unique {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
