package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// A layout saved for page 409 (the SMTP Setup list) moves to 910; other pages stay
func TestSMTPSetupPageLayoutsMoveTo910(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "User_Preferences" (user_id VARCHAR(50), page_id INTEGER, type VARCHAR(20), name VARCHAR(50), PRIMARY KEY (user_id, page_id, type, name))`,
		`INSERT INTO "User_Preferences" VALUES ('HANS', 409, 'LAYOUT', 'DEFAULT'), ('HANS', 22, 'LAYOUT', 'DEFAULT')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
	ctx.SetTx(tx)
	if err := (&Migration055RenumberSMTPSetupPage{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var pages []int
	rows, err := db.Query(`SELECT page_id FROM "User_Preferences" ORDER BY page_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		pages = append(pages, p)
	}
	if len(pages) != 2 || pages[0] != 22 || pages[1] != 910 {
		t.Errorf("pages %v, want 22 and 910", pages)
	}
}
