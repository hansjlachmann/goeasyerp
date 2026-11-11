package salesposting_test

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vatCompany: the demo company with VAT instead of sales tax — domestic Normal VAT (STANDARD
// 25 %, REDUCED 15 %, EXEMPT 0 %), abroad Reverse Charge VAT 25 %; sales VAT on 2330, purchase
// VAT on 1440, reverse charge on 2340
func vatCompany(t *testing.T) ftables.Source {
	t.Helper()
	src := demoCompany(t)
	for _, bus := range []string{"", "DOMESTIC", "EXPORT"} {
		for prod, percent := range map[string]string{"": "0", "STANDARD": "25", "REDUCED": "15", "EXEMPT": "0"} {
			setup := gtables.NewTaxPostingSetup(src)
			exists := setup.Get(map[string]interface{}{"tax_bus_posting_group": bus, "tax_prod_posting_group": prod})
			setup.Tax_bus_posting_group = types.Code(bus)
			setup.Tax_prod_posting_group = types.Code(prod)
			setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT
			if bus == "EXPORT" {
				setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT
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

// docGL: the amount of a document's G/L entries on an account
func docGL(src ftables.Source, documentNo types.Code, account string) types.Decimal {
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", documentNo)
	glEntry.SetRange("gl_account_no", account)
	_ = glEntry.CalcSums("amount")
	return glEntry.Amount
}

// docTax: the amount and base of a document's tax entries
func docTax(src ftables.Source, documentNo types.Code) (types.Decimal, types.Decimal, int) {
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", documentNo)
	_ = taxEntry.CalcSums("amount", "base")
	return taxEntry.Amount, taxEntry.Base, taxEntry.Count()
}

// A domestic invoice with Normal VAT: the lines' VAT %, the customer entry including the VAT,
// the revenue without it, the VAT on the sales VAT account and in tax entries per rate; the
// credit memo the other way round
func TestPostSalesInvoiceWithVAT(t *testing.T) {
	src := vatCompany(t)
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	line := addLine(t, invoice, 10000, gtables.SalesLine_Type.GLAccount, "3110", "2")
	must(t, line.Validate("unit_price", dec("100")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	if line.Tax_calculation_type != gtables.SalesLine_Tax_calculation_type.NormalVAT || !line.Tax_percent.Equal(dec("25")) ||
		!line.Amount.Equal(dec("200")) || !line.Amount_including_tax.Equal(dec("250")) {
		t.Fatalf("VAT line: type %v %% %s amount %s incl. %s", line.Tax_calculation_type, line.Tax_percent, line.Amount, line.Amount_including_tax)
	}
	line = addLine(t, invoice, 20000, gtables.SalesLine_Type.GLAccount, "3140", "1")
	must(t, line.Validate("unit_price", dec("40")))
	must(t, line.ModifyOrError(true))
	salesPost := post(t, invoice, false, false)
	postedInvoice := salesPost.InvoiceNo
	glBalance(t, src, postedInvoice)
	wantDec(t, "revenue 3110", docGL(src, postedInvoice, "3110"), dec("-200"))
	wantDec(t, "revenue 3140", docGL(src, postedInvoice, "3140"), dec("-40"))
	wantDec(t, "sales VAT", docGL(src, postedInvoice, "2330"), dec("-56")) // 50 + 6
	entry := custEntry(t, src, postedInvoice)
	entry.CalcFields("amount")
	wantDec(t, "customer entry", entry.Amount, dec("296"))
	amount, base, n := docTax(src, postedInvoice)
	if n != 2 {
		t.Errorf("%d tax entries, want one per rate", n)
	}
	wantDec(t, "VAT", amount, dec("-56"))
	wantDec(t, "VAT base", base, dec("-240"))

	creditMemo := newDocument(t, src, gtables.SalesHeader_Document_type.CreditMemo, "C00020")
	line = addLine(t, creditMemo, 10000, gtables.SalesLine_Type.GLAccount, "3110", "1")
	must(t, line.Validate("unit_price", dec("100")))
	must(t, line.ModifyOrError(true))
	salesPost = post(t, creditMemo, false, false)
	glBalance(t, src, salesPost.CrMemoNo)
	wantDec(t, "credit memo revenue", docGL(src, salesPost.CrMemoNo, "3110"), dec("100"))
	wantDec(t, "credit memo VAT", docGL(src, salesPost.CrMemoNo, "2330"), dec("25"))
	amount, base, _ = docTax(src, salesPost.CrMemoNo)
	wantDec(t, "credit memo VAT entry", amount, dec("25"))
	wantDec(t, "credit memo VAT base", base, dec("100"))
}

// Prices Including VAT: the unit price includes the VAT, the line's amount is without it
func TestPostSalesPricesIncludingVAT(t *testing.T) {
	src := vatCompany(t)
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	must(t, invoice.Validate("prices_including_tax", true))
	must(t, invoice.ModifyOrError(true))
	line := addLine(t, invoice, 10000, gtables.SalesLine_Type.GLAccount, "3110", "1")
	must(t, line.Validate("unit_price", dec("125")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	wantDec(t, "amount", line.Amount, dec("100"))
	wantDec(t, "amount including VAT", line.Amount_including_tax, dec("125"))
	salesPost := post(t, invoice, false, false)
	glBalance(t, src, salesPost.InvoiceNo)
	wantDec(t, "revenue", docGL(src, salesPost.InvoiceNo, "3110"), dec("-100"))
	wantDec(t, "VAT", docGL(src, salesPost.InvoiceNo, "2330"), dec("-25"))
	entry := custEntry(t, src, salesPost.InvoiceNo)
	entry.CalcFields("amount")
	wantDec(t, "customer entry", entry.Amount, dec("125"))
}

// A sale abroad with Reverse Charge VAT: the customer pays no VAT (the line's VAT % is 0), no
// VAT is posted, the tax entry has the base (the EC sales list); EU 3-Party Trade goes onto it
func TestPostSalesReverseChargeVAT(t *testing.T) {
	src := vatCompany(t)
	cust := gtables.NewCustomer(src)
	cust.SetRange("tax_bus_posting_group", types.Code("EXPORT"))
	cust.SetRange("currency_code", types.Code(""))
	if !cust.FindFirst() {
		cust.SetRange("currency_code")
		if !cust.FindFirst() {
			t.Skip("no demo customer abroad")
		}
	}
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, cust.No.String())
	invoice.Eu_3_party_trade = true
	must(t, invoice.ModifyOrError(true))
	line := addLine(t, invoice, 10000, gtables.SalesLine_Type.GLAccount, "3120", "1")
	must(t, line.Validate("tax_prod_posting_group", types.Code("STANDARD")))
	must(t, line.Validate("unit_price", dec("1000")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	if line.Tax_calculation_type != gtables.SalesLine_Tax_calculation_type.ReverseChargeVAT || !line.Tax_percent.IsZero() ||
		!line.Amount_including_tax.Equal(line.Amount) {
		t.Fatalf("reverse charge line: type %v %% %s amount %s incl. %s", line.Tax_calculation_type, line.Tax_percent, line.Amount, line.Amount_including_tax)
	}
	salesPost := post(t, invoice, false, false)
	glBalance(t, src, salesPost.InvoiceNo)
	if !docGL(src, salesPost.InvoiceNo, "2330").IsZero() {
		t.Errorf("VAT posted on a reverse charge sale: %s", docGL(src, salesPost.InvoiceNo, "2330"))
	}
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", salesPost.InvoiceNo)
	if taxEntry.Count() != 1 || !taxEntry.FindFirst() {
		t.Fatalf("%d tax entries", taxEntry.Count())
	}
	if !taxEntry.Amount.IsZero() || !taxEntry.Base.Equal(line.Amount.Neg().Mul(invoiceFactor(src, invoice))) || !taxEntry.Eu_3_party_trade ||
		taxEntry.Tax_calculation_type != gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT || taxEntry.Country_code != cust.Country_code {
		t.Errorf("tax entry: amount %s base %s 3-party %v calc %v country %q", taxEntry.Amount, taxEntry.Base, taxEntry.Eu_3_party_trade,
			taxEntry.Tax_calculation_type, taxEntry.Country_code)
	}
	posted := gtables.NewSalesInvoiceHeader(src)
	must(t, posted.GetOrError(salesPost.InvoiceNo))
	if !posted.Eu_3_party_trade {
		t.Error("the posted invoice lost EU 3-Party Trade")
	}
}

// invoiceFactor: 1 for an LCY document, else its LCY amount per currency unit
func invoiceFactor(src ftables.Source, salesHeader *gtables.SalesHeader) types.Decimal {
	if salesHeader.Currency_code == "" || salesHeader.Currency_factor.IsZero() {
		return dec("1")
	}
	return dec("1").Div(salesHeader.Currency_factor)
}
