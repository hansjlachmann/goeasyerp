package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Every ledger entry and register table is posted (written by posting only); the tables that
// are entries by name but entered by users are listed
func TestEntryTablesArePosted(t *testing.T) {
	notPosted := map[string]bool{
		"Job_Queue_Entry":    true, // the job queue's jobs
		"Document_Entry":     true, // Navigate's temporary lines
		"Res_Capacity_Entry": true, // a resource's capacity, set by the user
		"G_L_Budget_Entry":   true, // the budgets' amounts, entered in the budget matrix
	}
	metadata := pages.GetTableMetadata()
	for _, name := range gtables.ListTableNames() {
		key := strings.ToLower(name)
		isEntry := strings.HasSuffix(key, "entry") || strings.HasSuffix(key, "register")
		if isEntry && !notPosted[name] && !metadata.IsPosted(name) {
			t.Errorf("%s: an entry table must be posted: true (or listed as entered by users here)", name)
		}
		if notPosted[name] && metadata.IsPosted(name) {
			t.Errorf("%s is entered by users and must not be posted", name)
		}
	}
}

// The table API does not insert, delete or rename posted records, for any user, and changes only
// their editable: true fields
func TestPostedTablesAPI(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GLEntry{}, &gtables.CustomerLedgerEntry{}} {
		if err := rec.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	glEntry := gtables.NewGLEntry(src)
	glEntry.Entry_no = 1
	glEntry.Gl_account_no = "1210"
	glEntry.Amount = types.MustDecimal("100")
	glEntry.Description = "Posted"
	if err := glEntry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	custLedgEntry.Entry_no = 1
	custLedgEntry.Customer_no = "C00010"
	custLedgEntry.Description = "Invoice"
	if err := custLedgEntry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}

	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// SUPER may do everything, but posted records are still written by posting only
		c.Locals("session", &session.Session{UserID: "ADMIN", Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Post("/api/tables/:table/insert-many", h.InsertRecords)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Delete("/api/tables/:table/delete/:id", h.DeleteRecord)
	app.Delete("/api/tables/:table/delete-many", h.DeleteRecords)
	app.Post("/api/tables/:table/validate", h.ValidateField)

	refused := func(what string, status int, out map[string]interface{}) {
		t.Helper()
		msg, _ := out["error"].(string)
		if status != 403 || !strings.Contains(msg, "written by posting only") {
			t.Errorf("%s: %d %q, want refused", what, status, msg)
		}
	}
	status, out := sendJSON(t, app, "POST", "/api/tables/G_L_Entry/insert", `{"entry_no":2,"gl_account_no":"1210","amount":"-100"}`)
	refused("insert", status, out)
	status, out = sendJSON(t, app, "POST", "/api/tables/G_L_Entry/insert-many", `{"records":[{"entry_no":3,"amount":"5"}]}`)
	if status == 200 {
		t.Errorf("insert-many: %d %v", status, out)
	}
	status, out = sendJSON(t, app, "DELETE", "/api/tables/G_L_Entry/delete/1", ``)
	refused("delete", status, out)
	status, out = sendJSON(t, app, "DELETE", "/api/tables/G_L_Entry/delete-many", `{"keys":["1"]}`)
	refused("delete-many", status, out)
	if n := gtables.NewGLEntry(src).Count(); n != 1 {
		t.Fatalf("%d G/L entries, want the one posted", n)
	}

	// a modify leaves the posted fields and the key as they are
	_, _ = sendJSON(t, app, "PUT", "/api/tables/G_L_Entry/modify/1", `{"entry_no":7,"amount":"999","description":"Changed"}`)
	glEntry = gtables.NewGLEntry(src)
	if !glEntry.Get(1) || !glEntry.Amount.Equal(types.MustDecimal("100")) || glEntry.Description != "Posted" {
		t.Errorf("a posted G/L entry changed: %+v", glEntry)
	}
	status, out = sendJSON(t, app, "POST", "/api/tables/G_L_Entry/validate", `{"field":"amount","value":"5","record":{"entry_no":1}}`)
	if status != 400 || !strings.Contains(out["error"].(string), "cannot be changed") {
		t.Errorf("validate a posted field: %d %v", status, out["error"])
	}

	// a customer entry's editable fields change, the others not
	status, out = sendJSON(t, app, "PUT", "/api/tables/Customer_ledger_entry/modify/1", `{"due_date":"2026-11-30","on_hold":"X","description":"Changed","customer_no":"C00020"}`)
	if status != 200 {
		t.Fatalf("modify a customer entry: %d %v", status, out["error"])
	}
	custLedgEntry = gtables.NewCustomerLedgerEntry(src)
	if !custLedgEntry.Get(1) {
		t.Fatal("customer entry gone")
	}
	if custLedgEntry.Due_date.String() != "2026-11-30" || custLedgEntry.On_hold != "X" {
		t.Errorf("the editable fields were not saved: due date %s, on hold %q", custLedgEntry.Due_date, custLedgEntry.On_hold)
	}
	if custLedgEntry.Description != "Invoice" || custLedgEntry.Customer_no != "C00010" {
		t.Errorf("posted fields changed: %q %q", custLedgEntry.Description, custLedgEntry.Customer_no)
	}
}

// The pages of posted tables have no New/Delete and show the posted fields read-only
func TestPostedTablePages(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/pages/:id", NewPagesHandler().GetPage)
	get := func(id string) pages.PageDefinition {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest("GET", "/api/pages/"+id, nil))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Data pages.PageDefinition `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, raw)
		}
		return out.Data
	}

	bank := get("372") // Bank Account Ledger Entries
	if !bank.Page.Posted {
		t.Error("page 372 must say posted")
	}
	for _, action := range bank.Page.Actions {
		if action.Name == "New" || action.Name == "Delete" {
			t.Errorf("page 372 has %s", action.Name)
		}
	}
	for _, f := range bank.Page.Layout.Repeater.Fields {
		if f.Editable == nil || *f.Editable {
			t.Errorf("page 372: posted field %s is editable", f.Source)
		}
	}
	editable := map[string]bool{}
	for _, f := range get("25").Page.Layout.Repeater.Fields { // Customer Ledger Entries
		editable[f.Source] = f.Editable == nil || *f.Editable
	}
	if !editable["due_date"] || editable["description"] || editable["posting_date"] {
		t.Errorf("page 25 editable fields: %v", editable)
	}
}
