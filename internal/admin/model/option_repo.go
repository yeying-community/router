package model

type OptionRepository struct {
	AllOption    func() ([]*Option, error)
	UpdateOption func(key string, value string) error
}

var optionRepo OptionRepository

func BindOptionRepository(repo OptionRepository) {
	optionRepo = repo
}

func mustOptionRepo() OptionRepository {
	if optionRepo.UpdateOption == nil {
		panic("option repository not initialized")
	}
	return optionRepo
}
