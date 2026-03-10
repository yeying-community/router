package model

type ChannelRepository struct {
	GetChannelById               func(id string) (*Channel, error)
	Insert                       func(channel *Channel) error
	Update                       func(channel *Channel) error
	UpdateResponseTime           func(channel *Channel, responseTime int64)
	UpdateBalance                func(channel *Channel, balance float64)
	Delete                       func(channel *Channel) error
	UpdateChannelStatusById      func(id string, status int)
	UpdateChannelUsedQuota       func(id string, quota int64)
	UpdateChannelUsedQuotaDirect func(id string, quota int64)
	UpdateChannelTestModelByID   func(id string, testModel string) error
	DeleteChannelByStatus        func(status int64) (int64, error)
	DeleteDisabledChannel        func() (int64, error)
}

var channelRepo ChannelRepository

func BindChannelRepository(repo ChannelRepository) {
	channelRepo = repo
}

func mustChannelRepo() ChannelRepository {
	if channelRepo.GetChannelById == nil {
		panic("channel repository not initialized")
	}
	return channelRepo
}
