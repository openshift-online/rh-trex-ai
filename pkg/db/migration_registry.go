package db

import (
	"sort"

	"github.com/go-gormigrate/gormigrate/v2"
)

var migrationRegistry []*gormigrate.Migration

func RegisterMigration(m *gormigrate.Migration) {
	migrationRegistry = append(migrationRegistry, m)
}

func LoadDiscoveredMigrations() []*gormigrate.Migration {
	sorted := make([]*gormigrate.Migration, len(migrationRegistry))
	copy(sorted, migrationRegistry)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}
