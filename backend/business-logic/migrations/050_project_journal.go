package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func init() {
	Register(&Migration050ProjectJournal{})
}

// Migration050ProjectJournal goes with the project journal and ledger: Resource Price 201 and
// Resource Price Change 335 get Project No. as their first key field (a project's own prices;
// blank: every project's). A primary key the table sync cannot change: per company the rows
// are read, the table is created again with the new key and the rows go back with a blank
// Project No. The READER role may read the new tables (templates, batches, lines, ledger
// entries, registers); they and the new columns come from the table sync.
type Migration050ProjectJournal struct{}

func (m *Migration050ProjectJournal) Version() int {
	return 50
}

func (m *Migration050ProjectJournal) Name() string {
	return "project_journal"
}

func (m *Migration050ProjectJournal) Description() string {
	return "resource prices per project (Project No. in the key); READER reads the project journal and ledger"
}

func (m *Migration050ProjectJournal) Up(ctx *fmigrations.Context) error {
	for _, company := range ctx.Companies {
		err := rekeyWithProjectNo(ctx, company, gtables.ResourcePriceTableName, (&gtables.ResourcePrice{}).GetFields(), func() error {
			return (&gtables.ResourcePrice{}).CreateTableWithDBType(ctx.Tx(), company, ctx.DBType)
		})
		if err != nil {
			return err
		}
		err = rekeyWithProjectNo(ctx, company, gtables.ResourcePriceChangeTableName, (&gtables.ResourcePriceChange{}).GetFields(), func() error {
			return (&gtables.ResourcePriceChange{}).CreateTableWithDBType(ctx.Tx(), company, ctx.DBType)
		})
		if err != nil {
			return err
		}
	}
	return grantReaderRead(ctx, []string{"PROJECT_JOURNAL_TEMPLATE", "PROJECT_JOURNAL_BATCH", "PROJECT_JOURNAL_LINE",
		"PROJECT_LEDGER_ENTRY", "PROJECT_REGISTER"})
}

// rekeyWithProjectNo: a company's table without project_no is created again (create) and gets
// its rows back with a blank Project No. The table is created as its definition is today
// (fields), which may differ from the one the old rows were written with (S15): a column whose
// field was renamed since (the same field number in _field_definition) goes into the renamed
// column, and a column whose field is gone is added to the new table as it was (its data kept,
// as the table sync keeps such columns).
func rekeyWithProjectNo(ctx *fmigrations.Context, company, tableName string, fields []ftables.FieldInfo, create func() error) error {
	table := company + "$" + tableName
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	hasProjectNo, err := ctx.ColumnExists(table, "project_no")
	if err != nil || hasProjectNo {
		return err
	}

	rows, err := ctx.QuerySQL(fmt.Sprintf(`SELECT * FROM "%s"`, table))
	if err != nil {
		return err
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return err
	}
	var records [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		err = rows.Scan(pointers...)
		if err != nil {
			_ = rows.Close()
			return err
		}
		records = append(records, values)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}

	oldTypes, err := ctx.ColumnTypes(table)
	if err != nil {
		return err
	}
	renamed, err := renamedColumns(ctx, table, fields)
	if err != nil {
		return err
	}

	err = ctx.ExecuteSQL(fmt.Sprintf(`DROP TABLE "%s"`, table))
	if err != nil {
		return err
	}
	err = create()
	if err != nil {
		return fmt.Errorf("create %s: %w", table, err)
	}
	newTypes, err := ctx.ColumnTypes(table)
	if err != nil {
		return err
	}

	quoted := make([]string, len(columns))
	marks := make([]string, len(columns)+1)
	for i, c := range columns {
		target := strings.ToLower(c)
		_, exists := newTypes[target]
		if !exists && renamed[target] != "" {
			target = renamed[target]
			_, exists = newTypes[target]
		}
		if !exists {
			// a field removed from the definition: its column comes back as it was
			target = strings.ToLower(c)
			err = ctx.AddColumn(table, target, oldTypes[target])
			if err != nil {
				return fmt.Errorf("keep column %s of %s: %w", c, table, err)
			}
			newTypes[target] = oldTypes[target]
		}
		quoted[i] = `"` + target + `"`
	}
	for i := range marks {
		marks[i] = "?"
		if ctx.DBType == database.DBTypePostgres {
			marks[i] = fmt.Sprintf("$%d", i+1)
		}
	}
	insert := fmt.Sprintf(`INSERT INTO "%s" (project_no, %s) VALUES (%s)`, table, strings.Join(quoted, ", "), strings.Join(marks, ", "))
	for _, values := range records {
		args := append([]interface{}{""}, values...)
		err = ctx.ExecuteSQL(insert, args...)
		if err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
	}
	return nil
}

// renamedColumns: old column → today's column of the fields renamed since the table's last
// sync (the field numbers recorded in _field_definition; none recorded: none renamed)
func renamedColumns(ctx *fmigrations.Context, table string, fields []ftables.FieldInfo) (map[string]string, error) {
	out := map[string]string{}
	exists, err := ctx.TableExists("_field_definition")
	if err != nil || !exists {
		return out, err
	}
	now := map[int]string{}
	for _, f := range fields {
		if !f.FlowField && f.No > 0 {
			now[f.No] = strings.ToLower(f.Name)
		}
	}
	query := `SELECT field_no, column_name FROM "_field_definition" WHERE physical_table = ?`
	if ctx.DBType == database.DBTypePostgres {
		query = `SELECT field_no, column_name FROM "_field_definition" WHERE physical_table = $1`
	}
	rows, err := ctx.QuerySQL(query, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var no int
		var column string
		err = rows.Scan(&no, &column)
		if err != nil {
			return nil, err
		}
		current, ok := now[no]
		if ok && current != strings.ToLower(column) {
			out[strings.ToLower(column)] = current
		}
	}
	return out, rows.Err()
}
