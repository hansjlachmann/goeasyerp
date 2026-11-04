package pages

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func projectDemo(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

// Pages 227 and 225: in a period the budget line's resource capacity, its quantity on projects on
// Order and Quote and the capacity left after them
func TestProjectResourceAvailability(t *testing.T) {
	src := projectDemo(t)
	line := gtables.NewProjectBudgetLine(src)
	line.SetRange("type", gtables.ProjectBudgetLine_Type.Resource)
	line.SetRange("no", types.Code("R0010"))
	if !line.FindFirst() {
		t.Fatal("no budget line of R0010")
	}
	period := "2026-10-01..2026-10-31"
	if err := line.SetFlowFilter("date_filter", period); err != nil {
		t.Fatal(err)
	}
	line.CalcFields("quantity", "qty_on_order_project", "qty_quoted_project")
	page, err := pages.RunAfterGetRecordControl(227, line)
	if err != nil {
		t.Fatal(err)
	}
	res := gtables.NewResource(src)
	res.No = "R0010"
	_ = res.SetFlowFilter("date_filter", period)
	res.CalcFields("capacity", "qty_on_order_project", "qty_quoted_project")
	values := page.Values()
	afterOrders := res.Capacity.Sub(line.Qty_on_order_project)
	if !decimalValue(values["capacity"]).Equal(res.Capacity) || !decimalValue(values["capacity_after_orders"]).Equal(afterOrders) ||
		!decimalValue(values["capacity_after_quotes"]).Equal(afterOrders.Sub(line.Qty_quoted_project)) {
		t.Errorf("227: %v (capacity %s, on order %s, quoted %s)", values, res.Capacity, line.Qty_on_order_project, line.Qty_quoted_project)
	}
	if line.Qty_on_order_project.IsZero() || res.Capacity.IsZero() {
		t.Errorf("the demo has no order quantity (%s) or capacity (%s) in October", line.Qty_on_order_project, res.Capacity)
	}

	page, err = pages.RunAfterGetRecordControl(225, res)
	if err != nil {
		t.Fatal(err)
	}
	if !decimalValue(page.Values()["capacity_after_orders"]).Equal(res.Capacity.Sub(res.Qty_on_order_project)) {
		t.Errorf("225: %v", page.Values())
	}
}

// decimalValue: a calculated field's value (decimals are kept as text)
func decimalValue(v interface{}) types.Decimal {
	s, _ := v.(string)
	d, _ := types.NewDecimalFromString(s)
	return d
}

// Change Budget Dates (form 366): the line's budget entries and its Starting Date move by the
// date formula; the line's quantity stays
func TestChangeProjectBudgetDates(t *testing.T) {
	src := projectDemo(t)
	line := gtables.NewProjectBudgetLine(src)
	line.SetRange("type", gtables.ProjectBudgetLine_Type.Resource)
	if !line.FindFirst() {
		t.Fatal("no budget line")
	}
	line.CalcFields("quantity")
	quantity, starting := line.Quantity, line.Starting_date
	entries := func() []types.Date {
		e := gtables.NewProjectBudgetEntry(src)
		e.SetRange("project_no", line.Project_no)
		e.SetRange("phase_code", line.Phase_code)
		e.SetRange("task_code", line.Task_code)
		e.SetRange("step_code", line.Step_code)
		e.SetRange("type", line.Type)
		e.SetRange("no", line.No)
		var dates []types.Date
		for found := e.FindSet(); found; found = e.Next() {
			dates = append(dates, e.Date)
		}
		return dates
	}
	before := entries()
	if len(before) == 0 {
		t.Fatal("no budget entries")
	}
	if err := tables.ChangeProjectBudgetDates(src, line, "1M"); err != nil {
		t.Fatal(err)
	}
	after := entries()
	for i := range before {
		if after[i] != before[i].AddMonths(1) {
			t.Errorf("entry %d: %s, want %s", i, after[i], before[i].AddMonths(1))
		}
	}
	changed := gtables.NewProjectBudgetLine(src)
	if err := changed.GetOrError(map[string]interface{}{"project_no": line.Project_no, "phase_code": line.Phase_code, "task_code": line.Task_code, "step_code": line.Step_code, "type": line.Type, "no": line.No, "variant_code": line.Variant_code}); err != nil {
		t.Fatal(err)
	}
	changed.CalcFields("quantity")
	if !changed.Quantity.Equal(quantity) || (!starting.IsZero() && changed.Starting_date != starting.AddMonths(1)) {
		t.Errorf("line: quantity %s (was %s), starting date %s (was %s)", changed.Quantity, quantity, changed.Starting_date, starting)
	}
}
