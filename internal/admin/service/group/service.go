package group

import "github.com/yeying-community/router/internal/admin/model"

func ListCatalog() ([]model.GroupCatalog, error) {
	return model.ListGroupCatalog()
}

func Get(id string) (model.GroupCatalog, error) {
	return model.GetGroupCatalogByID(id)
}

func Create(item model.GroupCatalog) (model.GroupCatalog, error) {
	return model.CreateGroupCatalog(item)
}

func CreateWithChannelBindings(item model.GroupCatalog, channelIDs []string) (model.GroupCatalog, error) {
	return model.CreateGroupCatalogWithChannelBindings(item, channelIDs)
}

func CreateWithConfig(item model.GroupCatalog, channelIDs []string, modelConfigs []model.GroupModelConfigItem) (model.GroupCatalog, error) {
	return model.CreateGroupCatalogWithConfig(item, channelIDs, modelConfigs)
}

func Update(item model.GroupCatalog) (model.GroupCatalog, error) {
	return model.UpdateGroupCatalog(item)
}

func UpdateWithChannelBindings(item model.GroupCatalog, channelIDs []string) (model.GroupCatalog, error) {
	return model.UpdateGroupCatalogWithChannelBindings(item, channelIDs)
}

func UpdateWithConfig(item model.GroupCatalog, channelIDs []string, modelConfigs []model.GroupModelConfigItem, updateChannels bool, updateModels bool) (model.GroupCatalog, error) {
	return model.UpdateGroupCatalogWithConfig(item, channelIDs, modelConfigs, updateChannels, updateModels)
}

func Delete(id string) error {
	return model.DeleteGroupCatalog(id)
}

func ListChannelBindings(id string) ([]model.GroupChannelBindingItem, error) {
	return model.ListGroupChannelBindings(id)
}

func ListChannelBindingCandidates() ([]model.GroupChannelBindingItem, error) {
	return model.ListGroupChannelBindingCandidates()
}

func ListModelSummaries(id string) ([]model.GroupModelSummaryItem, error) {
	return model.ListGroupModelSummaries(id)
}

func GetModelConfigPayload(id string) (model.GroupModelConfigPayload, error) {
	return model.ListGroupModelConfigPayload(id)
}

func ReplaceChannelBindings(id string, channelIDs []string) error {
	return model.ReplaceGroupChannelBindings(id, channelIDs)
}

func ReplaceModelConfigs(id string, channelIDs []string, modelConfigs []model.GroupModelConfigItem, explicitChannels bool) error {
	return model.ReplaceGroupModelConfigs(id, channelIDs, modelConfigs, explicitChannels)
}
