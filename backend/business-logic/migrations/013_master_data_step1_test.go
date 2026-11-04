package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// READER reads the new master data tables (Payment Terms is widened on Postgres only: see
// the pattern of migration 009's test for a Postgres run)
func TestMasterDataStep1(t *testing.T) {
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
	// An administrator already let readers modify locations: kept as it is
	if _, err := db.Exec(`INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES ('READER', 'LOCATION', 1, 0, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	run(&Migration013MasterDataStep1{})
	for _, table := range masterDataTables {
		var read, modify bool
		if err := db.QueryRow(`SELECT can_read, can_modify FROM "Permission" WHERE role_id = 'READER' AND table_name = ?`, table).Scan(&read, &modify); err != nil || !read {
			t.Errorf("%s: read %v (%v)", table, read, err)
		}
		if modify != (table == "LOCATION") {
			t.Errorf("%s: modify %v", table, modify)
		}
	}
}
