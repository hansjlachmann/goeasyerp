package reports_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectCompany: the demo company with the project on Order's posted usage and invoice
func projectCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:              time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine:    func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:          func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		ProjectJnlPostLine: func(src ftables.Source) demodata.ProjectJnlPoster { return projectjnl.NewProjectJnlPostLine(src) },
	})
	mustOK(t, err)
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

// The project reports print the demo projects: their lists, the register of the usage posted,
// the customers' projects with the totals per customer, the budgets by phase/task/step with
// group totals and the actual usage against them
func TestProjectReports(t *testing.T) {
	src := projectCompany(t)
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	project.CalcFields("usage_cost", "invoiced_price", "budgeted_cost")
	if project.Usage_cost.IsZero() || project.Invoiced_price.IsZero() {
		t.Fatalf("the demo project has no usage or invoicing: %s %s", project.Usage_cost, project.Invoiced_price)
	}
	contains := func(id int, wants ...string) {
		t.Helper()
		texts := runList(t, src, id, nil)
		for _, want := range wants {
			if !strings.Contains(texts, want) {
				t.Errorf("report %d does not print %q", id, want)
			}
		}
	}
	contains(reports.ProjectListID, "Project List", project.No.String(), project.Description.String(), amountText(project.Budgeted_cost))
	contains(reports.ProjectRegisterID, "Project Register", "Register No: 1", project.No.String(), "R0010")
	contains(reports.CustomerProjectsCostID, "Customer Projects - Cost", project.No.String(), amountText(project.Usage_cost), "Total for", "Report Total")
	contains(reports.CustomerProjectsPriceID, "Customer Projects - Price", amountText(project.Invoiced_price))
	contains(reports.ProjectCostSuggestedBillingID, "Project Cost Suggested Billing", "Suggested Billing")
	contains(reports.ProjectCostBudgetID, "Project Cost Budget for Project: "+project.No.String(), "PLAN", "Planning", "Total Planning", "Total for Project No. "+project.No.String())
	contains(reports.ActualToBudgetCostID, "Actual Cost to Budget Cost for Project "+project.No.String(), "Total Execution", amountText(project.Usage_cost))
	contains(reports.ActualToBudgetPriceID, "Actual Price to Budget Price for Project "+project.No.String())
	contains(reports.ProjectCostTransactionDetail, "Project Cost Transaction Detail", "Total Usage for", "Total Sales for")

	completed := gtables.NewProject(src)
	completed.SetRange("status", gtables.Project_Status.Completed)
	if !completed.FindFirst() {
		t.Fatal("no completed demo project")
	}
	contains(reports.CompletedProjectsID, "Completed Projects", completed.No.String(), "Report Total")
}

// Report 1202 run from a project journal line prints its batch's lines and, under a line, what
// posting would refuse: a blocked resource, a project not on Order, a missing number
func TestProjectJournalTestReport(t *testing.T) {
	src := projectCompany(t)
	order := gtables.NewProject(src)
	order.SetRange("status", gtables.Project_Status.Order)
	quote := gtables.NewProject(src)
	quote.SetRange("status", gtables.Project_Status.Quote)
	if !order.FindFirst() || !quote.FindFirst() {
		t.Fatal("no demo projects on Order and Quote")
	}
	res := gtables.NewResource(src)
	mustOK(t, res.GetOrError("R0040"))
	res.Blocked = true
	mustOK(t, res.ModifyOrError(false))
	lines := []struct {
		projectNo types.Code
		no        types.Code
	}{{order.No, "R0010"}, {order.No, "R0040"}, {quote.No, "R0010"}, {order.No, ""}}
	var first *gtables.ProjectJournalLine
	for i, l := range lines {
		line := gtables.NewProjectJournalLine(src)
		line.Init()
		line.Journal_template_name, line.Journal_batch_name, line.Line_no = "PROJECT", "DEFAULT", (i+1)*10000
		line.Posting_date = types.NewDate(2026, 10, 5)
		line.Document_no = "T1"
		line.Project_no = l.projectNo
		line.Type = gtables.ProjectJournalLine_Type.Resource
		line.No = l.no
		line.Gen_prod_posting_group = "SERVICES"
		line.Quantity = types.MustDecimal("1")
		mustOK(t, line.InsertOrError(false))
		if first == nil {
			first = line
		}
	}
	result, err := reports.Execute(reports.ProjectJnlTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Context: first})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	for _, want := range []string{"Project Journal - Test", "PROJECT", "DEFAULT", "Warning!",
		"Blocked must be No for resource R0040.", "Status must be Order for project " + quote.No.String() + ".", "No. must be specified."} {
		if !strings.Contains(texts, want) {
			t.Errorf("report 1202 does not print %q:\n%s", want, texts)
		}
	}
	if strings.Count(texts, "Warning!") != 3 {
		t.Errorf("%d warnings, want 3:\n%s", strings.Count(texts, "Warning!"), texts)
	}
}
