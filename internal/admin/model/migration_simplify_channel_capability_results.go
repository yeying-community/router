package model

import (
	"fmt"

	"gorm.io/gorm"
)

type legacyChannelCapabilityResult struct{}

func (legacyChannelCapabilityResult) TableName() string {
	return ChannelCapabilityResultsTableName
}

func simplifyChannelCapabilityResultsWithDB(tx *gorm.DB) error {
	if tx == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !tx.Migrator().HasTable(ChannelCapabilityResultsTableName) {
		return nil
	}
	hasClientProfile := tx.Migrator().HasColumn(&legacyChannelCapabilityResult{}, "client_profile")
	hasUserAgent := tx.Migrator().HasColumn(&legacyChannelCapabilityResult{}, "user_agent")
	if !hasClientProfile && !hasUserAgent {
		return nil
	}

	const tempTable = "channel_capability_results_v2"
	if err := tx.Exec(`DROP TABLE IF EXISTS ` + tempTable).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
CREATE TABLE channel_capability_results_v2 (
	channel_id char(36) NOT NULL,
	capability varchar(128) NOT NULL,
	label varchar(255),
	endpoint varchar(255),
	model varchar(255),
	status varchar(32),
	supported boolean NOT NULL DEFAULT false,
	message text,
	latency_ms bigint,
	sort_order bigint DEFAULT 0,
	tested_at bigint,
	PRIMARY KEY (channel_id, capability)
)`).Error; err != nil {
		return err
	}

	if err := tx.Exec(`
INSERT INTO channel_capability_results_v2 (
	channel_id,
	capability,
	label,
	endpoint,
	model,
	status,
	supported,
	message,
	latency_ms,
	sort_order,
	tested_at
)
SELECT DISTINCT ON (channel_id, capability)
	channel_id,
	capability,
	label,
	endpoint,
	model,
	status,
	supported,
	message,
	latency_ms,
	sort_order,
	tested_at
FROM (
	SELECT
		BTRIM(channel_id) AS channel_id,
		CASE
			WHEN POSITION('responses:' IN LOWER(BTRIM(capability))) = 1 THEN 'responses'
			ELSE LOWER(BTRIM(capability))
		END AS capability,
		CASE
			WHEN POSITION('responses:' IN LOWER(BTRIM(capability))) = 1 THEN 'Responses'
			ELSE BTRIM(label)
		END AS label,
		CASE
			WHEN POSITION('responses:' IN LOWER(BTRIM(capability))) = 1 THEN '/v1/responses'
			ELSE BTRIM(endpoint)
		END AS endpoint,
		BTRIM(model) AS model,
		CASE
			WHEN LOWER(BTRIM(status)) = 'supported' THEN 'supported'
			WHEN LOWER(BTRIM(status)) = 'skipped' THEN 'skipped'
			ELSE 'unsupported'
		END AS status,
		(COALESCE(supported, false) OR LOWER(BTRIM(status)) = 'supported') AS supported,
		BTRIM(message) AS message,
		latency_ms,
		COALESCE(sort_order, 0) AS sort_order,
		COALESCE(tested_at, 0) AS tested_at,
		CASE
			WHEN COALESCE(supported, false) OR LOWER(BTRIM(status)) = 'supported' THEN 0
			WHEN LOWER(BTRIM(status)) = 'skipped' THEN 1
			ELSE 2
		END AS priority
	FROM channel_capability_results
	WHERE BTRIM(channel_id) <> '' AND BTRIM(capability) <> ''
) AS normalized
ORDER BY
	channel_id,
	capability,
	priority ASC,
	tested_at DESC,
	sort_order ASC`).Error; err != nil {
		return err
	}

	if err := tx.Exec(`DROP TABLE ` + ChannelCapabilityResultsTableName).Error; err != nil {
		return err
	}
	if err := tx.Exec(`ALTER TABLE ` + tempTable + ` RENAME TO ` + ChannelCapabilityResultsTableName).Error; err != nil {
		return err
	}
	if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_channel_capability_results_channel_id ON ` + ChannelCapabilityResultsTableName + ` (channel_id)`).Error; err != nil {
		return err
	}
	if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_channel_capability_results_status ON ` + ChannelCapabilityResultsTableName + ` (status)`).Error; err != nil {
		return err
	}
	return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_channel_capability_results_tested_at ON ` + ChannelCapabilityResultsTableName + ` (tested_at)`).Error
}
