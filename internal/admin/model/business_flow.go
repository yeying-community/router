package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type AdminTopupOrderRecord struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	Username        string  `json:"username"`
	Status          string  `json:"status"`
	Source          string  `json:"source"`
	ProviderName    string  `json:"provider_name"`
	ProviderOrderID string  `json:"provider_order_id"`
	RedemptionID    string  `json:"redemption_id"`
	RedemptionName  string  `json:"redemption_name"`
	GroupID         string  `json:"group_id"`
	GroupName       string  `json:"group_name"`
	FaceValueAmount float64 `json:"face_value_amount"`
	FaceValueUnit   string  `json:"face_value_unit"`
	YYCValue        int64   `json:"yyc_value"`
	TransactionID   string  `json:"transaction_id"`
	StatusMessage   string  `json:"status_message"`
	PaidAt          int64   `json:"paid_at"`
	RedeemedAt      int64   `json:"redeemed_at"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type AdminUserPackageRecord struct {
	ID                         string `json:"id"`
	UserID                     string `json:"user_id"`
	Username                   string `json:"username"`
	PackageID                  string `json:"package_id"`
	PackageName                string `json:"package_name"`
	GroupID                    string `json:"group_id"`
	GroupName                  string `json:"group_name"`
	DailyQuotaLimit            int64  `json:"daily_quota_limit"`
	PackageEmergencyQuotaLimit int64  `json:"package_emergency_quota_limit"`
	QuotaResetTimezone         string `json:"quota_reset_timezone"`
	StartedAt                  int64  `json:"started_at"`
	ExpiresAt                  int64  `json:"expires_at"`
	Status                     int    `json:"status"`
	UpdatedAt                  int64  `json:"updated_at"`
}

type AdminRedemptionRecord struct {
	ID                 string  `json:"id"`
	TopupOrderID       string  `json:"topup_order_id"`
	RedeemedByUserID   string  `json:"redeemed_by_user_id"`
	RedeemedByUsername string  `json:"redeemed_by_username"`
	GroupID            string  `json:"group_id"`
	GroupName          string  `json:"group_name"`
	Name               string  `json:"name"`
	FaceValueAmount    float64 `json:"face_value_amount"`
	FaceValueUnit      string  `json:"face_value_unit"`
	YYCValue           int64   `json:"yyc_value"`
	RedeemedTime       int64   `json:"redeemed_time"`
	CreatedTime        int64   `json:"created_time"`
}

func normalizeBusinessFlowPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

func applyKeywordFilter(query *gorm.DB, keyword string, conditions []string, args []any) *gorm.DB {
	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if normalizedKeyword == "" || len(conditions) == 0 {
		return query
	}
	likeKeyword := "%" + normalizedKeyword + "%"
	resolvedArgs := make([]any, 0, len(conditions))
	if len(args) == 0 {
		for range conditions {
			resolvedArgs = append(resolvedArgs, likeKeyword)
		}
	} else {
		resolvedArgs = append(resolvedArgs, args...)
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", resolvedArgs...)
}

func ListAdminTopupOrderRecordsPageWithDB(db *gorm.DB, page int, pageSize int, keyword string, status string) ([]AdminTopupOrderRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	page, pageSize = normalizeBusinessFlowPage(page, pageSize)
	query := db.Table(TopupOrdersTableName + " AS o").
		Joins("LEFT JOIN users u ON u.id = o.user_id").
		Joins("LEFT JOIN redemptions r ON r.id = o.redemption_id").
		Joins("LEFT JOIN " + GroupCatalog{}.TableName() + " g ON g.id = r.group_id")
	if normalizedStatus := NormalizeTopupOrderStatus(status); normalizedStatus != "" {
		query = query.Where("o.status = ?", normalizedStatus)
	}
	query = applyKeywordFilter(query, keyword, []string{
		"LOWER(o.id) LIKE ?",
		"LOWER(COALESCE(NULLIF(o.username, ''), u.username, '')) LIKE ?",
		"LOWER(COALESCE(o.transaction_id, '')) LIKE ?",
		"LOWER(COALESCE(r.name, '')) LIKE ?",
		"LOWER(COALESCE(g.name, '')) LIKE ?",
		"LOWER(COALESCE(o.status, '')) LIKE ?",
	}, nil)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]AdminTopupOrderRecord, 0, pageSize)
	if err := query.
		Select(`
			o.id,
			o.user_id,
			COALESCE(NULLIF(o.username, ''), u.username, '') AS username,
			o.status,
			o.source,
			o.provider_name,
			o.provider_order_id,
			o.redemption_id,
			COALESCE(r.name, '') AS redemption_name,
			COALESCE(r.group_id, '') AS group_id,
			COALESCE(g.name, '') AS group_name,
			COALESCE(r.face_value_amount, 0) AS face_value_amount,
			COALESCE(r.face_value_unit, '') AS face_value_unit,
			COALESCE(r.quota, 0) AS yyc_value,
			o.transaction_id,
			o.status_message,
			o.paid_at,
			o.redeemed_at,
			o.created_at,
			o.updated_at`).
		Order("o.created_at desc, o.id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func ListAdminUserPackageRecordsPageWithDB(db *gorm.DB, page int, pageSize int, keyword string, status int) ([]AdminUserPackageRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	page, pageSize = normalizeBusinessFlowPage(page, pageSize)
	query := db.Table(UserPackageSubscriptionsTableName + " AS s").
		Joins("LEFT JOIN users u ON u.id = s.user_id").
		Joins("LEFT JOIN " + GroupCatalog{}.TableName() + " g ON g.id = s.group_id")
	if status > 0 {
		query = query.Where("s.status = ?", status)
	}
	query = applyKeywordFilter(query, keyword, []string{
		"LOWER(s.id) LIKE ?",
		"LOWER(COALESCE(u.username, '')) LIKE ?",
		"LOWER(COALESCE(s.package_name, '')) LIKE ?",
		"LOWER(COALESCE(g.name, '')) LIKE ?",
	}, nil)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]AdminUserPackageRecord, 0, pageSize)
	if err := query.
		Select(`
			s.id,
			s.user_id,
			COALESCE(u.username, '') AS username,
			s.package_id,
			s.package_name,
			s.group_id,
			COALESCE(g.name, '') AS group_name,
			s.daily_quota_limit,
			s.package_emergency_quota_limit,
			s.quota_reset_timezone,
			s.started_at,
			s.expires_at,
			s.status,
			s.updated_at`).
		Order("s.started_at desc, s.id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func ListAdminRedemptionRecordsPageWithDB(db *gorm.DB, page int, pageSize int, keyword string) ([]AdminRedemptionRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	page, pageSize = normalizeBusinessFlowPage(page, pageSize)
	query := db.Table("redemptions AS r").
		Joins("LEFT JOIN users u ON u.id = r.redeemed_by_user_id").
		Joins("LEFT JOIN "+GroupCatalog{}.TableName()+" g ON g.id = r.group_id").
		Where("r.redeemed_time > 0 AND r.status = ?", RedemptionCodeStatusUsed)
	query = applyKeywordFilter(query, keyword, []string{
		"LOWER(r.id) LIKE ?",
		"LOWER(COALESCE(r.name, '')) LIKE ?",
		"LOWER(COALESCE(u.username, '')) LIKE ?",
		"LOWER(COALESCE(g.name, '')) LIKE ?",
	}, nil)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]AdminRedemptionRecord, 0, pageSize)
	if err := query.
		Select(`
			r.id,
			r.topup_order_id,
			r.redeemed_by_user_id,
			COALESCE(u.username, '') AS redeemed_by_username,
			r.group_id,
			COALESCE(g.name, '') AS group_name,
			r.name,
			r.face_value_amount,
			r.face_value_unit,
			r.quota AS yyc_value,
			r.redeemed_time,
			r.created_time`).
		Order("r.redeemed_time desc, r.id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
