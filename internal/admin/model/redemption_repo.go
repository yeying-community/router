package model

import "context"

type RedemptionRepository struct {
	GetAllRedemptions    func(startIdx int, num int) ([]*Redemption, error)
	SearchRedemptions    func(keyword string) ([]*Redemption, error)
	GetRedemptionById    func(id string) (*Redemption, error)
	ListRedemptionsByRedeemedUserID func(userID string, limit int) ([]*Redemption, error)
	Redeem               func(ctx context.Context, code string, userId string) (RedemptionResult, error)
	Insert               func(redemption *Redemption) error
	SelectUpdate         func(redemption *Redemption) error
	Update               func(redemption *Redemption) error
	Delete               func(redemption *Redemption) error
	DeleteRedemptionById func(id string) error
}

var redemptionRepo RedemptionRepository

func BindRedemptionRepository(repo RedemptionRepository) {
	redemptionRepo = repo
}

func mustRedemptionRepo() RedemptionRepository {
	if redemptionRepo.GetRedemptionById == nil {
		panic("redemption repository not initialized")
	}
	return redemptionRepo
}
