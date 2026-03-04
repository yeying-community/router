package group

import (
	"github.com/yeying-community/router/internal/admin/model"
)

func List() []string {
	groupNames, err := model.ListEnabledGroupNames()
	if err != nil {
		return []string{}
	}
	return groupNames
}

func ListCatalog() ([]model.GroupCatalog, error) {
	return model.ListGroupCatalog()
}

func Get(name string) (model.GroupCatalog, error) {
	return model.GetGroupCatalogByName(name)
}

func Create(item model.GroupCatalog) (model.GroupCatalog, error) {
	return model.CreateGroupCatalog(item)
}

func Update(item model.GroupCatalog) (model.GroupCatalog, error) {
	return model.UpdateGroupCatalog(item)
}

func Delete(name string) error {
	return model.DeleteGroupCatalog(name)
}
