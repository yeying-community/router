package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	"gorm.io/gorm"
)

type GroupCatalog struct {
	Id          string             `json:"id" gorm:"primaryKey;type:char(36)"`
	Name        string             `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Description string             `json:"description" gorm:"type:varchar(255);default:''"`
	Source      string             `json:"source" gorm:"type:varchar(32);default:'system'"`
	Enabled     bool               `json:"enabled" gorm:"index"`
	SortOrder   int                `json:"sort_order" gorm:"default:0;index"`
	CreatedAt   int64              `json:"created_at" gorm:"bigint;index"`
	UpdatedAt   int64              `json:"updated_at" gorm:"bigint;index"`
	Channels    []GroupChannelItem `json:"channels,omitempty" gorm:"-"`
}

func (GroupCatalog) TableName() string {
	return "groups"
}

func (item *GroupCatalog) NormalizeIdentity() {
	if item == nil {
		return
	}
	item.Id = strings.TrimSpace(item.Id)
	item.Name = strings.TrimSpace(item.Name)
}

func (item *GroupCatalog) AfterFind(tx *gorm.DB) error {
	item.NormalizeIdentity()
	return nil
}

func (item *GroupCatalog) EnsureID() {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Id) == "" {
		item.Id = random.GetUUID()
	}
}

func (item *GroupCatalog) Identifier() string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(item.Name)
}

func ListGroupCatalog() ([]GroupCatalog, error) {
	return listGroupCatalogWithDB(DB)
}

func ListGroupCatalogPage(page int, pageSize int, keyword string) ([]GroupCatalog, int64, error) {
	return listGroupCatalogPageWithDB(DB, page, pageSize, keyword)
}

func GetGroupCatalogByID(id string) (GroupCatalog, error) {
	return getGroupCatalogByIDWithDB(DB, id)
}

func ResolveGroupCatalogByReference(groupRef string) (GroupCatalog, error) {
	return resolveGroupCatalogByReferenceWithDB(DB, groupRef)
}

func ResolveUserCreateGroupAssignment(groupRef string) (string, error) {
	return resolveUserCreateGroupAssignmentWithDB(DB, groupRef)
}

func CreateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	row, err := createGroupCatalogWithDB(DB, item)
	if err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func CreateGroupCatalogWithChannels(item GroupCatalog, channelIDs []string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		created, err := createGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		if err := replaceGroupChannelsWithDB(tx, created.Id, channelIDs); err != nil {
			return err
		}
		row = created
		return nil
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshGroupModelChannelCachesForGroups(row.Id)
	return row, nil
}

func UpdateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	row, err := updateGroupCatalogWithDB(DB, item)
	if err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func UpdateGroupCatalogWithChannels(item GroupCatalog, channelIDs []string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		updated, err := updateGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		if err := replaceGroupChannelsWithDB(tx, updated.Id, channelIDs); err != nil {
			return err
		}
		row = updated
		return nil
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshGroupModelChannelCachesForGroups(row.Id)
	return row, nil
}

func DeleteGroupCatalog(id string) error {
	groupRefValues, err := deleteGroupCatalogWithDB(DB, id)
	if err != nil {
		return err
	}
	RefreshGroupModelChannelCachesForGroups(groupRefValues...)
	return syncGroupRuntimeCachesWithDB(DB)
}

func CreateGroupCatalogWithModels(item GroupCatalog, channelIDs []string, models []GroupModelBindingItem) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		created, err := createGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		row = created
		explicitChannels := channelIDs != nil
		explicitModels := models != nil
		switch {
		case explicitModels:
			return replaceGroupModelsWithDB(tx, created.Id, channelIDs, models, explicitChannels)
		case explicitChannels:
			return replaceGroupChannelsWithDB(tx, created.Id, channelIDs)
		default:
			return nil
		}
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshGroupModelChannelCachesForGroups(row.Id)
	return row, nil
}

func UpdateGroupCatalogWithModels(item GroupCatalog, channelIDs []string, models []GroupModelBindingItem, updateChannels bool, updateModels bool) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		updated, err := updateGroupCatalogWithDB(tx, item)
		if err != nil {
			return err
		}
		row = updated
		switch {
		case updateModels:
			return replaceGroupModelsWithDB(tx, updated.Id, channelIDs, models, updateChannels)
		case updateChannels:
			return replaceGroupChannelsWithDB(tx, updated.Id, channelIDs)
		default:
			return nil
		}
	}); err != nil {
		return GroupCatalog{}, err
	}
	if err := syncGroupRuntimeCachesWithDB(DB); err != nil {
		return GroupCatalog{}, err
	}
	RefreshGroupModelChannelCachesForGroups(row.Id)
	return row, nil
}

func listGroupCatalogWithDB(db *gorm.DB) ([]GroupCatalog, error) {
	rows := make([]GroupCatalog, 0)
	if err := db.Order("sort_order asc, name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := hydrateGroupCatalogChannelsWithDB(db, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func listGroupCatalogPageWithDB(db *gorm.DB, page int, pageSize int, keyword string) ([]GroupCatalog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	total := int64(0)
	query := buildGroupCatalogListQueryWithDB(db, keyword)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]GroupCatalog, 0, pageSize)
	if err := query.
		Order("sort_order asc, name asc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if err := hydrateGroupCatalogChannelsWithDB(db, rows); err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func buildGroupCatalogListQueryWithDB(db *gorm.DB, keyword string) *gorm.DB {
	query := db.Model(&GroupCatalog{})
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	return query.Where(
		"LOWER(id) LIKE ? OR LOWER(name) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?",
		likeKeyword,
		likeKeyword,
		likeKeyword,
	)
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

	type bindingRow struct {
		Group     string `gorm:"column:group"`
		ChannelId string `gorm:"column:channel_id"`
	}

	channelRows := make([]bindingRow, 0)
	groupCol := `"group"`
	if err := db.Model(&GroupChannel{}).
		Select(groupCol+" as \"group\", channel_id").
		Where(groupCol+" IN ?", groupIDs).
		Where("enabled = ?", true).
		Where("channel_id <> ''").
		Find(&channelRows).Error; err != nil {
		return err
	}

	groupChannelIDs := make(map[string][]string, len(groupIDs))
	channelIDSet := make(map[string]struct{})
	for _, item := range channelRows {
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
		Where("status = ?", ChannelStatusEnabled).
		Find(&channels).Error; err != nil {
		return err
	}

	channelsByID := make(map[string]GroupChannelItem, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		channelsByID[channelID] = GroupChannelItem{
			Id:       channelID,
			Name:     channel.DisplayName(),
			Protocol: channel.GetProtocol(),
			Status:   channel.Status,
			Updated:  channel.CreatedTime,
			Bound:    true,
		}
	}

	for index := range rows {
		groupID := strings.TrimSpace(rows[index].Id)
		channelIDs := normalizeChannelIDList(groupChannelIDs[groupID])
		items := make([]GroupChannelItem, 0, len(channelIDs))
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

func resolveGroupCatalogByReferenceWithDB(db *gorm.DB, groupRef string) (GroupCatalog, error) {
	normalizedRef := strings.TrimSpace(groupRef)
	if normalizedRef == "" {
		return GroupCatalog{}, gorm.ErrRecordNotFound
	}
	row, err := getGroupCatalogByIDWithDB(db, normalizedRef)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return GroupCatalog{}, err
	}
	return getGroupCatalogByNameWithDB(db, normalizedRef)
}

func resolveUserCreateGroupAssignmentWithDB(db *gorm.DB, groupRef string) (string, error) {
	normalizedRef := strings.TrimSpace(groupRef)
	if normalizedRef != "" {
		row, err := resolveGroupCatalogByReferenceWithDB(db, normalizedRef)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", fmt.Errorf("分组不存在")
			}
			return "", err
		}
		return strings.TrimSpace(row.Id), nil
	}
	return "", nil
}

func getGroupCatalogByNameWithDB(db *gorm.DB, name string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := db.Where("name = ?", strings.TrimSpace(name)).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func createGroupCatalogWithDB(db *gorm.DB, item GroupCatalog) (GroupCatalog, error) {
	item.NormalizeIdentity()
	if item.Identifier() == "" {
		return GroupCatalog{}, fmt.Errorf("分组标识不能为空")
	}
	if len(item.Identifier()) > 64 {
		return GroupCatalog{}, fmt.Errorf("分组标识长度不能超过 64")
	}
	if _, err := getGroupCatalogByNameWithDB(db, item.Identifier()); err == nil {
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
		Id:          strings.TrimSpace(item.Id),
		Name:        item.Identifier(),
		Description: strings.TrimSpace(item.Description),
		Source:      strings.TrimSpace(item.Source),
		Enabled:     true,
		SortOrder:   maxSortOrder + 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	row.EnsureID()
	if row.Source == "" {
		row.Source = "manual"
	}
	if err := db.Create(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func updateGroupCatalogWithDB(db *gorm.DB, item GroupCatalog) (GroupCatalog, error) {
	item.NormalizeIdentity()
	if strings.TrimSpace(item.Id) == "" {
		return GroupCatalog{}, fmt.Errorf("分组 ID 不能为空")
	}
	row := GroupCatalog{}
	if err := db.Where("id = ?", item.Id).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	nextName := row.Identifier()
	if item.Identifier() != "" {
		if len(item.Identifier()) > 64 {
			return GroupCatalog{}, fmt.Errorf("分组标识长度不能超过 64")
		}
		nextName = item.Identifier()
	}
	if nextName == "" {
		return GroupCatalog{}, fmt.Errorf("分组标识不能为空")
	}
	if nextName != row.Identifier() {
		existing, err := getGroupCatalogByNameWithDB(db, nextName)
		if err == nil && existing.Id != row.Id {
			return GroupCatalog{}, fmt.Errorf("分组已存在")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return GroupCatalog{}, err
		}
	}
	row.Name = nextName
	row.Description = strings.TrimSpace(item.Description)
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

func deleteGroupCatalogWithDB(db *gorm.DB, id string) ([]string, error) {
	groupID := strings.TrimSpace(id)
	if groupID == "" {
		return nil, fmt.Errorf("分组 ID 不能为空")
	}
	row, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	groupRefValues := buildGroupReferenceValues(row)
	inUse, err := isGroupInUseWithDB(db, groupRefValues)
	if err != nil {
		return nil, err
	}
	if inUse {
		return nil, fmt.Errorf("分组正在被用户使用，无法删除")
	}
	groupCol := `"group"`
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(groupCol+" IN ?", groupRefValues).Delete(&GroupModelChannel{}).Error; err != nil {
			return err
		}
		if err := tx.Where(groupCol+" IN ?", groupRefValues).Delete(&GroupModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where(groupCol+" IN ?", groupRefValues).Delete(&GroupChannel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", row.Id).Delete(&GroupCatalog{}).Error
	}); err != nil {
		return nil, err
	}
	return groupRefValues, nil
}

func isGroupInUseWithDB(db *gorm.DB, groupRefValues []string) (bool, error) {
	if len(groupRefValues) == 0 {
		return false, nil
	}
	groupRefSet := make(map[string]struct{}, len(groupRefValues))
	for _, value := range groupRefValues {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		groupRefSet[normalized] = struct{}{}
	}
	if len(groupRefSet) == 0 {
		return false, nil
	}

	users := make([]User, 0)
	if err := db.Select("group").Find(&users).Error; err != nil {
		return false, err
	}
	for _, user := range users {
		for _, userGroupID := range parseGroupNamesFromCSV(user.Group) {
			if _, ok := groupRefSet[userGroupID]; ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func buildGroupReferenceValues(row GroupCatalog) []string {
	values := []string{strings.TrimSpace(row.Id)}
	name := strings.TrimSpace(row.Name)
	if name != "" && name != strings.TrimSpace(row.Id) {
		values = append(values, name)
	}
	return normalizeGroupNames(values)
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
