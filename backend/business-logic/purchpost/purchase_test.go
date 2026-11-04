package purchpost_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchcalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchpost"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchquotetoorder"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func wantDec(t *testing.T, what string, got types.Decimal, want string) {
	t.Helper()
	if !got.Equal(dec(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

// demoCompany: the demo company (vendors V00010… with payment terms, items 1000… with their
// vendors' terms — 1000 from V00010 at 45.00 a PACK, a BOX holds 5 —, the purchase series), in a
// transaction
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

func newDocument(t *testing.T, src ftables.Source, documentType gtables.PurchaseHeaderDocument_type, vendor string) *gtables.PurchaseHeader {
	t.Helper()
	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.Document_type = documentType
	must(t, purchHeader.InsertOrError(true))
	must(t, purchHeader.Validate("buy_from_vendor_no", types.Code(vendor)))
	must(t, purchHeader.ModifyOrError(true))
	return purchHeader
}

func addLine(t *testing.T, purchHeader *gtables.PurchaseHeader, lineNo int, lineType gtables.PurchaseLineType, no, qty string) *gtables.PurchaseLine {
	t.Helper()
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = lineNo
	must(t, purchLine.Validate("type", lineType))
	must(t, purchLine.Validate("no", types.Code(no)))
	must(t, purchLine.Validate("quantity", dec(qty)))
	must(t, purchLine.InsertOrError(true))
	return purchLine
}

// A purchase order: its number, the vendor's fields, the company as ship-to, the dates; an item
// line costs the vendor's direct unit cost (Item Vendor), a unit without one the item's Last
// Direct Cost per unit; the amounts, outstanding and quantities to receive and invoice
func TestPurchaseOrder(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.PurchaseHeader_Document_type.Order, "V00010")
	if order.No == "" || order.No_series == "" {
		t.Fatalf("order number %q series %q", order.No, order.No_series)
	}
	if order.Pay_to_vendor_no != "V00010" || order.Buy_from_vendor_name == "" || order.Pay_to_name != order.Buy_from_vendor_name ||
		order.Vendor_posting_group == "" || order.Payment_terms_code != "30 DAYS" || order.Due_date.IsZero() {
		t.Errorf("vendor fields: %+v", order)
	}
	companyInfo := gtables.GetCompanyInformation(src)
	if order.Ship_to_name != companyInfo.Ship_to_name || order.Location_code != companyInfo.Location_code || order.Location_code == "" {
		t.Errorf("ship-to %q location %q, want the company's", order.Ship_to_name, order.Location_code)
	}
	if order.Order_date.IsZero() || order.Expected_receipt_date.IsZero() || order.Posting_date.IsZero() || order.Posting_no_series == "" || order.Receiving_no_series == "" {
		t.Errorf("dates/series: %+v", order)
	}

	line := addLine(t, order, 10000, gtables.PurchaseLine_Type.Item, "1000", "10")
	wantDec(t, "direct unit cost (Item Vendor)", line.Direct_unit_cost, "45")
	wantDec(t, "amount", line.Amount, "450")
	if line.Vendor_item_no != "NP-A4-80" || line.Unit_of_measure_code != "PACK" || line.Location_code != order.Location_code {
		t.Errorf("line: vendor item %q unit %q location %q", line.Vendor_item_no, line.Unit_of_measure_code, line.Location_code)
	}
	wantDec(t, "qty to receive", line.Qty_to_receive, "10")
	wantDec(t, "qty to invoice", line.Qty_to_invoice, "10")
	wantDec(t, "outstanding quantity", line.Outstanding_quantity, "10")
	if !line.Outstanding_amount.Equal(line.Amount_including_tax) || line.Amount_including_tax.LessThan(line.Amount) {
		t.Errorf("outstanding %s, incl. tax %s", line.Outstanding_amount, line.Amount_including_tax)
	}

	// a box (5 packs) has no vendor terms: the item's Last Direct Cost per box
	must(t, line.Validate("unit_of_measure_code", types.Code("BOX")))
	must(t, line.ModifyOrError(true))
	wantDec(t, "direct unit cost per box", line.Direct_unit_cost, "225")
	wantDec(t, "quantity (base)", line.Quantity_base, "50")

	// line discount and indirect cost: the unit cost
	must(t, line.Validate("line_discount_percent", dec("10")))
	wantDec(t, "line discount amount", line.Line_discount_amount, "225")
	wantDec(t, "amount after discount", line.Amount, "2025")
	must(t, line.Validate("indirect_cost_percent", dec("2")))
	wantDec(t, "unit cost", line.Unit_cost, "206.55") // (225 - 22.5) × 1.02
	wantDec(t, "unit cost (LCY)", line.Unit_cost_lcy, "206.55")

	// use tax: no tax on the line
	must(t, line.Validate("use_tax", true))
	wantDec(t, "amount incl. tax with use tax", line.Amount_including_tax, "2025")

	// not more than outstanding to receive
	if err := line.Validate("qty_to_receive", dec("11")); err == nil {
		t.Error("receiving more than outstanding must be refused")
	}
	must(t, line.Validate("qty_to_receive", dec("4")))
	wantDec(t, "qty to invoice follows", line.Qty_to_invoice, "4")
	must(t, line.ModifyOrError(true))

	// the vendor's outstanding orders
	vend := gtables.NewVendor(src)
	must(t, vend.GetOrError(types.Code("V00010")))
	vend.CalcFields("outstanding_orders_lcy")
	wantDec(t, "vendor outstanding orders (LCY)", vend.Outstanding_orders_lcy, "2025")
}

// Newer vendor terms (a later starting date) give the cost on the order date; a changed vendor
// recreates the lines with that vendor's costs
func TestPurchaseVendorTermsAndRecreate(t *testing.T) {
	src := demoCompany(t)
	itemVend := gtables.NewItemVendor(src)
	itemVend.Item_no = "1000"
	itemVend.Vendor_no = "V00010"
	itemVend.Starting_date = types.MustDate("2026-01-01")
	itemVend.Direct_unit_cost = dec("40")
	must(t, itemVend.InsertOrError(false))

	quote := newDocument(t, src, gtables.PurchaseHeader_Document_type.Quote, "V00010")
	if !quote.Posting_date.IsZero() {
		t.Errorf("a quote has no posting date: %s", quote.Posting_date)
	}
	line := addLine(t, quote, 10000, gtables.PurchaseLine_Type.Item, "1000", "3")
	wantDec(t, "cost from the newer terms", line.Direct_unit_cost, "40")

	// another vendor without terms for the item: its lines again at the Last Direct Cost
	must(t, quote.Validate("buy_from_vendor_no", types.Code("V00020")))
	must(t, quote.ModifyOrError(true))
	purchLine := gtables.NewPurchaseLine(src)
	must(t, purchLine.GetOrError(map[string]interface{}{"document_type": int(quote.Document_type), "document_no": quote.No, "line_no": 10000}))
	if purchLine.Buy_from_vendor_no != "V00020" || purchLine.Pay_to_vendor_no != "V00020" {
		t.Errorf("recreated line vendor %q / %q", purchLine.Buy_from_vendor_no, purchLine.Pay_to_vendor_no)
	}
	wantDec(t, "recreated cost", purchLine.Direct_unit_cost, "45")
	wantDec(t, "recreated quantity", purchLine.Quantity, "3")
}

// The invoice discount from the vendor's Vendor Invoice Disc., the statistics' totals, and Make
// Order: the quote becomes an order with its lines and comments, the quote is gone
func TestPurchaseInvoiceDiscountStatisticsMakeOrder(t *testing.T) {
	src := demoCompany(t)
	vend := gtables.NewVendor(src)
	must(t, vend.GetOrError(types.Code("V00010")))
	vend.Invoice_disc_code = "V00010"
	must(t, vend.ModifyOrError(false))
	vendInvDisc := gtables.NewVendorInvoiceDisc(src)
	vendInvDisc.Code = "V00010"
	vendInvDisc.Minimum_amount = dec("100")
	vendInvDisc.Discount_percent = dec("5")
	must(t, vendInvDisc.InsertOrError(false))

	quote := newDocument(t, src, gtables.PurchaseHeader_Document_type.Quote, "V00010")
	if quote.Invoice_disc_code != "V00010" {
		t.Fatalf("invoice disc code %q", quote.Invoice_disc_code)
	}
	addLine(t, quote, 10000, gtables.PurchaseLine_Type.Item, "1000", "4") // 180.00
	addLine(t, quote, 20000, gtables.PurchaseLine_Type.Item, "1010", "1") // 89.00
	must(t, purchcalcdisc.CalculateInvoiceDiscount(quote))

	totals, err := purchpost.SumPurchLines(quote, purchpost.General)
	must(t, err)
	wantDec(t, "invoice discount", totals.TotalPurchLine.Inv_discount_amount, "13.45") // 5 % of 269.00
	wantDec(t, "total after invoice discount", totals.TotalPurchLine.Amount, "255.55")
	wantDec(t, "quantity", totals.TotalPurchLine.Quantity, "5")

	comment := gtables.NewPurchCommentLine(src)
	comment.Document_type = gtables.PurchCommentLine_Document_type.Quote
	comment.No = quote.No
	comment.Line_no = 10000
	comment.Comment = "Deliver to the back door"
	must(t, comment.InsertOrError(false))

	order, err := purchquotetoorder.Run(quote)
	must(t, err)
	if order.Document_type != gtables.PurchaseHeader_Document_type.Order || order.No == "" || order.No == quote.No ||
		order.Expected_receipt_date.IsZero() || order.Posting_date.IsZero() {
		t.Fatalf("order %+v", order)
	}
	orderLine := gtables.NewPurchaseLine(src)
	orderLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	orderLine.SetRange("document_no", order.No)
	if orderLine.Count() != 2 {
		t.Errorf("order lines %d", orderLine.Count())
	}
	orderComment := gtables.NewPurchCommentLine(src)
	orderComment.SetRange("document_type", gtables.PurchCommentLine_Document_type.Order)
	orderComment.SetRange("no", order.No)
	if orderComment.Count() != 1 {
		t.Errorf("order comments %d", orderComment.Count())
	}
	gone := gtables.NewPurchaseHeader(src)
	if gone.Get(map[string]interface{}{"document_type": int(gtables.PurchaseHeader_Document_type.Quote), "no": quote.No}) {
		t.Error("the quote must be deleted")
	}
}
