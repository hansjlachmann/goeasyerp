package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// The resource prices of a company before projects get a blank Project No. in the new key;
// a table already with the column is left alone
func TestProjectJournalRekeysResourcePrices(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "c1$Resource_Price" (type INTEGER, code TEXT(20), work_type_code TEXT(10), currency_code TEXT(10), unit_price TEXT, PRIMARY KEY (type, code, work_type_code, currency_code))`,
		`CREATE INDEX "c1$Resource_Price$Primary" ON "c1$Resource_Price" (type, code, work_type_code, currency_code)`,
		`INSERT INTO "c1$Resource_Price" VALUES (0, 'R0010', '', '', '1250'), (1, 'CONSULT', 'OVERTIME', '', '1600')`,
		`CREATE TABLE "User_Role" (code VARCHAR(20) PRIMARY KEY)`,
		`INSERT INTO "User_Role" VALUES ('READER')`,
		`CREATE TABLE "Permission" (role_id VARCHAR(20), table_name VARCHAR(50), can_read BOOLEAN, can_insert BOOLEAN, can_modify BOOLEAN, can_delete BOOLEAN, PRIMARY KEY (role_id, table_name))`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1"}}
	ctx.SetTx(tx)
	if err := (&Migration050ProjectJournal{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "c1$Resource_Price" WHERE project_no = ''`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("prices with a blank project: %d, %v", n, err)
	}
	var price string
	if err := db.QueryRow(`SELECT unit_price FROM "c1$Resource_Price" WHERE code = 'CONSULT' AND work_type_code = 'OVERTIME'`).Scan(&price); err != nil || price != "1600" {
		t.Errorf("price kept: %q, %v", price, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Permission" WHERE role_id = 'READER' AND table_name LIKE 'PROJECT%'`).Scan(&n); err != nil || n != 5 {
		t.Errorf("READER permissions: %d, %v", n, err)
	}
	// the new key: the same price for a project is another record
	if _, err := db.Exec(`INSERT INTO "c1$Resource_Price" (project_no, type, code, work_type_code, currency_code, unit_price) VALUES ('P00001', 0, 'R0010', '', '', '1100')`); err != nil {
		t.Errorf("a project's own price: %v", err)
	}
}
