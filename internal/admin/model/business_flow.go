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
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	CreditAmount    int64   `json:"credit_amount"`
	TransactionID   string  `json:"transaction_id"`
	StatusMessage   string  `json:"status_message"`
	PaidAt          int64   `json:"paid_at"`
	RedeemedAt      int64   `json:"redeemed_at"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type AdminUserPackageRecord struct {
	ID                         string  `json:"id"`
	UserID                     string  `json:"user_id"`
	Username                   string  `json:"username"`
	PackageID                  string  `json:"package_id"`
	PackageName                string  `json:"package_name"`
	GroupID                    string  `json:"group_id"`
	GroupName                  string  `json:"group_name"`
	Amount                     float64 `json:"amount"`
	Currency                   string  `json:"currency"`
	DailyQuotaLimit            int64   `json:"daily_quota_limit"`
	PackageEmergencyQuotaLimit int64   `json:"package_emergency_quota_limit"`
	QuotaResetTimezone         string  `json:"quota_reset_timezone"`
	StartedAt                  int64   `json:"started_at"`
	ExpiresAt                  int64   `json:"expires_at"`
	Status                     int     `json:"status"`
	UpdatedAt                  int64   `json:"updated_at"`
}

type AdminRedemptionRecord struct {
	ID                 string  `json:"id"`
	RedeemedByUserID   string  `json:"redeemed_by_user_id"`
	RedeemedByUsername string  `json:"redeemed_by_username"`
	GroupID            string  `json:"group_id"`
	GroupName          string  `json:"group_name"`
	Name               string  `json:"name"`
	FaceValueAmount    float64 `json:"face_value_amount"`
	FaceValueUnit      string  `json:"face_value_unit"`
	CreditAmount       int64   `json:"credit_amount"`
	RedeemedTime       int64   `json:"redeemed_time"`
	CreatedTime        int64   `json:"created_time"`
}

type AdminTopupReconcileRecord struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	Username        string  `json:"username"`
	Status          string  `json:"status"`
	Source          string  `json:"source"`
	ProviderName    string  `json:"provider_name"`
	ProviderOrderID string  `json:"provider_order_id"`
	TransactionID   string  `json:"transaction_id"`
	Title           string  `json:"title"`
	BusinessType    string  `json:"business_type"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	StatusMessage   string  `json:"status_message"`
	PaidAt          int64   `json:"paid_at"`
	RedeemedAt      int64   `json:"redeemed_at"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
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
		Joins("LEFT JOIN users u ON u.id = o.user_id")
	if normalizedStatus := NormalizeTopupOrderStatus(status); normalizedStatus != "" {
		query = query.Where("o.status = ?", normalizedStatus)
	}
	query = applyKeywordFilter(query, keyword, []string{
		"LOWER(o.id) LIKE ?",
		"LOWER(COALESCE(NULLIF(o.username, ''), u.username, '')) LIKE ?",
		"LOWER(COALESCE(o.transaction_id, '')) LIKE ?",
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
			o.amount,
			o.currency,
			COALESCE(o.quota, 0) AS credit_amount,
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

func GetAdminTopupOrderRecordByIDWithDB(db *gorm.DB, id string) (AdminTopupOrderRecord, error) {
	if db == nil {
		return AdminTopupOrderRecord{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return AdminTopupOrderRecord{}, gorm.ErrRecordNotFound
	}
	row := AdminTopupOrderRecord{}
	err := db.Table(TopupOrdersTableName+" AS o").
		Joins("LEFT JOIN users u ON u.id = o.user_id").
		Where("o.id = ?", normalizedID).
		Select(`
			o.id,
			o.user_id,
			COALESCE(NULLIF(o.username, ''), u.username, '') AS username,
			o.status,
			o.source,
			o.provider_name,
			o.provider_order_id,
			o.amount,
			o.currency,
			COALESCE(o.quota, 0) AS credit_amount,
			o.transaction_id,
			o.status_message,
			o.paid_at,
			o.redeemed_at,
			o.created_at,
			o.updated_at`).
		Take(&row).Error
	if err != nil {
		return AdminTopupOrderRecord{}, err
	}
	return row, nil
}

func ListAdminUserPackageRecordsPageWithDB(db *gorm.DB, page int, pageSize int, keyword string, status int) ([]AdminUserPackageRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	page, pageSize = normalizeBusinessFlowPage(page, pageSize)
	query := db.Table(UserPackageSubscriptionsTableName + " AS s").
		Joins("LEFT JOIN users u ON u.id = s.user_id").
		Joins("LEFT JOIN " + GroupCatalog{}.TableName() + " g ON g.id = s.group_id").
		Joins("LEFT JOIN " + ServicePackage{}.TableName() + " p ON p.id = s.package_id")
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
			COALESCE(p.sale_price, 0) AS amount,
			COALESCE(p.sale_currency, '') AS currency,
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

func GetAdminUserPackageRecordByIDWithDB(db *gorm.DB, id string) (AdminUserPackageRecord, error) {
	if db == nil {
		return AdminUserPackageRecord{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return AdminUserPackageRecord{}, gorm.ErrRecordNotFound
	}
	row := AdminUserPackageRecord{}
	err := db.Table(UserPackageSubscriptionsTableName+" AS s").
		Joins("LEFT JOIN users u ON u.id = s.user_id").
		Joins("LEFT JOIN "+GroupCatalog{}.TableName()+" g ON g.id = s.group_id").
		Joins("LEFT JOIN "+ServicePackage{}.TableName()+" p ON p.id = s.package_id").
		Where("s.id = ?", normalizedID).
		Select(`
			s.id,
			s.user_id,
			COALESCE(u.username, '') AS username,
			s.package_id,
			s.package_name,
			s.group_id,
			COALESCE(g.name, '') AS group_name,
			COALESCE(p.sale_price, 0) AS amount,
			COALESCE(p.sale_currency, '') AS currency,
			s.daily_quota_limit,
			s.package_emergency_quota_limit,
			s.quota_reset_timezone,
			s.started_at,
			s.expires_at,
			s.status,
			s.updated_at`).
		Take(&row).Error
	if err != nil {
		return AdminUserPackageRecord{}, err
	}
	return row, nil
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
			r.redeemed_by_user_id,
			COALESCE(u.username, '') AS redeemed_by_username,
			r.group_id,
			COALESCE(g.name, '') AS group_name,
			r.name,
			r.face_value_amount,
			r.face_value_unit,
			r.quota AS credit_amount,
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

func GetAdminRedemptionRecordByIDWithDB(db *gorm.DB, id string) (AdminRedemptionRecord, error) {
	if db == nil {
		return AdminRedemptionRecord{}, fmt.Errorf("database handle is nil")
	}
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return AdminRedemptionRecord{}, gorm.ErrRecordNotFound
	}
	row := AdminRedemptionRecord{}
	err := db.Table("redemptions AS r").
		Joins("LEFT JOIN users u ON u.id = r.redeemed_by_user_id").
		Joins("LEFT JOIN "+GroupCatalog{}.TableName()+" g ON g.id = r.group_id").
		Where("r.id = ? AND r.redeemed_time > 0 AND r.status = ?", normalizedID, RedemptionCodeStatusUsed).
		Select(`
			r.id,
			r.redeemed_by_user_id,
			COALESCE(u.username, '') AS redeemed_by_username,
			r.group_id,
			COALESCE(g.name, '') AS group_name,
			r.name,
			r.face_value_amount,
			r.face_value_unit,
			r.quota AS credit_amount,
			r.redeemed_time,
			r.created_time`).
		Take(&row).Error
	if err != nil {
		return AdminRedemptionRecord{}, err
	}
	return row, nil
}

func ListAdminTopupReconcileRecordsPageWithDB(db *gorm.DB, page int, pageSize int, keyword string, status string) ([]AdminTopupReconcileRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database handle is nil")
	}
	page, pageSize = normalizeBusinessFlowPage(page, pageSize)
	query := db.Table(TopupOrdersTableName+" AS o").
		Joins("LEFT JOIN users u ON u.id = o.user_id").
		Where("o.source = ?", TopupOrderSourceTopUpAPI)
	if normalizedStatus := NormalizeTopupOrderStatus(status); normalizedStatus != "" {
		query = query.Where("o.status = ?", normalizedStatus)
	}
	query = applyKeywordFilter(query, keyword, []string{
		"LOWER(o.id) LIKE ?",
		"LOWER(COALESCE(NULLIF(o.username, ''), u.username, '')) LIKE ?",
		"LOWER(COALESCE(o.transaction_id, '')) LIKE ?",
		"LOWER(COALESCE(o.provider_order_id, '')) LIKE ?",
		"LOWER(COALESCE(o.title, '')) LIKE ?",
		"LOWER(COALESCE(o.status_message, '')) LIKE ?",
	}, nil)
	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]AdminTopupReconcileRecord, 0, pageSize)
	if err := query.
		Select(`
			o.id,
			o.user_id,
			COALESCE(NULLIF(o.username, ''), u.username, '') AS username,
			o.status,
			o.source,
			o.provider_name,
			o.provider_order_id,
			o.transaction_id,
			o.title,
			o.business_type,
			o.amount,
			o.currency,
			o.status_message,
			o.paid_at,
			o.redeemed_at,
			o.created_at,
			o.updated_at`).
		Order("o.updated_at desc, o.created_at desc, o.id desc").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
