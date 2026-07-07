package model

import "context"

type UserRepository struct {
	GetMaxUserId                             func() string
	GetAllUsers                              func(startIdx int, num int, order string) ([]*User, error)
	SearchUsers                              func(keyword string) ([]*User, error)
	GetUserById                              func(id string, selectAll bool) (*User, error)
	GetUserIdByAffCode                       func(affCode string) (string, error)
	DeleteUserById                           func(id string) error
	Insert                                   func(ctx context.Context, user *User, inviterId string) error
	Update                                   func(user *User, updatePassword bool) error
	Delete                                   func(user *User) error
	ValidateAndFill                          func(user *User) error
	FillUserById                             func(user *User) error
	FillUserByEmail                          func(user *User) error
	FillUserByGitHubId                       func(user *User) error
	FillUserByLarkId                         func(user *User) error
	FillUserByOidcId                         func(user *User) error
	FillUserByWeChatId                       func(user *User) error
	FillUserByUsername                       func(user *User) error
	FillUserByWalletAddress                  func(user *User) error
	IsEmailAlreadyTaken                      func(email string) bool
	IsWeChatIdAlreadyTaken                   func(wechatId string) bool
	IsGitHubIdAlreadyTaken                   func(githubId string) bool
	IsLarkIdAlreadyTaken                     func(larkId string) bool
	IsOidcIdAlreadyTaken                     func(oidcId string) bool
	IsWalletAddressAlreadyTaken              func(address string) bool
	IsUsernameAlreadyTaken                   func(username string) bool
	ResetUserPasswordByEmail                 func(email string, password string) error
	IsAdmin                                  func(userId string) bool
	IsUserEnabled                            func(userId string) (bool, error)
	ValidateAccessToken                      func(token string) *User
	GetUserQuota                             func(id string) (int64, error)
	GetUserUsedQuota                         func(id string) (int64, error)
	GetUserEmail                             func(id string) (string, error)
	GetUserGroup                             func(id string) (string, error)
	GetRootUserEmail                         func() string
	UpdateUserUsedQuotaAndRequestCount       func(id string, quota int64)
	UpdateUserUsedQuotaAndRequestCountDirect func(id string, quota int64, count int)
	UpdateUserUsedQuotaDirect                func(id string, quota int64)
	UpdateUserRequestCountDirect             func(id string, count int)
	GetUsernameById                          func(id string) string
}

var userRepo UserRepository

func BindUserRepository(repo UserRepository) {
	userRepo = repo
}

func mustUserRepo() UserRepository {
	if userRepo.GetUserById == nil {
		panic("user repository not initialized")
	}
	return userRepo
}
