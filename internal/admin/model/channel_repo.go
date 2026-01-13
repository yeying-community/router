package model

type ChannelRepository struct {
	GetAllChannels               func(startIdx int, num int, scope string) ([]*Channel, error)
	SearchChannels               func(keyword string) ([]*Channel, error)
	GetChannelById               func(id int, selectAll bool) (*Channel, error)
	BatchInsertChannels          func(channels []Channel) error
	Insert                       func(channel *Channel) error
	Update                       func(channel *Channel) error
	UpdateResponseTime           func(channel *Channel, responseTime int64)
	UpdateBalance                func(channel *Channel, balance float64)
	Delete                       func(channel *Channel) error
	UpdateChannelStatusById      func(id int, status int)
	UpdateChannelUsedQuota       func(id int, quota int64)
	UpdateChannelUsedQuotaDirect func(id int, quota int64)
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
