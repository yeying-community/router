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

func ListGroupChannelBindingCandidates() ([]GroupChannelBindingItem, error) {
	return listGroupChannelBindingsWithDB(DB, "")
}

func ListGroupChannelBindings(groupID string) ([]GroupChannelBindingItem, error) {
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("分组标识不能为空")
	}
	return listGroupChannelBindingsWithDB(DB, groupID)
}

func listGroupChannelBindingsWithDB(db *gorm.DB, groupID string) ([]GroupChannelBindingItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupID = strings.TrimSpace(groupID)

	channels := make([]Channel, 0)
	if err := db.
		Select("id", "name", "protocol", "status", "created_time").
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

	boundIDs := make([]string, 0)
	if groupID != "" {
		groupCol := `"group"`
		if err := db.Model(&Ability{}).
			Distinct("channel_id").
			Where(groupCol+" = ?", groupID).
			Pluck("channel_id", &boundIDs).Error; err != nil {
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
		_, bound := boundSet[channel.Id]
		items = append(items, GroupChannelBindingItem{
			Id:       channel.Id,
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
		return fmt.Errorf("分组标识不能为空")
	}

	groupCatalog := GroupCatalog{}
	if err := db.Where("id = ?", groupID).First(&groupCatalog).Error; err != nil {
		return err
	}

	normalizedChannelIDs := normalizeChannelIDList(channelIDs)

	channelsByID := make(map[string]Channel, len(normalizedChannelIDs))
	if len(normalizedChannelIDs) > 0 {
		channels := make([]Channel, 0)
		if err := db.
			Select("id", "name", "status", "priority").
			Where("id IN ?", normalizedChannelIDs).
			Find(&channels).Error; err != nil {
			return err
		}
		channelRefs := make([]*Channel, 0, len(channels))
		for i := range channels {
			channelRefs = append(channelRefs, &channels[i])
		}
		if err := HydrateChannelsWithModels(db, channelRefs); err != nil {
			return err
		}
		for _, channel := range channels {
			channelsByID[channel.Id] = channel
		}
		if len(channelsByID) != len(normalizedChannelIDs) {
			missing := make([]string, 0)
			for _, id := range normalizedChannelIDs {
				if _, ok := channelsByID[id]; !ok {
					missing = append(missing, id)
				}
			}
			sort.Strings(missing)
			return fmt.Errorf("渠道不存在: %s", strings.Join(missing, ", "))
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

	if err := db.Where(groupCol+" = ?", groupID).Delete(&Ability{}).Error; err != nil {
		return err
	}
	if len(abilities) == 0 {
		return nil
	}
	return db.Create(&abilities).Error
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
		item.Enabled = channel.Status == ChannelStatusEnabled
		item.Priority = channel.Priority
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
