package model

import "gorm.io/gorm"

func dropChannelModelStreamOnlyWithDB(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}
	tableName := (&ChannelModel{}).TableName()
	if tx.Migrator().HasColumn(tableName, "is_stream_only") {
		if err := tx.Migrator().DropColumn(tableName, "is_stream_only"); err != nil {
			return err
		}
	}
	return nil
}
