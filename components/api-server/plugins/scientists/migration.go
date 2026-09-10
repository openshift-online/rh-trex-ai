package scientists

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

func migration() *gormigrate.Migration {
	type Scientist struct {
		db.Model
		Name  string
		Field string
	}

	return &gormigrate.Migration{
		ID: "2026022421425426",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Scientist{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Scientist{})
		},
	}
}
