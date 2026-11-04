package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// READER reads the general journal tables; running it again changes nothing
func TestGeneralJournalMigration(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, m := range []fmigrations.Migration{&Migration002SeedRoles{}, &Migration023GeneralJournal{}, &Migration023GeneralJournal{}} {
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
	for _, table := range generalJournalTables {
		var read bool
		var n int
		if err := db.QueryRow(`SELECT can_read, (SELECT COUNT(*) FROM "Permission" WHERE role_id = 'READER' AND table_name = ?) FROM "Permission" WHERE role_id = 'READER' AND table_name = ?`, table, table).Scan(&read, &n); err != nil || !read || n != 1 {
			t.Errorf("%s: read %v, %d rows (%v)", table, read, n, err)
		}
	}
}
