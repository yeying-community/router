package model

import (
	"fmt"

	"gorm.io/gorm"
)

func ensureTopupOrderOperationTypeWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if err := tx.AutoMigrate(&TopupOrder{}); err != nil {
		return err
	}
	if err := tx.Exec(
		`UPDATE topup_orders
		 SET operation_type = ''
		 WHERE COALESCE(TRIM(business_type), '') <> ?
		   AND COALESCE(TRIM(operation_type), '') <> ''`,
		TopupOrderBusinessPackage,
	).Error; err != nil {
		return err
	}
	return tx.Exec(
		`UPDATE topup_orders
		 SET operation_type = ?
		 WHERE COALESCE(TRIM(business_type), '') = ?
		   AND (COALESCE(TRIM(operation_type), '') = ''
		     OR TRIM(operation_type) NOT IN (?, ?, ?))`,
		TopupOrderOperationNew,
		TopupOrderBusinessPackage,
		TopupOrderOperationNew,
		TopupOrderOperationRenew,
		TopupOrderOperationUpgrade,
	).Error
}
