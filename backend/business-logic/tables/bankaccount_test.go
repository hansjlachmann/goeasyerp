package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The classic Bank Account code: numbers from the Bank Account Nos. series of the General
// Ledger Setup, Search Name, Currency Code only while the balance is zero and no entry is open,
// the delete check (MoveBankAccEntries: ledger and check entries lose the number) and the
// delete of the comments; Balance as the sum of the entries
func TestBankAccountTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.BankAccount{}, &gtables.BankAccountLedgerEntry{}, &gtables.CheckLedgerEntry{}, &gtables.AccountingPeriod{},
		&gtables.CommentLine{}, &gtables.Currency{}, &gtables.GeneralLedgerSetup{}, &gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	series := gtables.NewNoSeries(src)
	series.Code, series.Default_nos = "BANK", true
	line := gtables.NewNoSeriesLine(src)
	line.InitRecord()
	line.Series_code, line.Line_no, line.Starting_no = "BANK", 10000, "B010"
	glSetup := gtables.NewGeneralLedgerSetup(src)
	glSetup.InitRecord()
	glSetup.Bank_account_nos = "BANK"
	currency := gtables.NewCurrency(src)
	currency.InitRecord()
	currency.Code = "EUR"
	if !series.Insert(false) || !line.Insert(false) || !glSetup.Insert(false) || !currency.Insert(false) {
		t.Fatal("insert setup")
	}

	bankAcc := gtables.NewBankAccount(src)
	bankAcc.InitRecord()
	_ = bankAcc.Validate("name", types.Text("Operating Account"))
	if err := bankAcc.InsertOrError(true); err != nil || bankAcc.No != "B010" || bankAcc.No_series != "BANK" || bankAcc.Search_name != "OPERATING ACCOUNT" {
		t.Fatalf("insert: no %q series %q search %q, %v", bankAcc.No, bankAcc.No_series, bankAcc.Search_name, err)
	}

	// An open deposit of 500: Balance 500; Currency Code cannot change
	entry := gtables.NewBankAccountLedgerEntry(src)
	entry.Entry_no, entry.Bank_account_no, entry.Posting_date, entry.Open = 1, "B010", types.NewDate(2025, 3, 1), true
	entry.Amount, entry.Amount_lcy = types.NewDecimal(500), types.NewDecimal(500)
	SetBankDebitCredit(entry)
	check := gtables.NewCheckLedgerEntry(src)
	check.Entry_no, check.Bank_account_no, check.Bank_account_ledger_entry_no, check.Check_no = 1, "B010", 1, "1001"
	comment := gtables.NewCommentLine(src)
	comment.Table_name, comment.No, comment.Line_no, comment.Comment = gtables.CommentLine_Table_name.BankAccount, "B010", 10000, "Note"
	for _, err := range []error{entry.InsertOrError(false), check.InsertOrError(false), comment.InsertOrError(true)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	bankAcc.CalcFields("balance", "debit_amount", "comment")
	if !bankAcc.Balance.Equal(types.NewDecimal(500)) || !bankAcc.Debit_amount.Equal(types.NewDecimal(500)) || !bankAcc.Comment {
		t.Errorf("balance %s, debit %s, comment %v", bankAcc.Balance, bankAcc.Debit_amount, bankAcc.Comment)
	}
	if err := bankAcc.Validate("currency_code", types.Code("EUR")); err == nil {
		t.Error("Currency Code changed with a balance")
	}

	// No delete while the entry is in an open year (no periods yet), nor while it is open
	if bankAcc.Delete(true) || errCode(t, bankAcc.TriggerError()) != apperrors.ErrDeleteHasEntries {
		t.Fatalf("delete with an entry: %v", bankAcc.TriggerError())
	}
	addPeriods(t, db, 2025, 2, types.NewDate(2025, 12, 1))
	if bankAcc.Delete(true) || errCode(t, bankAcc.TriggerError()) != apperrors.ErrDeleteOpenEntries {
		t.Fatalf("delete with an open entry: %v", bankAcc.TriggerError())
	}
	entry.Open = false
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := bankAcc.DeleteOrError(true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !entry.Get(1) || entry.Bank_account_no != "" || !check.Get(1) || check.Bank_account_no != "" || gtables.NewCommentLine(src).Count() != 0 {
		t.Errorf("after the delete: entry %q, check %q, comments %d", entry.Bank_account_no, check.Bank_account_no, gtables.NewCommentLine(src).Count())
	}

	// A bank account without balance and open entries may change its currency
	other := gtables.NewBankAccount(src)
	other.InitRecord()
	if err := other.InsertOrError(true); err != nil {
		t.Fatal(err)
	}
	if err := other.Validate("currency_code", types.Code("EUR")); err != nil {
		t.Errorf("currency of an empty bank account: %v", err)
	}
}
