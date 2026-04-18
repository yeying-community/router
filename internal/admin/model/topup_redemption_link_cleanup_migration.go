package model

import "gorm.io/gorm"

func dropTopupRedemptionLinkColumnsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}
	if tx.Migrator().HasTable(TopupOrdersTableName) && tx.Migrator().HasColumn(TopupOrdersTableName, "redemption_id") {
		if err := tx.Migrator().DropColumn(TopupOrdersTableName, "redemption_id"); err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable("redemptions") && tx.Migrator().HasColumn("redemptions", "topup_order_id") {
		if err := tx.Migrator().DropColumn("redemptions", "topup_order_id"); err != nil {
			return err
		}
	}
	return nil
}
