package genjnl

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// withAddCurrency: euro is the additional reporting currency (1 EUR = 11.50 in the demo), its
// residuals on 6160/6170
func withAddCurrency(t *testing.T, src ftables.Source) {
	t.Helper()
	currency := gtables.NewCurrency(src)
	must(t, currency.GetOrError(types.Code("EUR")))
	currency.Residual_gains_account = "6160"
	currency.Residual_losses_account = "6170"
	must(t, currency.ModifyOrError(false))
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Additional_reporting_currency = "EUR"
	must(t, glSetup.ModifyOrError(false))
}

// glAddCurr: the additional-currency amount of an account's entries of a document
func glAddCurr(src ftables.Source, account, docNo string) types.Decimal {
	entry := gtables.NewGLEntry(src)
	entry.SetRange("gl_account_no", account)
	entry.SetRange("document_no", docNo)
	_ = entry.CalcSums("additional_currency_amount")
	return entry.Additional_currency_amount.Round(2)
}

// docAddCurrBalanced: the additional-currency amounts of a document's entries add up to 0
func docAddCurrBalanced(t *testing.T, src ftables.Source, docNo string) {
	t.Helper()
	entry := gtables.NewGLEntry(src)
	entry.SetRange("document_no", docNo)
	must(t, entry.CalcSums("additional_currency_amount", "add_currency_debit_amount", "add_currency_credit_amount"))
	if !entry.Additional_currency_amount.Round(2).IsZero() || !entry.Add_currency_debit_amount.Round(2).Equal(entry.Add_currency_credit_amount.Round(2)) {
		t.Errorf("%s: additional-currency amounts %s (debit %s, credit %s)", docNo, entry.Additional_currency_amount,
			entry.Add_currency_debit_amount, entry.Add_currency_credit_amount)
	}
}

// An LCY line gets its amounts exchanged at the posting date's rate (1000 / 11.50 = 86.96), with
// their debit and credit; without an additional reporting currency they stay 0
func TestAddCurrencyLCYLine(t *testing.T) {
	src := demoCompany(t)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000", balNo: "1230"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAddCurr(src, "1210", "G00001").IsZero() {
		t.Fatal("an additional-currency amount without an additional reporting currency")
	}

	withAddCurrency(t, src)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00002", accNo: "1210", amount: "1000", balNo: "1230"})
	_, err = postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if got := glAddCurr(src, "1210", "G00002"); !got.Equal(dec("86.96")) {
		t.Errorf("1210: %s, want 86.96", got)
	}
	entry := gtables.NewGLEntry(src)
	entry.SetRange("gl_account_no", "1230")
	entry.SetRange("document_no", "G00002")
	if !entry.FindFirst() || !entry.Additional_currency_amount.Equal(dec("-86.96")) || !entry.Add_currency_credit_amount.Equal(dec("86.96")) {
		t.Errorf("1230: %s credit %s", entry.Additional_currency_amount, entry.Add_currency_credit_amount)
	}
	docAddCurrBalanced(t, src, "G00002")
}

// A line in the additional currency keeps its amounts exactly (100 EUR, not 1150 exchanged back)
func TestAddCurrencySourceCurrency(t *testing.T) {
	src := demoCompany(t)
	withAddCurrency(t, src)
	testCustomer(t, src, "TC5", "FOREIGN", "EUR", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV5", docType: invoice, accType: custType, accNo: "TC5", amount: "100", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	if got := glAddCurr(src, "1420", "INV5"); !got.Equal(dec("100")) {
		t.Errorf("receivables %s, want 100", got)
	}
	if got := glAmount(src, "1420", "INV5"); !got.Equal(dec("1150")) {
		t.Errorf("receivables LCY %s, want 1150", got)
	}
	docAddCurrBalanced(t, src, "INV5")

	// paid at 12.00: the realized gain is LCY only, the receivables close in euro
	rate := gtables.NewCurrencyExchangeRate(src)
	rate.Currency_code = "EUR"
	rate.Starting_date = date(t, 20, 10, 2026)
	rate.Exchange_rate_amount = dec("100")
	rate.Adjustment_exch_rate_amount = dec("100")
	rate.Relational_exch_rate_amount = dec("1200")
	rate.Relational_adjmt_exch_rate_amt = dec("1200")
	must(t, rate.InsertOrError(false))
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PAY5", docType: payment, accType: custType, accNo: "TC5", amount: "-100", postingDay: 25,
		balNo: "1210", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "INV5"})
	_, err = postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if got := glAddCurr(src, "1420", "INV5").Add(glAddCurr(src, "1420", "PAY5")); !got.IsZero() {
		t.Errorf("receivables in euro after the payment: %s", got)
	}
	if got := glAddCurr(src, "6140", "PAY5"); !got.IsZero() {
		t.Errorf("the realized gain in euro: %s, want 0", got)
	}
	docAddCurrBalanced(t, src, "PAY5")
}

// Amounts that balance in LCY but not once exchanged (1.00 against 0.50 + 0.50: 0.09 against
// 0.04 + 0.04) leave a residual: an entry of amount 0 on the residual gains account
func TestAddCurrencyResidual(t *testing.T) {
	src := demoCompany(t)
	withAddCurrency(t, src)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00003", accNo: "1210", amount: "1"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 20000, docNo: "G00003", accNo: "1230", amount: "-0.50"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 30000, docNo: "G00003", accNo: "1230", amount: "-0.50"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	residual := gtables.NewGLEntry(src)
	residual.SetRange("document_no", "G00003")
	residual.SetRange("gl_account_no", "6160")
	if !residual.FindFirst() || !residual.Amount.IsZero() || !residual.Additional_currency_amount.Equal(dec("-0.01")) || !residual.System_created_entry {
		t.Errorf("residual entry: amount %s, additional currency %s", residual.Amount, residual.Additional_currency_amount)
	}
	docAddCurrBalanced(t, src, "G00003")
}

// A payment with payment discount: the discount and the receivables in euro too, balanced
func TestAddCurrencyPaymentDiscount(t *testing.T) {
	src := demoCompany(t)
	withAddCurrency(t, src)
	testCustomer(t, src, "TC2", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV2", docType: invoice, accType: custType, accNo: "TC2", amount: "1000", balNo: "1210",
		pmtDiscPercent: "2", pmtDiscDay: 25})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PAY2", docType: payment, accType: custType, accNo: "TC2", amount: "-980", postingDay: 20,
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "INV2"})
	_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
	must(t, err)
	if got := glAddCurr(src, "3150", "PAY2"); !got.Equal(dec("1.74")) {
		t.Errorf("payment discount in euro %s, want 1.74 (20 / 11.50)", got)
	}
	docAddCurrBalanced(t, src, "PAY2")
}

// A sale with sales tax: the tax entry and the tax account in euro at the entry's factor
func TestAddCurrencySalesTax(t *testing.T) {
	src := demoCompany(t)
	withAddCurrency(t, src)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00004", accNo: "3110", amount: "-1250", balNo: "1210"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", "G00004")
	if !taxEntry.FindFirst() || !taxEntry.Additional_currency_amount.Equal(dec("-21.74")) || !taxEntry.Additional_currency_base.Equal(dec("-86.96")) {
		t.Errorf("tax entry in euro: amount %s base %s", taxEntry.Additional_currency_amount, taxEntry.Additional_currency_base)
	}
	if got := glAddCurr(src, "2330", "G00004"); !got.Equal(dec("-21.74")) {
		t.Errorf("tax account in euro %s", got)
	}
	docAddCurrBalanced(t, src, "G00004")
}
