package option

import "github.com/yeying-community/router/internal/admin/model"

func init() {
	model.BindOptionRepository(model.OptionRepository{
		AllOption:    All,
		UpdateOption: Update,
	})
}

func All() ([]*model.Option, error) {
	var options []*model.Option
	err := model.DB.Find(&options).Error
	return options, err
}

func Update(key string, value string) error {
	option := model.Option{Key: key}
	model.DB.FirstOrCreate(&option, model.Option{Key: key})
	option.Value = value
	model.DB.Save(&option)
	return model.UpdateOptionMap(key, value)
}
