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
