package model

type TokenRepository struct {
	GetAllUserTokens         func(userId int, startIdx int, num int, order string) ([]*Token, error)
	GetFirstAvailableToken   func(userId int) (*Token, error)
	SearchUserTokens         func(userId int, keyword string) ([]*Token, error)
	ValidateUserToken        func(key string) (*Token, error)
	GetTokenByIds            func(id int, userId int) (*Token, error)
	GetTokenById             func(id int) (*Token, error)
	Insert                   func(token *Token) error
	Update                   func(token *Token) error
	SelectUpdate             func(token *Token) error
	Delete                   func(token *Token) error
	DeleteTokenById          func(id int, userId int) error
	IncreaseTokenQuota       func(id int, quota int64) error
	DecreaseTokenQuota       func(id int, quota int64) error
	IncreaseTokenQuotaDirect func(id int, quota int64) error
	DecreaseTokenQuotaDirect func(id int, quota int64) error
}

var tokenRepo TokenRepository

func BindTokenRepository(repo TokenRepository) {
	tokenRepo = repo
}

func mustTokenRepo() TokenRepository {
	if tokenRepo.GetTokenById == nil {
		panic("token repository not initialized")
	}
	return tokenRepo
}
