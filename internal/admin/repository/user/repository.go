package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/yeying-community/router/common"
	"github.com/yeying-community/router/common/blacklist"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/random"
	"github.com/yeying-community/router/internal/admin/model"
)

func init() {
	model.BindUserRepository(model.UserRepository{
		GetMaxUserId:                             GetMaxUserId,
		GetAllUsers:                              GetAll,
		SearchUsers:                              Search,
		GetUserById:                              GetByID,
		GetUserIdByAffCode:                       GetIDByAffCode,
		DeleteUserById:                           DeleteByID,
		Insert:                                   Create,
		Update:                                   Update,
		Delete:                                   Delete,
		ValidateAndFill:                          ValidateAndFill,
		FillUserById:                             FillByID,
		FillUserByEmail:                          FillByEmail,
		FillUserByGitHubId:                       FillByGitHubID,
		FillUserByLarkId:                         FillByLarkID,
		FillUserByOidcId:                         FillByOidcID,
		FillUserByWeChatId:                       FillByWeChatID,
		FillUserByUsername:                       FillByUsername,
		FillUserByWalletAddress:                  FillByWalletAddress,
		IsEmailAlreadyTaken:                      IsEmailAlreadyTaken,
		IsWeChatIdAlreadyTaken:                   IsWeChatIdAlreadyTaken,
		IsGitHubIdAlreadyTaken:                   IsGitHubIdAlreadyTaken,
		IsLarkIdAlreadyTaken:                     IsLarkIdAlreadyTaken,
		IsOidcIdAlreadyTaken:                     IsOidcIdAlreadyTaken,
		IsWalletAddressAlreadyTaken:              IsWalletAddressAlreadyTaken,
		IsUsernameAlreadyTaken:                   IsUsernameAlreadyTaken,
		ResetUserPasswordByEmail:                 ResetUserPasswordByEmail,
		IsAdmin:                                  IsAdmin,
		IsUserEnabled:                            IsUserEnabled,
		ValidateAccessToken:                      ValidateAccessToken,
		GetUserQuota:                             GetQuota,
		GetUserUsedQuota:                         GetUsedQuota,
		GetUserEmail:                             GetEmail,
		GetUserGroup:                             GetGroup,
		IncreaseUserQuota:                        IncreaseQuota,
		DecreaseUserQuota:                        DecreaseQuota,
		IncreaseUserQuotaDirect:                  IncreaseQuotaDirect,
		DecreaseUserQuotaDirect:                  DecreaseQuotaDirect,
		GetRootUserEmail:                         GetRootEmail,
		UpdateUserUsedQuotaAndRequestCount:       UpdateUsedQuotaAndRequestCount,
		UpdateUserUsedQuotaAndRequestCountDirect: UpdateUsedQuotaAndRequestCountDirect,
		UpdateUserUsedQuotaDirect:                UpdateUsedQuotaDirect,
		UpdateUserRequestCountDirect:             UpdateRequestCountDirect,
		GetUsernameById:                          GetUsernameById,
	})
}

func GetMaxUserId() int {
	var user model.User
	model.DB.Last(&user)
	return user.Id
}

func GetAll(startIdx int, num int, order string) ([]*model.User, error) {
	query := model.DB.Limit(num).Offset(startIdx).Omit("password").Where("status != ?", model.UserStatusDeleted)

	switch order {
	case "quota":
		query = query.Order("quota desc")
	case "used_quota":
		query = query.Order("used_quota desc")
	case "request_count":
		query = query.Order("request_count desc")
	default:
		query = query.Order("id desc")
	}

	var users []*model.User
	err := query.Find(&users).Error
	return users, err
}

func Search(keyword string) ([]*model.User, error) {
	var users []*model.User
	var err error
	if !common.UsingPostgreSQL {
		err = model.DB.Omit("password").Where("id = ? or username LIKE ? or email LIKE ? or display_name LIKE ?", keyword, keyword+"%", keyword+"%", keyword+"%").Find(&users).Error
	} else {
		err = model.DB.Omit("password").Where("username LIKE ? or email LIKE ? or display_name LIKE ?", keyword+"%", keyword+"%", keyword+"%").Find(&users).Error
	}
	return users, err
}

func GetByID(id int, selectAll bool) (*model.User, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	user := model.User{Id: id}
	var err error
	if selectAll {
		err = model.DB.First(&user, "id = ?", id).Error
	} else {
		err = model.DB.Omit("password", "access_token").First(&user, "id = ?", id).Error
	}
	return &user, err
}

func GetByUsername(username string) (*model.User, error) {
	user := model.User{Username: username}
	err := model.DB.Where(&user).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetIDByAffCode(code string) (int, error) {
	if code == "" {
		return 0, errors.New("affCode 为空！")
	}
	var user model.User
	err := model.DB.Select("id").First(&user, "aff_code = ?", code).Error
	return user.Id, err
}

func DeleteByID(id int) error {
	if id == 0 {
		return errors.New("id 为空！")
	}
	user := model.User{Id: id}
	return Delete(&user)
}

func Create(ctx context.Context, user *model.User, inviterId int) error {
	var err error
	if user.Password != "" {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	if user.WalletAddress != nil {
		trimmed := strings.TrimSpace(*user.WalletAddress)
		if trimmed == "" {
			user.WalletAddress = nil
		} else {
			lower := strings.ToLower(trimmed)
			user.WalletAddress = &lower
		}
	}
	user.Quota = config.QuotaForNewUser
	user.AccessToken = random.GetUUID()
	user.AffCode = random.GetRandomString(4)
	result := model.DB.Create(user)
	if result.Error != nil {
		return result.Error
	}
	if config.QuotaForNewUser > 0 {
		model.RecordLog(ctx, user.Id, model.LogTypeSystem, fmt.Sprintf("新用户注册赠送 %s", common.LogQuota(config.QuotaForNewUser)))
	}
	if inviterId != 0 {
		if config.QuotaForInvitee > 0 {
			_ = IncreaseQuota(user.Id, config.QuotaForInvitee)
			model.RecordLog(ctx, user.Id, model.LogTypeSystem, fmt.Sprintf("使用邀请码赠送 %s", common.LogQuota(config.QuotaForInvitee)))
		}
		if config.QuotaForInviter > 0 {
			_ = IncreaseQuota(inviterId, config.QuotaForInviter)
			model.RecordLog(ctx, inviterId, model.LogTypeSystem, fmt.Sprintf("邀请用户赠送 %s", common.LogQuota(config.QuotaForInviter)))
		}
	}
	cleanToken := model.Token{
		UserId:         user.Id,
		Name:           "default",
		Key:            random.GenerateKey(),
		CreatedTime:    helper.GetTimestamp(),
		AccessedTime:   helper.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    -1,
		UnlimitedQuota: true,
	}
	result.Error = cleanToken.Insert()
	if result.Error != nil {
		logger.SysError(fmt.Sprintf("create default token for user %d failed: %s", user.Id, result.Error.Error()))
	}
	return nil
}

func Update(user *model.User, updatePassword bool) error {
	var err error
	if updatePassword {
		user.Password, err = common.Password2Hash(user.Password)
		if err != nil {
			return err
		}
	}
	if user.WalletAddress != nil {
		trimmed := strings.TrimSpace(*user.WalletAddress)
		if trimmed == "" {
			user.WalletAddress = nil
		} else {
			lower := strings.ToLower(trimmed)
			user.WalletAddress = &lower
		}
	}
	if user.Status == model.UserStatusDisabled {
		blacklist.BanUser(user.Id)
	} else if user.Status == model.UserStatusEnabled {
		blacklist.UnbanUser(user.Id)
	}

	updates := model.DB.Model(user)
	if user.WalletAddress == nil {
		err = updates.Omit("wallet_address").Updates(user).Error
	} else {
		err = updates.Updates(user).Error
	}
	return err
}

func Delete(user *model.User) error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	blacklist.BanUser(user.Id)
	user.Username = fmt.Sprintf("deleted_%s", random.GetUUID())
	user.Status = model.UserStatusDeleted
	user.WalletAddress = nil
	err := model.DB.Model(user).Updates(map[string]interface{}{
		"username":       user.Username,
		"status":         user.Status,
		"wallet_address": nil,
	}).Error
	model.DB.Where("user_id = ?", user.Id).Delete(&model.Token{})
	return err
}

func ValidateAndFill(user *model.User) error {
	password := user.Password
	if user.Username == "" || password == "" {
		return errors.New("用户名或密码为空")
	}
	err := model.DB.Where("username = ?", user.Username).First(user).Error
	if err != nil {
		err := model.DB.Where("email = ?", user.Username).First(user).Error
		if err != nil {
			return errors.New("用户名或密码错误，或用户已被封禁")
		}
	}
	okay := common.ValidatePasswordAndHash(password, user.Password)
	if !okay || user.Status != model.UserStatusEnabled {
		return errors.New("用户名或密码错误，或用户已被封禁")
	}
	return nil
}

func FillByID(user *model.User) error {
	if user.Id == 0 {
		return errors.New("id 为空！")
	}
	model.DB.Where(model.User{Id: user.Id}).First(user)
	return nil
}

func FillByEmail(user *model.User) error {
	if user.Email == "" {
		return errors.New("email 为空！")
	}
	model.DB.Where(model.User{Email: user.Email}).First(user)
	return nil
}

func FillByGitHubID(user *model.User) error {
	if user.GitHubId == "" {
		return errors.New("GitHub id 为空！")
	}
	model.DB.Where(model.User{GitHubId: user.GitHubId}).First(user)
	return nil
}

func FillByLarkID(user *model.User) error {
	if user.LarkId == "" {
		return errors.New("lark id 为空！")
	}
	model.DB.Where(model.User{LarkId: user.LarkId}).First(user)
	return nil
}

func FillByOidcID(user *model.User) error {
	if user.OidcId == "" {
		return errors.New("oidc id 为空！")
	}
	model.DB.Where(model.User{OidcId: user.OidcId}).First(user)
	return nil
}

func FillByWeChatID(user *model.User) error {
	if user.WeChatId == "" {
		return errors.New("WeChat id 为空！")
	}
	model.DB.Where(model.User{WeChatId: user.WeChatId}).First(user)
	return nil
}

func FillByUsername(user *model.User) error {
	if user.Username == "" {
		return errors.New("username 为空！")
	}
	model.DB.Where(model.User{Username: user.Username}).First(user)
	return nil
}

func FillByWalletAddress(user *model.User) error {
	if user.WalletAddress == nil || *user.WalletAddress == "" {
		return errors.New("wallet address 为空！")
	}
	model.DB.Where(model.User{WalletAddress: user.WalletAddress}).First(user)
	return nil
}

func IsEmailAlreadyTaken(email string) bool {
	return model.DB.Where("email = ?", email).Find(&model.User{}).RowsAffected == 1
}

func IsWeChatIdAlreadyTaken(wechatId string) bool {
	return model.DB.Where("wechat_id = ?", wechatId).Find(&model.User{}).RowsAffected == 1
}

func IsGitHubIdAlreadyTaken(githubId string) bool {
	return model.DB.Where("github_id = ?", githubId).Find(&model.User{}).RowsAffected == 1
}

func IsLarkIdAlreadyTaken(larkId string) bool {
	return model.DB.Where("lark_id = ?", larkId).Find(&model.User{}).RowsAffected == 1
}

func IsOidcIdAlreadyTaken(oidcId string) bool {
	return model.DB.Where("oidc_id = ?", oidcId).Find(&model.User{}).RowsAffected == 1
}

func IsWalletAddressAlreadyTaken(address string) bool {
	if address == "" {
		return false
	}
	return model.DB.Where("wallet_address = ?", address).Find(&model.User{}).RowsAffected == 1
}

func IsUsernameAlreadyTaken(username string) bool {
	return model.DB.Where("username = ?", username).Find(&model.User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	if email == "" || password == "" {
		return errors.New("邮箱地址或密码为空！")
	}
	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	err = model.DB.Model(&model.User{}).Where("email = ?", email).Update("password", hashedPassword).Error
	return err
}

func IsAdmin(userId int) bool {
	if userId == 0 {
		return false
	}
	var user model.User
	err := model.DB.Where("id = ?", userId).Select("role").Find(&user).Error
	if err != nil {
		logger.SysError("no such user " + err.Error())
		return false
	}
	return user.Role >= model.RoleAdminUser
}

func IsUserEnabled(userId int) (bool, error) {
	if userId == 0 {
		return false, errors.New("user id is empty")
	}
	var user model.User
	err := model.DB.Where("id = ?", userId).Select("status").Find(&user).Error
	if err != nil {
		return false, err
	}
	return user.Status == model.UserStatusEnabled, nil
}

func ValidateAccessToken(token string) *model.User {
	if token == "" {
		return nil
	}
	token = strings.Replace(token, "Bearer ", "", 1)
	user := &model.User{}
	if model.DB.Where("access_token = ?", token).First(user).RowsAffected == 1 {
		return user
	}
	return nil
}

func GetQuota(id int) (int64, error) {
	var quota int64
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Select("quota").Find(&quota).Error
	return quota, err
}

func GetUsedQuota(id int) (int64, error) {
	var quota int64
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Select("used_quota").Find(&quota).Error
	return quota, err
}

func GetEmail(id int) (string, error) {
	var email string
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Select("email").Find(&email).Error
	return email, err
}

func GetGroup(id int) (string, error) {
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	var group string
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Select(groupCol).Find(&group).Error
	return group, err
}

func IncreaseQuota(id int, quota int64) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeUserQuota, id, quota)
		return nil
	}
	return IncreaseQuotaDirect(id, quota)
}

func IncreaseQuotaDirect(id int, quota int64) error {
	return model.DB.Model(&model.User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota + ?", quota)).Error
}

func DecreaseQuota(id int, quota int64) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeUserQuota, id, -quota)
		return nil
	}
	return DecreaseQuotaDirect(id, quota)
}

func DecreaseQuotaDirect(id int, quota int64) error {
	return model.DB.Model(&model.User{}).Where("id = ?", id).Update("quota", gorm.Expr("quota - ?", quota)).Error
}

func GetRootEmail() string {
	var email string
	model.DB.Model(&model.User{}).Where("role = ?", model.RoleRootUser).Select("email").Find(&email)
	return email
}

func UpdateUsedQuotaAndRequestCount(id int, quota int64) {
	if config.BatchUpdateEnabled {
		model.AddBatchUpdateRecord(model.BatchUpdateTypeUsedQuota, id, quota)
		model.AddBatchUpdateRecord(model.BatchUpdateTypeRequestCount, id, 1)
		return
	}
	UpdateUsedQuotaAndRequestCountDirect(id, quota, 1)
}

func UpdateUsedQuotaAndRequestCountDirect(id int, quota int64, count int) {
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", count),
		},
	).Error
	if err != nil {
		logger.SysError("failed to update user used quota and request count: " + err.Error())
	}
}

func UpdateUsedQuotaDirect(id int, quota int64) {
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Updates(
		map[string]interface{}{
			"used_quota": gorm.Expr("used_quota + ?", quota),
		},
	).Error
	if err != nil {
		logger.SysError("failed to update user used quota: " + err.Error())
	}
}

func UpdateRequestCountDirect(id int, count int) {
	err := model.DB.Model(&model.User{}).Where("id = ?", id).Update("request_count", gorm.Expr("request_count + ?", count)).Error
	if err != nil {
		logger.SysError("failed to update user request count: " + err.Error())
	}
}

func GetUsernameById(id int) string {
	var username string
	model.DB.Model(&model.User{}).Where("id = ?", id).Select("username").Find(&username)
	return username
}

func SearchLogsByPeriodAndModel(userId, start, end int, granularity string, models []string) ([]*model.LogStatistic, error) {
	return model.SearchLogsByPeriodAndModel(userId, start, end, granularity, models)
}

func SearchLogModelsByPeriod(userId, start, end int) ([]string, error) {
	return model.SearchLogModelsByPeriod(userId, start, end)
}

func AccessTokenExists(token string) (bool, error) {
	var user model.User
	err := model.DB.Where("access_token = ?", token).First(&user).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func Redeem(ctx context.Context, key string, userId int) (int64, error) {
	return model.Redeem(ctx, key, userId)
}

func RecordLog(ctx context.Context, userId int, logType int, content string) {
	model.RecordLog(ctx, userId, logType, content)
}

func RecordTopupLog(ctx context.Context, userId int, remark string, quota int) {
	model.RecordTopupLog(ctx, userId, remark, quota)
}
