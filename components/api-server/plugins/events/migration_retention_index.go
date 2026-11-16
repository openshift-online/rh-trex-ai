package events

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// eventsCreatedAtIndex supports the retention job's max-age delete, which
// filters and orders by created_at. The reconciled_date column already has
// idx_events_reconciled_date from the first events migration, so it is not
// created again here.
const eventsCreatedAtIndex = "idx_events_created_at"

// retentionIndexMigration adds an index on events(created_at).
//
// The index is built CONCURRENTLY so it does not block writes on a large,
// busy events table. That only works outside a transaction, which holds
// because gormigrate.DefaultOptions has UseTransaction false.
//
// A failed concurrent build leaves an INVALID index behind, and
// CREATE INDEX IF NOT EXISTS would then skip it. So an invalid index of the
// same name is dropped first, which makes a re-run repair it.
func retentionIndexMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "2026111612000000",
		Migrate: func(tx *gorm.DB) error {
			var invalid int64
			if err := tx.Raw(`
				SELECT count(*) FROM pg_index i
				JOIN pg_class c ON c.oid = i.indexrelid
				WHERE c.relname = ? AND NOT i.indisvalid`, eventsCreatedAtIndex).Scan(&invalid).Error; err != nil {
				return err
			}
			if invalid > 0 {
				if err := tx.Exec(fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s", eventsCreatedAtIndex)).Error; err != nil {
					return err
				}
			}
			return tx.Exec(fmt.Sprintf(
				"CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON events (created_at)", eventsCreatedAtIndex)).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s", eventsCreatedAtIndex)).Error
		},
	}
}
