package handlers

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	_ "github.com/hansjlachmann/openerp/backend/business-logic/pages"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The budget matrix of G/L Budget by Periods (113): the accounts' budgeted amounts per month of
// the card's budget; a value typed into a cell enters a budget entry, the total account follows
func TestMatrix(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Skipf("pages: %v", err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	for _, a := range []struct {
		no, totaling string
		typ          gtables.GLAccountAccount_type
	}{{"6100", "", gtables.GLAccount_Account_type.Posting}, {"6200", "", gtables.GLAccount_Account_type.Posting},
		{"6999", "6100..6200", gtables.GLAccount_Account_type.Total}} {
		acc := gtables.NewGLAccount(src)
		acc.No, acc.Name, acc.Account_type, acc.Totaling = types.Code(a.no), types.Text("Account "+a.no), a.typ, types.Text(a.totaling)
		if err := acc.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []types.Code{"B1", "B2"} {
		budget := gtables.NewGLBudgetName(src)
		budget.Name = name
		if err := budget.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	h := NewPageTriggersHandler(db, database.DBTypeSQLite)
	app.Post("/api/pages/:id/matrix", h.Matrix)
	app.Post("/api/pages/:id/matrix/validate", h.MatrixValidate)

	request := func(budget string) string {
		return fmt.Sprintf(`"section": "Budget", "record": {"name": %q}, "view_by": 2, "view_as": 0, "date": "2026-01-15", "columns": 3`, budget)
	}
	matrix := func(budget string) (columns []interface{}, rows map[string][]interface{}) {
		t.Helper()
		status, out := sendJSON(t, app, "POST", "/api/pages/113/matrix", "{"+request(budget)+"}")
		if status != 200 {
			t.Fatalf("matrix: %d %v", status, out)
		}
		data := out["data"].(map[string]interface{})
		rows = map[string][]interface{}{}
		for _, r := range data["rows"].([]interface{}) {
			row := r.(map[string]interface{})
			rows[row["key"].(string)] = row["values"].([]interface{})
		}
		return data["columns"].([]interface{}), rows
	}
	enter := func(budget, key, start, end, value string, rounding int) (int, map[string]interface{}) {
		t.Helper()
		return sendJSON(t, app, "POST", "/api/pages/113/matrix/validate", fmt.Sprintf(`{%s, "key": %q, "start": %q, "end": %q, "value": %q, "rounding_factor": %d}`,
			request(budget), key, start, end, value, rounding))
	}

	columns, rows := matrix("B1")
	if len(columns) != 3 || columns[0].(map[string]interface{})["start"] != "2026-01-01" || columns[2].(map[string]interface{})["end"] != "2026-03-31" {
		t.Fatalf("columns: %v", columns)
	}
	if len(rows) != 3 || fmt.Sprint(rows["6100"][0]) != "0" {
		t.Fatalf("rows: %v", rows)
	}

	// typed into January and (in thousands) February; into a total account: refused
	if status, out := enter("B1", "6100", "2026-01-01", "2026-01-31", "1250.50", 0); status != 200 {
		t.Fatalf("enter January: %d %v", status, out)
	}
	if status, out := enter("B1", "6200", "2026-02-01", "2026-02-28", "2.5", 2); status != 200 {
		t.Fatalf("enter February: %d %v", status, out)
	}
	if status, _ := enter("B1", "6999", "2026-01-01", "2026-01-31", "1", 0); status != 400 {
		t.Errorf("a total account took a value: %d", status)
	}
	_, rows = matrix("B1")
	for _, c := range []struct {
		key  string
		col  int
		want string
	}{{"6100", 0, "1250.5"}, {"6200", 1, "2500"}, {"6999", 0, "1250.5"}, {"6999", 1, "2500"}, {"6999", 2, "0"}} {
		got, _ := types.NewDecimalFromString(fmt.Sprint(rows[c.key][c.col]))
		want, _ := types.NewDecimalFromString(c.want)
		if !got.Equal(want) {
			t.Errorf("%s column %d: %v, want %s", c.key, c.col, rows[c.key][c.col], c.want)
		}
	}
	// the other budget has none of it
	_, rows = matrix("B2")
	if fmt.Sprint(rows["6999"][0]) != "0" {
		t.Errorf("budget B2 shows %v", rows["6999"][0])
	}
	if n := gtables.NewGLBudgetEntry(src).Count(); n != 2 {
		t.Errorf("%d budget entries, want 2", n)
	}
}
