package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GroupModelBindingItem struct {
	Model           string `json:"model"`
	ChannelId       string `json:"channel_id"`
	UpstreamModel   string `json:"upstream_model"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Priority        *int64 `json:"priority,omitempty"`
	ChannelName     string `json:"channel_name,omitempty"`
	ChannelProtocol string `json:"channel_protocol,omitempty"`
	ChannelStatus   int    `json:"channel_status,omitempty"`
}

type GroupChannelModelOption struct {
	Model         string `json:"model"`
	UpstreamModel string `json:"upstream_model"`
	Label         string `json:"label"`
}

type GroupChannelModels struct {
	Id       string                    `json:"id"`
	Name     string                    `json:"name"`
	Protocol string                    `json:"protocol"`
	Status   int                       `json:"status"`
	Priority *int64                    `json:"priority,omitempty"`
	Bound    bool                      `json:"bound"`
	Models   []GroupChannelModelOption `json:"models"`
}

type GroupModelViewChannel struct {
	ChannelId       string `json:"channel_id"`
	ChannelName     string `json:"channel_name"`
	ChannelProtocol string `json:"channel_protocol"`
	ChannelStatus   int    `json:"channel_status"`
	UpstreamModel   string `json:"upstream_model"`
	Priority        *int64 `json:"priority,omitempty"`
}

type GroupModelViewItem struct {
	Model    string                  `json:"model"`
	Provider string                  `json:"provider,omitempty"`
	Enabled  bool                    `json:"enabled"`
	Channels []GroupModelViewChannel `json:"channels"`
}

type GroupModelsPayload struct {
	Items []GroupModelViewItem `json:"items"`
}

func ListGroupModelsPayload(groupID string) (GroupModelsPayload, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return GroupModelsPayload{}, fmt.Errorf("分组 ID 不能为空")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return GroupModelsPayload{}, err
	}
	groupModels, err := listGroupModelRowsWithDB(DB, groupCatalog.Id, false)
	if err != nil {
		return GroupModelsPayload{}, err
	}
	bindings, err := listGroupModelBindingItemsWithDB(DB, groupCatalog.Id)
	if err != nil {
		return GroupModelsPayload{}, err
	}
	channelsByModel := make(map[string][]GroupModelViewChannel)
	for _, item := range bindings {
		modelName := strings.TrimSpace(item.Model)
		if modelName == "" {
			continue
		}
		channelsByModel[modelName] = append(channelsByModel[modelName], GroupModelViewChannel{
			ChannelId:       strings.TrimSpace(item.ChannelId),
			ChannelName:     strings.TrimSpace(item.ChannelName),
			ChannelProtocol: strings.TrimSpace(item.ChannelProtocol),
			ChannelStatus:   item.ChannelStatus,
			UpstreamModel:   NormalizeGroupModelChannelUpstreamModel(modelName, item.UpstreamModel),
			Priority:        helperInt64Pointer(item.Priority),
		})
	}
	items := make([]GroupModelViewItem, 0, len(groupModels))
	for _, row := range groupModels {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		modelChannels := append([]GroupModelViewChannel(nil), channelsByModel[modelName]...)
		sort.Slice(modelChannels, func(i, j int) bool {
			left := modelChannels[i]
			right := modelChannels[j]
			leftPriority := int64(0)
			rightPriority := int64(0)
			if left.Priority != nil {
				leftPriority = *left.Priority
			}
			if right.Priority != nil {
				rightPriority = *right.Priority
			}
			if leftPriority != rightPriority {
				return leftPriority > rightPriority
			}
			if left.ChannelName != right.ChannelName {
				return left.ChannelName < right.ChannelName
			}
			return left.ChannelId < right.ChannelId
		})
		items = append(items, GroupModelViewItem{
			Model:    modelName,
			Provider: NormalizeGroupModelChannelProvider(row.Provider),
			Enabled:  row.Enabled,
			Channels: modelChannels,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Model < items[j].Model
	})
	return GroupModelsPayload{Items: items}, nil
}

func ReplaceGroupModels(groupID string, channelIDs []string, items []GroupModelBindingItem, explicitChannels bool) error {
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return err
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return replaceGroupModelsWithDB(tx, groupID, channelIDs, items, explicitChannels)
	}); err != nil {
		return err
	}
	RefreshGroupModelChannelCachesForGroups(groupCatalog.Id)
	return nil
}

func ReplaceSingleGroupModel(groupID string, modelName string, items []GroupModelBindingItem) error {
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return err
	}
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" {
		return fmt.Errorf("模型不能为空")
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return replaceSingleGroupModelWithDB(tx, groupCatalog.Id, normalizedModelName, items)
	}); err != nil {
		return err
	}
	RefreshGroupModelChannelCachesForGroups(groupCatalog.Id)
	return nil
}

func DeleteSingleGroupModel(groupID string, modelName string) error {
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return err
	}
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" {
		return fmt.Errorf("模型不能为空")
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return deleteSingleGroupModelWithDB(tx, groupCatalog.Id, normalizedModelName)
	}); err != nil {
		return err
	}
	RefreshGroupModelChannelCachesForGroups(groupCatalog.Id)
	return nil
}

func replaceSingleGroupModelWithDB(db *gorm.DB, groupID string, modelName string, items []GroupModelBindingItem) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return err
	}
	groupID = groupCatalog.Id
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return fmt.Errorf("模型不能为空")
	}

	modelItems := make([]GroupModelBindingItem, 0, len(items))
	for _, item := range items {
		next := item
		next.Model = modelName
		modelItems = append(modelItems, next)
	}
	normalizedItems, err := normalizeGroupModelBindingItems(modelItems)
	if err != nil {
		return err
	}
	if len(normalizedItems) == 0 {
		return fmt.Errorf("分组模型至少需要一个渠道映射")
	}

	boundChannelIDs, err := listGroupBoundChannelIDsWithDB(db, groupID)
	if err != nil {
		return err
	}
	allowedSet := make(map[string]struct{}, len(boundChannelIDs))
	for _, channelID := range boundChannelIDs {
		allowedSet[channelID] = struct{}{}
	}
	channelsByID, err := loadConfigurableChannelsByIDWithDB(db, boundChannelIDs)
	if err != nil {
		return err
	}
	priorityByChannelID, err := listGroupChannelPriorityByChannelWithDB(db, groupID)
	if err != nil {
		return err
	}

	groupCol := `"group"`
	if err := db.Where(groupCol+" = ? AND model = ?", groupID, modelName).Delete(&GroupModelChannel{}).Error; err != nil {
		return err
	}

	rows := make([]GroupModelChannel, 0, len(normalizedItems))
	provider := ""
	modelEnabled := false
	seenChannels := make(map[string]struct{}, len(normalizedItems))
	for _, item := range normalizedItems {
		if item.Model != modelName {
			return fmt.Errorf("模型不匹配: %s", item.Model)
		}
		if _, ok := allowedSet[item.ChannelId]; !ok {
			return fmt.Errorf("渠道未绑定到当前分组: %s", item.ChannelId)
		}
		channel := channelsByID[item.ChannelId]
		if channel == nil {
			return fmt.Errorf("渠道不存在: %s", item.ChannelId)
		}
		if _, ok := seenChannels[item.ChannelId]; ok {
			return fmt.Errorf("同一分组模型下的渠道不能重复: %s / %s", item.Model, item.ChannelId)
		}
		seenChannels[item.ChannelId] = struct{}{}

		catalog := buildGroupChannelModelCatalog(channel)
		upstreamModel := catalog.ResolveUpstream(item)
		if upstreamModel == "" {
			target := item.UpstreamModel
			if target == "" {
				target = item.Model
			}
			return fmt.Errorf("渠道 %s 不支持模型 %s", item.ChannelId, target)
		}
		itemProvider := NormalizeGroupModelChannelProvider(catalog.ResolveProvider(item, upstreamModel))
		if provider != "" && itemProvider != "" && provider != itemProvider {
			return fmt.Errorf("同一分组模型仅允许一个供应商: %s (%s / %s)", item.Model, provider, itemProvider)
		}
		if provider == "" {
			provider = itemProvider
		}
		enabled := resolveGroupModelBindingEnabled(item)
		modelEnabled = modelEnabled || enabled
		priority := priorityByChannelID[item.ChannelId]
		if priority == nil {
			priority = resolveGroupModelBindingPriority(item, channel)
		}
		rows = append(rows, GroupModelChannel{
			Group:         groupID,
			Model:         modelName,
			ChannelId:     item.ChannelId,
			UpstreamModel: NormalizeGroupModelChannelUpstreamModel(modelName, upstreamModel),
			Provider:      itemProvider,
			Priority:      priority,
		})
	}

	now := helper.GetTimestamp()
	groupModel := GroupModel{
		Group:     groupID,
		Model:     modelName,
		Provider:  provider,
		Enabled:   modelEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group"}, {Name: "model"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider",
			"enabled",
			"updated_at",
		}),
	}).Create(&groupModel).Error; err != nil {
		return err
	}
	rows = normalizeGroupModelChannelRowsPreserveOrder(rows)
	if len(rows) == 0 {
		return nil
	}
	return db.Create(&rows).Error
}

func deleteSingleGroupModelWithDB(db *gorm.DB, groupID string, modelName string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return err
	}
	groupID = groupCatalog.Id
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return fmt.Errorf("模型不能为空")
	}
	groupCol := `"group"`
	if err := db.Where(groupCol+" = ? AND model = ?", groupID, modelName).Delete(&GroupModelChannel{}).Error; err != nil {
		return err
	}
	return db.Where(groupCol+" = ? AND model = ?", groupID, modelName).Delete(&GroupModel{}).Error
}

func listGroupModelBindingItemsWithDB(db *gorm.DB, groupID string) ([]GroupModelBindingItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	rows := make([]GroupModelChannel, 0)
	groupCol := `"group"`
	if err := db.
		Where(groupCol+" = ?", groupCatalog.Id).
		Order("model asc, priority desc, channel_id asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []GroupModelBindingItem{}, nil
	}

	groupModels, err := listGroupModelRowsWithDB(db, groupCatalog.Id, false)
	if err != nil {
		return nil, err
	}
	enabledByModel := make(map[string]bool, len(groupModels))
	for _, row := range groupModels {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		enabledByModel[modelName] = row.Enabled
	}

	channelIDs := make([]string, 0, len(rows))
	channelIDSet := make(map[string]struct{}, len(rows))
	for _, item := range rows {
		channelID := strings.TrimSpace(item.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := channelIDSet[channelID]; ok {
			continue
		}
		channelIDSet[channelID] = struct{}{}
		channelIDs = append(channelIDs, channelID)
	}
	sort.Strings(channelIDs)

	channels := make([]Channel, 0, len(channelIDs))
	if err := db.
		Select("id", "name", "protocol", "status").
		Where("id IN ?", channelIDs).
		Find(&channels).Error; err != nil {
		return nil, err
	}
	channelByID := make(map[string]Channel, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		channelByID[channelID] = channel
	}

	items := buildGroupModelBindingItems(rows, channelByID, enabledByModel)
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		if left.ChannelName != right.ChannelName {
			return left.ChannelName < right.ChannelName
		}
		return left.ChannelId < right.ChannelId
	})
	return items, nil
}

func buildGroupModelBindingItems(rows []GroupModelChannel, channelByID map[string]Channel, enabledByModel map[string]bool) []GroupModelBindingItem {
	if len(rows) == 0 {
		return []GroupModelBindingItem{}
	}
	items := make([]GroupModelBindingItem, 0, len(rows))
	for _, route := range rows {
		modelName := strings.TrimSpace(route.Model)
		channelID := strings.TrimSpace(route.ChannelId)
		if modelName == "" || channelID == "" {
			continue
		}
		channel, ok := channelByID[channelID]
		if !ok {
			continue
		}
		enabled, ok := enabledByModel[modelName]
		if !ok {
			enabled = true
		}
		items = append(items, GroupModelBindingItem{
			Model:           modelName,
			ChannelId:       channelID,
			UpstreamModel:   NormalizeGroupModelChannelUpstreamModel(modelName, route.UpstreamModel),
			Enabled:         helperBoolPointer(enabled),
			Priority:        helperInt64Pointer(route.Priority),
			ChannelName:     channel.DisplayName(),
			ChannelProtocol: channel.GetProtocol(),
			ChannelStatus:   channel.Status,
		})
	}
	return items
}

func listGroupChannelModelsWithDB(db *gorm.DB, groupID string) ([]GroupChannelModels, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	boundIDs, err := listGroupBoundChannelIDsWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	boundSet := make(map[string]struct{}, len(boundIDs))
	for _, channelID := range boundIDs {
		boundSet[channelID] = struct{}{}
	}

	channels := make([]Channel, 0)
	if err := db.
		Select("id", "name", "protocol", "status", "priority", "created_time").
		Where("status = ?", ChannelStatusEnabled).
		Order("created_time desc").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	channelRefs := make([]*Channel, 0, len(channels))
	for i := range channels {
		channelRefs = append(channelRefs, &channels[i])
	}
	if err := HydrateChannelsWithModels(db, channelRefs); err != nil {
		return nil, err
	}
	priorityByChannelID, err := listGroupChannelPriorityByChannelWithDB(db, groupID)
	if err != nil {
		return nil, err
	}

	items := make([]GroupChannelModels, 0, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		_, bound := boundSet[channelID]
		items = append(items, GroupChannelModels{
			Id:       channelID,
			Name:     channel.DisplayName(),
			Protocol: channel.GetProtocol(),
			Status:   channel.Status,
			Priority: resolveGroupChannelPriority(bound, priorityByChannelID[channelID], channel.Priority),
			Bound:    bound,
			Models:   buildGroupChannelModelOptions(&channel),
		})
	}
	return items, nil
}

func replaceGroupModelsWithDB(db *gorm.DB, groupID string, channelIDs []string, items []GroupModelBindingItem, explicitChannels bool) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("分组 ID 不能为空")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return err
	}
	groupID = groupCatalog.Id

	normalizedItems, err := normalizeGroupModelBindingItems(items)
	if err != nil {
		return err
	}
	allowedChannelIDs, channelsByID, err := loadGroupModelBindingChannelsByIDWithDB(db, groupID, channelIDs, explicitChannels, normalizedItems)
	if err != nil {
		return err
	}
	if err := syncGroupChannelRowsByChannelIDsDB(db, groupID, allowedChannelIDs); err != nil {
		return err
	}

	allowedSet := make(map[string]struct{}, len(allowedChannelIDs))
	for _, channelID := range allowedChannelIDs {
		allowedSet[channelID] = struct{}{}
	}
	selectedCatalogs := make(map[string]groupChannelModelCatalog, len(channelsByID))
	for channelID, channel := range channelsByID {
		selectedCatalogs[channelID] = buildGroupChannelModelCatalog(channel)
	}

	groupModels := make([]GroupModel, 0, len(normalizedItems))
	groupModelProviders := make(map[string]string, len(normalizedItems))
	seenKeys := make(map[string]struct{}, len(normalizedItems))
	for _, item := range normalizedItems {
		if _, ok := allowedSet[item.ChannelId]; !ok {
			return fmt.Errorf("渠道未绑定到当前分组: %s", item.ChannelId)
		}
		channel := channelsByID[item.ChannelId]
		if channel == nil {
			return fmt.Errorf("渠道不存在: %s", item.ChannelId)
		}
		upstreamModel := selectedCatalogs[item.ChannelId].ResolveUpstream(item)
		if upstreamModel == "" {
			target := item.UpstreamModel
			if target == "" {
				target = item.Model
			}
			return fmt.Errorf("渠道 %s 不支持模型 %s", item.ChannelId, target)
		}
		key := item.Model + "::" + item.ChannelId
		if _, ok := seenKeys[key]; ok {
			return fmt.Errorf("同一分组模型下的渠道不能重复: %s / %s", item.Model, item.ChannelId)
		}
		seenKeys[key] = struct{}{}
		provider := NormalizeGroupModelChannelProvider(selectedCatalogs[item.ChannelId].ResolveProvider(item, upstreamModel))
		if existingProvider, ok := groupModelProviders[item.Model]; ok && existingProvider != "" && provider != "" && existingProvider != provider {
			return fmt.Errorf("同一分组模型仅允许一个供应商: %s (%s / %s)", item.Model, existingProvider, provider)
		}
		if _, ok := groupModelProviders[item.Model]; !ok {
			groupModelProviders[item.Model] = provider
			groupModels = append(groupModels, GroupModel{
				Group:    groupID,
				Model:    item.Model,
				Provider: provider,
				Enabled:  resolveGroupModelBindingEnabled(item),
			})
		} else if groupModelProviders[item.Model] == "" && provider != "" {
			groupModelProviders[item.Model] = provider
			for index := range groupModels {
				if groupModels[index].Model == item.Model {
					groupModels[index].Provider = provider
					break
				}
			}
		}
	}
	if err := replaceGroupModelRowsWithDB(db, groupID, groupModels); err != nil {
		return err
	}

	groupCol := `"group"`
	rows := make([]GroupModelChannel, 0, len(groupModels)*len(allowedChannelIDs))
	priorityByChannelID, err := listGroupChannelPriorityByChannelWithDB(db, groupID)
	if err != nil {
		return err
	}
	for _, channelID := range allowedChannelIDs {
		channel := channelsByID[channelID]
		if channel == nil {
			continue
		}
		rows = append(rows, BuildGroupModelChannelsForChannel(groupID, channel, groupModels, priorityByChannelID[channelID])...)
	}
	rows = normalizeGroupModelChannelRowsPreserveOrder(rows)

	if err := db.Where(groupCol+" = ?", groupID).Delete(&GroupModelChannel{}).Error; err != nil {
		return err
	}
	if len(rows) > 0 {
		if err := db.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeGroupModelBindingItems(items []GroupModelBindingItem) ([]GroupModelBindingItem, error) {
	if len(items) == 0 {
		return []GroupModelBindingItem{}, nil
	}
	result := make([]GroupModelBindingItem, 0, len(items))
	for _, item := range items {
		normalized := GroupModelBindingItem{
			Model:         strings.TrimSpace(item.Model),
			ChannelId:     strings.TrimSpace(item.ChannelId),
			UpstreamModel: strings.TrimSpace(item.UpstreamModel),
			Enabled:       item.Enabled,
			Priority:      helperInt64Pointer(item.Priority),
		}
		if normalized.Model == "" && normalized.ChannelId == "" && normalized.UpstreamModel == "" {
			continue
		}
		if normalized.Model == "" || normalized.ChannelId == "" {
			return nil, fmt.Errorf("分组模型存在未填写完整的行")
		}
		result = append(result, normalized)
	}
	return result, nil
}

func loadGroupModelBindingChannelsByIDWithDB(db *gorm.DB, groupID string, channelIDs []string, explicitChannels bool, items []GroupModelBindingItem) ([]string, map[string]*Channel, error) {
	allowedChannelIDs := normalizeChannelIDList(channelIDs)
	if !explicitChannels && len(allowedChannelIDs) == 0 {
		boundIDs, err := listGroupBoundChannelIDsWithDB(db, groupID)
		if err != nil {
			return nil, nil, err
		}
		allowedChannelIDs = boundIDs
	}
	if len(allowedChannelIDs) == 0 {
		allowedChannelIDs = collectChannelIDsFromGroupModelBindingItems(items)
	}
	if len(allowedChannelIDs) == 0 {
		return []string{}, map[string]*Channel{}, nil
	}

	channelsByID, err := loadConfigurableChannelsByIDWithDB(db, allowedChannelIDs)
	if err != nil {
		return nil, nil, err
	}
	return allowedChannelIDs, channelsByID, nil
}

func loadConfigurableChannelsByIDWithDB(db *gorm.DB, channelIDs []string) (map[string]*Channel, error) {
	channelsByID, err := loadChannelsByIDWithDB(db, channelIDs)
	if err != nil {
		return nil, err
	}
	if len(channelsByID) == 0 {
		return channelsByID, nil
	}

	normalizedChannelIDs := normalizeChannelIDList(channelIDs)
	if len(channelsByID) != len(normalizedChannelIDs) {
		missing := make([]string, 0)
		for _, channelID := range normalizedChannelIDs {
			if _, ok := channelsByID[channelID]; !ok {
				missing = append(missing, channelID)
			}
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("渠道不存在: %s", strings.Join(missing, ", "))
	}

	channelRefs := make([]*Channel, 0, len(channelsByID))
	for _, channelID := range normalizedChannelIDs {
		channel := channelsByID[channelID]
		if channel == nil {
			continue
		}
		channelRefs = append(channelRefs, channel)
	}
	if err := HydrateChannelsWithModels(db, channelRefs); err != nil {
		return nil, err
	}
	return channelsByID, nil
}

func collectChannelIDsFromGroupModelBindingItems(items []GroupModelBindingItem) []string {
	if len(items) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		channelID := strings.TrimSpace(item.ChannelId)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		result = append(result, channelID)
	}
	sort.Strings(result)
	return result
}

func resolveGroupModelBindingEnabled(item GroupModelBindingItem) bool {
	if item.Enabled == nil {
		return true
	}
	return *item.Enabled
}

func resolveGroupModelBindingPriority(item GroupModelBindingItem, channel *Channel) *int64 {
	if item.Priority != nil {
		return helperInt64Pointer(item.Priority)
	}
	if channel == nil {
		return nil
	}
	return helperInt64Pointer(channel.Priority)
}

func helperBoolPointer(value bool) *bool {
	result := value
	return &result
}

func helperInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func channelSelectedModels(channel *Channel) []ChannelModel {
	if channel == nil {
		return nil
	}
	rows := channel.GetChannelModels()
	if len(rows) == 0 {
		return nil
	}
	selected := make([]ChannelModel, 0, len(rows))
	for _, row := range rows {
		if row.Inactive || !row.Selected {
			continue
		}
		selected = append(selected, row)
	}
	return selected
}

func buildGroupChannelModelOptions(channel *Channel) []GroupChannelModelOption {
	selectedConfigs := channelSelectedModels(channel)
	if len(selectedConfigs) == 0 {
		return []GroupChannelModelOption{}
	}
	items := make([]GroupChannelModelOption, 0, len(selectedConfigs))
	for _, row := range selectedConfigs {
		modelName := strings.TrimSpace(row.Model)
		upstream := NormalizeGroupModelChannelUpstreamModel(modelName, row.UpstreamModel)
		label := modelName
		if upstream != "" && upstream != modelName {
			label = fmt.Sprintf("%s -> %s", modelName, upstream)
		}
		items = append(items, GroupChannelModelOption{
			Model:         modelName,
			UpstreamModel: upstream,
			Label:         label,
		})
	}
	return items
}

type groupChannelModelCatalog struct {
	aliasToUpstream  map[string]string
	upstreamSet      map[string]struct{}
	aliasToProvider  map[string]string
	upstreamProvider map[string]string
}

func buildGroupChannelModelCatalog(channel *Channel) groupChannelModelCatalog {
	catalog := groupChannelModelCatalog{
		aliasToUpstream:  make(map[string]string),
		upstreamSet:      make(map[string]struct{}),
		aliasToProvider:  make(map[string]string),
		upstreamProvider: make(map[string]string),
	}
	for _, row := range channelSelectedModels(channel) {
		modelName := strings.TrimSpace(row.Model)
		upstream := NormalizeGroupModelChannelUpstreamModel(modelName, row.UpstreamModel)
		provider := commonutils.NormalizeProvider(row.Provider)
		if modelName != "" {
			catalog.aliasToUpstream[modelName] = upstream
			if provider != "" {
				catalog.aliasToProvider[modelName] = provider
			}
		}
		if upstream != "" {
			catalog.upstreamSet[upstream] = struct{}{}
			if provider != "" {
				catalog.upstreamProvider[upstream] = provider
			}
		}
	}
	return catalog
}

func (catalog groupChannelModelCatalog) ResolveUpstream(item GroupModelBindingItem) string {
	upstream := strings.TrimSpace(item.UpstreamModel)
	if upstream != "" {
		if _, ok := catalog.upstreamSet[upstream]; ok {
			return upstream
		}
		return ""
	}
	if resolved, ok := catalog.aliasToUpstream[strings.TrimSpace(item.Model)]; ok {
		return resolved
	}
	return ""
}

func (catalog groupChannelModelCatalog) ResolveProvider(item GroupModelBindingItem, resolvedUpstream string) string {
	modelName := strings.TrimSpace(item.Model)
	if modelName != "" {
		if provider, ok := catalog.aliasToProvider[modelName]; ok && provider != "" {
			return provider
		}
	}
	upstream := strings.TrimSpace(resolvedUpstream)
	if upstream == "" {
		upstream = strings.TrimSpace(item.UpstreamModel)
	}
	if upstream != "" {
		if provider, ok := catalog.upstreamProvider[upstream]; ok && provider != "" {
			return provider
		}
	}
	return ""
}
