package channel

import (
	"github.com/yeying-community/router/internal/admin/model"
	channelrepo "github.com/yeying-community/router/internal/admin/repository/channel"
)

func GetAll(start, num int, status string) ([]*model.Channel, error) {
	return channelrepo.GetAll(start, num, status)
}

func GetAllBasic(start, num int, status string, selectAll bool) ([]*model.Channel, error) {
	return channelrepo.GetAllBasic(start, num, status, selectAll)
}

func ListPage(page int, pageSize int, keyword string) ([]*model.Channel, int64, error) {
	return channelrepo.ListPage(page, pageSize, keyword)
}

func GetByID(id string) (*model.Channel, error) {
	return channelrepo.GetByID(id)
}

func GetBasicByID(id string) (*model.Channel, error) {
	return channelrepo.GetBasicByID(id)
}

func Insert(channel *model.Channel) error {
	return channelrepo.Insert(channel)
}

func DeleteByID(id string) error {
	return channelrepo.DeleteByID(id)
}

func DeleteDisabled() (int64, error) {
	return channelrepo.DeleteDisabled()
}

func Update(channel *model.Channel) error {
	return channelrepo.Update(channel)
}

func UpdateTestModelByID(id string, testModel string) error {
	return channelrepo.UpdateTestModelByID(id, testModel)
}
