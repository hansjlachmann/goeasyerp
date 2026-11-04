package resjnl_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const testCompany = "demo"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

// demoCompany is a company with the demo data, the demo resources' year of postings included
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

// newLine is a journal line of the demo batch, validated as the user enters it
func newLine(t *testing.T, src ftables.Source, lineNo int, entryType gtables.ResJournalLineEntry_type, resNo, workType, qty string) *gtables.ResJournalLine {
	t.Helper()
	line := gtables.NewResJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RESOURCES", "DEFAULT", lineNo
	must(t, tables.SetUpNewResJnlLine(line, gtables.NewResJournalLine(src)))
	line.Document_no = "T1"
	must(t, line.Validate("posting_date", types.NewDate(2026, 10, 5)))
	must(t, line.Validate("entry_type", entryType))
	must(t, line.Validate("resource_no", types.Code(resNo)))
	must(t, line.Validate("work_type_code", types.Code(workType)))
	must(t, line.Validate("quantity", dec(qty)))
	must(t, line.InsertOrError(true))
	return line
}

// A resource journal: the line takes the resource's cost and price for the work type (Find
// Cost/Price); posting the batch writes usage and sale entries (a sale with the opposite sign),
// one register, deletes the lines and the resource's FlowFields follow
func TestPostResourceJournal(t *testing.T) {
	src := demoCompany(t)
	res := gtables.NewResource(src)
	must(t, res.GetOrError(types.Code("R0010")))
	must(t, res.SetFlowFilter("date_filter", "2026-10-05"))
	res.CalcFields("usage_qty", "sales_qty", "sales_price")
	usageBefore, salesBefore := res.Usage_qty, res.Sales_qty

	// Anna Berg: normal hours at the card's cost and price; overtime at the group's price and
	// 50 % extra cost (demo costs and prices)
	normal := newLine(t, src, 10000, gtables.ResJournalLine_Entry_type.Usage, "R0010", "NORMAL", "8")
	if !normal.Unit_cost.Equal(dec("605")) || !normal.Total_cost.Equal(dec("4840")) || !normal.Unit_price.Equal(dec("1250")) || normal.Description != "Anna Berg" || normal.Source_code != "RESJNL" {
		t.Errorf("normal line: cost %s/%s, price %s, %q %s", normal.Unit_cost, normal.Total_cost, normal.Unit_price, normal.Description, normal.Source_code)
	}
	overtime := newLine(t, src, 20000, gtables.ResJournalLine_Entry_type.Usage, "R0010", "OVERTIME", "2")
	if !overtime.Unit_cost.Equal(dec("907.5")) || !overtime.Unit_price.Equal(dec("1600")) || overtime.Unit_of_measure_code != "HOUR" {
		t.Errorf("overtime line: cost %s, price %s, unit %s", overtime.Unit_cost, overtime.Unit_price, overtime.Unit_of_measure_code)
	}
	newLine(t, src, 30000, gtables.ResJournalLine_Entry_type.Sale, "R0010", "", "10")

	lastEntry := gtables.NewResLedgerEntry(src)
	if !lastEntry.FindLast() {
		t.Fatal("the demo data has no resource ledger entries")
	}
	lastEntryNo := lastEntry.Entry_no

	// posted in a transaction, as codeunit 271 does
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	line := gtables.NewResJournalLine(txSrc)
	line.Journal_template_name, line.Journal_batch_name = "RESOURCES", "DEFAULT"
	result, err := resjnl.NewResJnlPostBatch(txSrc).Run(line)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatal(err)
	}
	must(t, ftables.Commit(tx))
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	resReg := gtables.NewResourceRegister(src)
	must(t, resReg.GetOrError(result.RegisterNo))
	if resReg.From_entry_no != lastEntryNo+1 || resReg.To_entry_no != lastEntryNo+3 || resReg.Source_code != "RESJNL" {
		t.Errorf("register %d: entries %d..%d (%s), want %d..%d", resReg.No, resReg.From_entry_no, resReg.To_entry_no, resReg.Source_code, lastEntryNo+1, lastEntryNo+3)
	}
	sale := gtables.NewResLedgerEntry(src)
	must(t, sale.GetOrError(lastEntryNo+3))
	if !sale.Quantity.Equal(dec("-10")) || !sale.Total_price.Equal(dec("-12500")) || !sale.Unit_price.Equal(dec("1250")) || !sale.Chargeable || sale.Description != "" {
		t.Errorf("sale entry: qty %s, price %s/%s, chargeable %v, %q", sale.Quantity, sale.Unit_price, sale.Total_price, sale.Chargeable, sale.Description)
	}
	lines := gtables.NewResJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("RESOURCES"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	if lines.Count() != 1 { // the posted lines are deleted, an empty line is left (no batch series)
		t.Errorf("%d lines after posting, want the one empty line", lines.Count())
	}

	res = gtables.NewResource(src)
	must(t, res.GetOrError(types.Code("R0010")))
	must(t, res.SetFlowFilter("date_filter", "2026-10-05"))
	res.CalcFields("usage_qty", "sales_qty", "sales_price")
	if !res.Usage_qty.Sub(usageBefore).Equal(dec("10")) || !res.Sales_qty.Sub(salesBefore).Equal(dec("10")) {
		t.Errorf("usage %s → %s, sales %s → %s, want +10 each", usageBefore, res.Usage_qty, salesBefore, res.Sales_qty)
	}
	// Chargeable Filter: only the chargeable usage
	must(t, res.SetFlowFilter("chargeable_filter", "No"))
	res.CalcFields("usage_qty")
	if res.Usage_qty.IsNegative() {
		t.Errorf("non-chargeable usage %s", res.Usage_qty)
	}

	// a resource with entries in an open fiscal year cannot be deleted
	if err := res.DeleteOrError(true); err == nil {
		t.Error("resource with ledger entries in an open year deleted")
	}
}

// The journal page opens on the only template and its batch; the template's source code comes
// from the Source Code Setup; a template without page gets the resource journal's
func TestResourceJournalTemplate(t *testing.T) {
	src := demoCompany(t)
	template := gtables.NewResJournalTemplate(src)
	template.InitRecord()
	template.Name = "SECOND"
	must(t, template.InsertOrError(true))
	if template.Page_id != 207 || template.Source_code != "RESJNL" {
		t.Errorf("new template: page %d, source code %s", template.Page_id, template.Source_code)
	}
}
