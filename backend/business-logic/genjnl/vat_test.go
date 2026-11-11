package genjnl

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// VAT in codeunit 12: the demo company's tax posting setups turned into VAT —
//   - DOMESTIC/STANDARD Normal VAT 25 % (sales VAT 2330, purchase VAT 1440),
//   - EXPORT/STANDARD Reverse Charge VAT 25 % (reverse charge 2340),
//   - DOMESTIC/FULL Full VAT (the VAT accounts themselves).
func vatCompany(t *testing.T) ftables.Source {
	t.Helper()
	src := demoCompany(t)
	full := gtables.NewTaxProductPostingGroup(src)
	full.Code = "FULL"
	must(t, full.InsertOrError(false))
	setVATSetup(t, src, "DOMESTIC", "STANDARD", gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT, "25")
	setVATSetup(t, src, "EXPORT", "STANDARD", gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT, "25")
	setVATSetup(t, src, "DOMESTIC", "FULL", gtables.TaxPostingSetup_Tax_calculation_type.FullVAT, "0")
	return src
}

func setVATSetup(t *testing.T, src ftables.Source, bus, prod string, calcType gtables.TaxPostingSetupTax_calculation_type, percent string) {
	t.Helper()
	setup := gtables.NewTaxPostingSetup(src)
	exists := setup.Get(map[string]interface{}{"tax_bus_posting_group": bus, "tax_prod_posting_group": prod})
	setup.Tax_bus_posting_group = types.Code(bus)
	setup.Tax_prod_posting_group = types.Code(prod)
	setup.Tax_calculation_type = calcType
	setup.Vat_percent = dec(percent)
	setup.Sales_vat_account = "2330"
	setup.Purchase_vat_account = "1440"
	setup.Reverse_chrg_vat_acc = "2340"
	if exists {
		must(t, setup.ModifyOrError(false))
	} else {
		must(t, setup.InsertOrError(false))
	}
}

// vatTaxEntry is the one tax entry of a document
func vatTaxEntry(t *testing.T, src ftables.Source, docNo string) *gtables.TaxEntry {
	t.Helper()
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", docNo)
	if taxEntry.Count() != 1 || !taxEntry.FindFirst() {
		t.Fatalf("%s: %d tax entries, want 1", docNo, taxEntry.Count())
	}
	return taxEntry
}

// Normal VAT: the amount includes the VAT; the G/L account gets the amount without it, the
// setup's sales / purchase VAT account the VAT, a tax entry of the VAT and its base
func TestPostNormalVAT(t *testing.T) {
	src := vatCompany(t)
	line := addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "V00001", accNo: "3110", amount: "-1250", balNo: "1210"})
	if line.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.NormalVAT || !line.Tax_percent.Equal(dec("25")) || !line.Tax_amount.Equal(dec("-250")) {
		t.Fatalf("sales line: type %v %% %s tax %s", line.Tax_calculation_type, line.Tax_percent, line.Tax_amount)
	}
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 20000, docNo: "V00002", accNo: "4110", amount: "1250", balNo: "1210"})
	sales0 := glAmount(src, "3110", "V00001")
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)

	if !glAmount(src, "3110", "V00001").Sub(sales0).Equal(dec("-1000")) || !glAmount(src, "2330", "V00001").Equal(dec("-250")) ||
		!glAmount(src, "1210", "V00001").Equal(dec("1250")) {
		t.Errorf("sale G/L: 3110 %s, 2330 %s, 1210 %s", glAmount(src, "3110", "V00001"), glAmount(src, "2330", "V00001"), glAmount(src, "1210", "V00001"))
	}
	sale := vatTaxEntry(t, src, "V00001")
	if !sale.Amount.Equal(dec("-250")) || !sale.Base.Equal(dec("-1000")) || sale.Type != gtables.TaxEntry_Type.Sale ||
		sale.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.NormalVAT || sale.Tax_jurisdiction_code != "" ||
		sale.Tax_bus_posting_group != "DOMESTIC" || sale.Tax_prod_posting_group != "STANDARD" {
		t.Errorf("sale tax entry: amount %s base %s type %v calc %v jurisdiction %q", sale.Amount, sale.Base, sale.Type, sale.Tax_calculation_type, sale.Tax_jurisdiction_code)
	}
	revenue := gtables.NewGLEntry(src)
	revenue.SetRange("gl_account_no", "3110")
	revenue.SetRange("document_no", "V00001")
	if !revenue.FindFirst() || !revenue.Tax_amount.Equal(dec("-250")) || revenue.Transaction_no != sale.Transaction_no {
		t.Errorf("revenue entry: tax %s transaction %d / %d", revenue.Tax_amount, revenue.Transaction_no, sale.Transaction_no)
	}

	if !glAmount(src, "4110", "V00002").Equal(dec("1000")) || !glAmount(src, "1440", "V00002").Equal(dec("250")) ||
		!glAmount(src, "1210", "V00002").Equal(dec("-1250")) {
		t.Errorf("purchase G/L: 4110 %s, 1440 %s, 1210 %s", glAmount(src, "4110", "V00002"), glAmount(src, "1440", "V00002"), glAmount(src, "1210", "V00002"))
	}
	purchase := vatTaxEntry(t, src, "V00002")
	if !purchase.Amount.Equal(dec("250")) || !purchase.Base.Equal(dec("1000")) || purchase.Type != gtables.TaxEntry_Type.Purchase {
		t.Errorf("purchase tax entry: amount %s base %s type %v", purchase.Amount, purchase.Base, purchase.Type)
	}
}

// Reverse Charge VAT: a purchase posts its whole amount and owes the VAT on top — on the
// purchase VAT account and back on the reverse charge account; a sale has no VAT (a tax entry of
// its base only)
func TestPostReverseChargeVAT(t *testing.T) {
	src := vatCompany(t)
	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	line.Document_no = "R00001"
	must(t, line.Validate("account_no", types.Code("4120")))
	must(t, line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	must(t, line.Validate("bal_account_no", types.Code("1210")))
	must(t, line.Validate("amount", dec("1000")))
	if line.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.ReverseChargeVAT || !line.Tax_amount.IsZero() {
		t.Fatalf("reverse charge line: type %v tax %s", line.Tax_calculation_type, line.Tax_amount)
	}
	must(t, line.InsertOrError(true))

	line = newJnlLine(src, "GENERAL", "DEFAULT", 20000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	line.Document_no = "R00002"
	must(t, line.Validate("account_no", types.Code("3120")))
	must(t, line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	must(t, line.Validate("bal_account_no", types.Code("1210")))
	must(t, line.Validate("amount", dec("-1000")))
	must(t, line.InsertOrError(true))

	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAmount(src, "4120", "R00001").Equal(dec("1000")) || !glAmount(src, "1440", "R00001").Equal(dec("250")) ||
		!glAmount(src, "2340", "R00001").Equal(dec("-250")) || !glAmount(src, "1210", "R00001").Equal(dec("-1000")) {
		t.Errorf("purchase G/L: 4120 %s, 1440 %s, 2340 %s, 1210 %s", glAmount(src, "4120", "R00001"), glAmount(src, "1440", "R00001"),
			glAmount(src, "2340", "R00001"), glAmount(src, "1210", "R00001"))
	}
	purchase := vatTaxEntry(t, src, "R00001")
	if !purchase.Amount.Equal(dec("250")) || !purchase.Base.Equal(dec("1000")) ||
		purchase.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT {
		t.Errorf("purchase tax entry: amount %s base %s calc %v", purchase.Amount, purchase.Base, purchase.Tax_calculation_type)
	}

	if !glAmount(src, "3120", "R00002").Equal(dec("-1000")) || !glAmount(src, "2330", "R00002").IsZero() || !glAmount(src, "1210", "R00002").Equal(dec("1000")) {
		t.Errorf("sale G/L: 3120 %s, 2330 %s, 1210 %s", glAmount(src, "3120", "R00002"), glAmount(src, "2330", "R00002"), glAmount(src, "1210", "R00002"))
	}
	sale := vatTaxEntry(t, src, "R00002")
	if !sale.Amount.IsZero() || !sale.Base.Equal(dec("-1000")) || sale.Type != gtables.TaxEntry_Type.Sale {
		t.Errorf("sale tax entry: amount %s base %s type %v", sale.Amount, sale.Base, sale.Type)
	}
}

// Full VAT: the line is all VAT, on the setup's VAT account (its own account); the tax entry has
// the amount and no base
func TestPostFullVAT(t *testing.T) {
	src := vatCompany(t)
	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	line.Document_no = "F00001"
	must(t, line.Validate("account_no", types.Code("1440")))
	must(t, line.Validate("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Purchase))
	must(t, line.Validate("tax_bus_posting_group", types.Code("DOMESTIC")))
	must(t, line.Validate("tax_prod_posting_group", types.Code("FULL")))
	must(t, line.Validate("bal_account_no", types.Code("1210")))
	must(t, line.Validate("amount", dec("250")))
	if line.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.FullVAT {
		t.Fatalf("full VAT line: type %v", line.Tax_calculation_type)
	}
	must(t, line.InsertOrError(true))
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAmount(src, "1440", "F00001").Equal(dec("250")) || !glAmount(src, "1210", "F00001").Equal(dec("-250")) {
		t.Errorf("G/L: 1440 %s, 1210 %s", glAmount(src, "1440", "F00001"), glAmount(src, "1210", "F00001"))
	}
	taxEntry := vatTaxEntry(t, src, "F00001")
	if !taxEntry.Amount.Equal(dec("250")) || !taxEntry.Base.IsZero() || taxEntry.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.FullVAT {
		t.Errorf("tax entry: amount %s base %s calc %v", taxEntry.Amount, taxEntry.Base, taxEntry.Tax_calculation_type)
	}

	// another account than the setup's VAT account is refused
	line = newJnlLine(src, "GENERAL", "DEFAULT", 20000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	must(t, line.Validate("account_no", types.Code("4110")))
	if err := line.Validate("tax_prod_posting_group", types.Code("FULL")); err == nil {
		t.Error("Full VAT on an account that is not the VAT account")
	}
}

// Manual VAT Entry (as the documents post them, codeunit 12 line by line): the line's amount is
// without the VAT, the VAT is the line's own (not calculated from the setup)
func TestPostManualVAT(t *testing.T) {
	src := vatCompany(t)
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	postLine := NewGenJnlPostLine(txSrc)
	sale := newJnlLine(txSrc, "GENERAL", "DEFAULT", 10000)
	must(t, sale.Validate("posting_date", date(t, 15, 10, 2026)))
	sale.Document_no = "M00001"
	must(t, sale.Validate("account_no", types.Code("3110")))
	must(t, sale.Validate("amount", dec("-1000")))
	sale.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
	sale.Tax_amount, sale.Tax_amount_lcy = dec("-249"), dec("-249") // a VAT difference: the line's own amount is posted
	sale.Tax_base_amount, sale.Tax_base_amount_lcy = dec("-1000"), dec("-1000")
	sale.System_created_entry = true
	bank := newJnlLine(txSrc, "GENERAL", "DEFAULT", 20000)
	must(t, bank.Validate("posting_date", date(t, 15, 10, 2026)))
	bank.Document_no = "M00001"
	must(t, bank.Validate("account_no", types.Code("1210")))
	must(t, bank.Validate("amount", dec("1249")))
	for _, line := range []*gtables.GenJournalLine{sale, bank} {
		err = postLine.RunWithoutCheck(line)
		if err != nil {
			_ = ftables.Rollback(tx)
			t.Fatal(err)
		}
	}
	must(t, ftables.Commit(tx))
	if !glAmount(src, "3110", "M00001").Equal(dec("-1000")) || !glAmount(src, "2330", "M00001").Equal(dec("-249")) || !glAmount(src, "1210", "M00001").Equal(dec("1249")) {
		t.Errorf("G/L: 3110 %s, 2330 %s, 1210 %s", glAmount(src, "3110", "M00001"), glAmount(src, "2330", "M00001"), glAmount(src, "1210", "M00001"))
	}
	taxEntry := vatTaxEntry(t, src, "M00001")
	if !taxEntry.Amount.Equal(dec("-249")) || !taxEntry.Base.Equal(dec("-1000")) {
		t.Errorf("tax entry: amount %s base %s", taxEntry.Amount, taxEntry.Base)
	}
}

// VAT with the additional reporting currency (EUR at 11.50): an LCY line's VAT and base are
// exchanged at the G/L entry's factor; a line in euro keeps its own euro amounts (125 including
// 25 % VAT: 100 and 25); a reverse charge purchase owes the VAT of the euro amount; every
// document balances in both currencies
func TestPostVATAddCurrency(t *testing.T) {
	src := vatCompany(t)
	withAddCurrency(t, src)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "A00001", accNo: "3110", amount: "-1250", balNo: "1210"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 20000, docNo: "A00002", accNo: "3110", amount: "-125", balNo: "1210", currency: "EUR"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)

	lcy := vatTaxEntry(t, src, "A00001")
	if !lcy.Additional_currency_amount.Equal(dec("-21.74")) || !lcy.Additional_currency_base.Equal(dec("-86.96")) {
		t.Errorf("LCY line's tax entry in euro: amount %s base %s", lcy.Additional_currency_amount, lcy.Additional_currency_base)
	}
	if got := glAddCurr(src, "2330", "A00001"); !got.Equal(dec("-21.74")) {
		t.Errorf("LCY line's VAT account in euro %s", got)
	}
	docAddCurrBalanced(t, src, "A00001")

	eur := vatTaxEntry(t, src, "A00002")
	if !eur.Amount.Equal(dec("-287.5")) || !eur.Base.Equal(dec("-1150")) ||
		!eur.Additional_currency_amount.Equal(dec("-25")) || !eur.Additional_currency_base.Equal(dec("-100")) {
		t.Errorf("euro line's tax entry: amount %s base %s, in euro %s base %s", eur.Amount, eur.Base, eur.Additional_currency_amount, eur.Additional_currency_base)
	}
	if got := glAddCurr(src, "2330", "A00002"); !got.Equal(dec("-25")) {
		t.Errorf("euro line's VAT account in euro %s", got)
	}
	if got := glAddCurr(src, "3110", "A00002"); !got.Equal(dec("-100")) {
		t.Errorf("euro line's revenue in euro %s", got)
	}
	docAddCurrBalanced(t, src, "A00002")

	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	line.Document_no = "A00003"
	must(t, line.Validate("account_no", types.Code("4120")))
	must(t, line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	must(t, line.Validate("bal_account_no", types.Code("1210")))
	must(t, line.Validate("amount", dec("1000")))
	must(t, line.InsertOrError(true))
	_, err = postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	reverse := vatTaxEntry(t, src, "A00003")
	if !reverse.Additional_currency_amount.Equal(dec("21.74")) || !reverse.Additional_currency_base.Equal(dec("86.96")) {
		t.Errorf("reverse charge tax entry in euro: amount %s base %s", reverse.Additional_currency_amount, reverse.Additional_currency_base)
	}
	if !glAddCurr(src, "1440", "A00003").Equal(dec("21.74")) || !glAddCurr(src, "2340", "A00003").Equal(dec("-21.74")) {
		t.Errorf("reverse charge in euro: 1440 %s, 2340 %s", glAddCurr(src, "1440", "A00003"), glAddCurr(src, "2340", "A00003"))
	}
	docAddCurrBalanced(t, src, "A00003")
}
