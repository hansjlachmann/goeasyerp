package genjnl

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// testGLAccount adds a posting account
func testGLAccount(t *testing.T, src ftables.Source, no, name string) {
	t.Helper()
	glAcc := gtables.NewGLAccount(src)
	glAcc.No = types.Code(no)
	glAcc.Name = types.Text(name)
	must(t, glAcc.InsertOrError(false))
}

// glTotal: all G/L entries add up to 0 (SQLite sums decimals as floating point: rounded)
func glTotal(t *testing.T, src ftables.Source) {
	t.Helper()
	glEntry := gtables.NewGLEntry(src)
	must(t, glEntry.CalcSums("amount"))
	if !system.Round(glEntry.Amount).IsZero() {
		t.Errorf("G/L entries add up to %s", glEntry.Amount)
	}
}

// docTaxEntries: the tax entries of a document, in entry order
func docTaxEntries(src ftables.Source, docNo string) []*gtables.TaxEntry {
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", docNo)
	var entries []*gtables.TaxEntry
	for found := taxEntry.FindSet(); found; found = taxEntry.Next() {
		e := gtables.NewTaxEntry(src)
		e.Copy(taxEntry)
		entries = append(entries, e)
	}
	return entries
}

// unrealizedVATSetup: DOMESTIC/STANDARD with an Unrealized Tax Type, the unrealized accounts 2331
// (sales) and 1441 (purchases)
func unrealizedVATSetup(t *testing.T, src ftables.Source, unrealizedTaxType gtables.TaxPostingSetupUnrealized_tax_type) {
	t.Helper()
	testGLAccount(t, src, "2331", "Sales VAT, unrealized")
	testGLAccount(t, src, "1441", "Purchase VAT, unrealized")
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Unrealized_tax = true
	must(t, glSetup.ModifyOrError(false))
	setup := gtables.NewTaxPostingSetup(src)
	must(t, setup.GetOrError(map[string]interface{}{"tax_bus_posting_group": "DOMESTIC", "tax_prod_posting_group": "STANDARD"}))
	must(t, setup.Validate("unrealized_tax_type", unrealizedTaxType))
	must(t, setup.Validate("sales_vat_unreal_account", types.Code("2331")))
	must(t, setup.Validate("purch_vat_unreal_account", types.Code("1441")))
	must(t, setup.ModifyOrError(false))
}

// Unrealized VAT, Percentage: the invoice's VAT goes onto the unrealized account and into the tax
// entry's Unrealized Amount/Base; each payment realizes its share (500 of 1250: 40 % of the VAT),
// the payment closing the invoice the rest; unapplying the last payment makes its part unrealized
// again; the General Ledger Setup keeps Unrealized Tax while a setup uses it
func TestUnrealizedVATPercentage(t *testing.T) {
	src := vatCompany(t)
	unrealizedVATSetup(t, src, gtables.TaxPostingSetup_Unrealized_tax_type.Percentage)
	testCustomer(t, src, "TU1", "DOMESTIC", "", false)
	addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "UINV1", docType: invoice, accType: custType, accNo: "TU1", amount: "1250", balNo: "3110"})
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	if !glAmount(src, "2331", "UINV1").Equal(dec("-250")) || !glAmount(src, "2330", "UINV1").IsZero() {
		t.Fatalf("invoice: unrealized VAT %s, VAT %s", glAmount(src, "2331", "UINV1"), glAmount(src, "2330", "UINV1"))
	}
	invTax := vatTaxEntry(t, src, "UINV1")
	if !invTax.Amount.IsZero() || !invTax.Base.IsZero() || !invTax.Unrealized_amount.Equal(dec("-250")) || !invTax.Unrealized_base.Equal(dec("-1000")) ||
		!invTax.Remaining_unrealized_amount.Equal(dec("-250")) {
		t.Fatalf("invoice tax entry: amount %s base %s unrealized %s / %s remaining %s", invTax.Amount, invTax.Base,
			invTax.Unrealized_amount, invTax.Unrealized_base, invTax.Remaining_unrealized_amount)
	}

	pay := func(docNo, amount string) {
		t.Helper()
		addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: docNo, docType: payment, accType: custType, accNo: "TU1", amount: amount, postingDay: 20,
			balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "UINV1"})
		_, err := postBatch(t, src, "CASHRCPT", "DEFAULT")
		must(t, err)
	}
	pay("UPAY1", "-500")
	if !glAmount(src, "2331", "UPAY1").Equal(dec("100")) || !glAmount(src, "2330", "UPAY1").Equal(dec("-100")) {
		t.Errorf("first payment: unrealized %s, realized %s", glAmount(src, "2331", "UPAY1"), glAmount(src, "2330", "UPAY1"))
	}
	realized := vatTaxEntry(t, src, "UPAY1")
	if !realized.Amount.Equal(dec("-100")) || !realized.Base.Equal(dec("-400")) || realized.Unrealized_tax_entry_no != invTax.Entry_no ||
		realized.Type != gtables.TaxEntry_Type.Sale {
		t.Errorf("realized tax entry: amount %s base %s of %d", realized.Amount, realized.Base, realized.Unrealized_tax_entry_no)
	}
	invTax = vatTaxEntry(t, src, "UINV1")
	if !invTax.Remaining_unrealized_amount.Equal(dec("-150")) || !invTax.Remaining_unrealized_base.Equal(dec("-600")) {
		t.Errorf("remaining unrealized %s / %s", invTax.Remaining_unrealized_amount, invTax.Remaining_unrealized_base)
	}

	pay("UPAY2", "-750")
	if !glAmount(src, "2330", "UPAY2").Equal(dec("-150")) || !vatTaxEntry(t, src, "UINV1").Remaining_unrealized_amount.IsZero() {
		t.Errorf("closing payment: realized %s, remaining %s", glAmount(src, "2330", "UPAY2"), vatTaxEntry(t, src, "UINV1").Remaining_unrealized_amount)
	}
	sales := gtables.NewGLEntry(src)
	sales.SetRange("gl_account_no", "2331")
	sales.SetFilter("document_no", "UINV1|UPAY1|UPAY2")
	must(t, sales.CalcSums("amount"))
	if !sales.Amount.IsZero() {
		t.Errorf("unrealized account after the invoice is paid: %s", sales.Amount)
	}
	glTotal(t, src)

	// unapplying the closing payment: its VAT is unrealized again
	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		_, err := UnapplyCustEntry(txSrc, custEntry(t, txSrc, "TU1", "UPAY2"))
		return err
	}))
	if got := vatTaxEntry(t, src, "UINV1").Remaining_unrealized_amount; !got.Equal(dec("-150")) {
		t.Errorf("remaining unrealized after unapply %s", got)
	}
	entries := docTaxEntries(src, "UPAY2")
	var total types.Decimal
	for _, e := range entries {
		total = total.Add(e.Amount)
	}
	if len(entries) != 2 || !total.IsZero() || !glAmount(src, "2330", "UPAY2").IsZero() || !glAmount(src, "2331", "UPAY2").IsZero() {
		t.Errorf("unapplied: %d tax entries adding up to %s, VAT %s, unrealized %s", len(entries), total, glAmount(src, "2330", "UPAY2"), glAmount(src, "2331", "UPAY2"))
	}
	glTotal(t, src)

	glSetup := gtables.GetGeneralLedgerSetup(src)
	if err := glSetup.Validate("unrealized_tax", false); err == nil {
		t.Error("Unrealized Tax turned off while a setup has an Unrealized Tax Type")
	}
}

// Unrealized VAT of a purchase, First: the payment realizes the VAT first — 100 of 1250 paid
// realizes 100 of the 250 VAT
func TestUnrealizedVATFirstPurchase(t *testing.T) {
	src := vatCompany(t)
	unrealizedVATSetup(t, src, gtables.TaxPostingSetup_Unrealized_tax_type.First)
	vend := gtables.NewVendor(src)
	vend.No = "TVU"
	vend.Name = "Test vendor"
	vend.Vendor_posting_group = "DOMESTIC"
	must(t, vend.InsertOrError(false))
	inv := addLine(t, src, "PURCHASES", "DEFAULT", lineSpec{lineNo: 10000, docNo: "VUINV1", docType: invoice, accType: vendType, accNo: "TVU", amount: "-1250", balNo: "4110"})
	inv.External_document_no = "VU-EXT-1"
	must(t, inv.ModifyOrError(false))
	_, err := postBatch(t, src, "PURCHASES", "DEFAULT")
	must(t, err)
	if !glAmount(src, "1441", "VUINV1").Equal(dec("250")) || !glAmount(src, "1440", "VUINV1").IsZero() {
		t.Fatalf("invoice: unrealized VAT %s, VAT %s", glAmount(src, "1441", "VUINV1"), glAmount(src, "1440", "VUINV1"))
	}
	addLine(t, src, "PAYMENT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "VUPAY1", docType: payment, accType: vendType, accNo: "TVU", amount: "100", postingDay: 20,
		balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: "VUINV1"})
	_, err = postBatch(t, src, "PAYMENT", "DEFAULT")
	must(t, err)
	if !glAmount(src, "1440", "VUPAY1").Equal(dec("100")) || !glAmount(src, "1441", "VUPAY1").Equal(dec("-100")) {
		t.Errorf("payment: realized %s, unrealized %s", glAmount(src, "1440", "VUPAY1"), glAmount(src, "1441", "VUPAY1"))
	}
	realized := vatTaxEntry(t, src, "VUPAY1")
	if !realized.Amount.Equal(dec("100")) || realized.Type != gtables.TaxEntry_Type.Purchase {
		t.Errorf("realized tax entry: amount %s type %v", realized.Amount, realized.Type)
	}
	if got := vatTaxEntry(t, src, "VUINV1").Remaining_unrealized_amount; !got.Equal(dec("150")) {
		t.Errorf("remaining unrealized %s", got)
	}
	glTotal(t, src)
}

// The VAT adjustment of a payment discount: without Adjust for Payment Disc. the discount stays on
// the posting group's Payment Disc. Acc.; with it (and the setup's Adjust for Payment Discount)
// the 25 discount on an invoice of 1000 + 250 VAT is split: 20 on the General Posting Setup's
// Sales Payment Disc. Account, 5 off the VAT — a tax entry of the payment. Unapplying the payment
// takes the VAT adjustment back.
func TestPaymentDiscountVATAdjustment(t *testing.T) {
	src := vatCompany(t)
	testGLAccount(t, src, "3155", "Payment discounts, net")
	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	must(t, genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": "DOMESTIC", "gen_prod_posting_group": "RETAIL"}))
	genPostingSetup.Sales_payment_disc_account = "3155"
	must(t, genPostingSetup.ModifyOrError(false))
	testCustomer(t, src, "TC9", "DOMESTIC", "", false)
	for i, docNo := range []string{"INV9", "INV10"} {
		addLine(t, src, "SALES", "DEFAULT", lineSpec{lineNo: 10000 * (i + 1), docNo: docNo, docType: invoice, accType: custType, accNo: "TC9", amount: "1250", balNo: "3110",
			pmtDiscPercent: "2", pmtDiscDay: 25})
	}
	_, err := postBatch(t, src, "SALES", "DEFAULT")
	must(t, err)
	pay := func(docNo, appliesTo string) {
		t.Helper()
		addLine(t, src, "CASHRCPT", "DEFAULT", lineSpec{lineNo: 10000, docNo: docNo, docType: payment, accType: custType, accNo: "TC9", amount: "-1225", postingDay: 20,
			balType: gtables.GenJournalLine_Bal_account_type.BankAccount, balNo: "B010", appliesDocType: gtables.GenJournalLine_Applies_to_doc_type.Invoice, appliesDocNo: appliesTo})
		_, err := postBatch(t, src, "CASHRCPT", "DEFAULT")
		must(t, err)
	}
	pay("PAY9", "INV9")
	if !glAmount(src, "3150", "PAY9").Equal(dec("25")) || len(docTaxEntries(src, "PAY9")) != 0 {
		t.Errorf("discount without adjustment %s, %d tax entries", glAmount(src, "3150", "PAY9"), len(docTaxEntries(src, "PAY9")))
	}

	glSetup := gtables.GetGeneralLedgerSetup(src)
	must(t, glSetup.Validate("adjust_for_payment_disc", true))
	must(t, glSetup.ModifyOrError(false))
	setup := gtables.NewTaxPostingSetup(src)
	must(t, setup.GetOrError(map[string]interface{}{"tax_bus_posting_group": "DOMESTIC", "tax_prod_posting_group": "STANDARD"}))
	must(t, setup.Validate("adjust_for_payment_discount", true))
	must(t, setup.ModifyOrError(false))
	pay("PAY10", "INV10")
	if !glAmount(src, "3150", "PAY10").IsZero() || !glAmount(src, "3155", "PAY10").Equal(dec("20")) || !glAmount(src, "2330", "PAY10").Equal(dec("5")) ||
		!glAmount(src, "1410", "PAY10").Equal(dec("-1250")) {
		t.Errorf("adjusted discount: 3150 %s, 3155 %s, VAT %s, receivables %s", glAmount(src, "3150", "PAY10"), glAmount(src, "3155", "PAY10"),
			glAmount(src, "2330", "PAY10"), glAmount(src, "1410", "PAY10"))
	}
	taxEntry := vatTaxEntry(t, src, "PAY10")
	if !taxEntry.Amount.Equal(dec("5")) || !taxEntry.Base.Equal(dec("20")) || taxEntry.Type != gtables.TaxEntry_Type.Sale ||
		taxEntry.Tax_bus_posting_group != "DOMESTIC" || taxEntry.Tax_prod_posting_group != "STANDARD" {
		t.Errorf("discount tax entry: amount %s base %s type %v", taxEntry.Amount, taxEntry.Base, taxEntry.Type)
	}
	discount := gtables.NewGLEntry(src)
	discount.SetRange("gl_account_no", "3155")
	discount.SetRange("document_no", "PAY10")
	if !discount.FindFirst() || !discount.Tax_amount.Equal(dec("5")) || discount.Gen_posting_type != gtables.GLEntry_Gen_posting_type.Sale {
		t.Errorf("discount G/L entry: tax %s type %v", discount.Tax_amount, discount.Gen_posting_type)
	}
	if inv := custEntry(t, src, "TC9", "INV10"); inv.Open || !inv.Pmt_disc_given_lcy.Equal(dec("25")) {
		t.Errorf("invoice open %v, discount given %s", inv.Open, inv.Pmt_disc_given_lcy)
	}
	glTotal(t, src)

	must(t, inTx(t, src, func(txSrc ftables.Source) error {
		_, err := UnapplyCustEntry(txSrc, custEntry(t, txSrc, "TC9", "PAY10"))
		return err
	}))
	var total types.Decimal
	for _, e := range docTaxEntries(src, "PAY10") {
		total = total.Add(e.Amount)
	}
	if !total.IsZero() || !glAmount(src, "2330", "PAY10").IsZero() || !glAmount(src, "3155", "PAY10").IsZero() || !glAmount(src, "1410", "PAY10").Equal(dec("-1225")) {
		t.Errorf("unapplied: tax %s, VAT %s, 3155 %s, receivables %s", total, glAmount(src, "2330", "PAY10"), glAmount(src, "3155", "PAY10"), glAmount(src, "1410", "PAY10"))
	}
	if inv := custEntry(t, src, "TC9", "INV10"); !inv.Open || !inv.Remaining_amount.Equal(dec("1250")) {
		t.Errorf("unapplied invoice open %v remaining %s", inv.Open, inv.Remaining_amount)
	}
	glTotal(t, src)

	glSetup = gtables.GetGeneralLedgerSetup(src)
	if err := glSetup.Validate("adjust_for_payment_disc", false); err == nil {
		t.Error("Adjust for Payment Disc. turned off while a setup uses it")
	}
}
