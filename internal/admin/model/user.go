package model

import (
	"context"
	"strings"

	"github.com/yeying-community/router/common/config"
)

const (
	RoleGuestUser  = 0
	RoleCommonUser = 1
	RoleAdminUser  = 10
	RoleRootUser   = 100
)

const (
	UserStatusEnabled  = 1 // don't use 0, 0 is the default value!
	UserStatusDisabled = 2 // also don't use 0
	UserStatusDeleted  = 3
)

// User if you add sensitive fields, don't forget to clean them in setupLogin function.
// Otherwise, the sensitive information will be saved on local storage in plain text!
type User struct {
	Id                         string  `json:"id" gorm:"type:char(36);primaryKey"`
	Username                   string  `json:"username" gorm:"unique;index" validate:"max=20"`
	Password                   string  `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	DisplayName                string  `json:"display_name" gorm:"index" validate:"max=20"`
	Role                       int     `json:"role" gorm:"type:int;default:1"`   // admin, util
	Status                     int     `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email                      string  `json:"email" gorm:"index" validate:"max=50"`
	GitHubId                   string  `json:"github_id" gorm:"column:github_id;index"`
	WeChatId                   string  `json:"wechat_id" gorm:"column:wechat_id;index"`
	LarkId                     string  `json:"lark_id" gorm:"column:lark_id;index"`
	OidcId                     string  `json:"oidc_id" gorm:"column:oidc_id;index"`
	WalletAddress              *string `json:"wallet_address" gorm:"column:wallet_address;uniqueIndex" validate:"omitempty"`
	VerificationCode           string  `json:"verification_code" gorm:"-:all"`
	AccessToken                string  `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"`
	Quota                      int64   `json:"quota" gorm:"bigint;default:0"`
	UsedQuota                  int64   `json:"used_quota" gorm:"bigint;default:0;column:used_quota"`
	RequestCount               int     `json:"request_count" gorm:"type:int;default:0;"`
	Group                      string  `json:"group" gorm:"type:varchar(32);default:''"`
	DailyQuotaLimit            int64   `json:"daily_quota_limit" gorm:"type:bigint;not null;default:0"`
	MonthlyEmergencyQuotaLimit int64   `json:"monthly_emergency_quota_limit" gorm:"type:bigint;not null;default:0"`
	QuotaResetTimezone         string  `json:"quota_reset_timezone" gorm:"type:varchar(64);not null;default:'Asia/Shanghai'"`
	AffCode                    string  `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	InviterId                  string  `json:"inviter_id" gorm:"type:char(36);column:inviter_id;index"`
	HasPassword                bool    `json:"has_password" gorm:"column:has_password;default:false"`
	CreatedAt                  int64   `json:"created_at" gorm:"bigint;index"`
	UpdatedAt                  int64   `json:"updated_at" gorm:"bigint;index"`
	CanManageUsers             bool    `json:"can_manage_users" gorm:"-"`
}

func NormalizeWalletAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func IsRootWalletAddress(address string) bool {
	normalized := NormalizeWalletAddress(address)
	if normalized == "" {
		return false
	}
	for _, configured := range config.RootWalletAddresses {
		if normalized == NormalizeWalletAddress(configured) {
			return true
		}
	}
	return false
}

func EffectiveRole(user *User) int {
	if user == nil {
		return RoleGuestUser
	}
	if user.WalletAddress != nil && IsRootWalletAddress(*user.WalletAddress) {
		return RoleRootUser
	}
	if user.Role >= RoleRootUser {
		return RoleAdminUser
	}
	return user.Role
}

func ExposedRole(user *User) int {
	role := EffectiveRole(user)
	if role >= RoleRootUser {
		return RoleAdminUser
	}
	return role
}

func CanManageUsers(user *User) bool {
	return EffectiveRole(user) >= RoleRootUser
}

func IsProtectedRootUser(user *User) bool {
	if user == nil || user.WalletAddress == nil {
		return false
	}
	return IsRootWalletAddress(*user.WalletAddress)
}

func GetMaxUserId() string {
	return mustUserRepo().GetMaxUserId()
}

func GetAllUsers(startIdx int, num int, order string) ([]*User, error) {
	return mustUserRepo().GetAllUsers(startIdx, num, order)
}

func SearchUsers(keyword string) ([]*User, error) {
	return mustUserRepo().SearchUsers(keyword)
}

func GetUserById(id string, selectAll bool) (*User, error) {
	return mustUserRepo().GetUserById(id, selectAll)
}

func GetUserIdByAffCode(affCode string) (string, error) {
	return mustUserRepo().GetUserIdByAffCode(affCode)
}

func DeleteUserById(id string) error {
	return mustUserRepo().DeleteUserById(id)
}

func (user *User) Insert(ctx context.Context, inviterId string) error {
	return mustUserRepo().Insert(ctx, user, inviterId)
}

func (user *User) Update(updatePassword bool) error {
	return mustUserRepo().Update(user, updatePassword)
}

func (user *User) Delete() error {
	return mustUserRepo().Delete(user)
}

// ValidateAndFill check password & user status
func (user *User) ValidateAndFill() error {
	return mustUserRepo().ValidateAndFill(user)
}

func (user *User) FillUserById() error {
	return mustUserRepo().FillUserById(user)
}

func (user *User) FillUserByEmail() error {
	return mustUserRepo().FillUserByEmail(user)
}

func (user *User) FillUserByGitHubId() error {
	return mustUserRepo().FillUserByGitHubId(user)
}

func (user *User) FillUserByLarkId() error {
	return mustUserRepo().FillUserByLarkId(user)
}

func (user *User) FillUserByOidcId() error {
	return mustUserRepo().FillUserByOidcId(user)
}

func (user *User) FillUserByWeChatId() error {
	return mustUserRepo().FillUserByWeChatId(user)
}

func (user *User) FillUserByUsername() error {
	return mustUserRepo().FillUserByUsername(user)
}

func (user *User) FillUserByWalletAddress() error {
	return mustUserRepo().FillUserByWalletAddress(user)
}

func IsEmailAlreadyTaken(email string) bool {
	return mustUserRepo().IsEmailAlreadyTaken(email)
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return mustUserRepo().IsWeChatIdAlreadyTaken(wechatId)
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return mustUserRepo().IsGitHubIdAlreadyTaken(githubId)
}

func IsLarkIdAlreadyTaken(larkId string) bool {
	return mustUserRepo().IsLarkIdAlreadyTaken(larkId)
}

func IsOidcIdAlreadyTaken(oidcId string) bool {
	return mustUserRepo().IsOidcIdAlreadyTaken(oidcId)
}

func IsWalletAddressAlreadyTaken(address string) bool {
	return mustUserRepo().IsWalletAddressAlreadyTaken(address)
}

func IsUsernameAlreadyTaken(username string) bool {
	return mustUserRepo().IsUsernameAlreadyTaken(username)
}

func ResetUserPasswordByEmail(email string, password string) error {
	return mustUserRepo().ResetUserPasswordByEmail(email, password)
}

func IsAdmin(userId string) bool {
	return mustUserRepo().IsAdmin(userId)
}

func IsUserEnabled(userId string) (bool, error) {
	return mustUserRepo().IsUserEnabled(userId)
}

func ValidateAccessToken(token string) *User {
	return mustUserRepo().ValidateAccessToken(token)
}

func GetUserQuota(id string) (int64, error) {
	return mustUserRepo().GetUserQuota(id)
}

func GetUserUsedQuota(id string) (int64, error) {
	return mustUserRepo().GetUserUsedQuota(id)
}

func GetUserEmail(id string) (string, error) {
	return mustUserRepo().GetUserEmail(id)
}

func GetUserGroup(id string) (string, error) {
	return mustUserRepo().GetUserGroup(id)
}

func IncreaseUserQuota(id string, quota int64) error {
	return mustUserRepo().IncreaseUserQuota(id, quota)
}

func increaseUserQuota(id string, quota int64) error {
	return mustUserRepo().IncreaseUserQuotaDirect(id, quota)
}

func DecreaseUserQuota(id string, quota int64) error {
	return mustUserRepo().DecreaseUserQuota(id, quota)
}

func decreaseUserQuota(id string, quota int64) error {
	return mustUserRepo().DecreaseUserQuotaDirect(id, quota)
}

func GetRootUserEmail() string {
	return mustUserRepo().GetRootUserEmail()
}

func UpdateUserUsedQuotaAndRequestCount(id string, quota int64) {
	mustUserRepo().UpdateUserUsedQuotaAndRequestCount(id, quota)
}

func updateUserUsedQuotaAndRequestCount(id string, quota int64, count int) {
	mustUserRepo().UpdateUserUsedQuotaAndRequestCountDirect(id, quota, count)
}

func updateUserUsedQuota(id string, quota int64) {
	mustUserRepo().UpdateUserUsedQuotaDirect(id, quota)
}

func updateUserRequestCount(id string, count int) {
	mustUserRepo().UpdateUserRequestCountDirect(id, count)
}

func GetUsernameById(id string) string {
	return mustUserRepo().GetUsernameById(id)
}
