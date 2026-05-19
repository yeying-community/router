package model

import (
	"fmt"
	"sort"
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

type GroupChannelItem struct {
	Id           string                    `json:"id"`
	Name         string                    `json:"name"`
	Protocol     string                    `json:"protocol"`
	Status       int                       `json:"status"`
	Models       string                    `json:"models"`
	ModelOptions []GroupChannelModelOption `json:"model_options,omitempty"`
	Bound        bool                      `json:"bound"`
	Priority     *int64                    `json:"priority,omitempty"`
	Updated      int64                     `json:"updated_at"`
}

func ListGroupChannels(groupID string) ([]GroupChannelItem, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("分组 ID 不能为空")
	}
	return listGroupChannelsWithDB(DB, groupID, true)
}

func listGroupChannelsWithDB(db *gorm.DB, groupID string, enabledOnly bool) ([]GroupChannelItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupID = strings.TrimSpace(groupID)

	channels := make([]Channel, 0)
	query := db.
		Select("id", "name", "protocol", "status", "created_time").
		Order("created_time desc")
	if enabledOnly {
		query = query.Where("status = ?", ChannelStatusEnabled)
	}
	if err := query.Find(&channels).Error; err != nil {
		return nil, err
	}
	channelRefs := make([]*Channel, 0, len(channels))
	for i := range channels {
		channelRefs = append(channelRefs, &channels[i])
	}
	if err := HydrateChannelsWithModels(db, channelRefs); err != nil {
		return nil, err
	}

	channelRows, err := listGroupChannelRowsWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	boundSet := make(map[string]struct{}, len(channelRows))
	priorityByChannelID := make(map[string]*int64, len(channelRows))
	updatedByChannelID := make(map[string]int64, len(channelRows))
	for _, row := range channelRows {
		normalized := strings.TrimSpace(row.ChannelId)
		if normalized == "" {
			continue
		}
		boundSet[normalized] = struct{}{}
		priority := row.Priority
		priorityByChannelID[normalized] = &priority
		updatedByChannelID[normalized] = row.UpdatedAt
	}

	items := make([]GroupChannelItem, 0, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		_, bound := boundSet[channelID]
		items = append(items, GroupChannelItem{
			Id:           channelID,
			Name:         channel.DisplayName(),
			Protocol:     channel.GetProtocol(),
			Status:       channel.Status,
			Models:       strings.TrimSpace(channel.Models),
			ModelOptions: buildGroupChannelModelOptions(&channel),
			Bound:        bound,
			Priority:     resolveGroupChannelPriority(bound, priorityByChannelID[channelID], channel.Priority),
			Updated:      resolveGroupChannelUpdatedAt(bound, updatedByChannelID[channelID], channel.CreatedTime),
		})
	}
	return items, nil
}

func ReplaceGroupChannels(groupID string, channelIDs []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return replaceGroupChannelsWithDB(tx, groupID, channelIDs)
	})
}

func ReplaceGroupChannelsWithItems(groupID string, items []GroupChannelItem) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return replaceGroupChannelsWithItemsDB(tx, groupID, items)
	})
}

func replaceGroupChannelsWithDB(db *gorm.DB, groupID string, channelIDs []string) error {
	items := make([]GroupChannelItem, 0, len(channelIDs))
	for _, channelID := range normalizeChannelIDList(channelIDs) {
		items = append(items, GroupChannelItem{
			Id:    channelID,
			Bound: true,
		})
	}
	return replaceGroupChannelsWithItemsDB(db, groupID, items)
}

func replaceGroupChannelsWithItemsDB(db *gorm.DB, groupID string, items []GroupChannelItem) error {
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

	normalizedItems := normalizeGroupChannelItems(items)
	if err := replaceGroupChannelRowsWithItemsDB(db, groupID, normalizedItems); err != nil {
		return err
	}
	normalizedChannelIDs, err := listGroupBoundChannelIDsWithDB(db, groupID)
	if err != nil {
		return err
	}
	priorityByChannelID, err := listGroupChannelPriorityByChannelWithDB(db, groupID)
	if err != nil {
		return err
	}
	channelsByID := make(map[string]Channel, len(normalizedChannelIDs))
	if len(normalizedChannelIDs) > 0 {
		enabledChannels, err := loadEnabledChannelsByIDWithDB(db, normalizedChannelIDs)
		if err != nil {
			return err
		}
		for channelID, channel := range enabledChannels {
			channelsByID[channelID] = *channel
		}
	}

	groupCol := `"group"`
	groupModels, err := listGroupModelRowsWithDB(db, groupID, true)
	if err != nil {
		return err
	}
	rows := make([]GroupModelChannel, 0)
	for _, id := range normalizedChannelIDs {
		channel, ok := channelsByID[id]
		if !ok {
			continue
		}
		channelAbilities := BuildGroupModelChannelsForChannel(groupID, &channel, groupModels, priorityByChannelID[id])
		if priority, ok := priorityByChannelID[id]; ok {
			for idx := range channelAbilities {
				channelAbilities[idx].Priority = helperInt64Pointer(priority)
			}
		}
		rows = append(rows, channelAbilities...)
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
	if _, err := buildGroupModelChannelProviderMap(rows); err != nil {
		return err
	}
	return nil
}

func normalizeGroupChannelItems(items []GroupChannelItem) []GroupChannelItem {
	if len(items) == 0 {
		return []GroupChannelItem{}
	}
	result := make([]GroupChannelItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		channelID := strings.TrimSpace(item.Id)
		if channelID == "" {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}
		result = append(result, GroupChannelItem{
			Id:       channelID,
			Bound:    item.Bound,
			Priority: helperInt64Pointer(item.Priority),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Id < result[j].Id
	})
	return result
}

func resolveGroupChannelPriority(bound bool, abilityPriority *int64, channelPriority *int64) *int64 {
	if bound && abilityPriority != nil {
		return helperInt64Pointer(abilityPriority)
	}
	return helperInt64Pointer(channelPriority)
}

func resolveGroupChannelUpdatedAt(bound bool, bindingUpdatedAt int64, fallback int64) int64 {
	if bound && bindingUpdatedAt > 0 {
		return bindingUpdatedAt
	}
	return fallback
}

func loadEnabledChannelsByIDWithDB(db *gorm.DB, channelIDs []string) (map[string]*Channel, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedChannelIDs := normalizeChannelIDList(channelIDs)
	if len(normalizedChannelIDs) == 0 {
		return map[string]*Channel{}, nil
	}

	channels := make([]Channel, 0, len(normalizedChannelIDs))
	if err := db.
		Select("id", "name", "protocol", "status", "priority", "created_time").
		Where("id IN ?", normalizedChannelIDs).
		Find(&channels).Error; err != nil {
		return nil, err
	}

	channelsByID := make(map[string]*Channel, len(channels))
	disabled := make([]string, 0)
	for i := range channels {
		channel := &channels[i]
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		channelsByID[channelID] = channel
		if channel.Status != ChannelStatusEnabled {
			disabled = append(disabled, channelID)
		}
	}

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
	if len(disabled) > 0 {
		sort.Strings(disabled)
		return nil, fmt.Errorf("渠道未启用，不能绑定到分组: %s", strings.Join(disabled, ", "))
	}

	channelRefs := make([]*Channel, 0, len(channels))
	for i := range channels {
		channelRefs = append(channelRefs, &channels[i])
	}
	if err := HydrateChannelsWithModels(db, channelRefs); err != nil {
		return nil, err
	}
	return channelsByID, nil
}

func normalizeChannelIDList(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, item := range ids {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func normalizeModelNames(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, item := range models {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func BuildGroupModelChannelsForChannel(groupID string, channel *Channel, groupModels []GroupModel, channelPriority *int64) []GroupModelChannel {
	if channel == nil {
		return nil
	}
	catalog := buildGroupChannelModelCatalog(channel)
	result := make([]GroupModelChannel, 0, len(groupModels))
	seenGroupModelChannelKeys := make(map[string]struct{}, len(groupModels))
	priority := helperInt64Pointer(channel.Priority)
	if channelPriority != nil {
		priority = helperInt64Pointer(channelPriority)
	}
	configuredModels := make(map[string]GroupModel, len(groupModels))
	for _, groupModel := range groupModels {
		modelName := strings.TrimSpace(groupModel.Model)
		if modelName == "" {
			continue
		}
		configuredModels[modelName] = groupModel
		upstream, ok := catalog.aliasToUpstream[modelName]
		if !ok || strings.TrimSpace(upstream) == "" {
			continue
		}
		key := modelName + "::" + strings.TrimSpace(channel.Id)
		if _, ok := seenGroupModelChannelKeys[key]; ok {
			continue
		}
		seenGroupModelChannelKeys[key] = struct{}{}
		provider := NormalizeGroupModelChannelProvider(groupModel.Provider)
		if provider == "" {
			provider = NormalizeGroupModelChannelProvider(catalog.ResolveProvider(GroupModelBindingItem{Model: modelName}, upstream))
		}
		result = append(result, GroupModelChannel{
			Group:         strings.TrimSpace(groupID),
			Model:         modelName,
			ChannelId:     strings.TrimSpace(channel.Id),
			UpstreamModel: NormalizeGroupModelChannelUpstreamModel(modelName, upstream),
			Provider:      provider,
			Priority:      priority,
		})
	}
	for _, row := range channelSelectedModels(channel) {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		if _, ok := configuredModels[modelName]; ok {
			continue
		}
		upstream := NormalizeGroupModelChannelUpstreamModel(modelName, row.UpstreamModel)
		if upstream == "" {
			continue
		}
		key := modelName + "::" + strings.TrimSpace(channel.Id)
		if _, ok := seenGroupModelChannelKeys[key]; ok {
			continue
		}
		seenGroupModelChannelKeys[key] = struct{}{}
		provider := NormalizeGroupModelChannelProvider(commonutils.NormalizeProvider(row.Provider))
		if provider == "" {
			provider = NormalizeGroupModelChannelProvider(catalog.ResolveProvider(GroupModelBindingItem{Model: modelName}, upstream))
		}
		result = append(result, GroupModelChannel{
			Group:         strings.TrimSpace(groupID),
			Model:         modelName,
			ChannelId:     strings.TrimSpace(channel.Id),
			UpstreamModel: upstream,
			Provider:      provider,
			Priority:      priority,
		})
	}
	return result
}
