package setupchecklist_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/setupchecklist"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// twoCompanies: the demo company and an empty one, the objects synced, in a transaction
func twoCompanies(t *testing.T) (demo, empty ftables.Source) {
	t.Helper()
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
	mustOK(t, (&gtables.Object{}).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	mustOK(t, (&gtables.ObjectTranslation{}).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	mustOK(t, apps.SyncObjects(db, database.DBTypeSQLite))
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite), ftables.DBSource(tx, "new", database.DBTypeSQLite)
}

// The checklist has a line per setup table of this installation in its order under its area
// (areas without tables left out); made again it keeps the status the user set
func TestUpdateChecklist(t *testing.T) {
	demo, _ := twoCompanies(t)
	mustOK(t, setupchecklist.UpdateChecklist(demo))
	line := gtables.NewSetupChecklistLine(demo)
	n := line.Count()
	if n < 60 {
		t.Fatalf("%d checklist lines", n)
	}
	if !line.FindFirst() || line.Table_id != gtables.CompanyInformationTableID || line.Page_id != 1 || line.Page_name == "" || line.Application_area_id != gtables.GLAccountTableID {
		t.Errorf("first line %+v", line.ToMap())
	}
	line.CalcFields("application_area_name")
	if line.Application_area_name != "General Ledger" {
		t.Errorf("area %q", line.Application_area_name)
	}
	line.SetRange("table_id", gtables.GLAccountTableID)
	if !line.FindFirst() || !line.Copying_available {
		t.Errorf("G/L Account line: copying available %v", line.Copying_available)
	}
	if setupchecklist.NoOfRecords(demo, demo.GetCompany(), gtables.GLAccountTableID) == 0 {
		t.Error("no G/L accounts counted")
	}
	line.Status = gtables.SetupChecklistLine_Status.Completed
	line.Responsible_id = "ANNA"
	mustOK(t, line.ModifyOrError(false))

	area := gtables.NewApplicationAreaLine(demo)
	if !area.GetByPK(gtables.FixedAssetTableID) || !area.GetByPK(gtables.ProjectTableID) || area.Name != "Projects" {
		t.Error("Fixed Assets or Projects area missing")
	}

	mustOK(t, setupchecklist.UpdateChecklist(demo))
	again := gtables.NewSetupChecklistLine(demo)
	if again.Count() != n {
		t.Errorf("%d lines after the second update, %d before", again.Count(), n)
	}
	again.SetRange("table_id", gtables.GLAccountTableID)
	if !again.FindFirst() || again.Status != gtables.SetupChecklistLine_Status.Completed || again.Responsible_id != "ANNA" {
		t.Errorf("status lost: %+v", again.ToMap())
	}
}

// Copy Data fills an empty table (and the tables that go with it) from the other company; a
// table with data is refused
func TestCopyTableData(t *testing.T) {
	demo, empty := twoCompanies(t)
	mustOK(t, setupchecklist.UpdateChecklist(empty))
	line := gtables.NewSetupChecklistLine(empty)
	line.SetRange("table_id", gtables.NoSeriesTableID)
	if !line.FindFirst() {
		t.Fatal("no No. Series line")
	}
	mustOK(t, setupchecklist.CopyTableData("demo", line))
	for _, id := range []int{gtables.NoSeriesTableID, gtables.NoSeriesLineTableID} {
		if got, want := setupchecklist.NoOfRecords(empty, "new", id), setupchecklist.NoOfRecords(demo, "demo", id); got != want || got == 0 {
			t.Errorf("table %d: %d records copied, %d in the demo", id, got, want)
		}
	}
	if err := setupchecklist.CopyTableData("demo", line); err == nil {
		t.Error("copied into a table with data")
	}
	if err := setupchecklist.CopyTableData("new", line); err == nil {
		t.Error("copied from the company itself")
	}
}
