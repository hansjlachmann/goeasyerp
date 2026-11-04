package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration003SeedJobQueueMenu{})
}

// Migration003SeedJobQueueMenu seeds the JOBQUEUE menu profile
type Migration003SeedJobQueueMenu struct{}

func (m *Migration003SeedJobQueueMenu) Version() int {
	return 3
}

func (m *Migration003SeedJobQueueMenu) Name() string {
	return "seed_jobqueue_menu"
}

func (m *Migration003SeedJobQueueMenu) Description() string {
	return "Seed JOBQUEUE menu profile for job queue operators"
}

func (m *Migration003SeedJobQueueMenu) Up(ctx *fmigrations.Context) error {
	// Ensure the Menu table exists — migrations run before table sync, so on a fresh
	// database the table has not been created yet.
	menuSQL := `CREATE TABLE IF NOT EXISTS "Menu" (
		code VARCHAR(20) NOT NULL DEFAULT '',
		description VARCHAR(50) NOT NULL DEFAULT '',
		filename VARCHAR(50) NOT NULL DEFAULT '',
		PRIMARY KEY (code)
	)`
	err := ctx.ExecuteSQL(menuSQL)
	if err != nil {
		return fmt.Errorf("failed to ensure Menu table: %w", err)
	}

	var query string
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "Menu" (code, description, filename) VALUES ($1, $2, $3) ON CONFLICT (code) DO NOTHING`
	} else {
		query = `INSERT OR IGNORE INTO "Menu" (code, description, filename) VALUES (?, ?, ?)`
	}
	return ctx.ExecuteSQL(query, "JOBQUEUE", "Job Queue Operator", "jobqueue")
}
