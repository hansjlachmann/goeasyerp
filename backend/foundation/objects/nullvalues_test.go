package objects

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// The blank value of a NULL comes from the field's type, else the declared type — never the
// column's name (S15): a Code named like a number type gets '', a date (TEXT on SQLite) stays
// NULL — the blank date — whatever its name
func TestFixNullValuesByType(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	schema := `
		no TEXT PRIMARY KEY,
		realized_gains_acc TEXT,
		created_at TEXT,
		last_login TEXT,
		amount TEXT,
		active INTEGER,
		line_count INTEGER,
		note VARCHAR(50),
		picture BLOB`
	if _, err := db.Exec(`CREATE TABLE "t" (` + schema + `)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "t" (no) VALUES ('A')`); err != nil {
		t.Fatal(err)
	}
	columns := parseSchemaColumns(schema)
	existing := map[string]bool{}
	for c := range columns {
		existing[c] = true
	}
	types := map[string]ftables.FieldType{
		"no": ftables.FieldTypeCode, "realized_gains_acc": ftables.FieldTypeCode, "created_at": ftables.FieldTypeDateTime,
		"last_login": ftables.FieldTypeDate, "amount": ftables.FieldTypeDecimal, "active": ftables.FieldTypeBoolean,
		"picture": ftables.FieldTypeBlob,
		// line_count and note without field information: their declared types decide
	}
	if err := fixNullValues(db, "t", columns, existing, types); err != nil {
		t.Fatal(err)
	}
	var acc, amount, note sql.NullString
	var created, login sql.NullString
	var active, count sql.NullInt64
	var picture []byte
	err = db.QueryRow(`SELECT realized_gains_acc, created_at, last_login, amount, active, line_count, note, picture FROM "t"`).
		Scan(&acc, &created, &login, &amount, &active, &count, &note, &picture)
	if err != nil {
		t.Fatal(err)
	}
	if !acc.Valid || acc.String != "" {
		t.Errorf("realized_gains_acc = %+v, want ''", acc)
	}
	if created.Valid || login.Valid {
		t.Errorf("dates made blank text: created_at %+v, last_login %+v", created, login)
	}
	if !amount.Valid || amount.String != "0" || !active.Valid || active.Int64 != 0 || !count.Valid || count.Int64 != 0 {
		t.Errorf("amount %+v, active %+v, line_count %+v", amount, active, count)
	}
	if !note.Valid || note.String != "" || picture != nil {
		t.Errorf("note %+v, picture %v", note, picture)
	}
}
