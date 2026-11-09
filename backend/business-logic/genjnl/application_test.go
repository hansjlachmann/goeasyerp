package genjnl

import (
	"errors"
	"strings"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const (
	custType = gtables.GenJournalLineAccount_type(1)
	vendType = gtables.GenJournalLineAccount_type(2)
	invoice  = gtables.GenJournalLineDocument_type(2)
	payment  = gtables.GenJournalLineDocument_type(1)
)

// testCustomer is a new customer of a posting group (no entries of its own yet)
func testCustomer(t *testing.T, src ftables.Source, no, postingGroup, currency string, oldest bool) {
	t.Helper()
	cust := gtables.NewCustomer(src)
	cust.No = types.Code(no)
	cust.Name = types.Text("Test " + no)
	cust.Customer_posting_group = types.Code(postingGroup)
	cust.Currency_code = types.Code(currency)
	if oldest {
		cust.Application_method = gtables.Customer_Application_method.ApplyToOldest
	}
	must(t, cust.InsertOrError(false))
}

func custEntry(t *testing.T, src ftables.Source, customerNo, docNo string) *gtables.CustomerLedgerEntry {
	t.Helper()
	entry := gtables.NewCustomerLedgerEntry(src)
	entry.SetRange("customer_no", customerNo)
	entry.SetRange("document_no", docNo)
	if !entry.FindFirst() {
		t.Fatalf("no entry %s of %s", docNo, customerNo)
	}
	return entry
}

func glAmount(src ftables.Source, account, docNo string) types.Decimal {
	entry := gtables.NewGLEntry(src)
	entry.SetRange("gl_account_no", account)
	entry.SetRange("document_no", docNo)
	_ = entry.CalcSums("amount")
	return entry.Amount
}

// An invoice and a payment applied to it by Applies-to Doc. No.: both close, the receivables
// account goes up and down
func TestApplyByDocNo(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC1", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV1", docType: invoice, accType: custType, accNo: "TC1", amount: "1000", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	inv := custEntry(t, src, "TC1", "INV1")
	if !inv.Open || !inv.Remaining_amount.Equal(dec("1000")) || !glAmount(src, "1410", "INV1").Equal(dec("1000")) {
		t.Fatalf("invoice: open %v remaining %s receivables %s", inv.Open, inv.Remaining_amount, glAmount(src, "1410", "INV1"))
	}

	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PAY1", docType: payment, accType: custType, accNo: "TC1", amount: "-1000",
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "INV1"})
	_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
	must(t, err)
	inv = custEntry(t, src, "TC1", "INV1")
	pay := custEntry(t, src, "TC1", "PAY1")
	if inv.Open || pay.Open || !inv.Remaining_amount.IsZero() || inv.Closed_by_entry_no != pay.Entry_no || !inv.Closed_by_amount.Equal(dec("1000")) {
		t.Errorf("after the payment: invoice open %v remaining %s closed by %d (%s), payment open %v",
			inv.Open, inv.Remaining_amount, inv.Closed_by_entry_no, inv.Closed_by_amount, pay.Open)
	}
	if !glAmount(src, "1410", "PAY1").Equal(dec("-1000")) || !glAmount(src, "1520", "PAY1").Equal(dec("1000")) {
		t.Errorf("payment G/L: receivables %s bank %s", glAmount(src, "1410", "PAY1"), glAmount(src, "1520", "PAY1"))
	}
}

// A payment before the payment discount date takes the discount: the invoice closes with the
// discount given, the discount goes to the posting group's account
func TestApplyPaymentDiscount(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC2", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV2", docType: invoice, accType: custType, accNo: "TC2", amount: "1000", balNo: "1210",
		pmtDiscPercent: "2", pmtDiscDay: 25})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	inv := custEntry(t, src, "TC2", "INV2")
	if !inv.Pmt_disc_possible.Equal(dec("20")) {
		t.Fatalf("payment discount possible %s", inv.Pmt_disc_possible)
	}
	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PAY2", docType: payment, accType: custType, accNo: "TC2", amount: "-980", postingDay: 20,
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "INV2"})
	_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
	must(t, err)
	inv = custEntry(t, src, "TC2", "INV2")
	pay := custEntry(t, src, "TC2", "PAY2")
	if inv.Open || pay.Open || !inv.Pmt_disc_given_lcy.Equal(dec("20")) {
		t.Errorf("invoice open %v, payment open %v, discount given %s", inv.Open, pay.Open, inv.Pmt_disc_given_lcy)
	}
	if !glAmount(src, "3150", "PAY2").Equal(dec("20")) || !glAmount(src, "1410", "PAY2").Equal(dec("-1000")) {
		t.Errorf("discount account %s, receivables %s", glAmount(src, "3150", "PAY2"), glAmount(src, "1410", "PAY2"))
	}
}

// A payment with an Applies-to ID closes the entries marked with it; with Apply to Oldest a
// payment without one pays the oldest open entry
func TestApplyByIDAndOldest(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC3", "DOMESTIC", "", false)
	testCustomer(t, src, "TC4", "DOMESTIC", "", true)
	for i, no := range []string{"INV31", "INV32"} {
		addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: (i + 1) * 10000, docNo: no, docType: invoice, accType: custType, accNo: "TC3", amount: "300", balNo: "1210"})
	}
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 30000, docNo: "INV41", docType: invoice, accType: custType, accNo: "TC4", amount: "100", balNo: "1210", postingDay: 10})
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 40000, docNo: "INV42", docType: invoice, accType: custType, accNo: "TC4", amount: "100", balNo: "1210", postingDay: 12})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	for _, no := range []string{"INV31", "INV32"} {
		entry := custEntry(t, src, "TC3", no)
		entry.Applies_to_id = "PAY3"
		must(t, entry.ModifyOrError(false))
	}
	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PAY3", docType: payment, accType: custType, accNo: "TC3", amount: "-600",
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesID: "PAY3"})
	addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 20000, docNo: "PAY4", docType: payment, accType: custType, accNo: "TC4", amount: "-100",
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010"})
	_, err = postBatch(t, src, "CASHRCPT", "DEFAULT")
	must(t, err)
	for _, no := range []string{"INV31", "INV32"} {
		if entry := custEntry(t, src, "TC3", no); entry.Open || entry.Applies_to_id != "" {
			t.Errorf("%s open %v applies-to ID %q", no, entry.Open, entry.Applies_to_id)
		}
	}
	if custEntry(t, src, "TC4", "INV41").Open || !custEntry(t, src, "TC4", "INV42").Open {
		t.Error("Apply to Oldest must pay the oldest invoice")
	}
}

// A payment in a currency at a later exchange rate closes the invoice: the difference is a
// realized exchange-rate gain on the currency's account
func TestApplyExchangeRateGain(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC5", "FOREIGN", "EUR", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV5", docType: invoice, accType: custType, accNo: "TC5", amount: "100", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	inv := custEntry(t, src, "TC5", "INV5")
	if !inv.Open {
		t.Fatal("the invoice is not open after posting")
	}
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
	inv = custEntry(t, src, "TC5", "INV5")
	pay := custEntry(t, src, "TC5", "PAY5")
	gain := glAmount(src, "6140", "PAY5")
	if inv.Open || pay.Open || gain.IsZero() || !glAmount(src, "1420", "INV5").Add(glAmount(src, "1420", "PAY5")).IsZero() {
		t.Errorf("invoice open %v (LCY %s), payment open %v (LCY %s), gain %s, receivables INV5 %s PAY5 %s",
			inv.Open, inv.Amount_lcy, pay.Open, pay.Amount_lcy, gain, glAmount(src, "1420", "INV5"), glAmount(src, "1420", "PAY5"))
	}
	if !gain.Equal(dec("-50")) {
		t.Errorf("realized gain %s, want -50 (100 EUR at 11.50 paid at 12.00)", gain)
	}
}

// Application between currencies: a EUR invoice paid in the local currency by Applies-to Doc.
// No. is refused while Appln. between Currencies is None; with All the payment (1150 at 11.50)
// closes the invoice (100 EUR)
func TestApplyBetweenCurrencies(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC6", "FOREIGN", "EUR", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV6", docType: invoice, accType: custType, accNo: "TC6", amount: "100", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	inv := custEntry(t, src, "TC6", "INV6")
	payLine := lineSpec{lineNo: 10000, docNo: "PAY6", docType: payment, accType: custType, accNo: "TC6", amount: "-" + inv.Amount_lcy.String(), currency: "LCY",
		balNo: "1210", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "INV6"}

	addLine(t, src, "GENERAL", "DEFAULT", payLine)
	_, err = postBatch(t, src, "GENERAL", "DEFAULT")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrJnlApplnSameCurrency {
		t.Fatalf("a payment in another currency with Appln. between Currencies None: %v", err)
	}
	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	if line.Get(map[string]interface{}{"journal_template_name": "GENERAL", "journal_batch_name": "DEFAULT", "line_no": 10000}) {
		must(t, line.DeleteOrError(false))
	}

	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Appln_between_currencies = gtables.SalesReceivablesSetup_Appln_between_currencies.All
	must(t, salesSetup.ModifyOrError(false))
	addLine(t, src, "GENERAL", "DEFAULT", payLine)
	_, err = postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	inv = custEntry(t, src, "TC6", "INV6")
	pay := custEntry(t, src, "TC6", "PAY6")
	if inv.Open || pay.Open || !inv.Remaining_amount.IsZero() || !pay.Remaining_amount.IsZero() {
		t.Errorf("invoice open %v remaining %s, payment open %v remaining %s", inv.Open, inv.Remaining_amount, pay.Open, pay.Remaining_amount)
	}
	// the invoice was closed by 100 EUR, paid in the local currency
	if inv.Closed_by_entry_no != pay.Entry_no || !inv.Closed_by_amount.Equal(dec("100")) || inv.Closed_by_currency_code != "" ||
		!inv.Closed_by_currency_amount.Equal(inv.Amount_lcy) {
		t.Errorf("invoice closed by %d: amount %s, currency %q amount %s", inv.Closed_by_entry_no, inv.Closed_by_amount, inv.Closed_by_currency_code, inv.Closed_by_currency_amount)
	}
}

// An invoice number is posted once; a vendor's external document no. once per vendor
func TestDuplicateDocuments(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC6", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV6", docType: invoice, accType: custType, accNo: "TC6", amount: "10", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV6", docType: invoice, accType: custType, accNo: "TC6", amount: "10", balNo: "1210"})
	_, err = postBatch(t, src, "SALES", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("the same invoice no. again: %v", err)
	}
}

// Vendors post and apply like customers, with the opposite signs and the payables account
func TestVendorInvoiceAndPayment(t *testing.T) {
	src := demoCompany(t)
	vend := gtables.NewVendor(src)
	vend.No = "TV1"
	vend.Name = "Test vendor"
	vend.Vendor_posting_group = "DOMESTIC"
	must(t, vend.InsertOrError(false))
	inv := addLine(t, src, "PURCHASES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PINV1", docType: invoice, accType: vendType, accNo: "TV1", amount: "-500", balNo: "1270"})
	inv.External_document_no = "EXT-1"
	must(t, inv.ModifyOrError(false))
	_, err := postBatch(t, src, "PURCHASES", "DEFAULT")
	must(t, err)
	addLine(t, src, "PAYMENT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PPAY1", docType: payment, accType: vendType, accNo: "TV1", amount: "500",
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "PINV1"})
	_, err = postBatch(t, src, "PAYMENT", "DEFAULT")
	must(t, err)
	entry := gtables.NewVendorLedgerEntry(src)
	entry.SetRange("vendor_no", "TV1")
	entry.SetRange("open", true)
	if entry.Count() != 0 || !glAmount(src, "2310", "PINV1").Equal(dec("-500")) || !glAmount(src, "2310", "PPAY1").Equal(dec("500")) {
		t.Errorf("open vendor entries %d, payables %s / %s", entry.Count(), glAmount(src, "2310", "PINV1"), glAmount(src, "2310", "PPAY1"))
	}
	// the same external document no. of the vendor again
	again := addLine(t, src, "PURCHASES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "PINV2", docType: invoice, accType: vendType, accNo: "TV1", amount: "-10", balNo: "1270"})
	again.External_document_no = "EXT-1"
	must(t, again.ModifyOrError(false))
	_, err = postBatch(t, src, "PURCHASES", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "already exists for this vendor") {
		t.Errorf("the vendor's external document no. again: %v", err)
	}
}

// F9 on Apply Customer Entries marks the entry with the journal line's Applies-to ID (the
// page's parameter) and takes it off again; the footer shows the line's amount, the marked
// entries' amount and the balance
func TestSetApplIDAndTotals(t *testing.T) {
	src := demoCompany(t)
	testCustomer(t, src, "TC7", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "INV7", docType: invoice, accType: custType, accNo: "TC7", amount: "400", balNo: "1210"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	fcodeunits.SetCurrentContext("TESTER", "Tester", testCompany, "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)
	fcodeunits.SetCurrentPageParams(map[string]string{"applies_to_id": "PAY7", "amount": "-400", "posting_date": "2026-10-20", "document_type": "1"})

	inv := custEntry(t, src, "TC7", "INV7")
	_, err = SetCustApplID(inv)
	must(t, err)
	if custEntry(t, src, "TC7", "INV7").Applies_to_id != "PAY7" {
		t.Fatal("F9 must mark the entry with the line's Applies-to ID")
	}
	control, err := pages.RunAfterGetRecordControl(ApplyCustEntriesPageID, custEntry(t, src, "TC7", "INV7"))
	must(t, err)
	values := control.Values()
	if values["appln_amount"] != "-400" || values["applied_amount"] != "400" || values["appln_balance"] != "0" {
		t.Errorf("totals %v", values)
	}
	_, err = SetCustApplID(custEntry(t, src, "TC7", "INV7"))
	must(t, err)
	if custEntry(t, src, "TC7", "INV7").Applies_to_id != "" {
		t.Error("F9 again must take the mark off")
	}
	// without a journal line's ID: the user ID
	fcodeunits.SetCurrentPageParams(nil)
	_, err = SetCustApplID(custEntry(t, src, "TC7", "INV7"))
	must(t, err)
	if custEntry(t, src, "TC7", "INV7").Applies_to_id != "TESTER" {
		t.Errorf("no Applies-to ID: %q", custEntry(t, src, "TC7", "INV7").Applies_to_id)
	}
}
