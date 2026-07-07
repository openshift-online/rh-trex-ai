package builds

import (
	"time"

	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type Build struct {
		db.Model
		ProjectId   string
		Status      string
		BuildLog    *string
		TriggeredBy *string
		CompletedAt *time.Time
	}

	return &gormigrate.Migration{
		ID: "2026070712268594",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Build{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Build{})
		},
	}
}
