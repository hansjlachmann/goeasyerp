package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration019Customer{})
}

// Migration019Customer goes with the classic Customer (table 18), before the table sync:
//   - table 225 ZIP_Code is now Post_Code (the "Post Code" naming rule): the table, its field
//     numbers and the permissions are renamed; its indexes are dropped (the sync creates them
//     under the new name);
//   - Customer 900 Status became 39 Blocked: Blocked = Yes where Status was Blocked (4), and
//     the Status, Last Order Date (901) and Created At (902) columns are dropped (their field
//     numbers stay taken in _field_definition);
//   - Cust. Ledger Entry 58–61 Debit/Credit Amount (LCY) are filled from Amount and Amount (LCY).
//
// The renamed Customer fields keep their numbers and are renamed by the table sync. Idempotent.
type Migration019Customer struct{}

func (m *Migration019Customer) Version() int {
	return 19
}

func (m *Migration019Customer) Name() string {
	return "customer"
}

func (m *Migration019Customer) Description() string {
	return "Post Code table, Customer Status → Blocked, ledger debit/credit amounts"
}

func (m *Migration019Customer) Up(ctx *fmigrations.Context) error {
	for _, company := range ctx.Companies {
		err := renamePostCodeTable(ctx, company)
		if err != nil {
			return err
		}
		err = customerBlocked(ctx, company+"$Customer")
		if err != nil {
			return err
		}
		err = ledgerDebitCredit(ctx, company+"$Customer Ledger Entry")
		if err != nil {
			return err
		}
	}
	return renamePermission(ctx, "ZIP_CODE", "POST_CODE")
}

func renamePostCodeTable(ctx *fmigrations.Context, company string) error {
	oldName, newName := company+"$ZIP_Code", company+"$Post_Code"
	exists, err := ctx.TableExists(oldName)
	if err != nil || !exists {
		return err
	}
	taken, err := ctx.TableExists(newName)
	if err != nil || taken {
		if err == nil {
			err = fmt.Errorf("both %s and %s exist", oldName, newName)
		}
		return err
	}
	err = dropTableIndexes(ctx, oldName)
	if err != nil {
		return err
	}
	err = ctx.ExecuteSQL(fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, oldName, newName))
	if err != nil {
		return err
	}
	defs, err := ctx.TableExists("_field_definition")
	if err != nil || !defs {
		return err
	}
	return ctx.ExecuteSQL(rebind(ctx, `UPDATE "_field_definition" SET physical_table = ? WHERE physical_table = ?`), newName, oldName)
}

// dropTableIndexes drops a table's own indexes (not its primary key)
func dropTableIndexes(ctx *fmigrations.Context, table string) error {
	query := `SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`
	if ctx.DBType == database.DBTypePostgres {
		query = `SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = $1
			AND indexname NOT IN (SELECT conname FROM pg_constraint WHERE contype = 'p')`
	}
	rows, err := ctx.QuerySQL(query, table)
	if err != nil {
		return err
	}
	var indexes []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			_ = rows.Close()
			return err
		}
		indexes = append(indexes, name)
	}
	err = rows.Err()
	if err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, index := range indexes {
		err := ctx.ExecuteSQL(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, index))
		if err != nil {
			return err
		}
	}
	return nil
}

func customerBlocked(ctx *fmigrations.Context, table string) error {
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	hasStatus, err := ctx.ColumnExists(table, "status")
	if err != nil {
		return err
	}
	if hasStatus {
		boolType := "INTEGER"
		if ctx.DBType == database.DBTypePostgres {
			boolType = "BOOLEAN"
		}
		err := ctx.AddColumnIfNotExists(table, "blocked", boolType)
		if err != nil {
			return err
		}
		// Status Blocked is option 4 (" ", Open, Released, Posted, Blocked)
		err = ctx.ExecuteSQL(fmt.Sprintf(`UPDATE "%s" SET blocked = (status = 4)`, table))
		if err != nil {
			return err
		}
	}
	for _, column := range []string{"status", "last_order_date", "created_at"} {
		exists, err := ctx.ColumnExists(table, column)
		if err != nil {
			return err
		}
		if exists {
			err := ctx.DropColumn(table, column)
			if err != nil {
				return fmt.Errorf("drop %s.%s: %w", table, column, err)
			}
		}
	}
	return nil
}

func ledgerDebitCredit(ctx *fmigrations.Context, table string) error {
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	done, err := ctx.ColumnExists(table, "debit_amount")
	if err != nil || done {
		return err
	}
	// decimals are TEXT on SQLite, NUMERIC on PostgreSQL (as the table sync creates them)
	decimalType, number, zero := "TEXT", "CAST(%s AS REAL)", "'0'"
	if ctx.DBType == database.DBTypePostgres {
		decimalType, number, zero = "NUMERIC", "%s", "0"
	}
	for _, f := range []struct{ debit, credit, amount string }{
		{"debit_amount", "credit_amount", "amount"},
		{"debit_amount_lcy", "credit_amount_lcy", "amount_lcy"},
	} {
		err := ctx.AddColumn(table, f.debit, decimalType)
		if err != nil {
			return err
		}
		err = ctx.AddColumn(table, f.credit, decimalType)
		if err != nil {
			return err
		}
		amount := fmt.Sprintf(number, f.amount)
		credit := fmt.Sprintf(number, "-"+amount)
		if ctx.DBType != database.DBTypePostgres {
			credit = "CAST(-" + amount + " AS TEXT)"
		}
		update := fmt.Sprintf(`UPDATE "%s" SET %s = CASE WHEN %s > 0 THEN %s ELSE %s END, %s = CASE WHEN %s < 0 THEN %s ELSE %s END`,
			table, f.debit, amount, f.amount, zero, f.credit, amount, credit, zero)
		err = ctx.ExecuteSQL(update)
		if err != nil {
			return err
		}
	}
	return nil
}

// renamePermission moves the permissions on a renamed table to its new name (a role that has
// both keeps the new one)
func renamePermission(ctx *fmigrations.Context, oldName, newName string) error {
	exists, err := ctx.TableExists("Permission")
	if err != nil || !exists {
		return err
	}
	err = ctx.ExecuteSQL(rebind(ctx, `UPDATE "Permission" SET table_name = ? WHERE table_name = ?
		AND role_id NOT IN (SELECT role_id FROM "Permission" WHERE table_name = ?)`), newName, oldName, newName)
	if err != nil {
		return err
	}
	return ctx.ExecuteSQL(rebind(ctx, `DELETE FROM "Permission" WHERE table_name = ?`), oldName)
}

// rebind turns ? placeholders into $1, $2 … on PostgreSQL
func rebind(ctx *fmigrations.Context, query string) string {
	if ctx.DBType != database.DBTypePostgres {
		return query
	}
	for i := 1; strings.Contains(query, "?"); i++ {
		query = strings.Replace(query, "?", fmt.Sprintf("$%d", i), 1)
	}
	return query
}
