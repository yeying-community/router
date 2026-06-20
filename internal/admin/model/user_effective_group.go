package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/yeying-community/router/common/helper"
	"gorm.io/gorm"
)

func scanSingleGroupID(row *sql.Row) (string, bool, error) {
	groupID := ""
	if err := row.Scan(&groupID); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(groupID), strings.TrimSpace(groupID) != "", nil
}

func getActivePackageEntitlementGroupWithDB(db *gorm.DB, userID string, now int64) (string, bool, error) {
	if err := syncUserPackageSubscriptionsWithDB(db, userID, now); err != nil {
		return "", false, err
	}
	return scanSingleGroupID(db.Raw(`
		SELECT s.group_id
		FROM user_package_subscriptions s
		JOIN groups g ON g.id = s.group_id AND g.enabled = TRUE
		WHERE s.user_id = ?
		  AND s.status = ?
		  AND s.started_at <= ?
		  AND (s.expires_at = 0 OR s.expires_at > ?)
		  AND COALESCE(TRIM(s.group_id), '') <> ''
		ORDER BY s.updated_at DESC, s.started_at DESC, s.id DESC
		LIMIT 1
	`, userID, UserPackageSubscriptionStatusActive, now, now).Row())
}

func getActiveTopupEntitlementGroupWithDB(db *gorm.DB, userID string, now int64) (string, bool, error) {
	return scanSingleGroupID(db.Raw(`
		SELECT p.group_id
		FROM user_balance_lots l
		JOIN topup_orders o ON o.id = l.source_id
		JOIN topup_plans p ON p.id = o.topup_plan_id
		JOIN groups g ON g.id = p.group_id AND g.enabled = TRUE
		WHERE l.user_id = ?
		  AND l.source_type = ?
		  AND l.status = ?
		  AND l.remaining_amount > 0
		  AND (l.expires_at = 0 OR l.expires_at > ?)
		  AND COALESCE(TRIM(p.group_id), '') <> ''
		ORDER BY l.granted_at DESC, l.created_at DESC, l.id DESC
		LIMIT 1
	`, userID, UserBalanceLotSourceTopup, UserBalanceLotStatusActive, now).Row())
}

func getActiveRedemptionEntitlementGroupWithDB(db *gorm.DB, userID string, now int64) (string, bool, error) {
	return scanSingleGroupID(db.Raw(`
		SELECT r.group_id
		FROM user_balance_lots l
		JOIN redemptions r ON r.id = l.source_id
		JOIN groups g ON g.id = r.group_id AND g.enabled = TRUE
		WHERE l.user_id = ?
		  AND l.source_type = ?
		  AND l.status = ?
		  AND l.remaining_amount > 0
		  AND (l.expires_at = 0 OR l.expires_at > ?)
		  AND COALESCE(TRIM(r.group_id), '') <> ''
		ORDER BY l.granted_at DESC, l.created_at DESC, l.id DESC
		LIMIT 1
	`, userID, UserBalanceLotSourceRedeem, UserBalanceLotStatusActive, now).Row())
}

func getLegacyUserGroupWithDB(db *gorm.DB, userID string) (string, bool, error) {
	return scanSingleGroupID(db.Raw(`
		SELECT u."group"
		FROM users u
		JOIN groups g ON g.id = u."group" AND g.enabled = TRUE
		WHERE u.id = ?
		  AND COALESCE(TRIM(u."group"), '') <> ''
		LIMIT 1
	`, userID).Row())
}

func getUserEffectiveGroupWithDB(db *gorm.DB, userID string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database handle is nil")
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", nil
	}
	now := helper.GetTimestamp()
	resolvers := []func(*gorm.DB, string, int64) (string, bool, error){
		getActivePackageEntitlementGroupWithDB,
		getActiveTopupEntitlementGroupWithDB,
		getActiveRedemptionEntitlementGroupWithDB,
	}
	for _, resolve := range resolvers {
		groupID, ok, err := resolve(db, normalizedUserID, now)
		if err != nil {
			return "", err
		}
		if ok {
			return groupID, nil
		}
	}
	groupID, ok, err := getLegacyUserGroupWithDB(db, normalizedUserID)
	if err != nil {
		return "", err
	}
	if ok {
		return groupID, nil
	}
	return "", nil
}

func GetUserEffectiveGroup(userID string) (string, error) {
	return getUserEffectiveGroupWithDB(DB, userID)
}
