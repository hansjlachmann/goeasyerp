package salesposting_test

import (
	"testing"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Discount Posting (All Discounts): a discount is posted on the discount account of the General
// Posting Setup without posting type and tax groups (classic PostDiscount), the sales account
// gets the amount before the discount, and the sales tax is calculated on the amount after it
func TestPostLineDiscountWithSalesTax(t *testing.T) {
	src := demoCompany(t)
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Discount_posting = gtables.SalesReceivablesSetup_Discount_posting.AllDiscounts
	must(t, salesSetup.ModifyOrError(false))

	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00010")
	line := addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "10")
	must(t, line.Validate("line_discount_percent", dec("10")))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	if !line.Tax_liable || line.Tax_area_code == "" || line.Amount_including_tax.Equal(line.Amount) {
		t.Fatalf("the demo line is not taxed: liable %v, area %q, amount %s incl. tax %s", line.Tax_liable, line.Tax_area_code, line.Amount, line.Amount_including_tax)
	}
	lineDiscount := line.Line_discount_amount
	netAmount := line.Amount
	if lineDiscount.IsZero() {
		t.Fatal("no line discount")
	}
	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	must(t, genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": line.Gen_bus_posting_group, "gen_prod_posting_group": line.Gen_prod_posting_group}))
	genPostingSetup.Sales_line_disc_account = "3150" // Sales Discounts
	must(t, genPostingSetup.ModifyOrError(false))

	salesPost := post(t, invoice, false, false)
	postedInvoice := salesPost.InvoiceNo
	glBalance(t, src, postedInvoice)

	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", postedInvoice)
	glEntry.SetRange("gl_account_no", genPostingSetup.Sales_line_disc_account)
	if !glEntry.FindFirst() {
		t.Fatalf("no G/L entry on the line discount account %s", genPostingSetup.Sales_line_disc_account)
	}
	wantDec(t, "line discount G/L entry", glEntry.Amount, lineDiscount)
	if glEntry.Gen_posting_type != gtables.GLEntry_Gen_posting_type.Blank || glEntry.Tax_area_code != "" || glEntry.Tax_group_code != "" {
		t.Errorf("discount entry with posting type %v, tax area %q, group %q", glEntry.Gen_posting_type, glEntry.Tax_area_code, glEntry.Tax_group_code)
	}
	glEntry.SetRange("gl_account_no", genPostingSetup.Sales_account)
	if !glEntry.FindFirst() {
		t.Fatalf("no G/L entry on the sales account %s", genPostingSetup.Sales_account)
	}
	wantDec(t, "sales account G/L entry (before the discount)", glEntry.Amount, netAmount.Add(lineDiscount).Neg())

	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", postedInvoice)
	must(t, taxEntry.CalcSums("base"))
	wantDec(t, "tax base (after the discount)", taxEntry.Base.Abs(), netAmount)
}
