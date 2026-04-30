package model

import (
	"fmt"
	"sort"
	"strings"

	commonutils "github.com/yeying-community/router/common/utils"
	"gorm.io/gorm"
)

type GroupModelConfigItem struct {
	Model           string `json:"model"`
	ChannelId       string `json:"channel_id"`
	UpstreamModel   string `json:"upstream_model"`
	Enabled         *bool  `json:"enabled,omitempty"`
	Priority        *int64 `json:"priority,omitempty"`
	ChannelName     string `json:"channel_name,omitempty"`
	ChannelProtocol string `json:"channel_protocol,omitempty"`
	ChannelStatus   int    `json:"channel_status,omitempty"`
}

type GroupModelConfigChannelModel struct {
	Model         string `json:"model"`
	UpstreamModel string `json:"upstream_model"`
	Label         string `json:"label"`
}

type GroupModelConfigChannel struct {
	Id       string                         `json:"id"`
	Name     string                         `json:"name"`
	Protocol string                         `json:"protocol"`
	Status   int                            `json:"status"`
	Priority *int64                         `json:"priority,omitempty"`
	Bound    bool                           `json:"bound"`
	Models   []GroupModelConfigChannelModel `json:"models"`
}

type GroupModelConfigPayload struct {
	Items    []GroupModelConfigItem    `json:"items"`
	Channels []GroupModelConfigChannel `json:"channels"`
}

func ListGroupModelConfigPayload(groupID string) (GroupModelConfigPayload, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return GroupModelConfigPayload{}, fmt.Errorf("分组 ID 不能为空")
	}
	items, err := listGroupModelConfigItemsWithDB(DB, groupID)
	if err != nil {
		return GroupModelConfigPayload{}, err
	}
	channels, err := listGroupModelConfigChannelsWithDB(DB, groupID)
	if err != nil {
		return GroupModelConfigPayload{}, err
	}
	return GroupModelConfigPayload{
		Items:    items,
		Channels: channels,
	}, nil
}

func ReplaceGroupModelConfigs(groupID string, channelIDs []string, items []GroupModelConfigItem, explicitChannels bool) error {
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return err
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return replaceGroupModelConfigsWithDB(tx, groupID, channelIDs, items, explicitChannels)
	}); err != nil {
		return err
	}
	RefreshAbilityCachesForGroups(groupCatalog.Id)
	return nil
}

func ReplaceSingleGroupModelConfig(groupID string, modelName string, items []GroupModelConfigItem) error {
	groupCatalog, err := getGroupCatalogByIDWithDB(DB, groupID)
	if err != nil {
		return err
	}
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" {
		return fmt.Errorf("模型不能为空")
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		currentItems, err := listGroupModelConfigItemsWithDB(tx, groupCatalog.Id)
		if err != nil {
			return err
		}
		boundChannelIDs, err := listGroupBoundChannelIDsWithDB(tx, groupCatalog.Id)
		if err != nil {
			return err
		}
		nextItems := make([]GroupModelConfigItem, 0, len(currentItems)+len(items))
		for _, item := range currentItems {
			if strings.TrimSpace(item.Model) == normalizedModelName {
				continue
			}
			nextItems = append(nextItems, item)
		}
		for _, item := range items {
			next := item
			next.Model = normalizedModelName
			nextItems = append(nextItems, next)
		}
		return replaceGroupModelConfigsWithDB(tx, groupCatalog.Id, boundChannelIDs, nextItems, true)
	}); err != nil {
		return err
	}
	RefreshAbilityCachesForGroups(groupCatalog.Id)
	return nil
}

func listGroupModelConfigItemsWithDB(db *gorm.DB, groupID string) ([]GroupModelConfigItem, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	abilities := make([]Ability, 0)
	groupCol := `"group"`
	if err := db.
		Where(groupCol+" = ?", groupCatalog.Id).
		Order("model asc, priority desc, channel_id asc").
		Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return []GroupModelConfigItem{}, nil
	}

	channelIDs := make([]string, 0, len(abilities))
	channelIDSet := make(map[string]struct{}, len(abilities))
	for _, item := range abilities {
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
		if channel.Status != ChannelStatusEnabled {
			continue
		}
		channelByID[channelID] = channel
	}

	items := make([]GroupModelConfigItem, 0, len(abilities))
	for _, ability := range abilities {
		modelName := strings.TrimSpace(ability.Model)
		channelID := strings.TrimSpace(ability.ChannelId)
		if modelName == "" || channelID == "" {
			continue
		}
		channel, ok := channelByID[channelID]
		if !ok {
			continue
		}
		items = append(items, GroupModelConfigItem{
			Model:           modelName,
			ChannelId:       channelID,
			UpstreamModel:   NormalizeAbilityUpstreamModel(modelName, ability.UpstreamModel),
			Enabled:         helperBoolPointer(ability.Enabled),
			Priority:        helperInt64Pointer(ability.Priority),
			ChannelName:     channel.DisplayName(),
			ChannelProtocol: channel.GetProtocol(),
			ChannelStatus:   channel.Status,
		})
	}
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

func listGroupModelConfigChannelsWithDB(db *gorm.DB, groupID string) ([]GroupModelConfigChannel, error) {
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

	items := make([]GroupModelConfigChannel, 0, len(channels))
	for _, channel := range channels {
		channel.NormalizeIdentity()
		channelID := strings.TrimSpace(channel.Id)
		if channelID == "" {
			continue
		}
		_, bound := boundSet[channelID]
		items = append(items, GroupModelConfigChannel{
			Id:       channelID,
			Name:     channel.DisplayName(),
			Protocol: channel.GetProtocol(),
			Status:   channel.Status,
			Priority: helperInt64Pointer(channel.Priority),
			Bound:    bound,
			Models:   buildGroupModelConfigChannelModels(&channel),
		})
	}
	return items, nil
}

func replaceGroupModelConfigsWithDB(db *gorm.DB, groupID string, channelIDs []string, items []GroupModelConfigItem, explicitChannels bool) error {
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

	normalizedItems, err := normalizeGroupModelConfigItems(items)
	if err != nil {
		return err
	}
	allowedChannelIDs, channelsByID, err := loadGroupModelConfigChannelsByIDWithDB(db, groupID, channelIDs, explicitChannels, normalizedItems)
	if err != nil {
		return err
	}
	groupCol := `"group"`
	if len(allowedChannelIDs) == 0 {
		if err := db.Where(groupCol+" = ?", groupID).Delete(&Ability{}).Error; err != nil {
			return err
		}
		return replaceGroupModelProvidersWithDB(db, groupID, map[string]string{})
	}

	allowedSet := make(map[string]struct{}, len(allowedChannelIDs))
	for _, channelID := range allowedChannelIDs {
		allowedSet[channelID] = struct{}{}
	}
	selectedCatalogs := make(map[string]groupModelConfigChannelCatalog, len(channelsByID))
	for channelID, channel := range channelsByID {
		selectedCatalogs[channelID] = buildGroupModelConfigChannelCatalog(channel)
	}

	abilities := make([]Ability, 0, len(normalizedItems))
	requestedChannelCounts := make(map[string]int, len(allowedChannelIDs))
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
		requestedChannelCounts[item.ChannelId]++
		abilities = append(abilities, Ability{
			Group:         groupID,
			Model:         item.Model,
			ChannelId:     item.ChannelId,
			UpstreamModel: upstreamModel,
			Enabled:       resolveGroupModelConfigEnabled(item) && channel.Status == ChannelStatusEnabled,
			Priority:      resolveGroupModelConfigPriority(item, channel),
		})
	}

	for _, channelID := range allowedChannelIDs {
		if requestedChannelCounts[channelID] > 0 {
			continue
		}
		abilities = append(abilities, buildDefaultAbilitiesForGroupChannel(groupID, channelsByID[channelID])...)
	}
	abilities = normalizeAbilityRowsPreserveOrder(abilities)
	providerByModel, err := resolveGroupModelProvidersForAbilities(abilities, selectedCatalogs)
	if err != nil {
		return err
	}

	if err := db.Where(groupCol+" = ?", groupID).Delete(&Ability{}).Error; err != nil {
		return err
	}
	if len(abilities) > 0 {
		if err := db.Create(&abilities).Error; err != nil {
			return err
		}
	}
	return replaceGroupModelProvidersWithDB(db, groupID, providerByModel)
}

func listGroupBoundChannelIDsWithDB(db *gorm.DB, groupID string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	groupCatalog, err := getGroupCatalogByIDWithDB(db, groupID)
	if err != nil {
		return nil, err
	}
	boundIDs := make([]string, 0)
	groupCol := `"group"`
	if err := db.Model(&Ability{}).
		Distinct("channel_id").
		Where(groupCol+" = ?", groupCatalog.Id).
		Pluck("channel_id", &boundIDs).Error; err != nil {
		return nil, err
	}
	return normalizeChannelIDList(boundIDs), nil
}

func normalizeGroupModelConfigItems(items []GroupModelConfigItem) ([]GroupModelConfigItem, error) {
	if len(items) == 0 {
		return []GroupModelConfigItem{}, nil
	}
	result := make([]GroupModelConfigItem, 0, len(items))
	for _, item := range items {
		normalized := GroupModelConfigItem{
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
			return nil, fmt.Errorf("分组模型配置存在未填写完整的行")
		}
		result = append(result, normalized)
	}
	return result, nil
}

func loadGroupModelConfigChannelsByIDWithDB(db *gorm.DB, groupID string, channelIDs []string, explicitChannels bool, items []GroupModelConfigItem) ([]string, map[string]*Channel, error) {
	allowedChannelIDs := normalizeChannelIDList(channelIDs)
	if !explicitChannels && len(allowedChannelIDs) == 0 {
		boundIDs, err := listGroupBoundChannelIDsWithDB(db, groupID)
		if err != nil {
			return nil, nil, err
		}
		allowedChannelIDs = boundIDs
	}
	if len(allowedChannelIDs) == 0 {
		allowedChannelIDs = collectChannelIDsFromGroupModelConfigItems(items)
	}
	if len(allowedChannelIDs) == 0 {
		return []string{}, map[string]*Channel{}, nil
	}

	channelsByID, err := loadEnabledChannelsByIDWithDB(db, allowedChannelIDs)
	if err != nil {
		return nil, nil, err
	}
	return allowedChannelIDs, channelsByID, nil
}

func collectChannelIDsFromGroupModelConfigItems(items []GroupModelConfigItem) []string {
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

func buildDefaultAbilitiesForGroupChannel(groupID string, channel *Channel) []Ability {
	if channel == nil {
		return nil
	}
	selectedConfigs := channelSelectedModelConfigs(channel)
	if len(selectedConfigs) == 0 {
		return nil
	}
	abilities := make([]Ability, 0, len(selectedConfigs))
	for _, row := range selectedConfigs {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			continue
		}
		abilities = append(abilities, Ability{
			Group:         strings.TrimSpace(groupID),
			Model:         modelName,
			ChannelId:     strings.TrimSpace(channel.Id),
			UpstreamModel: NormalizeAbilityUpstreamModel(modelName, row.UpstreamModel),
			Enabled:       channel.Status == ChannelStatusEnabled,
			Priority:      channel.Priority,
		})
	}
	return abilities
}

func resolveGroupModelConfigEnabled(item GroupModelConfigItem) bool {
	if item.Enabled == nil {
		return true
	}
	return *item.Enabled
}

func resolveGroupModelConfigPriority(item GroupModelConfigItem, channel *Channel) *int64 {
	if item.Priority != nil {
		return helperInt64Pointer(item.Priority)
	}
	if channel == nil {
		return nil
	}
	return helperInt64Pointer(channel.Priority)
}

func resolveGroupModelProvidersForAbilities(abilities []Ability, catalogs map[string]groupModelConfigChannelCatalog) (map[string]string, error) {
	providerCandidates := make(map[string]map[string]struct{}, len(abilities))
	modelOrder := make([]string, 0, len(abilities))
	seenModel := make(map[string]struct{}, len(abilities))
	for _, ability := range abilities {
		modelName := strings.TrimSpace(ability.Model)
		channelID := strings.TrimSpace(ability.ChannelId)
		if modelName == "" || channelID == "" {
			continue
		}
		if _, ok := seenModel[modelName]; !ok {
			seenModel[modelName] = struct{}{}
			modelOrder = append(modelOrder, modelName)
		}
		catalog, ok := catalogs[channelID]
		if !ok {
			continue
		}
		provider := catalog.ResolveProvider(GroupModelConfigItem{
			Model:         modelName,
			UpstreamModel: strings.TrimSpace(ability.UpstreamModel),
		}, ability.UpstreamModel)
		provider = commonutils.NormalizeProvider(provider)
		if provider == "" {
			continue
		}
		if _, ok := providerCandidates[modelName]; !ok {
			providerCandidates[modelName] = make(map[string]struct{}, 1)
		}
		providerCandidates[modelName][provider] = struct{}{}
	}

	providerByModel := make(map[string]string, len(modelOrder))
	for _, modelName := range modelOrder {
		candidateSet := providerCandidates[modelName]
		if len(candidateSet) == 0 {
			providerByModel[modelName] = ""
			continue
		}
		providers := make([]string, 0, len(candidateSet))
		for provider := range candidateSet {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		if len(providers) > 1 {
			return nil, fmt.Errorf("同一分组模型仅允许一个供应商: %s (%s)", modelName, strings.Join(providers, " / "))
		}
		providerByModel[modelName] = providers[0]
	}
	return providerByModel, nil
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

func channelSelectedModelConfigs(channel *Channel) []ChannelModel {
	if channel == nil {
		return nil
	}
	rows := channel.GetModelConfigs()
	if len(rows) == 0 {
		return nil
	}
	selected := make([]ChannelModel, 0, len(rows))
	for _, row := range rows {
		if !row.Selected {
			continue
		}
		selected = append(selected, row)
	}
	return selected
}

func buildGroupModelConfigChannelModels(channel *Channel) []GroupModelConfigChannelModel {
	selectedConfigs := channelSelectedModelConfigs(channel)
	if len(selectedConfigs) == 0 {
		return []GroupModelConfigChannelModel{}
	}
	items := make([]GroupModelConfigChannelModel, 0, len(selectedConfigs))
	for _, row := range selectedConfigs {
		modelName := strings.TrimSpace(row.Model)
		upstream := NormalizeAbilityUpstreamModel(modelName, row.UpstreamModel)
		label := modelName
		if upstream != "" && upstream != modelName {
			label = fmt.Sprintf("%s -> %s", modelName, upstream)
		}
		items = append(items, GroupModelConfigChannelModel{
			Model:         modelName,
			UpstreamModel: upstream,
			Label:         label,
		})
	}
	return items
}

type groupModelConfigChannelCatalog struct {
	aliasToUpstream  map[string]string
	upstreamSet      map[string]struct{}
	aliasToProvider  map[string]string
	upstreamProvider map[string]string
}

func buildGroupModelConfigChannelCatalog(channel *Channel) groupModelConfigChannelCatalog {
	catalog := groupModelConfigChannelCatalog{
		aliasToUpstream:  make(map[string]string),
		upstreamSet:      make(map[string]struct{}),
		aliasToProvider:  make(map[string]string),
		upstreamProvider: make(map[string]string),
	}
	for _, row := range channelSelectedModelConfigs(channel) {
		modelName := strings.TrimSpace(row.Model)
		upstream := NormalizeAbilityUpstreamModel(modelName, row.UpstreamModel)
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

func (catalog groupModelConfigChannelCatalog) ResolveUpstream(item GroupModelConfigItem) string {
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

func (catalog groupModelConfigChannelCatalog) ResolveProvider(item GroupModelConfigItem, resolvedUpstream string) string {
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
