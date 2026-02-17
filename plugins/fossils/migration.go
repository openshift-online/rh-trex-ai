package fossils

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type Fossil struct {
		db.Model
		DiscoveryLocation string
		EstimatedAge      *int
		FossilType        *string
		ExcavatorName     *string
	}

	return &gormigrate.Migration{
		ID: "2026021620191012",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Fossil{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Fossil{})
		},
	}
}
