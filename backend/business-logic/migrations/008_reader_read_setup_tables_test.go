package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func TestReaderReadSetupTables(t *testing.T) {
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
		ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
		ctx.SetTx(tx)
		if err := m.Up(ctx); err != nil {
			t.Fatalf("migration %d: %v", m.Version(), err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	permission := func(table string) (read, modify bool, found bool) {
		err := db.QueryRow(`SELECT can_read, can_modify FROM "Permission" WHERE role_id = 'READER' AND table_name = ?`, table).Scan(&read, &modify)
		return read, modify, err == nil
	}

	run(&Migration002SeedRoles{})
	// An administrator already let readers modify SMTP Setup: kept as it is
	if _, err := db.Exec(`INSERT INTO "Permission" (role_id, table_name, can_read, can_insert, can_modify, can_delete) VALUES ('READER', 'SMTP_SETUP', 1, 0, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	run(&Migration008ReaderReadSetupTables{})

	if read, modify, found := permission("COUNTRY_REGION"); !found || !read || modify {
		t.Errorf("COUNTRY_REGION: found %v read %v modify %v, want read only", found, read, modify)
	}
	if read, modify, found := permission("SMTP_SETUP"); !found || !read || !modify {
		t.Errorf("SMTP_SETUP: found %v read %v modify %v, want the administrator's permission kept", found, read, modify)
	}

	// Without a READER role (deleted by an administrator) nothing is added
	if _, err := db.Exec(`DELETE FROM "Permission"; DELETE FROM "User_Role" WHERE code = 'READER'`); err != nil {
		t.Fatal(err)
	}
	run(&Migration008ReaderReadSetupTables{})
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Permission"`).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d permissions without a READER role (%v), want 0", n, err)
	}
}
