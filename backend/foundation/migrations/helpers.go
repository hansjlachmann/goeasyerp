package migrations

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// RenameColumn renames a column in a table
// Note: PostgreSQL supports ALTER TABLE RENAME COLUMN directly, and SQLite 3.25.0+
// (bundled with the driver) supports it as well. For structural changes SQLite cannot
// do in place (dropping columns on old SQLite, changing types, altering constraints),
// use RecreateTable.
func (ctx *Context) RenameColumn(tableName, oldName, newName string) error {
	switch ctx.DBType {
	case database.DBTypePostgres:
		sql := fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN %s TO %s`, tableName, oldName, newName)
		return ctx.ExecuteSQL(sql)
	default:
		// SQLite doesn't support RENAME COLUMN in older versions
		// For SQLite 3.25.0+, this works:
		sql := fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN %s TO %s`, tableName, oldName, newName)
		return ctx.ExecuteSQL(sql)
	}
}

// DropColumn removes a column from a table
// Note: PostgreSQL supports ALTER TABLE DROP COLUMN directly
// SQLite doesn't support DROP COLUMN (requires table recreation)
func (ctx *Context) DropColumn(tableName, columnName string) error {
	switch ctx.DBType {
	case database.DBTypePostgres:
		sql := fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN IF EXISTS %s`, tableName, columnName)
		return ctx.ExecuteSQL(sql)
	default:
		// SQLite 3.35.0+ supports DROP COLUMN
		sql := fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN %s`, tableName, columnName)
		return ctx.ExecuteSQL(sql)
	}
}

// AddColumn adds a new column to a table
func (ctx *Context) AddColumn(tableName, columnName, definition string) error {
	sql := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s %s`, tableName, columnName, definition)
	return ctx.ExecuteSQL(sql)
}

// AddColumnIfNotExists adds a column only if it doesn't exist
func (ctx *Context) AddColumnIfNotExists(tableName, columnName, definition string) error {
	exists, err := ctx.ColumnExists(tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return ctx.AddColumn(tableName, columnName, definition)
}

// ColumnExists checks if a column exists in a table
func (ctx *Context) ColumnExists(tableName, columnName string) (bool, error) {
	var count int

	switch ctx.DBType {
	case database.DBTypePostgres:
		err := ctx.tx.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		`, tableName, strings.ToLower(columnName)).Scan(&count)
		if err != nil {
			return false, err
		}
	default:
		rows, err := ctx.tx.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
		if err != nil {
			return false, err
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name string
			var dataType string
			var notNull int
			var dfltValue interface{}
			var pk int

			if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
				return false, err
			}
			if strings.EqualFold(name, columnName) {
				return true, nil
			}
		}
		return false, nil
	}

	return count > 0, nil
}

// TableExists checks if a table exists
func (ctx *Context) TableExists(tableName string) (bool, error) {
	var name string

	switch ctx.DBType {
	case database.DBTypePostgres:
		err := ctx.tx.QueryRow(`
			SELECT table_name FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1
		`, tableName).Scan(&name)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return false, nil
			}
			return false, err
		}
	default:
		err := ctx.tx.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name=?
		`, tableName).Scan(&name)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return false, nil
			}
			return false, err
		}
	}

	return true, nil
}

// CreateIndex creates an index on a table
func (ctx *Context) CreateIndex(indexName, tableName string, columns ...string) error {
	sql := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "%s" ON "%s" (%s)`,
		indexName, tableName, strings.Join(columns, ", "))
	return ctx.ExecuteSQL(sql)
}

// DropIndex removes an index
func (ctx *Context) DropIndex(indexName string) error {
	sql := fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, indexName)
	return ctx.ExecuteSQL(sql)
}

// UpdateData updates data in a table with a WHERE clause
func (ctx *Context) UpdateData(tableName, setClause, whereClause string, args ...interface{}) error {
	sql := fmt.Sprintf(`UPDATE "%s" SET %s WHERE %s`, tableName, setClause, whereClause)
	return ctx.ExecuteSQL(sql, args...)
}

// DeleteData deletes data from a table with a WHERE clause
func (ctx *Context) DeleteData(tableName, whereClause string, args ...interface{}) error {
	sql := fmt.Sprintf(`DELETE FROM "%s" WHERE %s`, tableName, whereClause)
	return ctx.ExecuteSQL(sql, args...)
}

// CopyColumnData copies data from one column to another (useful before dropping)
func (ctx *Context) CopyColumnData(tableName, sourceColumn, targetColumn string) error {
	sql := fmt.Sprintf(`UPDATE "%s" SET %s = %s`, tableName, targetColumn, sourceColumn)
	return ctx.ExecuteSQL(sql)
}

// SetColumnDefault sets a default value for existing NULL values
func (ctx *Context) SetColumnDefault(tableName, columnName string, defaultValue interface{}) error {
	var sql string
	switch v := defaultValue.(type) {
	case string:
		sql = fmt.Sprintf(`UPDATE "%s" SET %s = '%s' WHERE %s IS NULL`, tableName, columnName, v, columnName)
	default:
		sql = fmt.Sprintf(`UPDATE "%s" SET %s = %v WHERE %s IS NULL`, tableName, columnName, v, columnName)
	}
	return ctx.ExecuteSQL(sql)
}

// ChangeColumnType changes a column's data type.
// PostgreSQL alters the column in place. SQLite cannot change a column's type in place,
// so the table is rebuilt via RecreateTable: the current columns are introspected with
// PRAGMA table_info and the schema is reconstructed, substituting newType for the target
// column. Both require the existing data to be convertible to the new type.
//
// Limitation (SQLite): CHECK constraints and table-level constraints (composite PRIMARY
// KEY, FOREIGN KEY) are not reported by PRAGMA table_info and are therefore not preserved.
// If the table relies on those, build the new schema explicitly and call RecreateTable.
func (ctx *Context) ChangeColumnType(tableName, columnName, newType string) error {
	if ctx.DBType == database.DBTypePostgres {
		sql := fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN %s TYPE %s`, tableName, columnName, newType)
		return ctx.ExecuteSQL(sql)
	}

	cols, err := ctx.sqliteColumns(tableName)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return fmt.Errorf("table %q has no columns (does it exist?)", tableName)
	}

	// Count declared primary-key columns so we know whether to emit an inline
	// "PRIMARY KEY" (single-column PK) or a table-level clause (composite PK).
	pkCount := 0
	for _, c := range cols {
		if c.PK > 0 {
			pkCount++
		}
	}

	found := false
	defs := make([]string, 0, len(cols))
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		typ := c.Type
		if strings.EqualFold(c.Name, columnName) {
			typ = newType
			found = true
		}
		defs = append(defs, c.definition(typ, pkCount == 1))
		names = append(names, c.Name)
	}
	if !found {
		return fmt.Errorf("column %q not found in table %q", columnName, tableName)
	}

	schema := strings.Join(defs, ", ")
	if pkCount > 1 {
		schema += ", PRIMARY KEY (" + strings.Join(compositePKColumns(cols), ", ") + ")"
	}
	return ctx.RecreateTable(tableName, schema, names)
}

// RecreateTable rebuilds tableName with a new column schema, copying the listed columns
// from the old table into the new one. This is the SQLite-safe way to perform structural
// changes SQLite's ALTER TABLE cannot do in place — dropping columns on old SQLite,
// changing a column's type, or altering constraints — following SQLite's recommended
// "create new, copy, drop, rename" procedure.
//
// newSchema is everything that goes inside CREATE TABLE (...), e.g.
// "id INTEGER PRIMARY KEY, name TEXT NOT NULL, amount REAL". copyColumns lists the columns
// to carry over from the old table (names must exist in both the old table and newSchema);
// omit a column to drop it. The SQL is portable, so this also works on PostgreSQL.
//
// Indexes and triggers on the old table are dropped with it — recreate any needed indexes
// with CreateIndex afterwards. Migrations already run inside a transaction; if the table is
// referenced by SQLite foreign keys, ensure foreign-key enforcement is disabled.
func (ctx *Context) RecreateTable(tableName, newSchema string, copyColumns []string) error {
	tmpName := tableName + "__migration_tmp"

	// Clean up any temp table left behind by a previously failed attempt.
	if err := ctx.ExecuteSQL(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tmpName)); err != nil {
		return fmt.Errorf("recreate %q: drop stale temp table: %w", tableName, err)
	}

	// 1. Create the new table with the desired schema.
	if err := ctx.ExecuteSQL(fmt.Sprintf(`CREATE TABLE "%s" (%s)`, tmpName, newSchema)); err != nil {
		return fmt.Errorf("recreate %q: create new table: %w", tableName, err)
	}

	// 2. Copy the preserved columns across.
	if len(copyColumns) > 0 {
		quoted := make([]string, len(copyColumns))
		for i, c := range copyColumns {
			quoted[i] = fmt.Sprintf("%q", c)
		}
		colList := strings.Join(quoted, ", ")
		insert := fmt.Sprintf(`INSERT INTO "%s" (%s) SELECT %s FROM "%s"`, tmpName, colList, colList, tableName)
		if err := ctx.ExecuteSQL(insert); err != nil {
			return fmt.Errorf("recreate %q: copy data: %w", tableName, err)
		}
	}

	// 3. Drop the old table.
	if err := ctx.ExecuteSQL(fmt.Sprintf(`DROP TABLE "%s"`, tableName)); err != nil {
		return fmt.Errorf("recreate %q: drop old table: %w", tableName, err)
	}

	// 4. Rename the new table into place.
	if err := ctx.ExecuteSQL(fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, tmpName, tableName)); err != nil {
		return fmt.Errorf("recreate %q: rename new table: %w", tableName, err)
	}

	return nil
}

// sqliteColumn describes a column as reported by PRAGMA table_info.
type sqliteColumn struct {
	Name    string
	Type    string
	NotNull bool
	Default sql.NullString
	PK      int // position in the primary key (1-based), 0 if not part of the PK
}

// definition rebuilds this column's DDL using typ as its type. inlinePK controls whether a
// single-column primary key is emitted inline (composite PKs are emitted as a table-level
// clause by the caller instead).
func (c sqliteColumn) definition(typ string, inlinePK bool) string {
	def := fmt.Sprintf("%q %s", c.Name, typ)
	if c.PK > 0 && inlinePK {
		def += " PRIMARY KEY"
	}
	if c.NotNull {
		def += " NOT NULL"
	}
	if c.Default.Valid {
		def += " DEFAULT " + c.Default.String
	}
	return def
}

// compositePKColumns returns the PK column names ordered by their position in the key.
func compositePKColumns(cols []sqliteColumn) []string {
	ordered := make([]string, 0)
	// PRAGMA "pk" is the 1-based position within the primary key; walk positions in order.
	for pos := 1; pos <= len(cols); pos++ {
		for _, c := range cols {
			if c.PK == pos {
				ordered = append(ordered, fmt.Sprintf("%q", c.Name))
			}
		}
	}
	return ordered
}

// sqliteColumns introspects a table's columns via PRAGMA table_info.
func (ctx *Context) sqliteColumns(tableName string) ([]sqliteColumn, error) {
	rows, err := ctx.tx.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []sqliteColumn
	for rows.Next() {
		var (
			cid     int
			c       sqliteColumn
			notNull int
		)
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &c.Default, &c.PK); err != nil {
			return nil, err
		}
		c.NotNull = notNull != 0
		cols = append(cols, c)
	}
	return cols, rows.Err()
}
