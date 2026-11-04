package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// newSQLiteTestContext opens an in-memory SQLite database, begins a transaction, and
// returns a migration Context wired to it (mirroring how the runner drives migrations).
func newSQLiteTestContext(t *testing.T) (*Context, *sql.Tx, func()) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	ctx := &Context{DBType: database.DBTypeSQLite}
	ctx.SetTx(tx)

	cleanup := func() {
		_ = tx.Rollback()
		_ = db.Close()
	}
	return ctx, tx, cleanup
}

func tableColumns(t *testing.T, tx *sql.Tx, table string) []string {
	t.Helper()
	rows, err := tx.Query(`PRAGMA table_info("` + table + `")`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name)
	}
	return names
}

func TestRecreateTableDropsColumnAndCopiesData(t *testing.T) {
	ctx, tx, cleanup := newSQLiteTestContext(t)
	defer cleanup()

	if err := ctx.ExecuteSQL(`CREATE TABLE "t" (id INTEGER PRIMARY KEY, name TEXT, obsolete TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := ctx.ExecuteSQL(`INSERT INTO "t" (id, name, obsolete) VALUES (1, 'a', 'x'), (2, 'b', 'y')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Rebuild without the "obsolete" column.
	if err := ctx.RecreateTable("t", "id INTEGER PRIMARY KEY, name TEXT", []string{"id", "name"}); err != nil {
		t.Fatalf("RecreateTable: %v", err)
	}

	cols := tableColumns(t, tx, "t")
	if got, want := len(cols), 2; got != want {
		t.Fatalf("column count = %d, want %d (%v)", got, want, cols)
	}
	for _, c := range cols {
		if c == "obsolete" {
			t.Fatalf("obsolete column was not dropped: %v", cols)
		}
	}

	// Data for the preserved columns must survive.
	var name string
	if err := tx.QueryRow(`SELECT name FROM "t" WHERE id = 2`).Scan(&name); err != nil {
		t.Fatalf("query copied data: %v", err)
	}
	if name != "b" {
		t.Fatalf("copied name = %q, want %q", name, "b")
	}

	// No temp table should be left behind.
	var leftover int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='t__migration_tmp'`).Scan(&leftover); err != nil {
		t.Fatalf("check temp table: %v", err)
	}
	if leftover != 0 {
		t.Fatalf("temp table left behind")
	}
}

func TestChangeColumnTypeSQLite(t *testing.T) {
	ctx, tx, cleanup := newSQLiteTestContext(t)
	defer cleanup()

	if err := ctx.ExecuteSQL(`CREATE TABLE "t" (id INTEGER PRIMARY KEY, amount TEXT NOT NULL DEFAULT '0')`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := ctx.ExecuteSQL(`INSERT INTO "t" (id, amount) VALUES (1, '42')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := ctx.ChangeColumnType("t", "amount", "REAL"); err != nil {
		t.Fatalf("ChangeColumnType: %v", err)
	}

	// The declared type of "amount" should now be REAL, and preserved attributes intact.
	rows, err := tx.Query(`PRAGMA table_info("t")`)
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()

	var sawAmount, sawIDPK bool
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		switch name {
		case "amount":
			sawAmount = true
			if typ != "REAL" {
				t.Errorf("amount type = %q, want REAL", typ)
			}
			if notNull != 1 {
				t.Errorf("amount NOT NULL not preserved")
			}
			if !dflt.Valid || dflt.String != "'0'" {
				t.Errorf("amount default = %v, want '0'", dflt)
			}
		case "id":
			if pk != 1 {
				t.Errorf("id primary key not preserved (pk=%d)", pk)
			}
			sawIDPK = true
		}
	}
	if !sawAmount || !sawIDPK {
		t.Fatalf("expected columns not found (amount=%v id=%v)", sawAmount, sawIDPK)
	}

	// Existing data must be preserved and now readable as a number.
	var amount float64
	if err := tx.QueryRow(`SELECT amount FROM "t" WHERE id = 1`).Scan(&amount); err != nil {
		t.Fatalf("read converted value: %v", err)
	}
	if amount != 42 {
		t.Fatalf("converted amount = %v, want 42", amount)
	}
}

func TestChangeColumnTypeMissingColumn(t *testing.T) {
	ctx, _, cleanup := newSQLiteTestContext(t)
	defer cleanup()

	if err := ctx.ExecuteSQL(`CREATE TABLE "t" (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := ctx.ChangeColumnType("t", "nope", "REAL"); err == nil {
		t.Fatalf("expected error for missing column, got nil")
	}
}
