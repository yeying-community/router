package model

import (
	"fmt"

	"gorm.io/gorm"
)

func ensureUserBalanceLotAmountColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !tx.Migrator().HasTable(UserBalanceLotsTableName) {
		return tx.AutoMigrate(&UserBalanceLot{})
	}
	amountColumns := []string{"TotalAmount", "UsedAmount", "RemainingAmount"}
	for _, column := range amountColumns {
		if tx.Migrator().HasColumn(&UserBalanceLot{}, column) {
			continue
		}
		if err := tx.Migrator().AddColumn(&UserBalanceLot{}, column); err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(UserBalanceLotsTableName, "total_yyc") {
		if err := tx.Exec(
			"UPDATE user_balance_lots SET total_amount = total_yyc WHERE COALESCE(total_amount, 0) = 0 AND COALESCE(total_yyc, 0) > 0",
		).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(UserBalanceLotsTableName, "used_yyc") {
		if err := tx.Exec(
			"UPDATE user_balance_lots SET used_amount = used_yyc WHERE COALESCE(used_amount, 0) = 0 AND COALESCE(used_yyc, 0) > 0",
		).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(UserBalanceLotsTableName, "remaining_yyc") {
		if err := tx.Exec(
			"UPDATE user_balance_lots SET remaining_amount = remaining_yyc WHERE COALESCE(remaining_amount, 0) = 0 AND COALESCE(remaining_yyc, 0) > 0",
		).Error; err != nil {
			return err
		}
	}
	if err := tx.Exec(
		"CREATE INDEX IF NOT EXISTS idx_user_balance_lots_amount_active_expire ON user_balance_lots (user_id, remaining_amount, status, expires_at)",
	).Error; err != nil {
		return err
	}
	return nil
}

func ensureUserBalanceLotTransactionAmountColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !tx.Migrator().HasTable(UserBalanceLotTransactionsTableName) {
		return tx.AutoMigrate(&UserBalanceLotTransaction{})
	}
	amountColumns := []string{"DeltaAmount", "LotRemainingBefore", "LotRemainingAfter"}
	for _, column := range amountColumns {
		if tx.Migrator().HasColumn(&UserBalanceLotTransaction{}, column) {
			continue
		}
		if err := tx.Migrator().AddColumn(&UserBalanceLotTransaction{}, column); err != nil {
			return err
		}
	}
	if tx.Migrator().HasColumn(UserBalanceLotTransactionsTableName, "delta_yyc") {
		if err := tx.Exec(
			"UPDATE user_balance_lot_transactions SET delta_amount = delta_yyc WHERE COALESCE(delta_amount, 0) = 0 AND COALESCE(delta_yyc, 0) <> 0",
		).Error; err != nil {
			return err
		}
	}
	return nil
}
