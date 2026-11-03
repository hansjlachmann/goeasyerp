package reports_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A project's usage at cost: Calculate Project WIP Value (1295) sets the amount to post, Post
// Project WIP Value to G/L (1214) moves it from the cost adjustment account to WIP Costs;
// Calculate Project Recognition (1294) and Post Project Recognition to G/L (1213) move it on
// from WIP Costs to the posting group's Cost Account; a second run posts nothing
func TestProjectWIPAndRecognition(t *testing.T) {
	src := listCompany(t)
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	project.Project_usage_posting = gtables.Project_Project_usage_posting.Costs
	mustOK(t, project.ModifyOrError(false))

	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	if !genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": types.Code(""), "gen_prod_posting_group": types.Code("SERVICES")}) {
		genPostingSetup.Gen_prod_posting_group = "SERVICES"
		mustOK(t, genPostingSetup.InsertOrError(false))
	}
	genPostingSetup.Project_cost_adjmt_account = "4140"
	genPostingSetup.Project_sales_adjmt_account = "3150"
	mustOK(t, genPostingSetup.ModifyOrError(false))

	line := gtables.NewProjectJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name = "PROJECT", "DEFAULT"
	line.Posting_date = system.WorkDate()
	line.Document_no = "WIPTEST"
	line.Source_code = "PROJJNL"
	mustOK(t, line.Validate("project_no", project.No))
	mustOK(t, line.Validate("no", types.Code("R0010")))
	mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(4)))
	mustOK(t, projectjnl.NewProjectJnlPostLine(src).Run(line))
	cost := line.Total_cost

	run := func(id int, options map[string]string) {
		t.Helper()
		_, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
		mustOK(t, err)
	}
	postOptions := func(docNo string) map[string]string {
		return map[string]string{"posting_method": "2", "document_no": docNo, "posting_date": system.WorkDate().String(), "posting_description": "%1 %2"}
	}
	glAmount := func(docNo, accNo string) types.Decimal {
		t.Helper()
		glEntry := gtables.NewGLEntry(src)
		glEntry.SetRange("document_no", types.Code(docNo))
		glEntry.SetRange("gl_account_no", types.Code(accNo))
		mustOK(t, glEntry.CalcSums("amount"))
		return glEntry.Amount
	}
	entry := gtables.NewProjectLedgerEntry(src)
	entry.SetRange("document_no", "WIPTEST")

	run(reports.CalculateProjectWIPValueID, nil)
	if !entry.FindFirst() || !entry.Amt_to_post_to_g_l.Equal(cost) {
		t.Fatalf("amount to post %s, want the cost %s", entry.Amt_to_post_to_g_l, cost)
	}
	run(reports.PostProjectWIPValueToGLID, postOptions("WIP1"))
	if a := glAmount("WIP1", "1340"); !a.Equal(cost) {
		t.Errorf("WIP costs %s, want %s", a, cost)
	}
	if a := glAmount("WIP1", "4140"); !a.Equal(cost.Neg()) {
		t.Errorf("cost adjustment %s, want %s", a, cost.Neg())
	}
	mustOK(t, entry.GetOrError(entry.Entry_no))
	if !entry.Amt_posted_to_g_l.Equal(cost) {
		t.Errorf("amount posted %s", entry.Amt_posted_to_g_l)
	}

	run(reports.CalculateProjectRecognitionID, nil)
	run(reports.PostProjectRecognitionToGLID, postOptions("REC1"))
	if a := glAmount("REC1", "1340"); !a.Equal(cost.Neg()) {
		t.Errorf("WIP costs recognized %s, want %s", a, cost.Neg())
	}
	if a := glAmount("REC1", "4150"); !a.Equal(cost) {
		t.Errorf("project costs %s, want %s", a, cost)
	}

	// nothing changed: nothing posted
	run(reports.PostProjectWIPValueToGLID, postOptions("WIP2"))
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", "WIP2")
	if !glEntry.IsEmpty() {
		t.Error("a second WIP posting posted again")
	}
	mustOK(t, project.GetOrError(project.No))
	if project.Project_posting_date != system.WorkDate() || project.Recognition_date != system.WorkDate() {
		t.Errorf("project dates: %s %s", project.Project_posting_date, project.Recognition_date)
	}
}

// Copy Project Budget (1296) copies one project's budget lines with their quantities to
// another project a month on; Date Compr. Project Budget Entries (1297) makes one entry per line
// and month
func TestCopyAndCompressProjectBudget(t *testing.T) {
	src := listCompany(t)
	from := gtables.NewProject(src)
	from.SetRange("status", gtables.Project_Status.Order)
	if !from.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	to := gtables.NewProject(src)
	to.SetRange("status", gtables.Project_Status.Planning)
	if !to.FindFirst() {
		t.Fatal("no demo project on Planning")
	}
	fromLines := gtables.NewProjectBudgetLine(src)
	fromLines.SetRange("project_no", from.No)
	n := fromLines.Count()
	fromEntries := gtables.NewProjectBudgetEntry(src)
	fromEntries.SetRange("project_no", from.No)
	mustOK(t, fromEntries.CalcSums("total_cost"))
	fromCost := fromEntries.Total_cost

	_, err := reports.Execute(reports.CopyProjectBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"project_no": to.No.String(), "date_change_formula": "1M", "copy_qty_alloc": "true"},
		Record:  fromLines,
	})
	mustOK(t, err)
	toLines := gtables.NewProjectBudgetLine(src)
	toLines.SetRange("project_no", to.No)
	toEntries := gtables.NewProjectBudgetEntry(src)
	toEntries.SetRange("project_no", to.No)
	toEntries.SetFilter("no", "R0010|R0040|R0070|1300|1310")
	mustOK(t, toEntries.CalcSums("total_cost"))
	if toLines.Count() < n || !toEntries.Total_cost.Equal(fromCost) {
		t.Errorf("copied %d lines (from %d), cost %s (from %s)", toLines.Count(), n, toEntries.Total_cost, fromCost)
	}
	toEntries.SetRange("project_status", gtables.ProjectBudgetEntry_Project_status.Planning)
	if toEntries.IsEmpty() {
		t.Error("the copied entries do not take the project's status")
	}

	// compressing by year: one entry per budget line of the project on Order (one year's months)
	before := fromEntries.Count()
	_, err = reports.Execute(reports.DateComprProjectBudgetEntriesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"starting_date": "2020-01-01", "ending_date": "2030-12-31", "period_length": "4"},
		Record:  fromEntries,
	})
	mustOK(t, err)
	after := fromEntries.Count()
	mustOK(t, fromEntries.CalcSums("total_cost"))
	if after >= before || !fromEntries.Total_cost.Equal(fromCost) {
		t.Errorf("compressed %d entries to %d, cost %s (was %s)", before, after, fromEntries.Total_cost, fromCost)
	}
}
