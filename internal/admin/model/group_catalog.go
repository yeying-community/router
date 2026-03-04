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
	Name        string `json:"name" gorm:"primaryKey;type:varchar(32)"`
	DisplayName string `json:"display_name" gorm:"type:varchar(64);default:''"`
	Description string `json:"description" gorm:"type:varchar(255);default:''"`
	Source      string `json:"source" gorm:"type:varchar(32);default:'system'"`
	Enabled     bool   `json:"enabled" gorm:"default:true;index"`
	SortOrder   int    `json:"sort_order" gorm:"default:0;index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;index"`
}

func (GroupCatalog) TableName() string {
	return "groups"
}

func ListEnabledGroupNames() ([]string, error) {
	return listEnabledGroupNamesWithDB(DB)
}

func ListGroupCatalog() ([]GroupCatalog, error) {
	return listGroupCatalogWithDB(DB)
}

func GetGroupCatalogByName(name string) (GroupCatalog, error) {
	return getGroupCatalogByNameWithDB(DB, name)
}

func CreateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	return createGroupCatalogWithDB(DB, item)
}

func UpdateGroupCatalog(item GroupCatalog) (GroupCatalog, error) {
	return updateGroupCatalogWithDB(DB, item)
}

func DeleteGroupCatalog(name string) error {
	return deleteGroupCatalogWithDB(DB, name)
}

func listGroupCatalogWithDB(db *gorm.DB) ([]GroupCatalog, error) {
	rows := make([]GroupCatalog, 0)
	if err := db.Order("sort_order asc, name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func getGroupCatalogByNameWithDB(db *gorm.DB, name string) (GroupCatalog, error) {
	row := GroupCatalog{}
	if err := db.Where("name = ?", strings.TrimSpace(name)).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	return row, nil
}

func listEnabledGroupNamesWithDB(db *gorm.DB) ([]string, error) {
	rows := make([]GroupCatalog, 0)
	if err := db.Where("enabled = ?", true).
		Order("sort_order asc, name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func createGroupCatalogWithDB(db *gorm.DB, item GroupCatalog) (GroupCatalog, error) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return GroupCatalog{}, fmt.Errorf("分组名称不能为空")
	}
	if len(name) > 32 {
		return GroupCatalog{}, fmt.Errorf("分组名称长度不能超过 32")
	}
	existing := GroupCatalog{}
	if err := db.Where("name = ?", name).First(&existing).Error; err == nil {
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
		Name:        name,
		DisplayName: strings.TrimSpace(item.DisplayName),
		Description: strings.TrimSpace(item.Description),
		Source:      strings.TrimSpace(item.Source),
		Enabled:     true,
		SortOrder:   maxSortOrder + 1,
		UpdatedAt:   now,
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
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return GroupCatalog{}, fmt.Errorf("分组名称不能为空")
	}
	row := GroupCatalog{}
	if err := db.Where("name = ?", name).First(&row).Error; err != nil {
		return GroupCatalog{}, err
	}
	row.DisplayName = strings.TrimSpace(item.DisplayName)
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

func deleteGroupCatalogWithDB(db *gorm.DB, name string) error {
	groupName := strings.TrimSpace(name)
	if groupName == "" {
		return fmt.Errorf("分组名称不能为空")
	}
	inUse, err := isGroupInUseWithDB(db, groupName)
	if err != nil {
		return err
	}
	if inUse {
		return fmt.Errorf("分组正在被用户或渠道使用，无法删除")
	}
	return db.Where("name = ?", groupName).Delete(&GroupCatalog{}).Error
}

func isGroupInUseWithDB(db *gorm.DB, name string) (bool, error) {
	users := make([]User, 0)
	if err := db.Select("group").Find(&users).Error; err != nil {
		return false, err
	}
	for _, user := range users {
		for _, groupName := range parseGroupNamesFromCSV(user.Group) {
			if groupName == name {
				return true, nil
			}
		}
	}

	channels := make([]Channel, 0)
	if err := db.Select("group").Find(&channels).Error; err != nil {
		return false, err
	}
	for _, channel := range channels {
		for _, groupName := range parseGroupNamesFromCSV(channel.Group) {
			if groupName == name {
				return true, nil
			}
		}
	}

	abilities := make([]Ability, 0)
	if err := db.Select("group").Find(&abilities).Error; err != nil {
		return false, err
	}
	for _, ability := range abilities {
		if strings.TrimSpace(ability.Group) == name {
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
