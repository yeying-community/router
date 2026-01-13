package model

import "context"

type Ability struct {
	Group     string `json:"group" gorm:"type:varchar(32);primaryKey;autoIncrement:false"`
	Model     string `json:"model" gorm:"primaryKey;autoIncrement:false"`
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool   `json:"enabled"`
	Priority  *int64 `json:"priority" gorm:"bigint;default:0;index"`
}

func GetRandomSatisfiedChannel(group string, model string, ignoreFirstPriority bool) (*Channel, error) {
	return mustAbilityRepo().GetRandomSatisfiedChannel(group, model, ignoreFirstPriority)
}

func (channel *Channel) AddAbilities() error {
	return mustAbilityRepo().AddAbilities(channel)
}

func (channel *Channel) DeleteAbilities() error {
	return mustAbilityRepo().DeleteAbilities(channel)
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities() error {
	return mustAbilityRepo().UpdateAbilities(channel)
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return mustAbilityRepo().UpdateAbilityStatus(channelId, status)
}

// GetTopChannelByModel returns the highest-priority enabled channel for a given group+model.
// Order: priority desc, then channel_id asc (stable for UI usage).
func GetTopChannelByModel(group string, model string) (*Channel, error) {
	return mustAbilityRepo().GetTopChannelByModel(group, model)
}

func GetGroupModels(ctx context.Context, group string) ([]string, error) {
	return mustAbilityRepo().GetGroupModels(ctx, group)
}
