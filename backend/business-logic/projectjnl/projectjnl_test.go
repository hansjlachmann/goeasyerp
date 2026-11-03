package projectjnl_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

const testCompany = "demo"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

// demoCompany is a company with the demo data (the projects and the project journal templates)
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, testCompany, database.DBTypeSQLite))
	_, err = demodata.Create(db, testCompany, database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	return ftables.DBSource(db, testCompany, database.DBTypeSQLite)
}

// projectOnOrder: the demo project with status Order
func projectOnOrder(t *testing.T, src ftables.Source) *gtables.Project {
	t.Helper()
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	return project
}

// newLine is a journal line of the demo batch, validated as the user enters it
func newLine(t *testing.T, src ftables.Source, lineNo int, projectNo types.Code, lineType gtables.ProjectJournalLineType, no, qty string) *gtables.ProjectJournalLine {
	t.Helper()
	line := gtables.NewProjectJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "PROJECT", "DEFAULT", lineNo
	must(t, tables.SetUpNewProjectJnlLine(line, gtables.NewProjectJournalLine(src)))
	line.Document_no = "P1"
	must(t, line.Validate("posting_date", types.NewDate(2026, 10, 5)))
	must(t, line.Validate("project_no", projectNo))
	must(t, line.Validate("type", lineType))
	must(t, line.Validate("no", types.Code(no)))
	must(t, line.Validate("quantity", dec(qty)))
	must(t, line.InsertOrError(true))
	return line
}

// A project journal: a resource line takes the project's own price; posting the batch writes
// the resource usage (with the project), the project entries and one register, deletes the
// lines; a G/L account line with a balancing account posts its cost to the G/L; the project's
// Usage FlowFields follow
func TestPostProjectJournal(t *testing.T) {
	src := demoCompany(t)
	project := projectOnOrder(t, src)

	// a price of the project for Anna Berg (Resource Price with Project No.)
	resPrice := gtables.NewResourcePrice(src)
	resPrice.Project_no = project.No
	resPrice.Type = gtables.ResourcePrice_Type.Resource
	resPrice.Code = "R0010"
	resPrice.Unit_price = dec("1100")
	must(t, resPrice.InsertOrError(false))

	res := newLine(t, src, 10000, project.No, gtables.ProjectJournalLine_Type.Resource, "R0010", "8")
	if !res.Unit_price.Equal(dec("1100")) || !res.Total_price.Equal(dec("8800")) || !res.Unit_cost.Equal(dec("605")) || res.Source_code != "PROJJNL" {
		t.Errorf("resource line: price %s/%s, cost %s, source %s", res.Unit_price, res.Total_price, res.Unit_cost, res.Source_code)
	}
	gl := newLine(t, src, 20000, project.No, gtables.ProjectJournalLine_Type.AccountGL, "5520", "1")
	must(t, gl.Validate("unit_cost", dec("750")))
	must(t, gl.Validate("balancing_acc_no", types.Code("1510")))
	must(t, gl.ModifyOrError(true))

	line := gtables.NewProjectJournalLine(src)
	line.Journal_template_name, line.Journal_batch_name = "PROJECT", "DEFAULT"
	posted, err := projectjnl.NewProjectJnlPostBatch(src).Run(line)
	must(t, err)
	if posted.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}

	projectLedgEntry := gtables.NewProjectLedgerEntry(src)
	projectLedgEntry.SetRange("project_no", project.No)
	projectLedgEntry.SetRange("document_no", "P1")
	if n := projectLedgEntry.Count(); n != 2 {
		t.Fatalf("%d project entries, want 2", n)
	}
	resLedgEntry := gtables.NewResLedgerEntry(src)
	resLedgEntry.SetRange("document_no", "P1")
	if !resLedgEntry.FindFirst() || resLedgEntry.Project_no != project.No || !resLedgEntry.Quantity.Equal(dec("8")) {
		t.Errorf("resource entry: %+v", resLedgEntry.ToMap())
	}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", "P1")
	glEntry.SetRange("gl_account_no", "5520")
	if !glEntry.FindFirst() || glEntry.Project_no != project.No || !glEntry.Amount.Equal(dec("750")) {
		t.Errorf("G/L entry: %+v", glEntry.ToMap())
	}
	reg := gtables.NewProjectRegister(src)
	if !reg.Get(posted.RegisterNo) || reg.To_entry_no-reg.From_entry_no != 1 || reg.Source_code != "PROJJNL" {
		t.Errorf("register: %+v", reg.ToMap())
	}
	// the posted lines are deleted; a batch without a number series gets an empty line
	left := gtables.NewProjectJournalLine(src)
	left.SetFilter("no", "<>''")
	if n := left.Count(); n != 0 {
		t.Errorf("%d lines left", n)
	}

	must(t, project.SetFlowFilter("date_filter", "2026-10-05"))
	project.CalcFields("usage_cost", "usage_price", "invoiced_price")
	if !project.Usage_cost.Equal(dec("5590")) || !project.Usage_price.Equal(dec("8800")) || !project.Invoiced_price.IsZero() {
		t.Errorf("project usage: cost %s, price %s, invoiced %s", project.Usage_cost, project.Usage_price, project.Invoiced_price)
	}
}

// A sale posted for the project (as a sales invoice does: the project entry only) applies to
// the project's oldest open usage of the same resource (Apply to Oldest) and closes it; the
// Invoiced Price follows; a project that is not on Order takes no entries
func TestProjectSaleAppliesToUsage(t *testing.T) {
	src := demoCompany(t)
	project := projectOnOrder(t, src)
	newLine(t, src, 10000, project.No, gtables.ProjectJournalLine_Type.Resource, "R0010", "4")
	line := gtables.NewProjectJournalLine(src)
	line.Journal_template_name, line.Journal_batch_name = "PROJECT", "DEFAULT"
	_, err := projectjnl.NewProjectJnlPostBatch(src).Run(line)
	must(t, err)
	usage := gtables.NewProjectLedgerEntry(src)
	usage.SetRange("project_no", project.No)
	usage.SetRange("document_no", "P1")
	if !usage.FindFirst() || !usage.Open || !usage.Positive || !usage.Remaining_amount.Equal(usage.Total_price) {
		t.Fatalf("usage entry: %+v", usage.ToMap())
	}

	sale := gtables.NewProjectJournalLine(src)
	sale.Init()
	sale.Project_no = project.No
	sale.Posting_date = types.NewDate(2026, 10, 6)
	sale.Document_no = "INV1"
	sale.Entry_type = gtables.ProjectJournalLine_Entry_type.Sale
	sale.Type = gtables.ProjectJournalLine_Type.Resource
	sale.No = "R0010"
	sale.Quantity = dec("4") // an invoice's amounts: the sale entry turns the signs
	sale.Total_price = usage.Total_price
	sale.Gen_prod_posting_group = "SERVICES"
	sale.Post_project_entry_only = true
	must(t, projectjnl.NewProjectJnlPostLine(src).Run(sale))

	must(t, usage.GetOrError(usage.Entry_no))
	if usage.Open || !usage.Remaining_amount.IsZero() || usage.Closed_by_entry_no == 0 {
		t.Errorf("usage not closed by the sale: %+v", usage.ToMap())
	}
	project.CalcFields("invoiced_price")
	if !project.Invoiced_price.Equal(usage.Total_price) {
		t.Errorf("invoiced price %s, want %s", project.Invoiced_price, usage.Total_price)
	}

	// the demo project on Quote takes no journal lines
	quote := gtables.NewProject(src)
	quote.SetRange("status", gtables.Project_Status.Quote)
	if !quote.FindFirst() {
		t.Fatal("no demo project on Quote")
	}
	l := gtables.NewProjectJournalLine(src)
	l.InitRecord()
	err = l.Validate("project_no", quote.No)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrFieldMustBe {
		t.Errorf("a project on Quote: %v", err)
	}
}
