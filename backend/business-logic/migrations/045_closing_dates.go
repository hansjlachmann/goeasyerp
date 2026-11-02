package migrations

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
	"github.com/hansjlachmann/openerp/backend/foundation/sift"
)

func init() {
	Register(&Migration045ClosingDates{})
}

// Migration045ClosingDates lets date fields hold closing dates (types.Date: the date at
// 23:59:59). On PostgreSQL the DATE columns become TIMESTAMP (a date at midnight, a closing
// date at 23:59:59); the SIFT totals are dropped first — their triggers name the columns — and
// the table sync builds them again. SQLite keeps dates as text already: nothing to do.
type Migration045ClosingDates struct{}

func (m *Migration045ClosingDates) Version() int {
	return 45
}

func (m *Migration045ClosingDates) Name() string {
	return "closing_dates"
}

func (m *Migration045ClosingDates) Description() string {
	return "date columns hold closing dates (PostgreSQL: DATE becomes TIMESTAMP)"
}

func (m *Migration045ClosingDates) Up(ctx *fmigrations.Context) error {
	if ctx.DBType != database.DBTypePostgres {
		return nil
	}
	if err := sift.DropAll(ctx.DB, ctx.DBType); err != nil {
		return fmt.Errorf("drop SIFT totals: %w", err)
	}
	rows, err := ctx.DB.Query(`SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND data_type = 'date' ORDER BY table_name, column_name`)
	if err != nil {
		return err
	}
	var tables []string
	columns := map[string][]string{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			_ = rows.Close()
			return err
		}
		if columns[table] == nil {
			tables = append(tables, table)
		}
		columns[table] = append(columns[table], column)
	}
	_ = rows.Close()
	for _, table := range tables {
		var alters []string
		for _, column := range columns[table] {
			alters = append(alters, fmt.Sprintf(`ALTER COLUMN "%s" TYPE TIMESTAMP USING "%s"::timestamp`, column, column))
		}
		stmt := fmt.Sprintf(`ALTER TABLE "%s" %s`, table, strings.Join(alters, ", "))
		if _, err := ctx.DB.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}
	return nil
}
