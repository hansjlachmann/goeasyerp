package purchposting_test

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vatCompany: the demo company with VAT instead of sales tax — domestic Normal VAT (STANDARD
// 25 %, REDUCED 15 %, EXEMPT 0 %, FULL Full VAT), abroad Reverse Charge VAT 25 %; sales VAT on
// 2330, purchase VAT on 1440, reverse charge on 2340
func vatCompany(t *testing.T) ftables.Source {
	t.Helper()
	src := demoCompany(t)
	full := gtables.NewTaxProductPostingGroup(src)
	full.Code = "FULL"
	must(t, full.InsertOrError(false))
	for _, bus := range []string{"", "DOMESTIC", "EXPORT"} {
		for prod, percent := range map[string]string{"": "0", "STANDARD": "25", "REDUCED": "15", "EXEMPT": "0", "FULL": "0"} {
			setup := gtables.NewTaxPostingSetup(src)
			exists := setup.Get(map[string]interface{}{"tax_bus_posting_group": bus, "tax_prod_posting_group": prod})
			setup.Tax_bus_posting_group = types.Code(bus)
			setup.Tax_prod_posting_group = types.Code(prod)
			setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT
			if bus == "EXPORT" {
				setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT
			}
			if prod == "FULL" {
				setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.FullVAT
			}
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
	}
	return src
}

func docTax(src ftables.Source, documentNo types.Code) (types.Decimal, types.Decimal) {
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", documentNo)
	_ = taxEntry.CalcSums("amount", "base")
	return taxEntry.Amount, taxEntry.Base
}

// A domestic purchase invoice with Normal VAT and a Full VAT line (customs VAT billed alone): the
// vendor entry including the VAT, the expense without it, the VAT on the purchase VAT account
func TestPostPurchaseInvoiceWithVAT(t *testing.T) {
	src := vatCompany(t)
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	line := addLine(t, invoice, 10000, gtables.PurchaseLine_Type.GLAccount, "4130", "2")
	must(t, line.Validate("direct_unit_cost", dec("100")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	if line.Tax_calculation_type != gtables.PurchaseLine_Tax_calculation_type.NormalVAT || !line.Amount_including_tax.Equal(dec("250")) {
		t.Fatalf("VAT line: type %v amount incl. %s", line.Tax_calculation_type, line.Amount_including_tax)
	}
	line = addLine(t, invoice, 20000, gtables.PurchaseLine_Type.GLAccount, "1440", "1")
	must(t, line.Validate("tax_prod_posting_group", types.Code("FULL")))
	must(t, line.Validate("direct_unit_cost", dec("30")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 20000)
	if line.Tax_calculation_type != gtables.PurchaseLine_Tax_calculation_type.FullVAT || !line.Amount.IsZero() || !line.Amount_including_tax.Equal(dec("30")) {
		t.Fatalf("Full VAT line: type %v amount %s incl. %s", line.Tax_calculation_type, line.Amount, line.Amount_including_tax)
	}
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-VAT")))
	must(t, invoice.ModifyOrError(true))
	purchPost := post(t, invoice, false, false)
	glBalance(t, src, purchPost.InvoiceNo)
	wantDec(t, "expense", glAmount(t, src, purchPost.InvoiceNo, "4130"), dec("200"))
	wantDec(t, "purchase VAT", glAmount(t, src, purchPost.InvoiceNo, "1440"), dec("80")) // 50 + 30
	entry := vendEntry(t, src, purchPost.InvoiceNo)
	wantDec(t, "vendor entry", entry.Amount, dec("-280"))
	amount, base := docTax(src, purchPost.InvoiceNo)
	wantDec(t, "VAT", amount, dec("80"))
	wantDec(t, "VAT base", base, dec("200"))
}

// A purchase from abroad with Reverse Charge VAT: the vendor charges no VAT; the VAT owed is
// posted on the purchase VAT account and back on the reverse charge account
func TestPostPurchaseReverseChargeVAT(t *testing.T) {
	src := vatCompany(t)
	vend := gtables.NewVendor(src)
	vend.SetRange("tax_bus_posting_group", types.Code("EXPORT"))
	vend.SetRange("currency_code", types.Code(""))
	if !vend.FindFirst() {
		vend.SetRange("currency_code")
		if !vend.FindFirst() {
			t.Skip("no demo vendor abroad")
		}
	}
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, vend.No.String())
	line := addLine(t, invoice, 10000, gtables.PurchaseLine_Type.GLAccount, "4120", "1")
	must(t, line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	must(t, line.Validate("direct_unit_cost", dec("1000")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	if line.Tax_calculation_type != gtables.PurchaseLine_Tax_calculation_type.ReverseChargeVAT || !line.Tax_percent.IsZero() ||
		!line.Amount_including_tax.Equal(line.Amount) {
		t.Fatalf("reverse charge line: type %v %% %s amount %s incl. %s", line.Tax_calculation_type, line.Tax_percent, line.Amount, line.Amount_including_tax)
	}
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-RC")))
	must(t, invoice.ModifyOrError(true))
	purchPost := post(t, invoice, false, false)
	glBalance(t, src, purchPost.InvoiceNo)
	expense := glAmount(t, src, purchPost.InvoiceNo, "4120")
	vat := glAmount(t, src, purchPost.InvoiceNo, "1440")
	wantDec(t, "reverse charge VAT 25 % of the expense", vat, expense.Mul(dec("0.25")).Round(2))
	wantDec(t, "reverse charge account", glAmount(t, src, purchPost.InvoiceNo, "2340"), vat.Neg())
	entry := vendEntry(t, src, purchPost.InvoiceNo)
	entry.CalcFields("amount_lcy")
	wantDec(t, "vendor entry without VAT", entry.Amount_lcy, expense.Neg())
	amount, base := docTax(src, purchPost.InvoiceNo)
	wantDec(t, "VAT entry", amount, vat)
	wantDec(t, "VAT base", base, expense)
}
