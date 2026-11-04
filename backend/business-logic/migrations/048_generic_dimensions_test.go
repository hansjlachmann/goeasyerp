package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func TestGenericDimensions(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "c1$Department" (code TEXT(10) PRIMARY KEY, name TEXT(50))`,
		`CREATE INDEX "c1$Department$Primary" ON "c1$Department" (code)`,
		`INSERT INTO "c1$Department" VALUES ('ADM', 'Administration')`,
		`CREATE TABLE "c1$Project" (code TEXT(10) PRIMARY KEY, name TEXT(50))`,
		`INSERT INTO "c1$Project" VALUES ('WEB', 'Webshop')`,
		`CREATE TABLE "c1$General_Ledger_Setup" (primary_key TEXT(10) PRIMARY KEY)`,
		`INSERT INTO "c1$General_Ledger_Setup" VALUES ('')`,
		`CREATE TABLE "_field_definition" (physical_table VARCHAR(250), field_no INTEGER, column_name VARCHAR(250), PRIMARY KEY (physical_table, field_no))`,
		`INSERT INTO "_field_definition" VALUES ('c1$Department', 1, 'code'), ('c1$Project', 1, 'code')`,
		`CREATE TABLE "Permission" (role_id VARCHAR(20), table_name VARCHAR(50), can_read BOOLEAN, PRIMARY KEY (role_id, table_name))`,
		// READER gets the new names; SALES has a row for the new name already (kept as it is)
		`INSERT INTO "Permission" VALUES ('READER', 'DEPARTMENT', 1), ('READER', 'PROJECT', 1), ('SALES', 'DEPARTMENT', 1), ('SALES', 'DIMENSION_1', 0)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// c2 has no tables yet
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1", "c2"}}
	ctx.SetTx(tx)
	if err := (&Migration048GenericDimensions{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM "c1$Dimension_1" WHERE code = 'ADM'`).Scan(&name); err != nil || name != "Administration" {
		t.Errorf("Dimension_1: %q, %v", name, err)
	}
	if err := db.QueryRow(`SELECT name FROM "c1$Dimension_2" WHERE code = 'WEB'`).Scan(&name); err != nil || name != "Webshop" {
		t.Errorf("Dimension_2: %q, %v", name, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('c1$Department', 'c1$Project', 'c1$Department$Primary')`).Scan(&n); err != nil || n != 0 {
		t.Errorf("old tables or index left: %d, %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "_field_definition" WHERE physical_table IN ('c1$Dimension_1', 'c1$Dimension_2')`).Scan(&n); err != nil || n != 2 {
		t.Errorf("field numbers: %d, %v", n, err)
	}
	var dim1, dim2 string
	if err := db.QueryRow(`SELECT dimension_1, dimension_2 FROM "c1$General_Ledger_Setup"`).Scan(&dim1, &dim2); err != nil || dim1 != "Department" || dim2 != "Project" {
		t.Errorf("labels: %q %q, %v", dim1, dim2, err)
	}
	rows, err := db.Query(`SELECT role_id || ':' || table_name FROM "Permission" ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	_ = rows.Close()
	want := []string{"READER:DIMENSION_1", "READER:DIMENSION_2", "SALES:DEPARTMENT", "SALES:DIMENSION_1"}
	if len(got) != len(want) {
		t.Fatalf("permissions %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("permissions %v, want %v", got, want)
			break
		}
	}
}
