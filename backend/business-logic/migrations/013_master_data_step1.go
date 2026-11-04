package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration013MasterDataStep1{})
}

// Migration013MasterDataStep1 goes with the first migration step (the master data tables
// 3–14): Payment Terms.description is 50 characters now (it was 30) — table sync only adds
// columns, so the existing VARCHAR(30) columns are widened here (on Postgres; SQLite does not
// enforce the length) — and the READER role may read the new tables, as all business tables.
type Migration013MasterDataStep1 struct{}

func (m *Migration013MasterDataStep1) Version() int {
	return 13
}

func (m *Migration013MasterDataStep1) Name() string {
	return "master_data_step1"
}

func (m *Migration013MasterDataStep1) Description() string {
	return "Payment Terms.description: 50 characters; READER reads the master data tables 4-14"
}

// masterDataTables: the new tables of the step (permission keys: upper case API names)
var masterDataTables = []string{
	"CURRENCY", "FINANCE_CHARGE_TERMS", "PRICE_GROUP", "STANDARD_TEXT", "SHIPMENT_METHOD",
	"DEPARTMENT", "PROJECT", "SALESPERSON_PURCHASER", "LOCATION",
}

func (m *Migration013MasterDataStep1) Up(ctx *fmigrations.Context) error {
	if ctx.DBType == database.DBTypePostgres {
		err := ctx.ForEachCompanyTable("Payment Terms", func(table string) error {
			// A company entered for the first time gets its tables from the sync after this
			exists, err := ctx.TableExists(table)
			if err != nil || !exists {
				return err
			}
			err = ctx.ChangeColumnType(table, "description", "VARCHAR(50)")
			if err != nil {
				return fmt.Errorf("widen Payment Terms description: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// READER: read permission on the new tables (kept when an administrator set one already;
	// nothing when the READER role was deleted)
	query := `INSERT OR IGNORE INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = ?)`
	if ctx.DBType == database.DBTypePostgres {
		query = `INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete)
		SELECT $1, $2, $3, $4, $5, $6 WHERE EXISTS (SELECT 1 FROM "User_Role" WHERE code = $7)
		ON CONFLICT (role_id, table_name) DO NOTHING`
	}
	for _, table := range masterDataTables {
		err := ctx.ExecuteSQL(query, "READER", table, true, false, false, false, "READER")
		if err != nil {
			return fmt.Errorf("READER read permission on %s: %w", table, err)
		}
	}
	return nil
}
