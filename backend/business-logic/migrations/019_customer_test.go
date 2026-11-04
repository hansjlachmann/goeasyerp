package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// On a company of the old schema: ZIP_Code becomes Post_Code (data, field numbers,
// permissions), Status Blocked becomes Blocked = Yes and the old columns go, the ledger gets
// its debit/credit amounts; running it again changes nothing
func TestCustomerMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	run := func(m fmigrations.Migration) {
		t.Helper()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1"}}
		ctx.SetTx(tx)
		if err := m.Up(ctx); err != nil {
			t.Fatalf("migration %d: %v", m.Version(), err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	run(&Migration002SeedRoles{})
	for _, q := range []string{
		`CREATE TABLE "c1$ZIP_Code" (code TEXT PRIMARY KEY, city TEXT)`,
		`CREATE INDEX "c1$ZIP_Code$city" ON "c1$ZIP_Code" (city)`,
		`INSERT INTO "c1$ZIP_Code" VALUES ('0159', 'Oslo')`,
		`CREATE TABLE "_field_definition" (physical_table TEXT, field_no INTEGER, column_name TEXT, PRIMARY KEY (physical_table, field_no))`,
		`INSERT INTO "_field_definition" VALUES ('c1$ZIP_Code', 1, 'code'), ('c1$ZIP_Code', 2, 'city')`,
		`CREATE TABLE "c1$Customer" (no TEXT PRIMARY KEY, name TEXT, status INTEGER, last_order_date TEXT, created_at TEXT, phonenumber TEXT)`,
		`INSERT INTO "c1$Customer" VALUES ('C1', 'Open one', 1, NULL, NULL, '1'), ('C2', 'Blocked one', 4, '2026-01-01', NULL, '2')`,
		`CREATE TABLE "c1$Customer Ledger Entry" (entry_no INTEGER PRIMARY KEY, amount TEXT, amount_lcy TEXT)`,
		`INSERT INTO "c1$Customer Ledger Entry" VALUES (1, '100.5', '100.5'), (2, '-40', '-40'), (3, '0', '0')`,
		`INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES ('READER', 'ZIP_CODE', 1, 0, 0, 0)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	run(&Migration019Customer{})
	run(&Migration019Customer{})

	var city string
	if err := db.QueryRow(`SELECT city FROM "c1$Post_Code" WHERE code = '0159'`).Scan(&city); err != nil || city != "Oslo" {
		t.Errorf("Post_Code row: %q, %v", city, err)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM "_field_definition" WHERE physical_table = 'c1$Post_Code'`).Scan(&n)
	if n != 2 {
		t.Errorf("field numbers of Post_Code: %d", n)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'c1$ZIP_Code%'`).Scan(&n)
	if n != 0 {
		t.Errorf("%d ZIP_Code objects left", n)
	}
	var perms int
	_ = db.QueryRow(`SELECT COUNT(*) FROM "Permission" WHERE table_name = 'POST_CODE' AND role_id = 'READER' AND can_read`).Scan(&perms)
	_ = db.QueryRow(`SELECT COUNT(*) FROM "Permission" WHERE table_name = 'ZIP_CODE'`).Scan(&n)
	if perms != 1 || n != 0 {
		t.Errorf("permissions: POST_CODE %d, ZIP_CODE %d", perms, n)
	}

	blocked := map[string]bool{}
	rows, err := db.Query(`SELECT no, blocked FROM "c1$Customer"`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var no string
		var b bool
		_ = rows.Scan(&no, &b)
		blocked[no] = b
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	if blocked["C1"] || !blocked["C2"] {
		t.Errorf("blocked = %v", blocked)
	}
	for _, column := range []string{"status", "last_order_date", "created_at"} {
		if _, err := db.Exec(`SELECT ` + column + ` FROM "c1$Customer"`); err == nil {
			t.Errorf("column %s still exists", column)
		}
	}

	for entry, want := range map[int][2]float64{1: {100.5, 0}, 2: {0, 40}, 3: {0, 0}} {
		var debit, credit, debitLCY, creditLCY float64
		if err := db.QueryRow(`SELECT CAST(debit_amount AS REAL), CAST(credit_amount AS REAL), CAST(debit_amount_lcy AS REAL), CAST(credit_amount_lcy AS REAL)
			FROM "c1$Customer Ledger Entry" WHERE entry_no = ?`, entry).Scan(&debit, &credit, &debitLCY, &creditLCY); err != nil {
			t.Fatal(err)
		}
		if debit != want[0] || credit != want[1] || debitLCY != want[0] || creditLCY != want[1] {
			t.Errorf("entry %d: debit %v credit %v (LCY %v %v), want %v", entry, debit, credit, debitLCY, creditLCY, want)
		}
	}
}
