package handlers

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A new line saved before its quantity is typed: the insert validates only the user's
// changes — the values its No. filled in (direct and unit cost) are not validated again, so
// the checks meant for typing them (Unit Cost (LCY) needs a quantity) do not refuse the line
func TestInsertLineValidatesOnlyUserChanges(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, "TEST", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	order := gtables.NewPurchaseHeader(src)
	order.Document_type = gtables.PurchaseHeader_Document_type.Order
	if err := order.InsertOrError(true); err != nil {
		t.Fatal(err)
	}
	if err := order.Validate("buy_from_vendor_no", "V00010"); err != nil {
		t.Fatal(err)
	}
	if err := order.ModifyOrError(true); err != nil {
		t.Fatal(err)
	}

	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)

	// as the subform sends it after the No. was validated: the costs the No. gave, no quantity yet
	body := `{"document_type":1,"document_no":"` + order.No.String() + `","line_no":10000,"type":2,"no":"1000","quantity":"0",` +
		`"direct_unit_cost":"45","unit_cost_lcy":"45","unit_of_measure_code":"PACK","description":"Copy paper A4, 500 sheets"}`
	status, out := sendJSON(t, app, "POST", "/api/tables/Purchase_Line/insert?page_id=54", body)
	if status != 200 {
		t.Fatalf("insert a line without quantity: %d %v", status, out["error"])
	}

	// a cost the user typed is still validated (its checks run)
	body = strings.Replace(body, `"line_no":10000`, `"line_no":20000`, 1)
	body = strings.Replace(body, `"unit_cost_lcy":"45"`, `"unit_cost_lcy":"50"`, 1)
	status, out = sendJSON(t, app, "POST", "/api/tables/Purchase_Line/insert?page_id=54", body)
	if status == 200 || !strings.Contains(out["error"].(string), "Quantity") {
		t.Errorf("a typed unit cost without quantity: %d %v", status, out["error"])
	}
}
