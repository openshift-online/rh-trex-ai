package relationships

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type Relationship struct {
		db.Model
		ProjectId        string
		SourceEntityId   string
		TargetEntityId   string
		RelationshipType string
		ForeignKey       *string
	}

	return &gormigrate.Migration{
		ID: "2026070712249545",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Relationship{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Relationship{})
		},
	}
}
