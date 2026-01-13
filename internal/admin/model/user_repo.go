package model

import "context"

type UserRepository struct {
	GetMaxUserId                             func() int
	GetAllUsers                              func(startIdx int, num int, order string) ([]*User, error)
	SearchUsers                              func(keyword string) ([]*User, error)
	GetUserById                              func(id int, selectAll bool) (*User, error)
	GetUserIdByAffCode                       func(affCode string) (int, error)
	DeleteUserById                           func(id int) error
	Insert                                   func(ctx context.Context, user *User, inviterId int) error
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
	IsAdmin                                  func(userId int) bool
	IsUserEnabled                            func(userId int) (bool, error)
	ValidateAccessToken                      func(token string) *User
	GetUserQuota                             func(id int) (int64, error)
	GetUserUsedQuota                         func(id int) (int64, error)
	GetUserEmail                             func(id int) (string, error)
	GetUserGroup                             func(id int) (string, error)
	IncreaseUserQuota                        func(id int, quota int64) error
	DecreaseUserQuota                        func(id int, quota int64) error
	IncreaseUserQuotaDirect                  func(id int, quota int64) error
	DecreaseUserQuotaDirect                  func(id int, quota int64) error
	GetRootUserEmail                         func() string
	UpdateUserUsedQuotaAndRequestCount       func(id int, quota int64)
	UpdateUserUsedQuotaAndRequestCountDirect func(id int, quota int64, count int)
	UpdateUserUsedQuotaDirect                func(id int, quota int64)
	UpdateUserRequestCountDirect             func(id int, count int)
	GetUsernameById                          func(id int) string
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
