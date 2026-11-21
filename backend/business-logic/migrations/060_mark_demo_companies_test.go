package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// A company holding the demo data becomes a demo company; one with its own customers, or
// without a customer table, does not
func TestMarkDemoCompanies(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`CREATE TABLE "Company" (name VARCHAR(100) PRIMARY KEY, display_name VARCHAR(100))`,
		`INSERT INTO "Company" (name) VALUES ('demo'), ('real'), ('empty')`,
		`CREATE TABLE "demo$Customer" (no VARCHAR(20) PRIMARY KEY, name VARCHAR(100))`,
		`CREATE TABLE "real$Customer" (no VARCHAR(20) PRIMARY KEY, name VARCHAR(100))`,
		`INSERT INTO "demo$Customer" VALUES ('C00010', 'Fjordlys Interiør AS'), ('C00020', 'Bryggen Sjømat AS')`,
		`INSERT INTO "real$Customer" VALUES ('C00010', 'Our first customer')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"demo", "real", "empty"}}
	ctx.SetTx(tx)
	if err := (&Migration060MarkDemoCompanies{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT name FROM "Company" WHERE demo_company ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var marked []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		marked = append(marked, name)
	}
	if len(marked) != 1 || marked[0] != "demo" {
		t.Fatalf("marked %v, want [demo]", marked)
	}
}
