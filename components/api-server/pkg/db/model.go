package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Model struct {
	ID        string `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type FKMigration struct {
	Model     string
	Dest      string
	Field     string
	Reference string
}

func CreateFK(g2 *gorm.DB, fks ...FKMigration) error {
	var query = `ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE RESTRICT ON UPDATE RESTRICT;`
	var drop = `ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;`

	for _, fk := range fks {
		name := fmt.Sprintf("fk_%s_%s", fk.Model, fk.Dest)

		g2.Exec(fmt.Sprintf(drop, fk.Model, name))
		if err := g2.Exec(fmt.Sprintf(query, fk.Model, name, fk.Field, fk.Reference)).Error; err != nil {
			return err
		}
	}
	return nil
}
