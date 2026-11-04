package tables

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// newProjectCompany: a company with the Project Nos. series PROJ, a resource and a project
// P00001 starting on 2026-03-01
func newProjectCompany(t *testing.T) ftables.Source {
	t.Helper()
	db := newTestDB(t)
	mustNoErr(t, CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)

	series := gtables.NewNoSeries(src)
	series.Code, series.Default_nos, series.Manual_nos = "PROJ", true, true
	mustNoErr(t, series.InsertOrError(false))
	line := gtables.NewNoSeriesLine(src)
	line.Series_code, line.Line_no, line.Starting_no, line.Open = "PROJ", 10000, "P00001", true
	mustNoErr(t, line.InsertOrError(false))
	setup := gtables.GetProjectsSetup(src)
	setup.Project_nos = "PROJ"
	mustNoErr(t, setup.InsertOrError(false))

	res := gtables.NewResource(src)
	res.No, res.Name, res.Resource_group_no = "R1", "Anna", "CONSULT"
	res.Direct_unit_cost, res.Unit_cost, res.Unit_price = types.NewDecimalFromInt(400), types.NewDecimalFromInt(500), types.NewDecimalFromInt(1000)
	mustNoErr(t, res.InsertOrError(false))

	project := gtables.NewProject(src)
	project.Init()
	project.Description = "Office relocation"
	mustNoErr(t, project.Validate("description", project.Description))
	project.Starting_date = types.NewDate(2026, 3, 1)
	mustNoErr(t, project.InsertOrError(true))
	if project.No != "P00001" || project.No_series != "PROJ" || project.Search_description != "OFFICE RELOCATION" {
		t.Fatalf("new project: %s %s %s", project.No, project.No_series, project.Search_description)
	}
	return src
}

func newBudgetLine(t *testing.T, src ftables.Source) *gtables.ProjectBudgetLine {
	t.Helper()
	l := gtables.NewProjectBudgetLine(src)
	l.Init()
	l.Project_no = "P00001"
	l.Type = gtables.ProjectBudgetLine_Type.Resource
	mustNoErr(t, l.Validate("no", types.Code("R1")))
	mustNoErr(t, l.InsertOrError(true))
	return l
}

// A quantity entered for a period is a budget entry with the difference, at the line's cost and
// price; the line starts on the project's starting date, a blank date filter budgets there
func TestProjectBudgetQuantity(t *testing.T) {
	src := newProjectCompany(t)
	l := newBudgetLine(t, src)
	if l.Description != "Anna" || !l.Unit_cost.Equal(types.NewDecimalFromInt(500)) || !l.Unit_price.Equal(types.NewDecimalFromInt(1000)) || l.Starting_date != types.NewDate(2026, 3, 1) {
		t.Fatalf("line from the resource: %+v", l)
	}

	l.Date_filter = "2026-04-01"
	mustNoErr(t, SetProjectBudgetLineQuantity(l, types.NewDecimalFromInt(10)))
	mustNoErr(t, SetProjectBudgetLineQuantity(l, types.NewDecimalFromInt(8))) // −2
	l.Date_filter = ""
	mustNoErr(t, SetProjectBudgetLineQuantity(l, types.NewDecimalFromInt(12))) // +4 on the starting date

	entries := gtables.NewProjectBudgetEntry(src)
	if n := entries.Count(); n != 3 {
		t.Fatalf("%d entries, want 3", n)
	}
	entries.SetRange("date", types.NewDate(2026, 3, 1))
	if !entries.FindFirst() || !entries.Quantity.Equal(types.NewDecimalFromInt(4)) || entries.Resource_group_no != "CONSULT" {
		t.Errorf("entry on the starting date: %+v", entries)
	}
	l.CalcFields("quantity", "total_cost", "total_price")
	if !l.Quantity.Equal(types.NewDecimalFromInt(12)) || !l.Total_cost.Equal(types.NewDecimalFromInt(6000)) || !l.Total_price.Equal(types.NewDecimalFromInt(12000)) {
		t.Errorf("line totals: %s %s %s", l.Quantity, l.Total_cost, l.Total_price)
	}

	// a new unit cost revalues the entries
	mustNoErr(t, l.Validate("unit_cost", types.NewDecimalFromInt(600)))
	if !l.Total_cost.Equal(types.NewDecimalFromInt(7200)) {
		t.Errorf("total cost after the new unit cost: %s", l.Total_cost)
	}

	project := gtables.NewProject(src)
	mustNoErr(t, project.GetOrError(types.Code("P00001")))
	project.CalcFields("budgeted_cost", "budgeted_price", "budgeted_res_qty")
	if !project.Budgeted_cost.Equal(types.NewDecimalFromInt(7200)) || !project.Budgeted_price.Equal(types.NewDecimalFromInt(12000)) || !project.Budgeted_res_qty.Equal(types.NewDecimalFromInt(12)) {
		t.Errorf("project budget: %s %s %s", project.Budgeted_cost, project.Budgeted_price, project.Budgeted_res_qty)
	}
	mustNoErr(t, l.SetFlowFilter("date_filter", "2026-04-01..2026-04-30"))
	l.CalcFields("quantity")
	if !l.Quantity.Equal(types.NewDecimalFromInt(8)) {
		t.Errorf("April quantity: %s", l.Quantity)
	}
	// lists and matrices calculate many lines at once, in the same filters
	rows := []map[string]interface{}{l.ToMap()}
	l.CalcFieldsForRecords(rows, "quantity")
	if rows[0]["quantity"] != "8" {
		t.Errorf("April quantity of the list: %v", rows[0]["quantity"])
	}

	// a budget line keeps its key
	if l.Rename("P00001", "", "", "", gtables.ProjectBudgetLine_Type.Resource, "R2", "") {
		t.Fatal("a budget line was renamed")
	}
	var appErr *apperrors.AppError
	if !errors.As(l.TriggerError(), &appErr) || appErr.Code != apperrors.ErrProjectBudgetLineRename {
		t.Errorf("rename: %v", l.TriggerError())
	}
}

// The status only moves on and goes to the budget entries; Completed sets the completion date;
// a project on Order is not deleted, otherwise its budget goes with it
func TestProjectStatusAndDelete(t *testing.T) {
	src := newProjectCompany(t)
	l := newBudgetLine(t, src)
	mustNoErr(t, SetProjectBudgetLineQuantity(l, types.NewDecimalFromInt(5)))

	project := gtables.NewProject(src)
	mustNoErr(t, project.GetOrError(types.Code("P00001")))
	mustNoErr(t, project.Validate("status", gtables.Project_Status.Order))
	mustNoErr(t, project.ModifyOrError(true))
	res := gtables.NewResource(src)
	mustNoErr(t, res.GetOrError(types.Code("R1")))
	res.CalcFields("qty_on_order_project", "qty_quoted_project")
	if !res.Qty_on_order_project.Equal(types.NewDecimalFromInt(5)) || !res.Qty_quoted_project.IsZero() {
		t.Errorf("resource on order %s, quoted %s", res.Qty_on_order_project, res.Qty_quoted_project)
	}

	err := project.Validate("status", gtables.Project_Status.Quote)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrProjectPreviousStatus {
		t.Errorf("back to Quote: %v", err)
	}
	project.Status = gtables.Project_Status.Order
	err = project.DeleteOrError(true)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrProjectDeleteStatus {
		t.Errorf("delete on Order: %v", err)
	}

	mustNoErr(t, project.Validate("status", gtables.Project_Status.Completed))
	if project.Completion_date.IsZero() {
		t.Error("no completion date")
	}
	mustNoErr(t, project.ModifyOrError(true))
	mustNoErr(t, project.DeleteOrError(true))
	if n := gtables.NewProjectBudgetLine(src).Count(); n != 0 {
		t.Errorf("%d budget lines left", n)
	}
	if n := gtables.NewProjectBudgetEntry(src).Count(); n != 0 {
		t.Errorf("%d budget entries left", n)
	}
}
