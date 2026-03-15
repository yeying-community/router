package option

import (
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/internal/admin/model"
	optionrepo "github.com/yeying-community/router/internal/admin/repository/option"
)

func GetOptions() []*model.Option {
	options := make([]*model.Option, 0)
	config.OptionMapRWMutex.Lock()
	for k, v := range config.OptionMap {
		if strings.HasSuffix(k, "Token") || strings.HasSuffix(k, "Secret") {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: helper.Interface2String(v),
		})
	}
	config.OptionMapRWMutex.Unlock()
	return options
}

func UpdateOption(key string, value string) error {
	if strings.TrimSpace(key) == "DefaultUserGroup" {
		normalizedValue, err := model.ValidateDefaultUserGroupOptionValue(value)
		if err != nil {
			return err
		}
		value = normalizedValue
	}
	return optionrepo.Update(key, value)
}
