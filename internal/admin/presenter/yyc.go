package presenter

import (
	"strings"

	"github.com/yeying-community/router/internal/admin/model"
)

type User struct {
	*model.User
	BalanceAmount     int64  `json:"balance_amount"`
	UsedAmount        int64  `json:"used_amount"`
	ActivePackageName string `json:"active_package_name,omitempty"`
}

func NewUser(user *model.User) *User {
	if user == nil {
		return nil
	}
	return &User{
		User:          user,
		BalanceAmount: user.Quota,
		UsedAmount:    user.UsedQuota,
	}
}

func NewUsers(users []*model.User) []*User {
	items := make([]*User, 0, len(users))
	for _, user := range users {
		items = append(items, NewUser(user))
	}
	return items
}

type Token struct {
	*model.Token
	RemainingAmount int64 `json:"remaining_amount"`
	UsedAmount      int64 `json:"used_amount"`
}

func NewToken(token *model.Token) *Token {
	if token == nil {
		return nil
	}
	sanitized := *token
	sanitized.Key = ""
	return &Token{
		Token:           &sanitized,
		RemainingAmount: token.RemainQuota,
		UsedAmount:      token.UsedQuota,
	}
}

func NewCreatedToken(token *model.Token) *Token {
	if token == nil {
		return nil
	}
	return &Token{
		Token:           token,
		RemainingAmount: token.RemainQuota,
		UsedAmount:      token.UsedQuota,
	}
}

func NewTokens(tokens []*model.Token) []*Token {
	items := make([]*Token, 0, len(tokens))
	for _, token := range tokens {
		items = append(items, NewToken(token))
	}
	return items
}

type Redemption struct {
	*model.Redemption
	CreditAmount int64 `json:"credit_amount"`
}

func NewRedemption(redemption *model.Redemption) *Redemption {
	if redemption == nil {
		return nil
	}
	return &Redemption{
		Redemption:   redemption,
		CreditAmount: redemption.Quota,
	}
}

func NewRedemptions(redemptions []*model.Redemption) []*Redemption {
	items := make([]*Redemption, 0, len(redemptions))
	for _, redemption := range redemptions {
		items = append(items, NewRedemption(redemption))
	}
	return items
}

type Channel struct {
	*model.Channel
	UsedAmount int64 `json:"used_amount"`
}

func NewChannel(channel *model.Channel) *Channel {
	if channel == nil {
		return nil
	}
	return &Channel{
		Channel:    channel,
		UsedAmount: channel.UsedQuota,
	}
}

type Group struct {
	Id           string                   `json:"id"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Source       string                   `json:"source"`
	BillingRatio float64                  `json:"billing_ratio"`
	Enabled      bool                     `json:"enabled"`
	SortOrder    int                      `json:"sort_order"`
	CreatedAt    int64                    `json:"created_at"`
	UpdatedAt    int64                    `json:"updated_at"`
	Channels     []model.GroupChannelItem `json:"channels,omitempty"`
}

func NewGroup(group *model.GroupCatalog) *Group {
	if group == nil {
		return nil
	}
	return &Group{
		Id:           strings.TrimSpace(group.Id),
		Name:         strings.TrimSpace(group.Name),
		Description:  strings.TrimSpace(group.Description),
		Source:       strings.TrimSpace(group.Source),
		BillingRatio: group.BillingRatio,
		Enabled:      group.Enabled,
		SortOrder:    group.SortOrder,
		CreatedAt:    group.CreatedAt,
		UpdatedAt:    group.UpdatedAt,
		Channels:     group.Channels,
	}
}

func NewGroups(groups []model.GroupCatalog) []*Group {
	items := make([]*Group, 0, len(groups))
	for i := range groups {
		group := groups[i]
		items = append(items, NewGroup(&group))
	}
	return items
}

type UserDailyQuotaSnapshot struct {
	model.UserDailyQuotaSnapshot
	LimitAmount     int64 `json:"limit_amount"`
	ConsumedAmount  int64 `json:"consumed_amount"`
	ReservedAmount  int64 `json:"reserved_amount"`
	RemainingAmount int64 `json:"remaining_amount"`
}

func NewUserDailyQuotaSnapshot(snapshot model.UserDailyQuotaSnapshot) UserDailyQuotaSnapshot {
	return UserDailyQuotaSnapshot{
		UserDailyQuotaSnapshot: snapshot,
		LimitAmount:            snapshot.Limit,
		ConsumedAmount:         snapshot.ConsumedQuota,
		ReservedAmount:         snapshot.ReservedQuota,
		RemainingAmount:        snapshot.RemainingQuota,
	}
}

type UserPackageEmergencyQuotaSnapshot struct {
	model.UserPackageEmergencyQuotaSnapshot
	LimitAmount     int64 `json:"limit_amount"`
	ConsumedAmount  int64 `json:"consumed_amount"`
	ReservedAmount  int64 `json:"reserved_amount"`
	RemainingAmount int64 `json:"remaining_amount"`
}

func NewUserPackageEmergencyQuotaSnapshot(snapshot model.UserPackageEmergencyQuotaSnapshot) UserPackageEmergencyQuotaSnapshot {
	return UserPackageEmergencyQuotaSnapshot{
		UserPackageEmergencyQuotaSnapshot: snapshot,
		LimitAmount:                       snapshot.Limit,
		ConsumedAmount:                    snapshot.ConsumedQuota,
		ReservedAmount:                    snapshot.ReservedQuota,
		RemainingAmount:                   snapshot.RemainingQuota,
	}
}

type UserQuotaSummary struct {
	UserID           string                            `json:"user_id"`
	Daily            UserDailyQuotaSnapshot            `json:"daily"`
	PackageEmergency UserPackageEmergencyQuotaSnapshot `json:"package_emergency"`
}

func NewUserQuotaSummary(summary model.UserQuotaSummary) UserQuotaSummary {
	return UserQuotaSummary{
		UserID:           strings.TrimSpace(summary.UserID),
		Daily:            NewUserDailyQuotaSnapshot(summary.Daily),
		PackageEmergency: NewUserPackageEmergencyQuotaSnapshot(summary.PackageEmergency),
	}
}

type GroupDailyQuotaSnapshot struct {
	model.GroupDailyQuotaSnapshot
	GroupName       string `json:"group_name,omitempty"`
	LimitAmount     int64  `json:"limit_amount"`
	ConsumedAmount  int64  `json:"consumed_amount"`
	ReservedAmount  int64  `json:"reserved_amount"`
	RemainingAmount int64  `json:"remaining_amount"`
}

func NewGroupDailyQuotaSnapshot(snapshot model.GroupDailyQuotaSnapshot, groupName string) GroupDailyQuotaSnapshot {
	return GroupDailyQuotaSnapshot{
		GroupDailyQuotaSnapshot: snapshot,
		GroupName:               strings.TrimSpace(groupName),
		LimitAmount:             snapshot.Limit,
		ConsumedAmount:          snapshot.ConsumedQuota,
		ReservedAmount:          snapshot.ReservedQuota,
		RemainingAmount:         snapshot.RemainingQuota,
	}
}

type LogStatistic struct {
	model.LogStatistic
	ChargeAmount int `json:"charge_amount"`
}

func NewLogStatistic(row *model.LogStatistic) *LogStatistic {
	if row == nil {
		return nil
	}
	return &LogStatistic{
		LogStatistic: *row,
		ChargeAmount: row.Quota,
	}
}

func NewLogStatistics(rows []*model.LogStatistic) []*LogStatistic {
	items := make([]*LogStatistic, 0, len(rows))
	for _, row := range rows {
		items = append(items, NewLogStatistic(row))
	}
	return items
}

type Log struct {
	*model.Log
	ChargeAmount              int `json:"charge_amount"`
	UserDailyChargeAmount     int `json:"user_daily_charge_amount"`
	UserEmergencyChargeAmount int `json:"user_emergency_charge_amount"`
}

func NewLog(row *model.Log) *Log {
	if row == nil {
		return nil
	}
	return &Log{
		Log:                       row,
		ChargeAmount:              row.Quota,
		UserDailyChargeAmount:     row.UserDailyQuota,
		UserEmergencyChargeAmount: row.UserEmergencyQuota,
	}
}

func NewLogs(rows []*model.Log) []*Log {
	items := make([]*Log, 0, len(rows))
	for _, row := range rows {
		items = append(items, NewLog(row))
	}
	return items
}
