package reports_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/intrastatjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo's trade with Denmark in the Intrastat journal: Get Entries made a shipment line per
// item sold and a receipt line per item bought, with the documents' transaction type, transport
// method and the items' tariff numbers and weights; the checklist prints them, Make Diskette
// writes the declaration and marks the batch reported
func TestIntrastat(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	counts, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:       func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		PurchPost:       func(src ftables.Source) demodata.PurchPoster { return purchposting.NewPurchPost(src) },
	})
	mustOK(t, err)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	if counts.IntrastatJnlLines != 4 {
		t.Fatalf("%d Intrastat lines, want 4 (2 shipped, 2 received)", counts.IntrastatJnlLines)
	}
	line := gtables.NewIntrastatJnlLine(src)
	line.SetRange("item_no", types.Code("1300"))
	if !line.FindFirst() {
		t.Fatal("no Intrastat line of item 1300")
	}
	if line.Type != gtables.IntrastatJnlLine_Type.Shipment || line.Country_code != "DK" || line.Transaction_type != "11" ||
		line.Transport_method != "3" || line.Tariff_no != "94013100" || line.Entry_exit_point != "SVINESUND" {
		t.Errorf("line of 1300: %v %s %s %s %s %s", line.Type, line.Country_code, line.Transaction_type, line.Transport_method, line.Tariff_no, line.Entry_exit_point)
	}
	if !line.Quantity.Equal(types.NewDecimalFromInt(4)) || !line.Total_weight.Equal(types.NewDecimalFromInt(56)) || line.Statistical_value.IsZero() {
		t.Errorf("line of 1300: quantity %s weight %s value %s", line.Quantity, line.Total_weight, line.Statistical_value)
	}
	receipt := gtables.NewIntrastatJnlLine(src)
	receipt.SetRange("type", gtables.IntrastatJnlLine_Type.Receipt)
	if n := receipt.Count(); n != 2 {
		t.Errorf("%d receipt lines, want 2", n)
	}

	// run from the journal: the checklist, then the declaration file
	context := gtables.NewIntrastatJnlLine(src)
	context.Get(map[string]interface{}{"journal_template_name": line.Journal_template_name, "journal_batch_name": line.Journal_batch_name, "line_no": line.Line_no})
	result, err := reports.Execute(reports.IntrastatChecklistID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Context: context})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	for _, want := range []string{"Intrastat - Checklist", "94013100", "Denmark", "Statistics Period: 2610"} {
		if !strings.Contains(texts, want) {
			t.Errorf("checklist does not print %q", want)
		}
	}
	// the form of the shipments (the Type filter it starts with is Receipt): one line per group
	result, err = reports.Execute(reports.IntrastatFormID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Context: context,
		Filters: map[string][]ftables.FieldFilter{"IntrastatJnlLine": {{Field: "type", Expression: "1"}}}})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	for _, want := range []string{"Intrastat - Form", "Shipment", "94013100", "Denmark", "No. of Entries|2"} {
		if !strings.Contains(texts, want) {
			t.Errorf("form does not print %q:\n%s", want, texts)
		}
	}
	_, err = reports.Execute(reports.IntrastatFormID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Context: context,
		Filters: map[string][]ftables.FieldFilter{"IntrastatJnlLine": {{Field: "type", Expression: "0|1"}}}})
	if err == nil {
		t.Error("the form of both types was printed")
	}
	result, err = reports.Execute(reports.IntrastatMakeDiskID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Context: context})
	mustOK(t, err)
	file := string(result.File)
	if result.FileName == "" || !strings.HasPrefix(file, "00") || !strings.Contains(file, "INTRASTAT") || strings.Count(file, "\r\n03") != 4 {
		t.Errorf("declaration %q:\n%s", result.FileName, file)
	}
	batch := gtables.NewIntrastatJnlBatch(src)
	batch.Get(map[string]interface{}{"journal_template_name": line.Journal_template_name, "name": line.Journal_batch_name})
	if !batch.Reported {
		t.Error("the batch is not reported after Make Diskette")
	}
	// Get Entries again: nothing new
	result, err = reports.Execute(reports.IntrastatGetEntriesID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Context: context,
		Options: map[string]string{"starting_date": "2026-10-01", "ending_date": "2026-10-31"}})
	mustOK(t, err)
	if !strings.Contains(result.Message, "0") {
		t.Errorf("Get Entries again: %q", result.Message)
	}
	// VIES: a file with its header and total
	result, err = reports.Execute(reports.VIESDeclarationDiskID, db, "demo", database.DBTypeSQLite, reports.RunOptions{})
	mustOK(t, err)
	if !strings.Contains(string(result.File), "LIST") || !strings.Contains(string(result.File), "\r\n10") {
		t.Errorf("VIES file:\n%s", result.File)
	}

	// VIES printed (19): two sales to a Danish customer and one without a customer
	dkCust := gtables.NewCustomer(src)
	dkCust.SetRange("country_code", types.Code("DK"))
	if !dkCust.FindFirst() {
		t.Fatal("no Danish customer")
	}
	dkCust.Vat_registration_no = "DK12345678"
	mustOK(t, dkCust.ModifyOrError(false))
	lastEntry := gtables.NewTaxEntry(src)
	lastEntry.FindLast()
	for i, e := range []struct {
		cust types.Code
		base string
		eu3  bool
	}{{dkCust.No, "-10000", false}, {dkCust.No, "-2500", true}, {"", "-700", false}} {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.Entry_no = lastEntry.Entry_no + i + 1
		taxEntry.Type = gtables.TaxEntry_Type.Sale
		taxEntry.Posting_date = types.NewDate(2026, 10, 5)
		taxEntry.Country_code = "DK"
		taxEntry.Sell_to_buy_from_no = e.cust
		taxEntry.Base = types.MustDecimal(e.base)
		taxEntry.Eu_3_party_trade = e.eu3
		mustOK(t, taxEntry.InsertOrError(false))
	}
	result, err = reports.Execute(reports.VIESDeclarationTaxAuthID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF,
		Filters: map[string][]ftables.FieldFilter{"TaxEntry": {{Field: "posting_date", Expression: "2026-10-01..2026-10-31"}}}})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	for _, want := range []string{"VIES DECLARATION", "DK12345678", "12,500.00", "2,500.00", "|13|", "Warning!", "Signature"} {
		if !strings.Contains(texts, want) {
			t.Errorf("VIES declaration does not print %q:\n%s", want, texts)
		}
	}
	// (report 88 refuses a sale without a customer)
	noCust := gtables.NewTaxEntry(src)
	noCust.SetRange("sell_to_buy_from_no", types.Code(""))
	noCust.SetRange("country_code", types.Code("DK"))
	noCust.DeleteAll()

	// in euro as the additional reporting currency: Get Entries in a batch with Amounts in Add.
	// Currency takes the amounts in euro (11.50 a euro, rounded to whole euro), the checklist
	// says so
	lcyAmount := line.Amount
	_, err = reports.Execute(reports.AdjustAddReportingCurrencyID, db, "demo", database.DBTypeSQLite, reports.RunOptions{
		Options: map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}})
	mustOK(t, err)
	lines := gtables.NewIntrastatJnlLine(src)
	lines.SetRange("journal_template_name", batch.Journal_template_name)
	lines.SetRange("journal_batch_name", batch.Name)
	lines.DeleteAll()
	batch.Reported = false
	mustOK(t, batch.Validate("amounts_in_add_currency", true))
	mustOK(t, batch.ModifyOrError(false))
	n, err := intrastatjnl.GetEntries(batch, types.NewDate(2026, 10, 1), types.NewDate(2026, 10, 31), types.Decimal{})
	mustOK(t, err)
	if n != 4 {
		t.Fatalf("%d lines in euro, want 4", n)
	}
	euroLine := gtables.NewIntrastatJnlLine(src)
	euroLine.SetRange("item_no", types.Code("1300"))
	if !euroLine.FindFirst() || !euroLine.Amount.Equal(lcyAmount.Div(types.MustDecimal("11.5")).Round(0)) {
		t.Errorf("1300 in euro %s, want %s / 11.50", euroLine.Amount, lcyAmount)
	}
	result, err = reports.Execute(reports.IntrastatChecklistID, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Context: context})
	mustOK(t, err)
	if texts := printedTexts(result, 0); !strings.Contains(texts, "All amounts are in EUR") {
		t.Errorf("checklist in euro: %.300s", texts)
	}
	// (the demo's VIES file has no EU sales lines: the run with the option only)
	_, err = reports.Execute(reports.VIESDeclarationDiskID, db, "demo", database.DBTypeSQLite, reports.RunOptions{
		Options: map[string]string{"show_amounts_in_add_currency": "true"}})
	mustOK(t, err)
}
