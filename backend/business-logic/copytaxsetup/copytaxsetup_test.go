package copytaxsetup_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/copytaxsetup"
	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Copy Tax Setup from the demo company into a new one: the chosen tables only, all their
// records (tax areas with their lines); a second copy keeps the records that exist (a changed
// description stays)
func TestCopyTaxInfo(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	mustOK(t, err)
	mustOK(t, tables.CreateCompanyTables(db, "new", database.DBTypeSQLite))
	mustOK(t, (&gtables.Company{}).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	_, err = db.Exec(`INSERT INTO "Company" (name) VALUES ('demo'), ('new')`)
	mustOK(t, err)
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	demo := ftables.DBSource(tx, "demo", database.DBTypeSQLite)
	dst := ftables.DBSource(tx, "new", database.DBTypeSQLite)
	count := func(src ftables.Source) [5]int {
		return [5]int{gtables.NewTaxGroup(src).Count(), gtables.NewTaxJurisdiction(src).Count(), gtables.NewTaxArea(src).Count(),
			gtables.NewTaxDetail(src).Count(), gtables.NewTaxAreaLine(src).Count()}
	}
	want := count(demo)
	for i, n := range want {
		if n == 0 {
			t.Fatalf("the demo has no records of table %d", i)
		}
	}

	var progressed [4]int
	copytaxsetup.CopyTaxInfo(dst, "demo", [4]bool{true, false, true, false}, func(table, progress int) { progressed[table] = progress })
	if got := count(dst); got != [5]int{want[0], 0, want[2], 0, want[4]} {
		t.Errorf("groups and areas: %v, demo %v", got, want)
	}
	if progressed != [4]int{10000, 0, 10000, 0} {
		t.Errorf("progress %v", progressed)
	}

	group := gtables.NewTaxGroup(dst)
	if !group.FindFirst() {
		t.Fatal("no tax group")
	}
	group.Description = "changed"
	mustOK(t, group.ModifyOrError(false))
	copytaxsetup.CopyTaxInfo(dst, "demo", [4]bool{true, true, true, true}, nil)
	if got := count(dst); got != want {
		t.Errorf("all: %v, demo %v", got, want)
	}
	mustOK(t, group.GetOrError(group.Code))
	if group.Description != "changed" {
		t.Errorf("an existing tax group was overwritten: %q", group.Description)
	}
}
