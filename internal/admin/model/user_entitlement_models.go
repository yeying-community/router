package model

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

const (
	UserEntitlementSourcePackage    = "package"
	UserEntitlementSourceTopup      = "topup"
	UserEntitlementSourceRedemption = "redemption"
	UserEntitlementSourceLegacy     = "legacy"
)

type UserEntitlementSource struct {
	SourceType string   `json:"source_type"`
	SourceID   string   `json:"source_id,omitempty"`
	SourceName string   `json:"source_name,omitempty"`
	GroupID    string   `json:"group_id"`
	GroupName  string   `json:"group_name,omitempty"`
	Priority   int      `json:"priority"`
	Models     []string `json:"models,omitempty"`
}

type UserEntitlementModelSource struct {
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id,omitempty"`
	SourceName string `json:"source_name,omitempty"`
	GroupID    string `json:"group_id"`
	GroupName  string `json:"group_name,omitempty"`
	Priority   int    `json:"priority"`
}

type UserAvailableModel struct {
	Model   string                       `json:"model"`
	Sources []UserEntitlementModelSource `json:"sources"`
}

type UserEntitlementModelsPayload struct {
	Models  []string                           `json:"models"`
	Items   []UserAvailableModel               `json:"items"`
	Sources []UserEntitlementSource            `json:"sources"`
	ByModel map[string][]UserEntitlementSource `json:"-"`
}

func normalizeEntitlementSource(row UserEntitlementSource) UserEntitlementSource {
	row.SourceType = strings.TrimSpace(row.SourceType)
	row.SourceID = strings.TrimSpace(row.SourceID)
	row.SourceName = strings.TrimSpace(row.SourceName)
	row.GroupID = strings.TrimSpace(row.GroupID)
	row.GroupName = strings.TrimSpace(row.GroupName)
	row.Models = NormalizeChannelModelIDsPreserveOrder(row.Models)
	return row
}

func hydrateEntitlementSourceGroupNameWithDB(db *gorm.DB, row *UserEntitlementSource) {
	if db == nil || row == nil || strings.TrimSpace(row.GroupName) != "" || strings.TrimSpace(row.GroupID) == "" {
		return
	}
	group := GroupCatalog{}
	if err := db.Select("id", "name").Where("id = ?", strings.TrimSpace(row.GroupID)).Take(&group).Error; err == nil {
		row.GroupName = strings.TrimSpace(group.Name)
	}
}

func listUserEntitlementSourcesWithDB(db *gorm.DB, userID string, now int64) ([]UserEntitlementSource, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return []UserEntitlementSource{}, nil
	}
	effectiveNow := now
	if effectiveNow <= 0 {
		effectiveNow = helper.GetTimestamp()
	}
	if err := syncUserPackageSubscriptionsWithDB(db, normalizedUserID, effectiveNow); err != nil {
		return nil, err
	}
	sources := make([]UserEntitlementSource, 0)
	packageRows := make([]UserPackageSubscription, 0)
	if err := db.
		Joins("JOIN groups g ON g.id = user_package_subscriptions.group_id AND g.enabled = ?", true).
		Where("user_id = ? AND status = ? AND started_at <= ? AND (expires_at = 0 OR expires_at > ?) AND COALESCE(TRIM(group_id), '') <> ''",
			normalizedUserID,
			UserPackageSubscriptionStatusActive,
			effectiveNow,
			effectiveNow,
		).
		Order("user_package_subscriptions.updated_at desc, user_package_subscriptions.started_at desc, user_package_subscriptions.id desc").
		Find(&packageRows).Error; err != nil {
		return nil, err
	}
	for _, item := range packageRows {
		sources = append(sources, normalizeEntitlementSource(UserEntitlementSource{
			SourceType: UserEntitlementSourcePackage,
			SourceID:   item.Id,
			SourceName: item.PackageName,
			GroupID:    item.GroupID,
			Priority:   10,
		}))
	}

	topupRows := make([]struct {
		LotID       string
		OrderID     string
		TopupPlanID string
		GroupID     string
		Title       string
	}, 0)
	if err := db.Table(UserBalanceLotsTableName+" AS l").
		Select("l.id AS lot_id", "o.id AS order_id", "o.topup_plan_id", "o.group_id", "o.title").
		Joins("JOIN "+TopupOrdersTableName+" AS o ON o.id = l.source_id").
		Joins("JOIN groups g ON g.id = o.group_id AND g.enabled = ?", true).
		Where("l.user_id = ? AND l.source_type = ? AND l.status = ? AND l.remaining_amount > 0 AND (l.expires_at = 0 OR l.expires_at > ?)",
			normalizedUserID,
			UserBalanceLotSourceTopup,
			UserBalanceLotStatusActive,
			effectiveNow,
		).
		Where("o.business_type = ? AND COALESCE(TRIM(o.group_id), '') <> ?", TopupOrderBusinessBalance, "").
		Order("l.granted_at desc, l.created_at desc, l.id desc").
		Scan(&topupRows).Error; err != nil {
		return nil, err
	}
	for _, item := range topupRows {
		sourceID := strings.TrimSpace(item.TopupPlanID)
		if sourceID == "" {
			sourceID = strings.TrimSpace(item.OrderID)
		}
		sources = append(sources, normalizeEntitlementSource(UserEntitlementSource{
			SourceType: UserEntitlementSourceTopup,
			SourceID:   sourceID,
			SourceName: item.Title,
			GroupID:    item.GroupID,
			Priority:   20,
		}))
	}

	redemptionRows := make([]struct {
		LotID        string
		RedemptionID string
		GroupID      string
		Name         string
	}, 0)
	if err := db.Table(UserBalanceLotsTableName+" AS l").
		Select("l.id AS lot_id", "r.id AS redemption_id", "r.group_id", "r.name").
		Joins("JOIN redemptions AS r ON r.id = l.source_id").
		Joins("JOIN groups g ON g.id = r.group_id AND g.enabled = ?", true).
		Where("l.user_id = ? AND l.source_type = ? AND l.status = ? AND l.remaining_amount > 0 AND (l.expires_at = 0 OR l.expires_at > ?)",
			normalizedUserID,
			UserBalanceLotSourceRedeem,
			UserBalanceLotStatusActive,
			effectiveNow,
		).
		Where("COALESCE(TRIM(r.group_id), '') <> ?", "").
		Order("l.granted_at desc, l.created_at desc, l.id desc").
		Scan(&redemptionRows).Error; err != nil {
		return nil, err
	}
	for _, item := range redemptionRows {
		sources = append(sources, normalizeEntitlementSource(UserEntitlementSource{
			SourceType: UserEntitlementSourceRedemption,
			SourceID:   item.RedemptionID,
			SourceName: item.Name,
			GroupID:    item.GroupID,
			Priority:   30,
		}))
	}

	legacyGroup, ok, err := getLegacyUserGroupWithDB(db, normalizedUserID)
	if err != nil {
		return nil, err
	}
	if ok {
		sources = append(sources, normalizeEntitlementSource(UserEntitlementSource{
			SourceType: UserEntitlementSourceLegacy,
			GroupID:    legacyGroup,
			Priority:   40,
		}))
	}

	seen := make(map[string]struct{}, len(sources))
	result := make([]UserEntitlementSource, 0, len(sources))
	for _, source := range sources {
		source = normalizeEntitlementSource(source)
		if source.GroupID == "" {
			continue
		}
		key := source.SourceType + "::" + source.SourceID + "::" + source.GroupID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		hydrateEntitlementSourceGroupNameWithDB(db, &source)
		result = append(result, source)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		if result[i].SourceName != result[j].SourceName {
			return result[i].SourceName < result[j].SourceName
		}
		return result[i].GroupID < result[j].GroupID
	})
	return result, nil
}

func ListUserEntitlementSources(userID string) ([]UserEntitlementSource, error) {
	return listUserEntitlementSourcesWithDB(DB, userID, helper.GetTimestamp())
}

func listEntitlementGroupModels(ctx context.Context, db *gorm.DB, groupID string) ([]string, error) {
	if db == DB {
		return CacheGetGroupModels(ctx, groupID)
	}
	return listGroupModelNamesWithDB(db, groupID, true)
}

func BuildUserEntitlementModelsWithDB(ctx context.Context, db *gorm.DB, userID string) (UserEntitlementModelsPayload, error) {
	sources, err := listUserEntitlementSourcesWithDB(db, userID, helper.GetTimestamp())
	if err != nil {
		return UserEntitlementModelsPayload{}, err
	}
	byModel := make(map[string][]UserEntitlementSource)
	modelSet := make(map[string]struct{})
	enrichedSources := make([]UserEntitlementSource, 0, len(sources))
	for _, source := range sources {
		models, err := listEntitlementGroupModels(ctx, db, source.GroupID)
		if err != nil {
			return UserEntitlementModelsPayload{}, err
		}
		source.Models = NormalizeChannelModelIDsPreserveOrder(models)
		enrichedSources = append(enrichedSources, source)
		for _, modelName := range source.Models {
			if strings.TrimSpace(modelName) == "" {
				continue
			}
			modelSet[modelName] = struct{}{}
			byModel[modelName] = append(byModel[modelName], source)
		}
	}
	models := make([]string, 0, len(modelSet))
	for modelName := range modelSet {
		models = append(models, modelName)
	}
	sort.Strings(models)
	items := make([]UserAvailableModel, 0, len(models))
	for _, modelName := range models {
		sourceItems := byModel[modelName]
		sort.SliceStable(sourceItems, func(i, j int) bool {
			if sourceItems[i].Priority != sourceItems[j].Priority {
				return sourceItems[i].Priority < sourceItems[j].Priority
			}
			if sourceItems[i].SourceName != sourceItems[j].SourceName {
				return sourceItems[i].SourceName < sourceItems[j].SourceName
			}
			return sourceItems[i].GroupID < sourceItems[j].GroupID
		})
		modelSources := make([]UserEntitlementModelSource, 0, len(sourceItems))
		for _, source := range sourceItems {
			modelSources = append(modelSources, UserEntitlementModelSource{
				SourceType: source.SourceType,
				SourceID:   source.SourceID,
				SourceName: source.SourceName,
				GroupID:    source.GroupID,
				GroupName:  source.GroupName,
				Priority:   source.Priority,
			})
		}
		items = append(items, UserAvailableModel{Model: modelName, Sources: modelSources})
	}
	return UserEntitlementModelsPayload{
		Models:  models,
		Items:   items,
		Sources: enrichedSources,
		ByModel: byModel,
	}, nil
}

func BuildUserEntitlementModels(ctx context.Context, userID string) (UserEntitlementModelsPayload, error) {
	return BuildUserEntitlementModelsWithDB(ctx, DB, userID)
}

func ResolveUserEntitlementGroupForModelWithDB(ctx context.Context, db *gorm.DB, userID string, modelName string) (string, *UserEntitlementSource, error) {
	normalizedModel := strings.TrimSpace(modelName)
	if normalizedModel == "" {
		groupID, err := CacheGetUserGroup(userID)
		if err != nil {
			return "", nil, err
		}
		return groupID, nil, nil
	}
	payload, err := BuildUserEntitlementModelsWithDB(ctx, db, userID)
	if err != nil {
		return "", nil, err
	}
	sources := payload.ByModel[normalizedModel]
	if len(sources) == 0 {
		return "", nil, fmt.Errorf("当前权益下对于模型 %s 无可用分组", normalizedModel)
	}
	source := sources[0]
	return source.GroupID, &source, nil
}

func ResolveUserEntitlementGroupForModel(ctx context.Context, userID string, modelName string) (string, *UserEntitlementSource, error) {
	return ResolveUserEntitlementGroupForModelWithDB(ctx, DB, userID, modelName)
}
