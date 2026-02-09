package events

import (
	"time"

	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex/pkg/db"
)

func migration() *gormigrate.Migration {
	type Event struct {
		db.Model
		Source         string     `gorm:"index"`
		SourceID       string     `gorm:"index"`
		EventType      string
		ReconciledDate *time.Time `gorm:"null;index"`
	}

	return &gormigrate.Migration{
		ID: "202309020925",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Event{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Event{})
		},
	}
}
