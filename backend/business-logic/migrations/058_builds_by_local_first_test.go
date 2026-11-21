package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// Builds By's stored values swap with its options (Local Builder first); the launcher knows the
// migration by the same number
func TestBuildsByLocalFirst(t *testing.T) {
	if BuildsByLocalFirstVersion != appbundle.BuildsByLocalFirstMigration {
		t.Fatalf("the launcher waits for migration %d, the migration is %d", appbundle.BuildsByLocalFirstMigration, BuildsByLocalFirstVersion)
	}
	for _, c := range []struct{ before, after int }{{1, 0}, {0, 1}} {
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(`CREATE TABLE "App_Build_Setup" (primary_key VARCHAR(20) PRIMARY KEY, builds_by INTEGER, allow_upload BOOLEAN)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "App_Build_Setup" VALUES ('', ?, 0)`, c.before); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
		ctx.SetTx(tx)
		if err := (&Migration058BuildsByLocalFirst{}).Up(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		var got int
		if err := db.QueryRow(`SELECT builds_by FROM "App_Build_Setup"`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != c.after {
			t.Errorf("builds_by %d became %d, want %d", c.before, got, c.after)
		}
		_ = db.Close()
	}
	// a database without the table (a new one): nothing to do
	db, _ := sql.Open("sqlite3", ":memory:")
	defer func() { _ = db.Close() }()
	tx, _ := db.Begin()
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
	ctx.SetTx(tx)
	if err := (&Migration058BuildsByLocalFirst{}).Up(ctx); err != nil {
		t.Errorf("without the table: %v", err)
	}
	_ = tx.Rollback()
}
