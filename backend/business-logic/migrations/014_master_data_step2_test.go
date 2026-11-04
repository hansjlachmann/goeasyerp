package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// READER reads General Ledger Setup and Currency Exchange Rate; a permission an administrator
// set is kept; running it again changes nothing
func TestMasterDataStep2(t *testing.T) {
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
	if _, err := db.Exec(`INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES ('READER', 'CURRENCY_EXCHANGE_RATE', 1, 1, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	run(&Migration014MasterDataStep2{})
	run(&Migration014MasterDataStep2{})
	for _, table := range step2Tables {
		var read, modify bool
		var n int
		if err := db.QueryRow(`SELECT can_read, can_modify, (SELECT COUNT(*) FROM "Permission" WHERE role_id = 'READER' AND table_name = ?) FROM "Permission" WHERE role_id = 'READER' AND table_name = ?`, table, table).Scan(&read, &modify, &n); err != nil || !read || n != 1 {
			t.Errorf("%s: read %v, %d rows (%v)", table, read, n, err)
		}
		if modify != (table == "CURRENCY_EXCHANGE_RATE") {
			t.Errorf("%s: modify %v", table, modify)
		}
	}
}

// READER reads the posting groups
func TestPostingGroups(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, m := range []fmigrations.Migration{&Migration002SeedRoles{}, &Migration015PostingGroups{}} {
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
	for _, table := range postingGroupTables {
		var read bool
		if err := db.QueryRow(`SELECT can_read FROM "Permission" WHERE role_id = 'READER' AND table_name = ?`, table).Scan(&read); err != nil || !read {
			t.Errorf("%s: read %v (%v)", table, read, err)
		}
	}
}
