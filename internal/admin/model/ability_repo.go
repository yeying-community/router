package model

import "context"

type AbilityRepository struct {
	GetRandomSatisfiedChannel func(group string, model string, ignoreFirstPriority bool) (*Channel, error)
	AddAbilities              func(channel *Channel) error
	DeleteAbilities           func(channel *Channel) error
	UpdateAbilities           func(channel *Channel) error
	UpdateAbilityStatus       func(channelId int, status bool) error
	GetTopChannelByModel      func(group string, model string) (*Channel, error)
	GetGroupModels            func(ctx context.Context, group string) ([]string, error)
}

var abilityRepo AbilityRepository

func BindAbilityRepository(repo AbilityRepository) {
	abilityRepo = repo
}

func mustAbilityRepo() AbilityRepository {
	if abilityRepo.GetRandomSatisfiedChannel == nil {
		panic("ability repository not initialized")
	}
	return abilityRepo
}
