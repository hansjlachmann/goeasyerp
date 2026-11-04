package bankrecon_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bankrecon"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
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

func date(s string) types.Date {
	d, err := types.NewDateFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// demoCompany: the demo data in a transaction rolled back after the test (one connection)
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

func newRecon(t *testing.T, src ftables.Source, statementNo string) *gtables.BankAccReconciliation {
	t.Helper()
	recon := gtables.NewBankAccReconciliation(src)
	recon.Statement_no = types.Code(statementNo)
	must(t, recon.Validate("bank_account_no", types.Code("B010")))
	recon.Statement_date = date("2026-10-31")
	must(t, recon.InsertOrError(true))
	return recon
}

func openEntries(src ftables.Source, from, to string) *gtables.BankAccountLedgerEntry {
	e := gtables.NewBankAccountLedgerEntry(src)
	e.SetRange("bank_account_no", types.Code("B010"))
	e.SetRange("open", true)
	e.SetRange("posting_date", date(from), date(to))
	return e
}

// Suggest Lines applies a line to each open entry of the period; posted, the entries close, the
// bank account has the statement's balance, the reconciliation is a bank account statement
func TestSuggestAndPostReconciliation(t *testing.T) {
	src := demoCompany(t)
	entries := openEntries(src, "2026-10-01", "2026-10-31")
	count := entries.Count()
	if count == 0 {
		t.Fatal("no open bank entries in October")
	}
	must(t, entries.CalcSums("remaining_amount"))
	total := entries.Remaining_amount

	recon := newRecon(t, src, "")
	if recon.Statement_no != "ST2026-10" {
		t.Errorf("statement no. %q after the last ST2026-09", recon.Statement_no)
	}
	bankAcc := gtables.NewBankAccount(src)
	must(t, bankAcc.GetOrError(types.Code("B010")))
	if bankAcc.Last_statement_no != "ST2026-10" {
		t.Errorf("last statement no. %q", bankAcc.Last_statement_no)
	}
	_, err := reports.Execute(reports.SuggestBankAccReconLinesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: recon, Options: map[string]string{"starting_date": "2026-10-01", "ending_date": "2026-10-31"}})
	must(t, err)
	lines := gtables.NewBankAccReconciliationLine(src)
	lines.SetRange("bank_account_no", types.Code("B010"))
	lines.SetRange("statement_no", types.Code("ST2026-10"))
	if lines.Count() != count {
		t.Fatalf("%d lines, want %d", lines.Count(), count)
	}
	must(t, lines.CalcSums("statement_amount", "difference"))
	if !lines.Statement_amount.Equal(total) || !lines.Difference.IsZero() {
		t.Errorf("lines: statement amount %s (want %s), difference %s", lines.Statement_amount, total, lines.Difference)
	}
	applied := openEntries(src, "2026-10-01", "2026-10-31")
	applied.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied)
	if applied.Count() != count {
		t.Errorf("%d entries applied, want %d", applied.Count(), count)
	}

	// a wrong ending balance is refused
	must(t, recon.GetOrError(map[string]interface{}{"bank_account_no": "B010", "statement_no": "ST2026-10"}))
	recon.Statement_ending_balance = recon.Balance_last_statement.Add(total).Add(types.NewDecimalFromInt(1))
	must(t, recon.ModifyOrError(true))
	if _, err = bankrecon.Post(recon); err == nil {
		t.Fatal("posted with a wrong ending balance")
	}
	recon.Statement_ending_balance = recon.Balance_last_statement.Add(total)
	must(t, recon.ModifyOrError(true))
	posted, err := bankrecon.Post(recon)
	must(t, err)
	if !posted {
		t.Fatal("nothing posted")
	}
	if openEntries(src, "2026-10-01", "2026-10-31").Count() != 0 {
		t.Error("entries of the statement still open")
	}
	closed := gtables.NewBankAccountLedgerEntry(src)
	closed.SetRange("statement_no", types.Code("ST2026-10"))
	closed.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.Closed)
	if closed.Count() != count {
		t.Errorf("%d entries closed on the statement, want %d", closed.Count(), count)
	}
	must(t, bankAcc.GetOrError(types.Code("B010")))
	if !bankAcc.Balance_last_statement.Equal(recon.Statement_ending_balance) {
		t.Errorf("balance last statement %s, want %s", bankAcc.Balance_last_statement, recon.Statement_ending_balance)
	}
	stmt := gtables.NewBankAccountStatement(src)
	must(t, stmt.GetOrError(map[string]interface{}{"bank_account_no": "B010", "statement_no": "ST2026-10"}))
	stmtLines := gtables.NewBankAccountStatementLine(src)
	stmtLines.SetRange("statement_no", types.Code("ST2026-10"))
	if stmtLines.Count() != count || gtables.NewBankAccReconciliation(src).Get(map[string]interface{}{"bank_account_no": "B010", "statement_no": "ST2026-10"}) {
		t.Errorf("statement lines %d, reconciliation left", stmtLines.Count())
	}
	// the posted statement's number cannot come again
	again := gtables.NewBankAccReconciliation(src)
	again.Bank_account_no = "B010"
	again.Statement_no = "ST2026-10"
	if again.InsertOrError(true) == nil {
		t.Error("a reconciliation with a posted statement's number")
	}
}

// A line applied to an entry by hand (Apply Entries, codeunit 375) takes its amount; a difference
// line balances what the bank charged; deleting the applied line frees the entry
func TestApplyEntriesByHand(t *testing.T) {
	src := demoCompany(t)
	recon := newRecon(t, src, "")
	entry := openEntries(src, "2026-10-01", "2026-10-31")
	if !entry.FindFirst() {
		t.Fatal("no open bank entry")
	}
	line := gtables.NewBankAccReconciliationLine(src)
	line.Bank_account_no = recon.Bank_account_no
	line.Statement_no = recon.Statement_no
	line.Statement_line_no = 10000
	line.Transaction_date = entry.Posting_date
	must(t, line.InsertOrError(true))
	must(t, tables.ToggleBankAccReconNo(entry, line, true))
	if !line.Applied_amount.Equal(entry.Remaining_amount) || !line.Statement_amount.Equal(entry.Remaining_amount) || line.Applied_entries != 1 {
		t.Errorf("line: applied %s statement %s entries %d, want %s", line.Applied_amount, line.Statement_amount, line.Applied_entries, entry.Remaining_amount)
	}
	must(t, entry.GetOrError(entry.Entry_no))
	if entry.Statement_status != gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied || entry.Statement_line_no != 10000 {
		t.Errorf("entry %v line %d", entry.Statement_status, entry.Statement_line_no)
	}

	fee := gtables.NewBankAccReconciliationLine(src)
	fee.Bank_account_no = recon.Bank_account_no
	fee.Statement_no = recon.Statement_no
	fee.Statement_line_no = 20000
	fee.Type = gtables.BankAccReconciliationLine_Type.Difference
	must(t, fee.InsertOrError(true))
	must(t, fee.Validate("statement_amount", types.NewDecimalFromInt(-35)))
	must(t, fee.ModifyOrError(true))
	if !fee.Difference.Equal(types.NewDecimalFromInt(-35)) {
		t.Errorf("difference line %s", fee.Difference)
	}

	must(t, line.GetOrError(map[string]interface{}{"bank_account_no": "B010", "statement_no": "ST2026-10", "statement_line_no": 10000}))
	must(t, line.DeleteOrError(true))
	must(t, entry.GetOrError(entry.Entry_no))
	if entry.Statement_status != gtables.BankAccountLedgerEntry_Statement_status.Open || entry.Statement_no != "" {
		t.Errorf("entry after the line's deletion: %v %q", entry.Statement_status, entry.Statement_no)
	}
}
