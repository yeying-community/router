package model

import "context"

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
	Id               int     `json:"id"`
	Username         string  `json:"username" gorm:"unique;index" validate:"max=12"`
	Password         string  `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	DisplayName      string  `json:"display_name" gorm:"index" validate:"max=20"`
	Role             int     `json:"role" gorm:"type:int;default:1"`   // admin, util
	Status           int     `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email            string  `json:"email" gorm:"index" validate:"max=50"`
	GitHubId         string  `json:"github_id" gorm:"column:github_id;index"`
	WeChatId         string  `json:"wechat_id" gorm:"column:wechat_id;index"`
	LarkId           string  `json:"lark_id" gorm:"column:lark_id;index"`
	OidcId           string  `json:"oidc_id" gorm:"column:oidc_id;index"`
	WalletAddress    *string `json:"wallet_address" gorm:"column:wallet_address;uniqueIndex" validate:"omitempty"`
	VerificationCode string  `json:"verification_code" gorm:"-:all"`
	AccessToken      string  `json:"access_token" gorm:"type:char(32);column:access_token;uniqueIndex"`
	Quota            int64   `json:"quota" gorm:"bigint;default:0"`
	UsedQuota        int64   `json:"used_quota" gorm:"bigint;default:0;column:used_quota"`
	RequestCount     int     `json:"request_count" gorm:"type:int;default:0;"`
	Group            string  `json:"group" gorm:"type:varchar(32);default:'default'"`
	AffCode          string  `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	InviterId        int     `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
}

func GetMaxUserId() int {
	return mustUserRepo().GetMaxUserId()
}

func GetAllUsers(startIdx int, num int, order string) ([]*User, error) {
	return mustUserRepo().GetAllUsers(startIdx, num, order)
}

func SearchUsers(keyword string) ([]*User, error) {
	return mustUserRepo().SearchUsers(keyword)
}

func GetUserById(id int, selectAll bool) (*User, error) {
	return mustUserRepo().GetUserById(id, selectAll)
}

func GetUserIdByAffCode(affCode string) (int, error) {
	return mustUserRepo().GetUserIdByAffCode(affCode)
}

func DeleteUserById(id int) error {
	return mustUserRepo().DeleteUserById(id)
}

func (user *User) Insert(ctx context.Context, inviterId int) error {
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

func IsAdmin(userId int) bool {
	return mustUserRepo().IsAdmin(userId)
}

func IsUserEnabled(userId int) (bool, error) {
	return mustUserRepo().IsUserEnabled(userId)
}

func ValidateAccessToken(token string) *User {
	return mustUserRepo().ValidateAccessToken(token)
}

func GetUserQuota(id int) (int64, error) {
	return mustUserRepo().GetUserQuota(id)
}

func GetUserUsedQuota(id int) (int64, error) {
	return mustUserRepo().GetUserUsedQuota(id)
}

func GetUserEmail(id int) (string, error) {
	return mustUserRepo().GetUserEmail(id)
}

func GetUserGroup(id int) (string, error) {
	return mustUserRepo().GetUserGroup(id)
}

func IncreaseUserQuota(id int, quota int64) error {
	return mustUserRepo().IncreaseUserQuota(id, quota)
}

func increaseUserQuota(id int, quota int64) error {
	return mustUserRepo().IncreaseUserQuotaDirect(id, quota)
}

func DecreaseUserQuota(id int, quota int64) error {
	return mustUserRepo().DecreaseUserQuota(id, quota)
}

func decreaseUserQuota(id int, quota int64) error {
	return mustUserRepo().DecreaseUserQuotaDirect(id, quota)
}

func GetRootUserEmail() string {
	return mustUserRepo().GetRootUserEmail()
}

func UpdateUserUsedQuotaAndRequestCount(id int, quota int64) {
	mustUserRepo().UpdateUserUsedQuotaAndRequestCount(id, quota)
}

func updateUserUsedQuotaAndRequestCount(id int, quota int64, count int) {
	mustUserRepo().UpdateUserUsedQuotaAndRequestCountDirect(id, quota, count)
}

func updateUserUsedQuota(id int, quota int64) {
	mustUserRepo().UpdateUserUsedQuotaDirect(id, quota)
}

func updateUserRequestCount(id int, count int) {
	mustUserRepo().UpdateUserRequestCountDirect(id, count)
}

func GetUsernameById(id int) string {
	return mustUserRepo().GetUsernameById(id)
}
