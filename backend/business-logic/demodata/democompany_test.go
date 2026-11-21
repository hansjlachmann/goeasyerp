package demodata

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insertCompanyRecord makes the company's Company record (the global table too, when missing),
// not a demo company
func insertCompanyRecord(t *testing.T, db *sql.DB, name string, dbType database.DBType) {
	t.Helper()
	companyRec := gtables.NewCompany(ftables.DBSource(db, "", dbType))
	if err := companyRec.CreateTableWithDBType(db, "", dbType); err != nil {
		t.Fatal(err)
	}
	if dbType == database.DBTypePostgres {
		if _, err := db.Exec(`ALTER TABLE "Company" ADD COLUMN IF NOT EXISTS demo_company BOOLEAN DEFAULT false`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM "Company" WHERE name = $1`, name) })
	}
	companyRec.Name = types.Text(name)
	if err := companyRec.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
}

// The demo marks its company a demo company; a company with customers that is not one is never
// emptied — its data may be real
func TestCreateRefusesACompanyWithRealData(t *testing.T) {
	db := newTestDB(t)
	insertCompanyRecord(t, db, testCompany, database.DBTypeSQLite)
	cust := gtables.NewCustomer(ftables.DBSource(db, testCompany, database.DBTypeSQLite))
	cust.No = "REAL1"
	cust.Name = "A real customer"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}

	_, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today})
	var notDemo *NotDemoCompanyError
	if !errors.As(err, &notDemo) {
		t.Fatalf("Create in a company with real data: %v, want NotDemoCompanyError", err)
	}
	if !gtables.NewCustomer(ftables.DBSource(db, testCompany, database.DBTypeSQLite)).Get("REAL1") {
		t.Fatal("the real customer was deleted")
	}
	if IsDemoCompany(db, testCompany, database.DBTypeSQLite) {
		t.Fatal("the refused company became a demo company")
	}

	// an empty company becomes a demo company, which a new run may replace
	if _, err := db.Exec(`DELETE FROM "` + testCompany + `$Customer"`); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatal(err)
	}
	if !IsDemoCompany(db, testCompany, database.DBTypeSQLite) {
		t.Fatal("the demo did not mark its company")
	}
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("a demo company refused: %v", err)
	}
}
