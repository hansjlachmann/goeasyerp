package reports_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Adjust Exchange Rates after the euro and the pound rose (adjustment rates 12.00 and 14.00 from
// 1 October): the GBP bank account and the open EUR customer entries are worth their amounts at
// the new rates in LCY, the differences posted (the G/L balanced), a register per account type,
// posting group and currency, the currencies adjusted on the posting date
func TestAdjustExchangeRates(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	mustOK(t, err)
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "demo", database.DBTypeSQLite)

	// a customer invoice and a vendor invoice in euro at the old rate (11.50)
	post := genjnl.NewGenJnlPostLine(src)
	for i, l := range []struct {
		accType gtables.GenJournalLineAccount_type
		amount  int64
	}{{gtables.GenJournalLine_Account_type.Customer, 1000}, {gtables.GenJournalLine_Account_type.Vendor, -800}} {
		var accNo types.Code
		if l.accType == gtables.GenJournalLine_Account_type.Customer {
			cust := gtables.NewCustomer(src)
			cust.SetRange("customer_posting_group", types.Code("FOREIGN"))
			if !cust.FindFirst() {
				t.Fatal("no foreign customer")
			}
			accNo = cust.No
		} else {
			vend := gtables.NewVendor(src)
			vend.SetRange("vendor_posting_group", types.Code("FOREIGN"))
			if !vend.FindFirst() {
				t.Fatal("no foreign vendor")
			}
			accNo = vend.No
		}
		line := gtables.NewGenJournalLine(src)
		line.Init()
		line.Line_no = (i + 1) * 10000
		mustOK(t, line.Validate("posting_date", types.NewDate(2026, 9, 15)))
		line.Document_type = gtables.GenJournalLine_Document_type.Invoice
		line.Document_no = types.Code("EURINV" + string(rune('1'+i)))
		line.External_document_no = "EXT-EUR"
		line.Account_type = l.accType
		mustOK(t, line.Validate("account_no", accNo))
		mustOK(t, line.Validate("currency_code", types.Code("EUR")))
		mustOK(t, line.Validate("amount", types.NewDecimalFromInt(l.amount)))
		mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
		mustOK(t, post.Run(line))
	}

	for code, rate := range map[string]int64{"EUR": 1200, "GBP": 1400} {
		r := gtables.NewCurrencyExchangeRate(src)
		r.Currency_code = types.Code(code)
		r.Starting_date = types.NewDate(2026, 10, 1)
		r.Exchange_rate_amount = types.NewDecimalFromInt(100)
		r.Adjustment_exch_rate_amount = types.NewDecimalFromInt(100)
		r.Relational_exch_rate_amount = types.NewDecimalFromInt(rate)
		r.Relational_adjmt_exch_rate_amt = types.NewDecimalFromInt(rate)
		mustOK(t, r.InsertOrError(false))
	}
	bank := gtables.NewBankAccount(src)
	bank.SetRange("currency_code", types.Code("GBP"))
	if !bank.FindFirst() {
		t.Fatal("no GBP bank account in the demo")
	}
	mustOK(t, bank.SetFlowFilter("date_filter", "..2026-10-31"))
	bank.CalcFields("balance_at_date")
	if bank.Balance_at_date.IsZero() {
		t.Fatal("the GBP bank account has no balance")
	}

	_, err = reports.Execute(reports.AdjustExchangeRatesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"ending_date": "2026-10-31", "posting_date": "2026-10-31", "document_no": "EXCH-2610",
			"posting_description": "Exchange Rate Adjmt. of %1 %2"}})
	mustOK(t, err)

	bank.CalcFields("balance_at_date", "balance_at_date_lcy")
	if want := bank.Balance_at_date.Mul(types.NewDecimalFromInt(14)).Round(2); !bank.Balance_at_date_lcy.Round(2).Equal(want) {
		t.Errorf("GBP bank in LCY %s, want %s", bank.Balance_at_date_lcy, want)
	}
	entries := gtables.NewCustomerLedgerEntry(src)
	entries.SetRange("open", true)
	entries.SetRange("currency_code", types.Code("EUR"))
	n := 0
	for found := entries.FindSet(); found; found = entries.Next() {
		n++
		if want := entries.Remaining_amount.Mul(types.NewDecimalFromInt(12)).Round(2); !entries.Remaining_amt_lcy.Equal(want) {
			t.Errorf("entry %d: remaining %s EUR = %s LCY, want %s", entries.Entry_no, entries.Remaining_amount, entries.Remaining_amt_lcy, want)
		}
	}
	if n == 0 {
		t.Error("no open EUR customer entries")
	}
	vendEntry := gtables.NewVendorLedgerEntry(src)
	vendEntry.SetRange("document_no", types.Code("EURINV2"))
	if !vendEntry.FindFirst() || !vendEntry.Remaining_amt_lcy.Equal(types.NewDecimalFromInt(-9600)) {
		t.Errorf("vendor entry: %s EUR = %s LCY, want -9600", vendEntry.Remaining_amount, vendEntry.Remaining_amt_lcy)
	}
	gains := gtables.NewGLEntry(src)
	gains.SetRange("document_no", types.Code("EXCH-2610"))
	gains.SetRange("gl_account_no", types.Code("6160"))
	mustOK(t, gains.CalcSums("amount"))
	if !gains.Amount.Equal(types.NewDecimalFromInt(-500)) {
		t.Errorf("unrealized gains %s, want -500 (the customer's 1,000 EUR at 12.00 instead of 11.50)", gains.Amount)
	}
	reg := gtables.NewExchRateAdjmtReg(src)
	reg.SetRange("account_type", gtables.ExchRateAdjmtReg_Account_type.BankAccount)
	reg.SetRange("currency_code", types.Code("GBP"))
	if !reg.FindFirst() || reg.Adjusted_amt_lcy.IsZero() {
		t.Error("no register of the GBP bank adjustment")
	}
	reg.SetRange("account_type", gtables.ExchRateAdjmtReg_Account_type.Customer)
	reg.SetRange("currency_code", types.Code("EUR"))
	if !reg.FindFirst() {
		t.Error("no register of the EUR customer adjustment")
	}
	cur := gtables.NewCurrency(src)
	mustOK(t, cur.GetOrError(types.Code("EUR")))
	if !cur.Last_date_adjusted.Equal(types.NewDate(2026, 10, 31)) {
		t.Errorf("EUR last date adjusted %s", cur.Last_date_adjusted)
	}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("EXCH-2610"))
	if glEntry.Count() == 0 {
		t.Error("no G/L entries")
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L: %+v", r)
	}
}
