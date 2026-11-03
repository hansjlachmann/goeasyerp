package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/accsched"
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// acyTotals: the G/L entries' LCY and additional-currency totals (rounded)
func acyTotals(t *testing.T, src ftables.Source) (types.Decimal, types.Decimal) {
	t.Helper()
	glEntry := gtables.NewGLEntry(src)
	mustOK(t, glEntry.CalcSums("amount", "additional_currency_amount"))
	return glEntry.Amount.Round(2), glEntry.Additional_currency_amount.Round(2)
}

// Adjust Add. Reporting Currency to euro: every posted G/L entry gets its amount in euro, the
// whole G/L balances in euro too (residuals on the residual accounts), the open tax entries are
// exchanged, the setup has the currency; entries posted afterwards get their euro amounts from
// the posting; removing the currency again leaves the entries as they are
func TestAdjustAddReportingCurrency(t *testing.T) {
	src := listCompany(t)
	// an open tax entry
	post := genjnl.NewGenJnlPostLine(src)
	line := gtables.NewGenJournalLine(src)
	line.Init()
	mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 15)))
	line.Document_no = "TAX1"
	mustOK(t, line.Validate("account_no", types.Code("3110")))
	mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
	mustOK(t, line.Validate("amount", types.NewDecimalFromInt(-1250)))
	mustOK(t, post.Run(line))

	options := map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	if acy := gtables.GetGeneralLedgerSetup(src).Additional_reporting_currency; acy != "EUR" {
		t.Fatalf("setup currency %q", acy)
	}
	lcy, acy := acyTotals(t, src)
	if !lcy.IsZero() || !acy.IsZero() {
		t.Errorf("G/L totals: LCY %s, euro %s", lcy, acy)
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L check: %+v", r)
	}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("TAX1"))
	glEntry.SetRange("gl_account_no", types.Code("1210"))
	if !glEntry.FindFirst() || !glEntry.Additional_currency_amount.Equal(types.MustDecimal("108.70")) || !glEntry.Add_currency_debit_amount.Equal(types.MustDecimal("108.70")) {
		t.Errorf("1210 in euro: %s (debit %s), want 108.70", glEntry.Additional_currency_amount, glEntry.Add_currency_debit_amount)
	}
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", types.Code("TAX1"))
	if !taxEntry.FindFirst() || !taxEntry.Additional_currency_amount.Equal(types.MustDecimal("-21.74")) {
		t.Errorf("tax entry in euro %s", taxEntry.Additional_currency_amount)
	}

	// posted afterwards: the posting gives the euro amounts
	post = genjnl.NewGenJnlPostLine(src)
	line = gtables.NewGenJournalLine(src)
	line.Init()
	mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 16)))
	line.Document_no = "AFTER1"
	mustOK(t, line.Validate("account_no", types.Code("1210")))
	mustOK(t, line.Validate("bal_account_no", types.Code("1230")))
	mustOK(t, line.Validate("amount", types.NewDecimalFromInt(1000)))
	mustOK(t, post.Run(line))
	glEntry.SetRange("document_no", types.Code("AFTER1"))
	if !glEntry.FindFirst() || !glEntry.Additional_currency_amount.Equal(types.MustDecimal("86.96")) {
		t.Errorf("posted afterwards in euro %s", glEntry.Additional_currency_amount)
	}

	options["additional_reporting_currency"] = ""
	_, err = reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	if acy := gtables.GetGeneralLedgerSetup(src).Additional_reporting_currency; acy != "" {
		t.Errorf("setup currency %q after removing it", acy)
	}
	if !glEntry.FindFirst() || glEntry.Additional_currency_amount.IsZero() {
		t.Error("removing the currency changed the posted entries")
	}
}

// Adjust Exchange Rates with Adjust G/L Accounts for Add.-Reporting Currency after the euro rose
// to 12.00: an account adjusting its euro amount gets its LCY balance in euro at the new rate, an
// account adjusting its amount its euro balance in LCY; the open tax entries get their euro
// amounts at the new rate (Tax Exchange Rate Adjustment); the totals go to the realized G/L
// gains/losses accounts, so the G/L still balances in both currencies; a register of account type
// G/L Account
func TestAdjustExchangeRatesAddCurrency(t *testing.T) {
	src := listCompany(t)
	post := genjnl.NewGenJnlPostLine(src)
	line := gtables.NewGenJournalLine(src)
	line.Init()
	mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 15)))
	line.Document_no = "TAX1"
	mustOK(t, line.Validate("account_no", types.Code("3110")))
	mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
	mustOK(t, line.Validate("amount", types.NewDecimalFromInt(-1250)))
	mustOK(t, post.Run(line))
	options := map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)

	rate := gtables.NewCurrencyExchangeRate(src)
	rate.Currency_code = "EUR"
	rate.Starting_date = types.NewDate(2026, 10, 20)
	rate.Exchange_rate_amount = types.NewDecimalFromInt(100)
	rate.Adjustment_exch_rate_amount = types.NewDecimalFromInt(100)
	rate.Relational_exch_rate_amount = types.NewDecimalFromInt(1200)
	rate.Relational_adjmt_exch_rate_amt = types.NewDecimalFromInt(1200)
	mustOK(t, rate.InsertOrError(false))
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Tax_exchange_rate_adjustment = gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.AdjustAdditionalCurrencyAmount
	mustOK(t, glSetup.ModifyOrError(false))
	glAcc := gtables.NewGLAccount(src)
	mustOK(t, glAcc.GetOrError(types.Code("1210")))
	glAcc.Exchange_rate_adjustment = gtables.GLAccount_Exchange_rate_adjustment.AdjustAdditionalCurrencyAmount
	mustOK(t, glAcc.ModifyOrError(false))
	mustOK(t, glAcc.GetOrError(types.Code("1230")))
	glAcc.Exchange_rate_adjustment = gtables.GLAccount_Exchange_rate_adjustment.AdjustAmount
	mustOK(t, glAcc.ModifyOrError(false))

	_, err = reports.Execute(reports.AdjustExchangeRatesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: map[string]string{
		"posting_date": "2026-10-31", "document_no": "ADJACY", "adjust_cust_vend_bank": "false", "adjust_gl_accounts": "true"}})
	mustOK(t, err)

	twelve := types.NewDecimalFromInt(12)
	mustOK(t, glAcc.GetOrError(types.Code("1210")))
	glAcc.CalcFields("balance_at_date", "add_currency_balance_at_date")
	if want := glAcc.Balance_at_date.Div(twelve).Round(2); !glAcc.Add_currency_balance_at_date.Round(2).Equal(want) {
		t.Errorf("1210 in euro %s, want %s (LCY %s at 12.00)", glAcc.Add_currency_balance_at_date, want, glAcc.Balance_at_date)
	}
	mustOK(t, glAcc.GetOrError(types.Code("1230")))
	glAcc.CalcFields("balance_at_date", "add_currency_balance_at_date")
	if want := glAcc.Add_currency_balance_at_date.Mul(twelve).Round(2); !glAcc.Balance_at_date.Round(2).Equal(want) {
		t.Errorf("1230 in LCY %s, want %s (euro %s at 12.00)", glAcc.Balance_at_date, want, glAcc.Add_currency_balance_at_date)
	}
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", types.Code("TAX1"))
	if !taxEntry.FindFirst() || !taxEntry.Additional_currency_amount.Equal(types.MustDecimal("-20.83")) {
		t.Errorf("tax entry in euro %s, want -20.83", taxEntry.Additional_currency_amount)
	}
	lcy, acy := acyTotals(t, src)
	if !lcy.IsZero() || !acy.IsZero() {
		t.Errorf("G/L totals: LCY %s, euro %s", lcy, acy)
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L check: %+v", r)
	}
	reg := gtables.NewExchRateAdjmtReg(src)
	reg.SetRange("account_type", gtables.ExchRateAdjmtReg_Account_type.GLAccount)
	if !reg.FindFirst() || reg.Currency_code != "EUR" || reg.Adjusted_amt_lcy.IsZero() || reg.Adjusted_amt_add_curr.IsZero() {
		t.Errorf("register: %+v", reg)
	}
}

// Calc. and Post Tax Settlement with euro as the additional reporting currency: the settlement
// clears the tax accounts in euro too (the lines carry the entries' euro amounts) and Show
// Amounts in Add. Reporting Currency prints the euro total
func TestCalcAndPostTaxSettlementAddCurrency(t *testing.T) {
	src := listCompany(t)
	options := map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	post := genjnl.NewGenJnlPostLine(src)
	for i, l := range []struct {
		accNo  types.Code
		amount int64
	}{{"3110", -1250}, {"4110", 500}} {
		line := gtables.NewGenJournalLine(src)
		line.Init()
		mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 15)))
		line.Document_no = types.Code("TXA" + string(rune('1'+i)))
		mustOK(t, line.Validate("account_no", l.accNo))
		mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
		mustOK(t, line.Validate("amount", types.NewDecimalFromInt(l.amount)))
		mustOK(t, post.Run(line))
	}
	open := gtables.NewTaxEntry(src)
	open.SetRange("closed", false)
	open.SetRange("type", gtables.TaxEntry_Type.Purchase, gtables.TaxEntry_Type.Sale)
	open.SetRange("use_tax", false)
	mustOK(t, open.CalcSums("additional_currency_amount"))
	want := open.Additional_currency_amount

	settle := map[string]string{"posting_date": "2026-10-31", "document_no": "TAXSET-A", "settlement_account": "2340",
		"show_amounts_in_add_currency": "true", "post": "true"}
	result, err := reports.Execute(reports.CalcAndPostTaxSettlementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: settle})
	mustOK(t, err)
	if texts := printedTexts(result, 0); !strings.Contains(texts, "EUR") || !strings.Contains(texts, amountText(want)) {
		t.Errorf("euro total %s not printed: %s", amountText(want), texts)
	}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("TAXSET-A"))
	glEntry.SetRange("gl_account_no", types.Code("2340"))
	mustOK(t, glEntry.CalcSums("additional_currency_amount"))
	if !glEntry.Additional_currency_amount.Round(2).Equal(want.Round(2)) {
		t.Errorf("settlement account in euro %s, want %s", glEntry.Additional_currency_amount, want)
	}
	for _, acc := range []types.Code{"2330", "1440"} {
		gl := gtables.NewGLEntry(src)
		gl.SetRange("gl_account_no", acc)
		gl.SetFilter("document_no", "TXA1|TXA2|TAXSET-A")
		mustOK(t, gl.CalcSums("amount", "additional_currency_amount"))
		if !gl.Amount.IsZero() || !gl.Additional_currency_amount.Round(2).IsZero() {
			t.Errorf("tax account %s: %s, euro %s after the settlement", acc, gl.Amount, gl.Additional_currency_amount)
		}
	}
	lcy, acy := acyTotals(t, src)
	if !lcy.IsZero() || !acy.IsZero() {
		t.Errorf("G/L totals: LCY %s, euro %s", lcy, acy)
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L check: %+v", r)
	}
}

// Close Income Statement with euro as the additional reporting currency: the closing entries are
// posted at once (the batch stays empty) and clear the income statement up to the closing date
// in LCY and in euro
func TestCloseIncomeStatementAddCurrency(t *testing.T) {
	src := listCompany(t)
	options := map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	closedYear(t, src)
	_, err = reports.Execute(reports.CloseIncomeStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: map[string]string{
		"journal_template": "GENERAL", "journal_batch": "DEFAULT", "document_no": "CLOSE25", "retained_earnings_acc": "2120",
		"fiscal_year_ending_date": "2025-12-31"}})
	mustOK(t, err)
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("GENERAL"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	lines.SetRange("document_no", types.Code("CLOSE25"))
	if n := lines.Count(); n != 0 {
		t.Errorf("%d lines written into the batch", n)
	}
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	glAcc.SetRange("income_balance", gtables.GLAccount_Income_balance.IncomeStatement)
	mustOK(t, glAcc.SetFlowFilter("date_filter", "2025-01-01..C2025-12-31"))
	var lcy, acy types.Decimal
	for found := glAcc.FindSet(); found; found = glAcc.Next() {
		glAcc.CalcFields("net_change", "additional_currency_net_change")
		lcy = lcy.Add(glAcc.Net_change)
		acy = acy.Add(glAcc.Additional_currency_net_change)
	}
	if !lcy.Round(2).IsZero() || !acy.Round(2).IsZero() {
		t.Errorf("income statement up to the closing date: LCY %s, euro %s", lcy, acy)
	}
	closing := gtables.NewGLEntry(src)
	closing.SetRange("document_no", types.Code("CLOSE25"))
	if closing.Count() == 0 {
		t.Error("no closing entries posted")
	}
	totalLCY, totalACY := acyTotals(t, src)
	if !totalLCY.IsZero() || !totalACY.IsZero() {
		t.Errorf("G/L totals: LCY %s, euro %s", totalLCY, totalACY)
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L check: %+v", r)
	}
}

// The G/L reports in euro (Use Additional Reporting Currency): the trial balances print the
// accounts' euro amounts under "Amounts are in Euro", the account schedule and its overview
// calculate the cells from the euro amounts
func TestGLReportsAddCurrency(t *testing.T) {
	src := listCompany(t)
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}})
	mustOK(t, err)
	glAcc := gtables.NewGLAccount(src)
	mustOK(t, glAcc.GetOrError(types.Code("1210")))
	glAcc.CalcFields("add_currency_balance_at_date", "balance_at_date")
	euro := glAcc.Add_currency_balance_at_date
	if euro.IsZero() || euro.Equal(glAcc.Balance_at_date) {
		t.Fatalf("1210 in euro %s (LCY %s)", euro, glAcc.Balance_at_date)
	}
	for _, id := range []int{reports.TrialBalanceID, reports.TrialBalancePrevYearID} {
		texts := runList(t, src, id, map[string]string{"use_add_rpt_curr": "true"})
		if !strings.Contains(texts, "Amounts are in Euro") || !strings.Contains(texts, amountText(euro.Abs())) {
			t.Errorf("report %d in euro: %s not printed: %.300s", id, amountText(euro.Abs()), texts)
		}
		if lcy := runList(t, src, id, nil); strings.Contains(lcy, "Amounts are in") {
			t.Errorf("report %d in LCY names a currency", id)
		}
	}
	texts := runList(t, src, reports.AccountScheduleID, map[string]string{"use_add_rpt_curr": "true"})
	if !strings.Contains(texts, "Amounts are in Euro") {
		t.Errorf("account schedule in euro: %.300s", texts)
	}

	name := gtables.NewAccScheduleName(src)
	if !name.FindFirst() {
		t.Fatal("no account schedule")
	}
	line := gtables.NewAccScheduleLine(src)
	line.SetRange("schedule_name", name.Name)
	mustOK(t, line.SetFlowFilter("date_filter", "2025-11-01..2026-10-31"))
	_, lcyCells, err := accsched.Overview(name, line)
	mustOK(t, err)
	mustOK(t, line.SetFlowFilter("use_add_rpt_curr", "Yes"))
	_, euroCells, err := accsched.Overview(name, line)
	mustOK(t, err)
	different := false
	for key, row := range lcyCells {
		for i := range row {
			if euroCells[key][i] != row[i] {
				different = true
			}
		}
	}
	if !different {
		t.Error("the overview in euro shows the LCY cells")
	}
}
