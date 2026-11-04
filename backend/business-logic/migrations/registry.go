package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

var registered []fmigrations.Migration

// Register adds a migration to the registry
// Migrations should be registered in init() functions
func Register(m fmigrations.Migration) {
	registered = append(registered, m)
}

// GetAll returns all registered migrations
func GetAll() []fmigrations.Migration {
	return registered
}

// GetCount returns the number of registered migrations
func GetCount() int {
	return len(registered)
}
