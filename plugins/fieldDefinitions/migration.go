package fieldDefinitions

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type FieldDefinition struct {
		db.Model
		EntityDefinitionId string `gorm:"uniqueIndex:idx_field_def_entity_name"`
		FieldName          string `gorm:"uniqueIndex:idx_field_def_entity_name"`
		FieldType          string
		IsRequired         *bool
	}

	return &gormigrate.Migration{
		ID: "2026070712235733",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&FieldDefinition{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&FieldDefinition{})
		},
	}
}
