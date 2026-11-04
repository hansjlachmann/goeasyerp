package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration012GlobalUserPreferences{})
}

// Migration012GlobalUserPreferences makes User_Preferences a global table (one per database,
// as the preferences API always used it): the per-company tables the table sync created
// ("company$User_Preferences") are moved into the global "User_Preferences" and dropped.
type Migration012GlobalUserPreferences struct{}

func (m *Migration012GlobalUserPreferences) Version() int {
	return 12
}

func (m *Migration012GlobalUserPreferences) Name() string {
	return "global_user_preferences"
}

func (m *Migration012GlobalUserPreferences) Description() string {
	return "User_Preferences is a global table: company copies moved into it"
}

// The table as it is at this migration (the table sync adds later changes)
const userPreferencesColumns = `user_id VARCHAR(50),
	page_id INTEGER,
	preference_type VARCHAR(20),
	preference_name VARCHAR(50),
	preference_data TEXT,
	created_at TIMESTAMP,
	updated_at TIMESTAMP,
	PRIMARY KEY (user_id, page_id, preference_type, preference_name)`

func (m *Migration012GlobalUserPreferences) Up(ctx *fmigrations.Context) error {
	return ctx.ForEachCompanyTable("User_Preferences", func(table string) error {
		exists, err := ctx.TableExists(table)
		if err != nil || !exists {
			return err
		}
		err = ctx.ExecuteSQL(`CREATE TABLE IF NOT EXISTS "User_Preferences" (` + userPreferencesColumns + `)`)
		if err != nil {
			return fmt.Errorf("create User_Preferences: %w", err)
		}
		columns := "user_id, page_id, preference_type, preference_name, preference_data, created_at, updated_at"
		copyRows := fmt.Sprintf(`INSERT INTO "User_Preferences" (%s) SELECT %s FROM "%s" ON CONFLICT DO NOTHING`, columns, columns, table)
		if ctx.DBType != database.DBTypePostgres {
			copyRows = fmt.Sprintf(`INSERT OR IGNORE INTO "User_Preferences" (%s) SELECT %s FROM "%s"`, columns, columns, table)
		}
		err = ctx.ExecuteSQL(copyRows)
		if err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
		return ctx.ExecuteSQL(fmt.Sprintf(`DROP TABLE "%s"`, table))
	})
}
