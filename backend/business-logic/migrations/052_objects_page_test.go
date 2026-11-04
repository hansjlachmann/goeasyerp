package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

// A saved layout and a setup checklist line of page 358 move to page 908; other pages stay
func TestObjectsPageRenumbered(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "User_Preferences" (user_id VARCHAR(50), page_id INTEGER, preference_type VARCHAR(20), preference_name VARCHAR(50), PRIMARY KEY (user_id, page_id, preference_type, preference_name))`,
		`INSERT INTO "User_Preferences" VALUES ('HANS', 358, 'COLUMNS', 'DEFAULT'), ('HANS', 22, 'COLUMNS', 'DEFAULT')`,
		`CREATE TABLE "c1$Setup_Checklist_Line" (line_no INTEGER PRIMARY KEY, page_id INTEGER)`,
		`INSERT INTO "c1$Setup_Checklist_Line" VALUES (10000, 358), (20000, 4)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1", "c2"}}
	ctx.SetTx(tx)
	if err := (&Migration052ObjectsPage{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "User_Preferences" WHERE page_id IN (908, 22)`).Scan(&n); err != nil || n != 2 {
		t.Errorf("user preferences: %d, %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "c1$Setup_Checklist_Line" WHERE page_id IN (908, 4)`).Scan(&n); err != nil || n != 2 {
		t.Errorf("setup checklist: %d, %v", n, err)
	}
}
