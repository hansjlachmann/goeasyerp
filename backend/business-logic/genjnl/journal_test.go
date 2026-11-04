package genjnl

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const testCompany = "demo"

// demoCompany is a company with the demo data (chart of accounts, customers and vendors with
// entries, bank accounts, currencies, the journal templates)
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	err = tables.CreateCompanyTables(db, testCompany, database.DBTypeSQLite)
	if err != nil {
		t.Fatal(err)
	}
	_, err = demodata.Create(db, testCompany, database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	return ftables.DBSource(db, testCompany, database.DBTypeSQLite)
}

// newJnlLine is a new line of a batch (not inserted)
func newJnlLine(src ftables.Source, template, batch string, lineNo int) *gtables.GenJournalLine {
	line := gtables.NewGenJournalLine(src)
	line.InitRecord()
	line.Journal_template_name = types.Code(template)
	line.Journal_batch_name = types.Code(batch)
	line.Line_no = lineNo
	return line
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func date(t *testing.T, day, month, year int) types.Date {
	t.Helper()
	d, err := system.DMY2Date(day, month, year)
	must(t, err)
	return d
}

// The journal line's fields fill each other as in the classic journal
func TestJournalLineValidations(t *testing.T) {
	src := demoCompany(t)
	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	must(t, line.Validate("account_no", "1210"))
	if line.Description != "Land and Buildings" {
		t.Errorf("description from the G/L account: %q", line.Description)
	}
	must(t, line.Validate("amount", dec("100")))
	if !line.Debit_amount.Equal(dec("100")) || !line.Credit_amount.IsZero() || !line.Balance_lcy.Equal(dec("100")) || !line.Amount_lcy.Equal(dec("100")) {
		t.Errorf("amount 100: debit %s credit %s balance %s", line.Debit_amount, line.Credit_amount, line.Balance_lcy)
	}
	must(t, line.Validate("credit_amount", dec("50")))
	if !line.Amount.Equal(dec("-50")) || line.Correction || !line.Debit_amount.IsZero() {
		t.Errorf("credit 50: amount %s correction %v", line.Amount, line.Correction)
	}
	must(t, line.Validate("debit_amount", dec("-20")))
	if !line.Amount.Equal(dec("-20")) || !line.Correction || !line.Debit_amount.Equal(dec("-20")) {
		t.Errorf("debit -20 (a correction): amount %s debit %s correction %v", line.Amount, line.Debit_amount, line.Correction)
	}
	must(t, line.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.BankAccount))
	must(t, line.Validate("bal_account_no", "B010"))
	if !line.Balance_lcy.IsZero() || line.Source_type != gtables.GenJournalLine_Source_type.BankAccount || line.Source_no != "B010" {
		t.Errorf("with a balancing account: balance %s source %v %s", line.Balance_lcy, line.Source_type, line.Source_no)
	}

	// A customer in a currency: the line takes its currency, the exchange rate of the date
	cust := gtables.NewCustomer(src)
	if !cust.FindFirst() {
		t.Fatal("no customer in the demo data")
	}
	cust.Currency_code = "EUR"
	must(t, cust.ModifyOrError(false))
	factor, err := tables.ExchangeRates(src).ExchangeRate(date(t, 15, 10, 2026), cust.Currency_code)
	must(t, err)
	rate, err := tables.ExchangeRates(src).ExchangeAmtFCYToLCY(date(t, 15, 10, 2026), cust.Currency_code, dec("100"), factor)
	line = newJnlLine(src, "SALES", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	must(t, line.Validate("document_type", gtables.GenJournalLine_Document_type.Invoice))
	must(t, line.Validate("account_type", gtables.GenJournalLine_Account_type.Customer))
	must(t, line.Validate("account_no", cust.No))
	must(t, err)
	if line.Currency_code != cust.Currency_code || line.Description != types.Text(cust.Name) || line.Posting_group != cust.Customer_posting_group {
		t.Errorf("customer: currency %q description %q posting group %q", line.Currency_code, line.Description, line.Posting_group)
	}
	must(t, line.Validate("amount", dec("100")))
	if !line.Amount_lcy.Equal(system.Round(rate)) || line.Currency_factor.IsZero() {
		t.Errorf("100 %s: %s, want %s (factor %s)", cust.Currency_code, line.Amount_lcy, system.Round(rate), line.Currency_factor)
	}
	if cust.Payment_terms_code != "" && line.Due_date.IsZero() {
		t.Errorf("an invoice gets the due date of the payment terms %s", cust.Payment_terms_code)
	}

	// Two customers/vendors cannot be posted against each other
	err = line.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.Vendor)
	if err == nil || !strings.Contains(err.Error(), "must be G/L Account or Bank Account") {
		t.Errorf("customer against vendor: %v", err)
	}
	// Employee is not a balancing account
	err = line.Validate("bal_account_type", gtables.GenJournalLine_Bal_account_type.Employee)
	if err == nil {
		t.Error("Bal. Account Type Employee must be refused")
	}
	// Settlement is the system's
	glLine := newJnlLine(src, "GENERAL", "DEFAULT", 20000)
	err = glLine.Validate("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Settlement)
	if err == nil {
		t.Error("Gen. Posting Type Settlement must be refused")
	}
	// No currency factor without a currency
	err = glLine.Validate("currency_factor", dec("1.5"))
	if err == nil || !strings.Contains(err.Error(), "cannot be specified without") {
		t.Errorf("factor without currency: %v", err)
	}
}

// A new line takes the date and document no. of the line above — the next number below a
// balanced document; a new batch starts with the batch's number series
func TestSetUpNewLine(t *testing.T) {
	src := demoCompany(t)
	first := newJnlLine(src, "GENERAL", "DEFAULT", 0)
	must(t, tables.SetUpNewGenJnlLine(first, newJnlLine(src, "GENERAL", "DEFAULT", 0), types.Decimal{}, true))
	if first.Document_no != "G00001" || first.Posting_date != system.WorkDate() {
		t.Errorf("first line: document %q date %s", first.Document_no, first.Posting_date)
	}
	first.Line_no = 10000
	must(t, first.Validate("account_no", "1210"))
	must(t, first.Validate("amount", dec("100")))
	must(t, first.InsertOrError(true))

	second := newJnlLine(src, "GENERAL", "DEFAULT", 0)
	balance, _ := CalcBalance(second, first)
	must(t, tables.SetUpNewGenJnlLine(second, first, balance, true))
	if second.Document_no != "G00001" {
		t.Errorf("below an unbalanced document: %q", second.Document_no)
	}
	second.Line_no = 20000
	must(t, second.Validate("account_no", "1230"))
	must(t, second.Validate("amount", dec("-100")))
	must(t, second.InsertOrError(true))

	third := newJnlLine(src, "GENERAL", "DEFAULT", 0)
	balance, _ = CalcBalance(third, second)
	must(t, tables.SetUpNewGenJnlLine(third, second, balance, true))
	_, total := CalcBalance(second, second)
	if third.Document_no != "G00002" || !total.IsZero() {
		t.Errorf("below a balanced document: %q (total %s)", third.Document_no, total)
	}
	b1, t1 := CalcBalance(first, first)
	if !b1.Equal(dec("100")) || !t1.IsZero() {
		t.Errorf("balance up to the first line %s, total %s", b1, t1)
	}
}

// Opening a journal page: the only template of its type and its batch; several templates — the
// user picks one; none — one is created with a DEFAULT batch
func TestOpenJournalPage(t *testing.T) {
	src := demoCompany(t)
	line := newJnlLine(src, "", "", 0)

	page := pages.NewOpen(nil)
	must(t, OpenJournalPage(line, page, gtables.GenJournalTemplate_Type.General, false))
	if page.Param("template") != "GENERAL" || page.Param("batch") != "DEFAULT" || len(page.Filters()) != 2 {
		t.Errorf("one General template: params %v filters %v", page.Params(), page.Filters())
	}

	// an unknown batch: the template's first batch
	page = pages.NewOpen(map[string]string{"template": "GENERAL", "batch": "NOPE"})
	must(t, OpenJournalPage(line, page, gtables.GenJournalTemplate_Type.General, false))
	if page.Param("batch") != "DEFAULT" {
		t.Errorf("unknown batch: %v", page.Params())
	}

	second := gtables.NewGenJournalTemplate(src)
	second.Name = "GENERAL2"
	must(t, second.Validate("type", gtables.GenJournalTemplate_Type.General))
	must(t, second.InsertOrError(true))
	page = pages.NewOpen(nil)
	must(t, OpenJournalPage(line, page, gtables.GenJournalTemplate_Type.General, false))
	if page.Selection() == nil || page.Selection().PageID != TemplateListPageID || len(page.Filters()) != 0 {
		t.Errorf("two General templates: the user picks one (%+v)", page.Selection())
	}

	// no template of the type: one is created, with a DEFAULT batch
	page = pages.NewOpen(nil)
	must(t, OpenJournalPage(line, page, gtables.GenJournalTemplate_Type.PayrollAccrual, false))
	template := gtables.NewGenJournalTemplate(src)
	if !template.Get(types.Code(page.Param("template"))) || template.Type != gtables.GenJournalTemplate_Type.PayrollAccrual {
		t.Fatalf("created template %q", page.Param("template"))
	}
	batch := gtables.NewGenJournalBatch(src)
	if page.Param("batch") != "DEFAULT" || !batch.Get(tables.GenJnlBatchKey(template.Name, "DEFAULT")) {
		t.Errorf("created batch %v", page.Params())
	}

	page = pages.NewOpen(map[string]string{"template": "NOPE"})
	if err := OpenJournalPage(line, page, gtables.GenJournalTemplate_Type.General, false); err == nil {
		t.Error("an unknown template must be refused")
	}
}

// Templates and batches: the type gives page and source code; a batch's lines move with it
// and go with it
func TestTemplateAndBatch(t *testing.T) {
	src := demoCompany(t)
	template := gtables.NewGenJournalTemplate(src)
	template.Name = "RCPT2"
	must(t, template.Validate("type", gtables.GenJournalTemplate_Type.CashReceipts))
	must(t, template.InsertOrError(true))
	if template.Page_id != 255 || template.Source_code != "CASHRECJNL" || !template.Force_doc_balance {
		t.Errorf("Cash Receipts template: page %d source %q force balance %v", template.Page_id, template.Source_code, template.Force_doc_balance)
	}
	must(t, template.Validate("no_series", "GJNL-RCPT"))
	if err := template.Validate("posting_no_series", "GJNL-RCPT"); err == nil {
		t.Error("Posting No. Series equal to No. Series must be refused")
	}
	template.Posting_no_series = ""
	must(t, template.ModifyOrError(true))

	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "RCPT2"
	must(t, tables.SetupNewBatch(batch))
	batch.Name = "B1"
	must(t, batch.InsertOrError(true))
	if batch.No_series != "GJNL-RCPT" {
		t.Errorf("new batch takes the template's No. Series: %q", batch.No_series)
	}
	line := newJnlLine(src, "RCPT2", "B1", 10000)
	must(t, line.Validate("account_no", "1210"))
	must(t, line.InsertOrError(true))

	if !batch.Rename(types.Code("RCPT2"), types.Code("B2")) {
		t.Fatalf("rename batch: %v", batch.TriggerError())
	}
	moved := gtables.NewGenJournalLine(src)
	moved.SetRange("journal_batch_name", "B2")
	if moved.Count() != 1 {
		t.Errorf("the line did not move with its batch")
	}

	must(t, template.DeleteOrError(true))
	if moved.Count() != 0 || batch.Get(tables.GenJnlBatchKey("RCPT2", "B2")) {
		t.Error("deleting the template must delete its batches and lines")
	}
}
