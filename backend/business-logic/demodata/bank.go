package demodata

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Demo bank entries: every demo payment of a customer or vendor has its entry on the bank
// account it went through (B010, or B030 for the payments in pounds — the bank account's
// currency: its amount in pounds at the day's rate, the LCY amount as paid). Entries before this
// month are reconciled (a monthly statement, closed); this month's are still open. The bank
// accounts (setup.yaml) get their last statement.

// demoBankAccount is the bank account the demo payments go through
const demoBankAccount = types.Code("B010")

type bankEntry struct {
	bankAccount string
	date        time.Time
	docNo       string
	description string
	amount      float64
	balType     gtables.BankAccountLedgerEntryBal_account_type
	balNo       string
}

func insertBankEntries(exec database.Executor, company string, dbType database.DBType, custEntries []*ledgerEntry, vendEntries []*vendorEntry, today time.Time, counts *Counts) error {
	var entries []bankEntry
	for _, e := range custEntries {
		if e.docType == gtables.CustomerLedgerEntry_Document_type.Payment {
			entries = append(entries, bankEntry{string(demoBankAccount), e.date, e.docNo, e.description, -e.amount,
				gtables.BankAccountLedgerEntry_Bal_account_type.Customer, e.customerNo})
		}
	}
	for _, e := range vendEntries {
		if e.docType == gtables.VendorLedgerEntry_Document_type.Payment {
			entries = append(entries, bankEntry{e.bankAccount, e.date, e.docNo, e.description, -e.amount,
				gtables.BankAccountLedgerEntry_Bal_account_type.Vendor, e.vendorNo})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].date.Before(entries[j].date) })

	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	statementNo := func(t time.Time) string { return t.Format("ST2006-01") }
	type lastStatement struct {
		no      string
		balance float64
	}
	statements := map[string]*lastStatement{}

	last := gtables.NewBankAccountLedgerEntry(ftables.DBSource(exec, company, dbType))
	entryNo := 0
	if last.FindLast() {
		entryNo = last.Entry_no
	}
	src := ftables.DBSource(exec, company, dbType)
	currencies := map[string]types.Code{}
	currencyOf := func(no string) types.Code {
		if c, ok := currencies[no]; ok {
			return c
		}
		bankAcc := gtables.NewBankAccount(src)
		if bankAcc.Get(types.Code(no)) {
			currencies[no] = bankAcc.Currency_code
		}
		return currencies[no]
	}
	batch := make([]*gtables.BankAccountLedgerEntry, 0, len(entries))
	for _, e := range entries {
		entryNo++
		rec := gtables.NewBankAccountLedgerEntry(last)
		rec.Entry_no = entryNo
		rec.Bank_account_no = types.NewCode(e.bankAccount)
		rec.Posting_date = types.NewDateFromTime(e.date)
		rec.Document_date = types.NewDateFromTime(e.date)
		rec.Document_type = gtables.BankAccountLedgerEntry_Document_type.Payment
		rec.Document_no = types.NewCode(e.docNo)
		rec.Description = types.NewText(e.description)
		rec.Amount_lcy = types.NewDecimal(round2(e.amount))
		rec.Amount = rec.Amount_lcy
		if currencyCode := currencyOf(e.bankAccount); currencyCode != "" {
			amount, err := bankAmountFCY(src, rec.Posting_date, currencyCode, rec.Amount_lcy)
			if err != nil {
				return err
			}
			rec.Currency_code = currencyCode
			rec.Amount = amount
		}
		tables.SetBankDebitCredit(rec)
		rec.Bank_acc_posting_group = "CHECKING"
		rec.Bal_account_type = e.balType
		rec.Bal_account_no = types.NewCode(e.balNo)
		rec.Positive = e.amount > 0
		rec.Source_code = "GENJNL"
		if e.date.Before(monthStart) {
			// reconciled with the month's statement
			rec.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Closed
			rec.Statement_no = types.NewCode(statementNo(e.date))
			s := statements[e.bankAccount]
			if s == nil {
				s = &lastStatement{}
				statements[e.bankAccount] = s
			}
			s.no = statementNo(e.date)
			s.balance = round2(s.balance + rec.Amount.Float64())
		} else {
			rec.Open = true
			rec.Remaining_amount = rec.Amount
		}
		batch = append(batch, rec)
	}
	if len(batch) > 0 {
		err := last.InsertAll(batch, true)
		if err != nil {
			var batchErr *ftables.BatchInsertError
			key, cause := "", err
			if errors.As(err, &batchErr) {
				key, cause = fmt.Sprint(batch[batchErr.Index].Entry_no), batchErr.Err
			}
			return &InsertError{Table: gtables.BankAccountLedgerEntryTableName, Key: key, Err: cause}
		}
	}
	counts.BankLedgerEntries += len(batch)

	for no, s := range statements {
		bankAcc := gtables.NewBankAccount(last)
		if !bankAcc.Get(types.Code(no)) {
			continue
		}
		bankAcc.Last_statement_no = types.NewCode(s.no)
		bankAcc.Balance_last_statement = types.NewDecimal(s.balance)
		err := bankAcc.ModifyOrError(false)
		if err != nil {
			return &InsertError{Table: gtables.BankAccountTableName, Key: no, Err: err}
		}
	}
	return nil
}

// closeDemoBankStatements: the bank entries posted through the general journal (the fixed
// assets' payments and proceeds) are reconciled like the demo payments — before this month on
// the month's statement, closed; each bank account's last statement and its balance follow
// from all its closed entries
func closeDemoBankStatements(src ftables.Source, today time.Time) error {
	monthStart := types.NewDate(today.Year(), today.Month(), 1)
	entry := gtables.NewBankAccountLedgerEntry(src)
	entry.SetRange("open", true)
	entry.SetRange("posting_date", types.Date{}, monthStart.AddDays(-1))
	for found := entry.FindSetBuffered(); found; found = entry.Next() {
		entry.Open = false
		entry.Remaining_amount = types.Decimal{}
		entry.Statement_status = gtables.BankAccountLedgerEntry_Statement_status.Closed
		entry.Statement_no = types.NewCode(entry.Posting_date.Format("ST2006-01"))
		err := entry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	bankAcc := gtables.NewBankAccount(src)
	for found := bankAcc.FindSetBuffered(); found; found = bankAcc.Next() {
		closed := gtables.NewBankAccountLedgerEntry(src)
		closed.SetRange("bank_account_no", bankAcc.No)
		closed.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.Closed)
		if !closed.FindLast() {
			continue
		}
		err := closed.CalcSums("amount")
		if err != nil {
			return err
		}
		last := gtables.NewBankAccountLedgerEntry(src)
		last.SetCurrentKey("bank_account_no", "posting_date")
		last.SetRange("bank_account_no", bankAcc.No)
		last.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.Closed)
		last.FindLast()
		bankAcc.Last_statement_no = types.NewCode(last.Posting_date.Format("ST2006-01"))
		bankAcc.Balance_last_statement = closed.Amount
		err = bankAcc.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// bankAmountFCY: an LCY amount in the bank account's currency at the day's exchange rate
func bankAmountFCY(src ftables.Source, date types.Date, currencyCode types.Code, amountLCY types.Decimal) (types.Decimal, error) {
	rates := tables.ExchangeRates(src)
	factor, err := rates.ExchangeRate(date, currencyCode)
	if err != nil {
		return types.Decimal{}, err
	}
	amount, err := rates.ExchangeAmtLCYToFCY(date, currencyCode, amountLCY, factor)
	if err != nil {
		return types.Decimal{}, err
	}
	return amount.Round(2), nil
}
