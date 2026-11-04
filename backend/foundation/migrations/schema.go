package migrations

import (
	"database/sql"
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// SQLite schema for migration infrastructure tables
const sqliteSchema = `
-- Migration version tracking (global, not company-scoped)
CREATE TABLE IF NOT EXISTS "_migrations" (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration_ms INTEGER,
    status TEXT DEFAULT 'completed',
    error_message TEXT
);

-- Distributed lock for Kubernetes (single row table)
CREATE TABLE IF NOT EXISTS "_migration_lock" (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    locked_by TEXT,
    locked_at TIMESTAMP,
    expires_at TIMESTAMP
);

-- Insert the single lock row if it doesn't exist
INSERT OR IGNORE INTO "_migration_lock" (id) VALUES (1);
`

// PostgreSQL schema for migration infrastructure tables
const postgresSchema = `
-- Migration version tracking (global, not company-scoped)
CREATE TABLE IF NOT EXISTS "_migrations" (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    duration_ms INTEGER,
    status TEXT DEFAULT 'completed',
    error_message TEXT
);

-- Distributed lock for Kubernetes (single row table)
CREATE TABLE IF NOT EXISTS "_migration_lock" (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    locked_by TEXT,
    locked_at TIMESTAMP,
    expires_at TIMESTAMP
);

-- Insert the single lock row if it doesn't exist
INSERT INTO "_migration_lock" (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
`

// EnsureInfrastructureTables creates the migration infrastructure tables if they don't exist
func EnsureInfrastructureTables(db *sql.DB, dbType database.DBType) error {
	var schema string
	switch dbType {
	case database.DBTypePostgres:
		schema = postgresSchema
	default:
		schema = sqliteSchema
	}

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create migration infrastructure tables: %w", err)
	}

	return nil
}

// GetAppliedVersions returns a set of already applied migration versions
func GetAppliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM "_migrations" WHERE status = 'completed'`)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions[version] = true
	}

	return versions, nil
}

// RecordMigration records a migration in the _migrations table
func RecordMigration(tx *sql.Tx, record Record) error {
	_, err := tx.Exec(`
		INSERT INTO "_migrations" (version, name, description, applied_at, duration_ms, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, record.Version, record.Name, record.Description, record.AppliedAt, record.DurationMs, record.Status, record.ErrorMessage)

	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}
