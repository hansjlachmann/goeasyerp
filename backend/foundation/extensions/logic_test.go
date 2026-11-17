package extensions_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appLogic: an app's Customer logic — OnBeforeModify takes over (handled) for customer SKIP,
// OnModify marks the customer
type appLogic struct{}

func (appLogic) OnBeforeModify(rec *gtables.Customer, handled *bool) error {
	if rec.No == "SKIP" {
		*handled = true
	}
	return nil
}

func (appLogic) OnModify(rec *gtables.Customer) error {
	rec.Name_2 = "app ran"
	return nil
}

// The logic an app registers runs while the app is enabled; OnBefore… with handled skips the
// triggers of that kind
func TestAppLogic(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "c", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c", database.DBTypeSQLite)
	extensions.Install("app-logic", func() { gtables.RegisterCustomerLogic(appLogic{}) })
	t.Cleanup(func() { extensions.SetDisabled() })

	modify := func(no string) string {
		t.Helper()
		cust := gtables.NewCustomer(src)
		if !cust.Get(no) {
			cust.No = types.Code(no)
			if !cust.Insert(false) {
				t.Fatalf("insert %s", no)
			}
		}
		cust.Name_2 = ""
		if !cust.Modify(true) {
			t.Fatalf("modify %s: %v", no, cust.TriggerError())
		}
		return string(cust.Name_2)
	}
	if got := modify("C1"); got != "app ran" {
		t.Errorf("enabled app: Name 2 %q", got)
	}
	if got := modify("SKIP"); got != "" {
		t.Errorf("handled: OnModify ran (%q)", got)
	}
	extensions.SetDisabled("app-logic")
	if extensions.Enabled("app-logic") || !extensions.Enabled("") {
		t.Error("Enabled")
	}
	if got := modify("C1"); got != "" {
		t.Errorf("disabled app: OnModify ran (%q)", got)
	}
}
