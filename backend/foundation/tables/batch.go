package tables

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// maxBatchParameters keeps a multi-row INSERT below the bind parameter limits: Postgres
// allows 65,535, SQLite 32,766 (its default since 3.32).
const maxBatchParameters = 30000

// maxBatchRows caps the rows per statement for narrow tables.
const maxBatchRows = 1000

// BatchInsertError is the error of a generated InsertAll: the record at Index (in the
// slice passed to InsertAll) failed its OnInsert trigger, or a statement holding it failed.
type BatchInsertError struct {
	Index   int
	Trigger bool // the OnInsert trigger failed (a business-rule message), not the database
	Err     error
}

func (e *BatchInsertError) Error() string {
	return fmt.Sprintf("record %d: %v", e.Index, e.Err)
}

func (e *BatchInsertError) Unwrap() error { return e.Err }

// InsertRows inserts rows (one value per column each) into table with multi-row INSERT
// statements, as many rows per statement as the parameter limits allow. Used by the
// generated InsertAll for bulk loads: one round trip per batch instead of per record.
// A failed statement returns a BatchInsertError with the index of its first row.
func InsertRows(db database.Executor, dbType database.DBType, table string, columns []string, rows [][]interface{}) error {
	if len(rows) == 0 || len(columns) == 0 {
		return nil
	}
	perStatement := max(1, min(maxBatchRows, maxBatchParameters/len(columns)))
	head := `INSERT INTO "` + strings.ReplaceAll(table, `"`, `""`) + `" (` + strings.Join(columns, ", ") + `) VALUES `
	for start := 0; start < len(rows); start += perStatement {
		end := min(start+perStatement, len(rows))
		var b strings.Builder
		b.WriteString(head)
		args := make([]interface{}, 0, (end-start)*len(columns))
		n := 0
		for i, row := range rows[start:end] {
			if len(row) != len(columns) {
				return &BatchInsertError{Index: start + i, Err: fmt.Errorf("%d values for %d columns", len(row), len(columns))}
			}
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteByte('(')
			for j := range row {
				if j > 0 {
					b.WriteString(", ")
				}
				n++
				if dbType == database.DBTypePostgres {
					fmt.Fprintf(&b, "$%d", n)
				} else {
					b.WriteByte('?')
				}
			}
			b.WriteByte(')')
			args = append(args, row...)
		}
		if _, err := db.Exec(b.String(), args...); err != nil {
			return &BatchInsertError{Index: start, Err: err}
		}
	}
	return nil
}
