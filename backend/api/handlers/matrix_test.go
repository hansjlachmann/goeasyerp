package handlers

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
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

// The overview of an account schedule (490): a calculated matrix — the schedule's lines against
// its column layout's columns, for the period chosen (View by, previous/next)
func TestCalculatedMatrix(t *testing.T) {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	acc := gtables.NewGLAccount(src)
	acc.No = "3100"
	must(acc.InsertOrError(false))
	entry := gtables.NewGLEntry(src)
	entry.Entry_no, entry.Gl_account_no, entry.Posting_date, entry.Amount = 1, "3100", types.NewDate(2026, 3, 10), types.NewDecimalFromInt(-1000)
	must(entry.InsertOrError(false))
	layout := gtables.NewAccSchedColumnLayoutName(src)
	layout.Name = "L"
	must(layout.InsertOrError(true))
	column := gtables.NewAccSchedColumn(src)
	column.InitRecord()
	column.Column_layout_name, column.Line_no, column.Column_no, column.Column_header = "L", 10000, "A", "Actual"
	must(column.InsertOrError(true))
	name := gtables.NewAccScheduleName(src)
	name.Name, name.Default_column_layout = "S", "L"
	must(name.InsertOrError(true))
	line := gtables.NewAccScheduleLine(src)
	line.InitRecord()
	line.Schedule_name, line.Line_no, line.Row_no, line.Description, line.Totaling = "S", 10000, "R10", "Revenue", "3100"
	line.Sign_control = gtables.AccScheduleLine_Sign_control.Reverse
	must(line.InsertOrError(true))

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/pages/:id/matrix", NewPageTriggersHandler(db, database.DBTypeSQLite).Matrix)
	overview := func(step int) (columns []interface{}, rows []interface{}, period map[string]interface{}) {
		t.Helper()
		status, out := sendJSON(t, app, "POST", "/api/pages/490/matrix", fmt.Sprintf(
			`{"section": "Overview", "record": {"name": "S", "default_column_layout": "L"}, "view_by": 2, "date": "2026-03-15", "step": %d}`, step))
		if status != 200 {
			t.Fatalf("overview: %d %v", status, out)
		}
		data := out["data"].(map[string]interface{})
		return data["columns"].([]interface{}), data["rows"].([]interface{}), data["period"].(map[string]interface{})
	}
	columns, rows, period := overview(0)
	if len(columns) != 1 || columns[0].(map[string]interface{})["name"] != "Actual" || period["start"] != "2026-03-01" {
		t.Fatalf("columns %v, period %v", columns, period)
	}
	row := rows[0].(map[string]interface{})
	if row["key"] != "S,10000" || row["fields"].(map[string]interface{})["description"] != "Revenue" {
		t.Fatalf("row %v", row)
	}
	if v := row["values"].([]interface{})[0]; v != "1000" {
		t.Errorf("March: %v, want 1000", v)
	}
	_, rows, period = overview(-1)
	if v := rows[0].(map[string]interface{})["values"].([]interface{})[0]; v != "" || period["start"] != "2026-02-01" {
		t.Errorf("February: %v (%v), want blank", v, period)
	}
}

// A matrix page (Resource Capacity 213): the matrix of the page itself (section ""), a capacity
// typed into a day enters a capacity entry
func TestMatrixPage(t *testing.T) {
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
	res := gtables.NewResource(src)
	res.No, res.Name = "R1", "Resource 1"
	if err := res.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	h := NewPageTriggersHandler(db, database.DBTypeSQLite)
	app.Post("/api/pages/:id/matrix", h.Matrix)
	app.Post("/api/pages/:id/matrix/validate", h.MatrixValidate)
	request := `"section": "", "record": {}, "view_by": 0, "view_as": 0, "date": "2026-03-02", "columns": 5`
	status, out := sendJSON(t, app, "POST", "/api/pages/213/matrix/validate", `{`+request+`, "key": "R1", "start": "2026-03-03", "end": "2026-03-03", "value": "7.5"}`)
	if status != 200 {
		t.Fatalf("enter capacity: %d %v", status, out)
	}
	status, out = sendJSON(t, app, "POST", "/api/pages/213/matrix", `{`+request+`}`)
	if status != 200 {
		t.Fatalf("matrix: %d %v", status, out)
	}
	row := out["data"].(map[string]interface{})["rows"].([]interface{})[0].(map[string]interface{})
	values := row["values"].([]interface{})
	got, _ := types.NewDecimalFromString(fmt.Sprint(values[1]))
	if row["key"] != "R1" || !got.Equal(types.MustDecimal("7.5")) || fmt.Sprint(values[0]) != "0" {
		t.Errorf("row %v", row)
	}
}

// Items by Bin (5411): a matrix page with a column per bin of the Location Filter and the items'
// quantity on hand in each bin
func TestItemsByBin(t *testing.T) {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, no := range []string{"I1", "I2"} {
		item := gtables.NewItem(src)
		item.No = types.Code(no)
		must(item.InsertOrError(false))
	}
	for _, b := range [][2]string{{"MAIN", "A"}, {"MAIN", "B"}, {"OSLO", "S"}} {
		bin := gtables.NewBin(src)
		bin.Location_code, bin.Code = types.Code(b[0]), types.Code(b[1])
		must(bin.InsertOrError(false))
	}
	for i, e := range []struct {
		item, location, bin string
		qty                 int64
	}{{"I1", "MAIN", "A", 5}, {"I1", "MAIN", "A", -2}, {"I1", "MAIN", "B", 4}, {"I2", "OSLO", "S", 7}, {"I2", "MAIN", "", 9}} {
		entry := gtables.NewItemLedgerEntry(src)
		entry.Entry_no, entry.Item_no, entry.Location_code, entry.Bin_code = i+1, types.Code(e.item), types.Code(e.location), types.Code(e.bin)
		entry.Quantity = types.NewDecimalFromInt(e.qty)
		must(entry.InsertOrError(false))
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/pages/:id/matrix", NewPageTriggersHandler(db, database.DBTypeSQLite).Matrix)
	status, out := sendJSON(t, app, "POST", "/api/pages/5411/matrix",
		`{"section": "", "record": {}, "filters": [{"field": "location_filter", "expression": "MAIN"}], "view_by": 0}`)
	if status != 200 {
		t.Fatalf("items by bin: %d %v", status, out)
	}
	data := out["data"].(map[string]interface{})
	columns := data["columns"].([]interface{})
	if len(columns) != 2 || columns[0].(map[string]interface{})["name"] != "MAIN, A" || columns[1].(map[string]interface{})["name"] != "MAIN, B" {
		t.Fatalf("columns %v", columns)
	}
	values := map[string]string{}
	for _, r := range data["rows"].([]interface{}) {
		row := r.(map[string]interface{})
		values[fmt.Sprint(row["key"])] = fmt.Sprint(row["values"])
	}
	if values["I1"] != "[3 4]" || values["I2"] != "[ ]" {
		t.Errorf("cells %v, want I1 [3 4], I2 blank", values)
	}
}

// Items by Location (491): a column per location of the Location Filter, a cell the item's
// quantity on hand at the location
func TestItemsByLocation(t *testing.T) {
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, no := range []string{"I1", "I2"} {
		item := gtables.NewItem(src)
		item.No = types.Code(no)
		must(item.InsertOrError(false))
	}
	for _, code := range []string{"MAIN", "OSLO", "WEST"} {
		location := gtables.NewLocation(src)
		location.Code = types.Code(code)
		must(location.InsertOrError(false))
	}
	for i, e := range []struct {
		item, location string
		qty            int64
	}{{"I1", "MAIN", 5}, {"I1", "MAIN", -2}, {"I1", "OSLO", 4}, {"I2", "WEST", 7}, {"I2", "", 9}} {
		entry := gtables.NewItemLedgerEntry(src)
		entry.Entry_no, entry.Item_no, entry.Location_code = i+1, types.Code(e.item), types.Code(e.location)
		entry.Quantity = types.NewDecimalFromInt(e.qty)
		must(entry.InsertOrError(false))
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/pages/:id/matrix", NewPageTriggersHandler(db, database.DBTypeSQLite).Matrix)
	status, out := sendJSON(t, app, "POST", "/api/pages/491/matrix",
		`{"section": "", "record": {}, "filters": [{"field": "location_filter", "expression": "MAIN|OSLO"}], "view_by": 0}`)
	if status != 200 {
		t.Fatalf("items by location: %d %v", status, out)
	}
	data := out["data"].(map[string]interface{})
	columns := data["columns"].([]interface{})
	if len(columns) != 2 || columns[0].(map[string]interface{})["name"] != "MAIN" || columns[1].(map[string]interface{})["name"] != "OSLO" {
		t.Fatalf("columns %v", columns)
	}
	values := map[string]string{}
	for _, r := range data["rows"].([]interface{}) {
		row := r.(map[string]interface{})
		values[fmt.Sprint(row["key"])] = fmt.Sprint(row["values"])
	}
	if values["I1"] != "[3 4]" || values["I2"] != "[ ]" {
		t.Errorf("cells %v, want I1 [3 4], I2 blank", values)
	}
}
