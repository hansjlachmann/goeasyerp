package objects

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// fieldDefinitionTable remembers, per physical table ("company$Customer" or a global table),
// which column each field number (field ID) had at the last table sync. The number is a
// field's identity: a field renamed in the YAML keeps its number, so the sync renames its
// column (the data stays); a field whose number changed stops the sync.
const fieldDefinitionTable = "_field_definition"

// fieldNumberer is implemented by generated tables (field metadata with numbers)
type fieldNumberer interface {
	GetFields() []tables.FieldInfo
}

// storedColumns returns the stored fields of a table by number: the fields that are columns
// (FlowFields are calculated, not stored)
func storedColumns(t fieldNumberer) map[int]string {
	columns := map[int]string{}
	for _, f := range t.GetFields() {
		if !f.FlowField && f.No > 0 {
			columns[f.No] = strings.ToLower(f.Name)
		}
	}
	return columns
}

func ensureFieldDefinitionTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS "` + fieldDefinitionTable + `" (
		physical_table VARCHAR(250) NOT NULL,
		field_no INTEGER NOT NULL,
		column_name VARCHAR(250) NOT NULL,
		PRIMARY KEY (physical_table, field_no))`)
	return err
}

func placeholder(dbType database.DBType, n int) string {
	if dbType == database.DBTypePostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// knownFieldNumbers returns the field numbers and columns recorded for a physical table
func knownFieldNumbers(db *sql.DB, dbType database.DBType, physical string) (map[int]string, error) {
	rows, err := db.Query(`SELECT field_no, column_name FROM "`+fieldDefinitionTable+`" WHERE physical_table = `+placeholder(dbType, 1), physical)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	known := map[int]string{}
	for rows.Next() {
		var no int
		var column string
		if err := rows.Scan(&no, &column); err != nil {
			return nil, err
		}
		known[no] = column
	}
	return known, rows.Err()
}

// renameByFieldNumber compares a table's fields with the numbers recorded at the last sync,
// before missing columns are added: a field number whose column has another name now is a
// renamed field (the field ID stays), so its column is renamed and keeps its data. A
// column that now has another number stops the sync — field numbers never change.
// existingColumns is updated with the renames.
func renameByFieldNumber(db *sql.DB, dbType database.DBType, physical, tableName string, current map[int]string, existingColumns map[string]bool) error {
	known, err := knownFieldNumbers(db, dbType, physical)
	if err != nil {
		return err
	}
	numberOf := map[string]int{}
	for no, column := range current {
		numberOf[column] = no
	}
	var changed []string
	for no, column := range known {
		if now, ok := numberOf[column]; ok && now != no {
			changed = append(changed, fmt.Sprintf("%s: %d → %d", column, no, now))
		}
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		return fmt.Errorf("field numbers of table %s changed (%s): a field number never changes — rename the field instead and keep its number", tableName, strings.Join(changed, ", "))
	}

	nos := make([]int, 0, len(current))
	for no := range current {
		nos = append(nos, no)
	}
	sort.Ints(nos)
	for _, no := range nos {
		column, old := current[no], known[no]
		if old == "" || old == column || !existingColumns[old] {
			continue
		}
		if existingColumns[column] {
			return fmt.Errorf("table %s: field %d was renamed from %s to %s, but a column %s exists already", tableName, no, old, column, column)
		}
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`, physical, old, column)); err != nil {
			return fmt.Errorf("table %s: rename column %s to %s: %w", tableName, old, column, err)
		}
		fmt.Printf("✓ %s: field %d renamed %s → %s\n", physical, no, old, column)
		delete(existingColumns, old)
		existingColumns[column] = true
	}
	return nil
}

// recordFieldNumbers stores the table's current field numbers and columns (fields removed from
// the YAML keep their row: their number stays taken)
func recordFieldNumbers(db *sql.DB, dbType database.DBType, physical string, current map[int]string) error {
	known, err := knownFieldNumbers(db, dbType, physical)
	if err != nil {
		return err
	}
	for no, column := range current {
		switch old, ok := known[no]; {
		case !ok:
			_, err = db.Exec(`INSERT INTO "`+fieldDefinitionTable+`" (physical_table, field_no, column_name) VALUES (`+
				placeholder(dbType, 1)+`, `+placeholder(dbType, 2)+`, `+placeholder(dbType, 3)+`)`, physical, no, column)
		case old != column:
			_, err = db.Exec(`UPDATE "`+fieldDefinitionTable+`" SET column_name = `+placeholder(dbType, 1)+
				` WHERE physical_table = `+placeholder(dbType, 2)+` AND field_no = `+placeholder(dbType, 3), column, physical, no)
		}
		if err != nil {
			return fmt.Errorf("record field %d of %s: %w", no, physical, err)
		}
	}
	return nil
}
