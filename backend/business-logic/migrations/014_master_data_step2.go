package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration014MasterDataStep2{})
}

// Migration014MasterDataStep2 goes with the second migration step (98 General Ledger Setup,
// 330 Currency Exchange Rate): the READER role may read the new tables, as all business
// tables. The tables themselves come from the startup table sync.
type Migration014MasterDataStep2 struct{}

func (m *Migration014MasterDataStep2) Version() int {
	return 14
}

func (m *Migration014MasterDataStep2) Name() string {
	return "master_data_step2"
}

func (m *Migration014MasterDataStep2) Description() string {
	return "READER reads General Ledger Setup and Currency Exchange Rate"
}

// step2Tables: the new tables of the step (permission keys: upper case API names)
var step2Tables = []string{"GENERAL_LEDGER_SETUP", "CURRENCY_EXCHANGE_RATE"}

func (m *Migration014MasterDataStep2) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, step2Tables)
}

// grantReaderRead gives the READER role read permission on tables (kept when an administrator
// set one already; nothing when the READER role was deleted)
func grantReaderRead(ctx *fmigrations.Context, tables []string) error {
	query := `INSERT OR IGNORE INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = ?)`
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT $1, $2, $3, $4, $5, $6 WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = $7)
		ON CONFLICT (role_id, table_name) DO NOTHING`
	}
	for _, table := range tables {
		err := ctx.ExecuteSQL(query, "READER", table, true, false, false, false, "READER")
		if err != nil {
			return fmt.Errorf("READER read permission on %s: %w", table, err)
		}
	}
	return nil
}
