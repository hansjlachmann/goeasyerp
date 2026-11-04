package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func TestGlobalUserPreferences(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "c1$User_Preferences" (` + userPreferencesColumns + `)`,
		`INSERT INTO "c1$User_Preferences" (user_id, page_id, preference_type, preference_name, preference_data) VALUES ('HANS', 22, 'COLUMNS', 'DEFAULT', '{}')`,
		`CREATE TABLE "c2$User_Preferences" (` + userPreferencesColumns + `)`,
		// the same key in a second company: the first one is kept
		`INSERT INTO "c2$User_Preferences" (user_id, page_id, preference_type, preference_name, preference_data) VALUES ('HANS', 22, 'COLUMNS', 'DEFAULT', '{"x":1}'), ('ANNE', 22, 'COLUMNS', 'DEFAULT', '{}')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// c3 never had the table
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1", "c2", "c3"}}
	ctx.SetTx(tx)
	if err := (&Migration012GlobalUserPreferences{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "User_Preferences"`).Scan(&n); err != nil || n != 2 {
		t.Errorf("global rows %d, %v; want 2", n, err)
	}
	for _, table := range []string{"c1$User_Preferences", "c2$User_Preferences"} {
		if _, err := db.Exec(`SELECT 1 FROM "` + table + `"`); err == nil {
			t.Errorf("%s still exists", table)
		}
	}
}
