package taxstatement_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/taxstatement"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// demoCompany: the demo data with its posted sales and purchases (tax entries), in a transaction
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:       func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		PurchPost:       func(src ftables.Source) demodata.PurchPoster { return purchposting.NewPurchPost(src) },
	})
	must(t, err)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

func line(t *testing.T, src ftables.Source, lineNo int) *gtables.TaxStatementLine {
	t.Helper()
	l := gtables.NewTaxStatementLine(src)
	must(t, l.GetOrError(map[string]interface{}{"statement_template_name": "TAX", "statement_name": "DEFAULT", "line_no": lineNo}))
	return l
}

func taxEntries(src ftables.Source, entryType int, prodGroup string) types.Decimal {
	e := gtables.NewTaxEntry(src)
	e.SetRange("type", entryType)
	e.SetRange("tax_prod_posting_group", types.Code(prodGroup))
	_ = e.CalcSums("amount")
	return e.Amount
}

// The demo's sales tax statement: a tax entry row is the sum of its tax entries, the Tax payable
// row the sum of the sales and purchase tax rows, an account row the account's net change; a row
// total that refers to itself is refused; report 12 prints the rows with their amounts
func TestTaxStatement(t *testing.T) {
	src := demoCompany(t)
	calc := &taxstatement.Calculator{Src: src, Selection: taxstatement.SelectionOpenAndClosed}
	salesStandard := taxEntries(src, 2, "STANDARD")
	if salesStandard.IsZero() {
		t.Fatal("the demo has no sales tax entries")
	}
	got, err := calc.LineTotal(line(t, src, 30000))
	must(t, err)
	if !got.Equal(salesStandard) {
		t.Errorf("row 11: %s, want the tax entries' %s", got, salesStandard)
	}
	want := salesStandard.Add(taxEntries(src, 2, "REDUCED")).Add(taxEntries(src, 1, "STANDARD"))
	payable, err := calc.LineTotal(line(t, src, 90000))
	must(t, err)
	if !payable.Equal(want) {
		t.Errorf("row 30 Tax payable: %s, want %s", payable, want)
	}
	acc := gtables.NewGLAccount(src)
	must(t, acc.GetOrError(types.Code("2330")))
	acc.CalcFields("net_change")
	got, err = calc.LineTotal(line(t, src, 100000))
	must(t, err)
	if !got.Equal(acc.Net_change) {
		t.Errorf("row 40: %s, want 2330's net change %s", got, acc.Net_change)
	}

	loop := gtables.NewTaxStatementLine(src)
	loop.Statement_template_name, loop.Statement_name, loop.Line_no = "TAX", "DEFAULT", 110000
	loop.Row_no = "50"
	must(t, loop.Validate("type", gtables.TaxStatementLine_Type.RowTotaling))
	loop.Row_totaling = "50"
	must(t, loop.InsertOrError(true))
	if _, err = calc.LineTotal(loop); err == nil || !strings.Contains(err.Error(), "50 => 50") {
		t.Errorf("a row totaling itself: %v", err)
	}
	must(t, loop.DeleteOrError(true))

	// another type keeps the key, row no. and description and clears the rest
	acc40 := line(t, src, 100000)
	must(t, acc40.Validate("type", gtables.TaxStatementLine_Type.Description))
	if acc40.Account_totaling != "" || acc40.Print_with != 0 || acc40.Row_no != "40" || acc40.Line_no != 100000 || acc40.Statement_name != "DEFAULT" {
		t.Errorf("type changed: %+v", acc40.ToMap())
	}

	result, err := reports.Execute(reports.TaxStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Options: map[string]string{"include_tax_entries": "3"}})
	must(t, err)
	var texts []string
	for _, x := range result.Texts {
		texts = append(texts, x.Text)
	}
	all := strings.Join(texts, "|")
	if !strings.Contains(all, "Tax payable") || !strings.Contains(all, format.Decimal(payable.Neg(), "2:2", "en-US")) {
		t.Errorf("report 12 does not print Tax payable %s: %.400s", format.Decimal(payable.Neg(), "2:2", "en-US"), all)
	}

	// in euro as the additional reporting currency: the entries' euro amounts
	_, err = reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}})
	must(t, err)
	e := gtables.NewTaxEntry(src)
	e.SetRange("type", 2)
	e.SetRange("tax_prod_posting_group", types.Code("STANDARD"))
	must(t, e.CalcSums("additional_currency_amount"))
	calc.UseAmtsInAddCurr = true
	got, err = calc.LineTotal(line(t, src, 30000))
	must(t, err)
	if e.Additional_currency_amount.IsZero() || !got.Equal(e.Additional_currency_amount) {
		t.Errorf("row 11 in euro: %s, want %s", got, e.Additional_currency_amount)
	}
	result, err = reports.Execute(reports.TaxStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Options: map[string]string{"include_tax_entries": "3", "show_amounts_in_add_currency": "true"}})
	must(t, err)
	texts = nil
	for _, x := range result.Texts {
		texts = append(texts, x.Text)
	}
	if all := strings.Join(texts, "|"); !strings.Contains(all, "All amounts are in EUR") {
		t.Errorf("report 12 in euro: %.400s", all)
	}
}
