package model

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type GroupChannelBindingItem struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Status   int    `json:"status"`
	Models   string `json:"models"`
	Bound    bool   `json:"bound"`
	Updated  int64  `json:"updated_at"`
}

func ListGroupChannelBindings(groupID string) ([]GroupChannelBindingItem, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("分组 ID 不能为空")
	}
	return listGroupChannelBindingsWithDB(DB, groupID, true)
}

func listGroupChannelBindingsWithDB(db *gorm.DB, groupID string, enabledOnly bool) ([]GroupChannelBindingItem, error) {
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

	boundIDs := make([]string, 0)
	if groupID != "" {
		groupCol := `"group"`
		query := db.Model(&Ability{}).
			Distinct("channel_id").
			Where(groupCol+" = ?", groupID)
		if err := query.Pluck("channel_id", &boundIDs).Error; err != nil {
			return nil, err
		}
	}
	boundSet := make(map[string]struct{}, len(boundIDs))
	for _, id := range boundIDs {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		boundSet[normalized] = struct{}{}
	}

	items := make([]GroupChannelBindingItem, 0, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		_, bound := boundSet[channelID]
		items = append(items, GroupChannelBindingItem{
			Id:       channelID,
			Name:     channel.DisplayName(),
			Protocol: channel.GetProtocol(),
			Status:   channel.Status,
			Models:   strings.TrimSpace(channel.Models),
			Bound:    bound,
			Updated:  channel.CreatedTime,
		})
	}
	return items, nil
}

func ReplaceGroupChannelBindings(groupID string, channelIDs []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return replaceGroupChannelBindingsWithDB(tx, groupID, channelIDs)
	})
}

func replaceGroupChannelBindingsWithDB(db *gorm.DB, groupID string, channelIDs []string) error {
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

	normalizedChannelIDs := normalizeChannelIDList(channelIDs)

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
	existing := make([]Ability, 0)
	if err := db.Where(groupCol+" = ?", groupID).Find(&existing).Error; err != nil {
		return err
	}
	existingByChannelID := make(map[string][]Ability, len(existing))
	for _, item := range existing {
		channelID := strings.TrimSpace(item.ChannelId)
		if channelID == "" {
			continue
		}
		existingByChannelID[channelID] = append(existingByChannelID[channelID], item)
	}

	abilities := make([]Ability, 0)
	for _, id := range normalizedChannelIDs {
		channel := channelsByID[id]
		channelAbilities := SyncGroupAbilitiesForChannel(groupID, &channel, existingByChannelID[id])
		abilities = append(abilities, channelAbilities...)
	}
	abilities = normalizeAbilityRowsPreserveOrder(abilities)

	if err := db.Where(groupCol+" = ?", groupID).Delete(&Ability{}).Error; err != nil {
		return err
	}
	if len(abilities) > 0 {
		if err := db.Create(&abilities).Error; err != nil {
			return err
		}
	}
	return SyncGroupModelProvidersForGroupsWithDB(db, groupID)
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

func SyncGroupAbilitiesForChannel(groupID string, channel *Channel, existing []Ability) []Ability {
	if channel == nil {
		return nil
	}
	selectedConfigs := channelSelectedModelConfigs(channel)
	if len(existing) == 0 {
		return buildDefaultAbilitiesForGroupChannel(groupID, channel)
	}

	selectedUpstreamSet := make(map[string]struct{}, len(selectedConfigs))
	defaultByUpstream := make(map[string]Ability, len(selectedConfigs))
	for _, row := range selectedConfigs {
		upstream := NormalizeAbilityUpstreamModel(row.Model, row.UpstreamModel)
		if upstream == "" {
			continue
		}
		selectedUpstreamSet[upstream] = struct{}{}
		if _, ok := defaultByUpstream[upstream]; ok {
			continue
		}
		defaultByUpstream[upstream] = Ability{
			Group:         strings.TrimSpace(groupID),
			Model:         strings.TrimSpace(row.Model),
			ChannelId:     strings.TrimSpace(channel.Id),
			UpstreamModel: upstream,
			Enabled:       channel.Status == ChannelStatusEnabled,
			Priority:      channel.Priority,
		}
	}

	result := make([]Ability, 0, len(existing)+len(defaultByUpstream))
	existingUpstreamSet := make(map[string]struct{}, len(existing))
	seenAbilityKeys := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		modelName := strings.TrimSpace(item.Model)
		channelID := strings.TrimSpace(item.ChannelId)
		upstream := NormalizeAbilityUpstreamModel(modelName, item.UpstreamModel)
		if modelName == "" || channelID == "" || upstream == "" {
			continue
		}
		if _, ok := selectedUpstreamSet[upstream]; !ok {
			continue
		}
		key := modelName + "::" + channelID
		if _, ok := seenAbilityKeys[key]; ok {
			continue
		}
		seenAbilityKeys[key] = struct{}{}
		existingUpstreamSet[upstream] = struct{}{}
		item.Group = strings.TrimSpace(groupID)
		item.ChannelId = channelID
		item.Model = modelName
		item.UpstreamModel = upstream
		item.Enabled = item.Enabled && channel.Status == ChannelStatusEnabled
		if item.Priority == nil {
			item.Priority = helperInt64Pointer(channel.Priority)
		}
		result = append(result, item)
	}
	for upstream, ability := range defaultByUpstream {
		if _, ok := existingUpstreamSet[upstream]; ok {
			continue
		}
		key := ability.Model + "::" + ability.ChannelId
		if _, ok := seenAbilityKeys[key]; ok {
			continue
		}
		seenAbilityKeys[key] = struct{}{}
		result = append(result, ability)
	}
	return result
}
