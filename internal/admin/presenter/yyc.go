package presenter

import (
	"strings"

	"github.com/yeying-community/router/internal/admin/model"
)

type User struct {
	*model.User
	YYCBalance               int64  `json:"yyc_balance"`
	YYCUsed                  int64  `json:"yyc_used"`
	YYCDailyLimit            int64  `json:"yyc_daily_limit"`
	YYCPackageEmergencyLimit int64  `json:"yyc_package_emergency_limit"`
	ActivePackageName        string `json:"active_package_name,omitempty"`
}

func NewUser(user *model.User) *User {
	if user == nil {
		return nil
	}
	return &User{
		User:                     user,
		YYCBalance:               user.Quota,
		YYCUsed:                  user.UsedQuota,
		YYCDailyLimit:            user.DailyQuotaLimit,
		YYCPackageEmergencyLimit: user.PackageEmergencyQuotaLimit,
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
	YYCRemain int64 `json:"yyc_remain"`
	YYCUsed   int64 `json:"yyc_used"`
}

func NewToken(token *model.Token) *Token {
	if token == nil {
		return nil
	}
	return &Token{
		Token:     token,
		YYCRemain: token.RemainQuota,
		YYCUsed:   token.UsedQuota,
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
	YYCValue int64 `json:"yyc_value"`
}

func NewRedemption(redemption *model.Redemption) *Redemption {
	if redemption == nil {
		return nil
	}
	return &Redemption{
		Redemption: redemption,
		YYCValue:   redemption.Quota,
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
	YYCUsed int64 `json:"yyc_used"`
}

func NewChannel(channel *model.Channel) *Channel {
	if channel == nil {
		return nil
	}
	return &Channel{
		Channel: channel,
		YYCUsed: channel.UsedQuota,
	}
}

type Group struct {
	Id           string                          `json:"id"`
	Name         string                          `json:"name"`
	Description  string                          `json:"description"`
	Source       string                          `json:"source"`
	BillingRatio float64                         `json:"billing_ratio"`
	Enabled      bool                            `json:"enabled"`
	SortOrder    int                             `json:"sort_order"`
	CreatedAt    int64                           `json:"created_at"`
	UpdatedAt    int64                           `json:"updated_at"`
	Channels     []model.GroupChannelBindingItem `json:"channels,omitempty"`
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
	YYCLimit     int64 `json:"yyc_limit"`
	YYCConsumed  int64 `json:"yyc_consumed"`
	YYCReserved  int64 `json:"yyc_reserved"`
	YYCRemaining int64 `json:"yyc_remaining"`
}

func NewUserDailyQuotaSnapshot(snapshot model.UserDailyQuotaSnapshot) UserDailyQuotaSnapshot {
	return UserDailyQuotaSnapshot{
		UserDailyQuotaSnapshot: snapshot,
		YYCLimit:               snapshot.Limit,
		YYCConsumed:            snapshot.ConsumedQuota,
		YYCReserved:            snapshot.ReservedQuota,
		YYCRemaining:           snapshot.RemainingQuota,
	}
}

type UserPackageEmergencyQuotaSnapshot struct {
	model.UserPackageEmergencyQuotaSnapshot
	YYCLimit     int64 `json:"yyc_limit"`
	YYCConsumed  int64 `json:"yyc_consumed"`
	YYCReserved  int64 `json:"yyc_reserved"`
	YYCRemaining int64 `json:"yyc_remaining"`
}

func NewUserPackageEmergencyQuotaSnapshot(snapshot model.UserPackageEmergencyQuotaSnapshot) UserPackageEmergencyQuotaSnapshot {
	return UserPackageEmergencyQuotaSnapshot{
		UserPackageEmergencyQuotaSnapshot: snapshot,
		YYCLimit:                          snapshot.Limit,
		YYCConsumed:                       snapshot.ConsumedQuota,
		YYCReserved:                       snapshot.ReservedQuota,
		YYCRemaining:                      snapshot.RemainingQuota,
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
	GroupName    string `json:"group_name,omitempty"`
	YYCLimit     int64  `json:"yyc_limit"`
	YYCConsumed  int64  `json:"yyc_consumed"`
	YYCReserved  int64  `json:"yyc_reserved"`
	YYCRemaining int64  `json:"yyc_remaining"`
}

func NewGroupDailyQuotaSnapshot(snapshot model.GroupDailyQuotaSnapshot, groupName string) GroupDailyQuotaSnapshot {
	return GroupDailyQuotaSnapshot{
		GroupDailyQuotaSnapshot: snapshot,
		GroupName:               strings.TrimSpace(groupName),
		YYCLimit:                snapshot.Limit,
		YYCConsumed:             snapshot.ConsumedQuota,
		YYCReserved:             snapshot.ReservedQuota,
		YYCRemaining:            snapshot.RemainingQuota,
	}
}

type LogStatistic struct {
	model.LogStatistic
	YYCAmount int `json:"yyc_amount"`
}

func NewLogStatistic(row *model.LogStatistic) *LogStatistic {
	if row == nil {
		return nil
	}
	return &LogStatistic{
		LogStatistic: *row,
		YYCAmount:    row.Quota,
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
	YYCAmount        int `json:"yyc_amount"`
	YYCUserDaily     int `json:"yyc_user_daily"`
	YYCUserEmergency int `json:"yyc_user_emergency"`
}

func NewLog(row *model.Log) *Log {
	if row == nil {
		return nil
	}
	return &Log{
		Log:              row,
		YYCAmount:        row.Quota,
		YYCUserDaily:     row.UserDailyQuota,
		YYCUserEmergency: row.UserEmergencyQuota,
	}
}

func NewLogs(rows []*model.Log) []*Log {
	items := make([]*Log, 0, len(rows))
	for _, row := range rows {
		items = append(items, NewLog(row))
	}
	return items
}
