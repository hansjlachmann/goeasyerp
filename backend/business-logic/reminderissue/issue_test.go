package reminderissue_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/reminder"
	"github.com/hansjlachmann/openerp/backend/business-logic/reminderissue"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func date(s string) types.Date {
	d, err := types.NewDateFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	// in a transaction, as reports and codeunits run (the database has one connection)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

// overdueCustomer: a domestic customer with an open invoice due before the date
func overdueCustomer(t *testing.T, src ftables.Source, before string) *gtables.Customer {
	t.Helper()
	e := gtables.NewCustomerLedgerEntry(src)
	e.SetRange("open", true)
	e.SetRange("document_type", gtables.CustomerLedgerEntry_Document_type.Invoice)
	e.SetRange("customer_posting_group", types.Code("DOMESTIC"))
	e.SetRange("currency_code", types.Code(""))
	e.SetFilter("due_date", "<"+before)
	if !e.FindFirst() {
		t.Fatal("no overdue domestic invoice in the demo")
	}
	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(e.Customer_no))
	return cust
}

func makeReminder(t *testing.T, src ftables.Source, cust *gtables.Customer, documentDate string) *gtables.ReminderHeader {
	t.Helper()
	req := gtables.NewReminderHeader(src)
	req.Posting_date = date(documentDate)
	req.Document_date = date(documentDate)
	m := reminder.New(src)
	m.Set(cust, nil, req, true)
	must(t, m.Run())
	h := gtables.NewReminderHeader(src)
	h.SetRange("customer_no", cust.No)
	if !h.FindFirst() {
		t.Fatalf("no reminder made for %s", cust.No)
	}
	return h
}

// Reminders: the first reminder of a customer's overdue invoices (level 1: texts, no fee, no
// interest) is issued without posting — an issued reminder and a reminder entry per invoice;
// a month later the second (level 2: the additional fee and interest) posts the fee (with sales
// tax) and the interest to the G/L and the customer; the G/L stays balanced
func TestRemindersIssued(t *testing.T) {
	src := demoCompany(t)
	cust := overdueCustomer(t, src, "2026-09-01")
	first := makeReminder(t, src, cust, "2026-10-06")
	if first.Reminder_level != 1 || first.Reminder_terms_code != "DOMESTIC" || first.Issuing_no_series != "S-REM+" {
		t.Errorf("reminder: level %d terms %s issuing series %s", first.Reminder_level, first.Reminder_terms_code, first.Issuing_no_series)
	}
	lines := gtables.NewReminderLine(src)
	lines.SetRange("reminder_no", first.No)
	lines.SetRange("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
	entries := lines.Count()
	if entries == 0 {
		t.Fatal("no entry lines")
	}
	lines.SetRange("type", gtables.ReminderLine_Type.Blank)
	lines.SetFilter("description", "<>''")
	if lines.Count() < 2 {
		t.Errorf("%d text lines, want the level's beginning and ending texts", lines.Count())
	}
	first.CalcFields("remaining_amount", "additional_fee", "interest_amount")
	if first.Remaining_amount.IsZero() || !first.Additional_fee.IsZero() || !first.Interest_amount.IsZero() {
		t.Errorf("level 1: remaining %s fee %s interest %s", first.Remaining_amount, first.Additional_fee, first.Interest_amount)
	}
	issue := reminderissue.New(src)
	issue.Set(first, false, types.Date{})
	must(t, issue.Run())
	issued := issue.GetIssuedReminder()
	if issued == nil || issued.No == first.No || issued.Pre_assigned_no != first.No {
		t.Fatalf("issued reminder %+v", issued)
	}
	reminderEntries := gtables.NewReminderFinChargeEntry(src)
	reminderEntries.SetRange("no", issued.No)
	if reminderEntries.Count() != entries {
		t.Errorf("%d reminder entries, want %d", reminderEntries.Count(), entries)
	}
	if gtables.NewReminderHeader(src).Get(first.No) {
		t.Error("the reminder was not deleted")
	}

	second := makeReminder(t, src, cust, "2026-11-06")
	second.CalcFields("additional_fee", "interest_amount", "tax_amount")
	if second.Reminder_level != 2 || !second.Additional_fee.Equal(types.NewDecimalFromInt(70)) || second.Interest_amount.IsZero() {
		t.Fatalf("level 2: level %d fee %s interest %s", second.Reminder_level, second.Additional_fee, second.Interest_amount)
	}
	issue = reminderissue.New(src)
	issue.Set(second, false, types.Date{})
	must(t, issue.Run())
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	custLedgEntry.SetRange("document_no", issue.GetIssuedReminder().No)
	if !custLedgEntry.FindFirst() || custLedgEntry.Document_type != gtables.CustomerLedgerEntry_Document_type.Reminder ||
		!custLedgEntry.Amount.Equal(second.Additional_fee.Add(second.Interest_amount).Add(second.Tax_amount)) {
		t.Errorf("customer entry %s %v %s, want fee + interest + tax %s", custLedgEntry.Document_no, custLedgEntry.Document_type, custLedgEntry.Amount,
			second.Additional_fee.Add(second.Interest_amount).Add(second.Tax_amount))
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L: %+v", r)
	}
}

// Finance charge memos: the interest on a customer's overdue invoices by the STD terms (1.5 % per
// 30 days after a 5-day grace period) and the terms' fee, issued: posted to the interest and
// fee accounts and the customer
func TestFinanceChargeMemoIssued(t *testing.T) {
	src := demoCompany(t)
	cust := overdueCustomer(t, src, "2026-09-01")
	req := gtables.NewFinanceChargeMemoHeader(src)
	req.Posting_date = date("2026-10-06")
	req.Document_date = date("2026-10-06")
	m := reminder.NewFinChrgMemoMake(src)
	m.Set(cust, nil, req)
	must(t, m.Run())
	h := gtables.NewFinanceChargeMemoHeader(src)
	h.SetRange("customer_no", cust.No)
	if !h.FindFirst() {
		t.Fatal("no finance charge memo made")
	}
	h.CalcFields("interest_amount", "additional_fee", "tax_amount")
	if h.Interest_amount.IsZero() || !h.Additional_fee.Equal(types.NewDecimalFromInt(70)) || h.Fin_charge_terms_code != "STD" {
		t.Fatalf("memo: interest %s fee %s terms %s", h.Interest_amount, h.Additional_fee, h.Fin_charge_terms_code)
	}
	line := gtables.NewFinanceChargeMemoLine(h)
	line.SetRange("finance_charge_memo_no", h.No)
	line.SetRange("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
	line.FindFirst()
	if line.Description != "Interest on overdue invoices" || !line.Interest_rate.Equal(types.MustDecimal("1.5")) {
		t.Errorf("line %q rate %s", line.Description, line.Interest_rate)
	}
	issue := reminderissue.NewFinChrgMemoIssue(src)
	issue.Set(h, false, types.Date{})
	must(t, issue.Run())
	issued := issue.GetIssuedFinChrgMemo()
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", issued.No)
	glEntry.SetRange("gl_account_no", types.Code("6115"))
	must(t, glEntry.CalcSums("amount"))
	if !glEntry.Amount.Equal(h.Interest_amount.Neg()) {
		t.Errorf("interest account %s, want %s", glEntry.Amount, h.Interest_amount.Neg())
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L: %+v", r)
	}
}
