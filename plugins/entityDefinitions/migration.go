package entityDefinitions

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type EntityDefinition struct {
		db.Model
		ProjectId      string  `gorm:"uniqueIndex:idx_entity_def_project_kind"`
		KindName       string  `gorm:"uniqueIndex:idx_entity_def_project_kind"`
		PluralOverride *string
		Description    *string
	}

	return &gormigrate.Migration{
		ID: "2026070712232737",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&EntityDefinition{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&EntityDefinition{})
		},
	}
}
