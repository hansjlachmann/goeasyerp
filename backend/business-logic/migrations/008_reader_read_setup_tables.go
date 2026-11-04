package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration008ReaderReadSetupTables{})
}

// Migration008ReaderReadSetupTables gives the READER role read permission on the tables
// added after migration 002 seeded it: Country_Region and SMTP_Setup (whose password a
// reader only sees masked). Only when the READER role exists, and never over a permission
// an administrator already set for these tables.
type Migration008ReaderReadSetupTables struct{}

func (m *Migration008ReaderReadSetupTables) Version() int {
	return 8
}

func (m *Migration008ReaderReadSetupTables) Name() string {
	return "reader_read_setup_tables"
}

func (m *Migration008ReaderReadSetupTables) Description() string {
	return "READER role: read permission on Country_Region and SMTP_Setup"
}

func (m *Migration008ReaderReadSetupTables) Up(ctx *fmigrations.Context) error {
	query := `INSERT OR IGNORE INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = ?)`
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT $1, $2, $3, $4, $5, $6 WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = $7)
		ON CONFLICT (role_id, table_name) DO NOTHING`
	}
	// Permission keys are uppercase (migration 005, types.Code)
	for _, table := range []string{"COUNTRY_REGION", "SMTP_SETUP"} {
		err := ctx.ExecuteSQL(query, "READER", table, true, false, false, false, "READER")
		if err != nil {
			return fmt.Errorf("READER read permission on %s: %w", table, err)
		}
	}
	return nil
}
