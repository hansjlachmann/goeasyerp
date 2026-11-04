package migrations

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration005UppercasePermissionKeys{})
}

// Migration005UppercasePermissionKeys uppercases role_id and table_name in Permission table.
// The seed migration (002) inserted mixed-case table names (e.g., "Job_Queue_Entry") directly
// via SQL, bypassing the types.Code uppercase conversion. This causes 404 errors when the
// frontend sends the uppercased value back (Code fields auto-uppercase in the API layer).
type Migration005UppercasePermissionKeys struct{}

func (m *Migration005UppercasePermissionKeys) Version() int {
	return 5
}

func (m *Migration005UppercasePermissionKeys) Name() string {
	return "uppercase_permission_keys"
}

func (m *Migration005UppercasePermissionKeys) Description() string {
	return "Uppercase role_id and table_name in Permission table to match Code type behavior"
}

func (m *Migration005UppercasePermissionKeys) Up(ctx *fmigrations.Context) error {
	var query string
	if ctx.DBType == database.DBTypePostgres {
		query = `UPDATE "Permission" SET role_id = UPPER(role_id), table_name = UPPER(table_name)`
	} else {
		query = `UPDATE "Permission" SET role_id = UPPER(role_id), table_name = UPPER(table_name)`
	}
	return ctx.ExecuteSQL(query)
}
