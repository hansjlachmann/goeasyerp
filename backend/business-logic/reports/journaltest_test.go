package reports_test

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/bomjnl"
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

// journalCompany: the demo company with its posted items, BOM assemblies, resources, sales and
// projects (their registers)
func journalCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:              time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine:    func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		BOMJnlPostLine:     func(src ftables.Source) demodata.BOMJnlPoster { return bomjnl.NewBOMJnlPostLine(src) },
		SalesPost:          func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		ProjectJnlPostLine: func(src ftables.Source) demodata.ProjectJnlPoster { return projectjnl.NewProjectJnlPostLine(src) },
	})
	mustOK(t, err)
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

// printJournalTest runs a journal's test report from its line and returns what it printed
func printJournalTest(t *testing.T, src ftables.Source, id int, line interface{}) string {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Context: line})
	mustOK(t, err)
	return printedTexts(result, 0)
}

func wantTexts(t *testing.T, id int, texts string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(texts, want) {
			t.Errorf("report %d does not print %q:\n%s", id, want, texts)
		}
	}
}

// Report 2 prints a general journal batch's lines with what posting would refuse under them —
// a blocked customer, a line without account, a document that does not balance — and the
// reconciliation of the G/L accounts the batch posts to
func TestGeneralJournalTestReport(t *testing.T) {
	src := journalCompany(t)
	cust := gtables.NewCustomer(src)
	mustOK(t, cust.GetOrError("C00010"))
	cust.Blocked = true
	mustOK(t, cust.ModifyOrError(false))
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	glAcc.SetRange("direct_posting", true)
	glAcc.SetRange("reconciliation_account", true)
	if !glAcc.FindFirst() {
		t.Fatal("no direct posting reconciliation account")
	}
	insert := func(lineNo int, docNo types.Code, accountType gtables.GenJournalLineAccount_type, accountNo types.Code, amount string) *gtables.GenJournalLine {
		line := gtables.NewGenJournalLine(src)
		line.Init()
		line.Journal_template_name, line.Journal_batch_name, line.Line_no = "GENERAL", "DEFAULT", lineNo
		line.Posting_date = types.NewDate(2026, 10, 5)
		line.Document_no = docNo
		line.Account_type = accountType
		line.Account_no = accountNo
		line.Amount = types.MustDecimal(amount)
		line.Amount_lcy = line.Amount
		line.Balance_lcy = line.Amount
		mustOK(t, line.InsertOrError(false))
		return line
	}
	first := insert(10000, "T1", gtables.GenJournalLine_Account_type.GLAccount, glAcc.No, "100")
	insert(20000, "T2", gtables.GenJournalLine_Account_type.Customer, "C00010", "-50")
	insert(30000, "T3", gtables.GenJournalLine_Account_type.GLAccount, "", "10")
	texts := printJournalTest(t, src, reports.GenJnlTestID, first)
	wantTexts(t, reports.GenJnlTestID, texts, "General Journal - Test", "GENERAL", "DEFAULT", "Warning!",
		"Blocked must be No for Customer C00010.",
		"Account No. or Bal. Account No. must be specified.",
		"T1 is out of balance by 100.00.",
		"The total of the lines is out of balance by",
		"Reconciliation", glAcc.No.String())
}

// Report 702 prints an item journal batch's lines with their warnings — a blocked item, a
// transfer with an amount — and the batch's amounts per entry type
func TestInventoryPostingTestReport(t *testing.T) {
	src := journalCompany(t)
	item := gtables.NewItem(src)
	mustOK(t, item.GetOrError("1000"))
	item.Blocked = true
	mustOK(t, item.ModifyOrError(false))
	insert := func(lineNo int, itemNo types.Code, entryType gtables.ItemJournalLineEntry_type, amount string) *gtables.ItemJournalLine {
		line := gtables.NewItemJournalLine(src)
		line.Init()
		line.Journal_template_name, line.Journal_batch_name, line.Line_no = "ITEM", "DEFAULT", lineNo
		line.Posting_date = types.NewDate(2026, 10, 5)
		line.Document_no = "T1"
		line.Item_no = itemNo
		line.Entry_type = entryType
		line.Quantity = types.MustDecimal("1")
		line.Invoiced_quantity = line.Quantity
		line.Amount = types.MustDecimal(amount)
		line.Gen_prod_posting_group = "RETAIL"
		mustOK(t, line.InsertOrError(false))
		return line
	}
	first := insert(10000, "1000", gtables.ItemJournalLine_Entry_type.PositiveAdjmt, "10")
	insert(20000, "1020", gtables.ItemJournalLine_Entry_type.Transfer, "5")
	texts := printJournalTest(t, src, reports.ItemJnlTestID, first)
	wantTexts(t, reports.ItemJnlTestID, texts, "Inventory Posting - Test", "Warning!",
		"Blocked must be No for item 1000.",
		"Amount must be 0 when Entry Type is Transfer.",
		"Positive Adjmt.", "Total")
}

// Report 1102 prints a resource journal batch's lines with their warnings; report 1103 prints
// the resource registers with their entries
func TestResourceJournalTestAndRegister(t *testing.T) {
	src := journalCompany(t)
	res := gtables.NewResource(src)
	mustOK(t, res.GetOrError("R0040"))
	res.Blocked = true
	mustOK(t, res.ModifyOrError(false))
	line := gtables.NewResJournalLine(src)
	line.Init()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RESOURCES", "DEFAULT", 10000
	line.Posting_date = types.NewDate(2026, 10, 5)
	line.Document_no = "T1"
	line.Resource_no = "R0040"
	line.Gen_prod_posting_group = "SERVICES"
	line.Quantity = types.MustDecimal("2")
	mustOK(t, line.InsertOrError(false))
	texts := printJournalTest(t, src, reports.ResJnlTestID, line)
	wantTexts(t, reports.ResJnlTestID, texts, "Resource Journal - Test", "RESOURCES", "R0040", "Warning!",
		"Blocked must be No for resource R0040.")

	resReg := gtables.NewResourceRegister(src)
	if !resReg.FindFirst() {
		t.Fatal("no demo resource register")
	}
	register := gtables.NewResourceRegister(src)
	mustOK(t, register.GetOrError(resReg.No))
	register.SetRecFilter()
	result, err := reports.Execute(reports.ResourceRegisterID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Record: register})
	mustOK(t, err)
	wantTexts(t, reports.ResourceRegisterID, printedTexts(result, 0), "Resource Register", "Register No: "+strconv.Itoa(resReg.No),
		"Number of Entries in Register No.")
}

// Report 802 warns about an item that is no bill of materials; report 803 prints the BOM
// registers with the item and resource entries of the assemblies
func TestBOMJournalTestAndRegister(t *testing.T) {
	src := journalCompany(t)
	line := gtables.NewBOMJournalLine(src)
	line.Init()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "BOM", "DEFAULT", 10000
	line.Posting_date = types.NewDate(2026, 10, 5)
	line.Document_no = "T1"
	line.Item_no = "1020"
	line.Quantity = types.MustDecimal("1")
	mustOK(t, line.InsertOrError(false))
	texts := printJournalTest(t, src, reports.BOMJnlTestID, line)
	wantTexts(t, reports.BOMJnlTestID, texts, "BOM Journal - Test", "Warning!", "Item 1020 is not a bill of materials.")

	bomReg := gtables.NewBOMRegister(src)
	if !bomReg.FindFirst() {
		t.Fatal("no demo BOM register")
	}
	register := gtables.NewBOMRegister(src)
	mustOK(t, register.GetOrError(bomReg.No))
	register.SetRecFilter()
	result, err := reports.Execute(reports.BOMRegisterID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Record: register})
	mustOK(t, err)
	wantTexts(t, reports.BOMRegisterID, printedTexts(result, 0), "Bill of Materials Register", "Positive Adjmt.", "Negative Adjmt.")
}
