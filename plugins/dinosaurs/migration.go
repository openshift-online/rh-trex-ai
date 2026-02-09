package dinosaurs

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex/pkg/db"
)

func migration() *gormigrate.Migration {
	type Dinosaur struct {
		db.Model
		Species string `gorm:"index"`
	}

	return &gormigrate.Migration{
		ID: "201911212019",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Dinosaur{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Dinosaur{})
		},
	}
}
