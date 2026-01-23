package user

import (
	"context"

	"github.com/yeying-community/router/internal/admin/model"
	userrepo "github.com/yeying-community/router/internal/admin/repository/user"
)

func ValidateLogin(user *model.User) error {
	return user.ValidateAndFill()
}

func GetAll(start, num int, order string) ([]*model.User, error) {
	return userrepo.GetAll(start, num, order)
}

func Search(keyword string) ([]*model.User, error) {
	return userrepo.Search(keyword)
}

func GetByID(id int, selectAll bool) (*model.User, error) {
	return userrepo.GetByID(id, selectAll)
}

func GetByUsername(username string) (*model.User, error) {
	return userrepo.GetByUsername(username)
}

func GetIDByAffCode(code string) (int, error) {
	return userrepo.GetIDByAffCode(code)
}

func Create(ctx context.Context, user *model.User, inviterId int) error {
	return userrepo.Create(ctx, user, inviterId)
}

func Update(user *model.User, updatePassword bool) error {
	return userrepo.Update(user, updatePassword)
}

func DeleteByID(id int) error {
	return userrepo.DeleteByID(id)
}

func Delete(user *model.User) error {
	return userrepo.Delete(user)
}

func FillByID(user *model.User) error {
	return userrepo.FillByID(user)
}

func SearchLogsByPeriodAndModel(userId, start, end int, granularity string, models []string) ([]*model.LogStatistic, error) {
	return userrepo.SearchLogsByPeriodAndModel(userId, start, end, granularity, models)
}

func SearchLogModelsByPeriod(userId, start, end int) ([]string, error) {
	return userrepo.SearchLogModelsByPeriod(userId, start, end)
}

func AccessTokenExists(token string) (bool, error) {
	return userrepo.AccessTokenExists(token)
}

func Redeem(ctx context.Context, key string, userId int) (int64, error) {
	return userrepo.Redeem(ctx, key, userId)
}

func IncreaseQuota(userId int, quota int64) error {
	return userrepo.IncreaseQuota(userId, quota)
}

func RecordLog(ctx context.Context, userId int, logType int, content string) {
	userrepo.RecordLog(ctx, userId, logType, content)
}

func RecordTopupLog(ctx context.Context, userId int, remark string, quota int) {
	userrepo.RecordTopupLog(ctx, userId, remark, quota)
}

func GetQuota(userId int) (int64, error) {
	return userrepo.GetQuota(userId)
}

func GetUsedQuota(userId int) (int64, error) {
	return userrepo.GetUsedQuota(userId)
}
