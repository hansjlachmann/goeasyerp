package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// Templates without reports get their journal's test and posting report; a template's own
// reports are kept
func TestJournalTemplatesGetTheirReports(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "c1$Gen_Journal_Template" (name TEXT PRIMARY KEY, test_report_id INTEGER, posting_report_id INTEGER)`,
		`INSERT INTO "c1$Gen_Journal_Template" VALUES ('GENERAL', 0, 0), ('OWN', 50000, 50001)`,
		`CREATE TABLE "c1$Res_Journal_Template" (name TEXT PRIMARY KEY, test_report_id INTEGER, posting_report_id INTEGER)`,
		`INSERT INTO "c1$Res_Journal_Template" VALUES ('RESOURCES', NULL, 0)`,
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
	if err := (&Migration053JournalReports{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		table, name   string
		test, posting int
	}{
		{"c1$Gen_Journal_Template", "GENERAL", 2, 3},
		{"c1$Gen_Journal_Template", "OWN", 50000, 50001},
		{"c1$Res_Journal_Template", "RESOURCES", 1102, 1103},
	} {
		var test, posting int
		if err := db.QueryRow(`SELECT test_report_id, posting_report_id FROM "`+c.table+`" WHERE name = ?`, c.name).Scan(&test, &posting); err != nil {
			t.Fatal(err)
		}
		if test != c.test || posting != c.posting {
			t.Errorf("%s %s: test %d posting %d, want %d %d", c.table, c.name, test, posting, c.test, c.posting)
		}
	}
}
