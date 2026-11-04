package salesquotetoorder_test

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesquotetoorder"
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

// orderCompany: quote and order number series, customer C1, item 1000 at 10.00 (PCS, BOX of 10)
func orderCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "orders", database.DBTypeSQLite))
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "orders", database.DBTypeSQLite)

	for _, series := range []struct{ code, start string }{{"S-QUO", "SQ000001"}, {"S-ORD", "SO000001"}, {"S-INV", "SI000001"}, {"S-INV+", "PSI000001"}, {"S-CR", "SCM000001"}, {"S-CR+", "PSCM000001"}} {
		noSeries := gtables.NewNoSeries(src)
		noSeries.Code = types.Code(series.code)
		noSeries.Default_nos = true
		must(t, noSeries.InsertOrError(false))
		noSeriesLine := gtables.NewNoSeriesLine(src)
		noSeriesLine.Series_code = types.Code(series.code)
		noSeriesLine.Line_no = 10000
		noSeriesLine.Starting_no = types.Code(series.start)
		noSeriesLine.Increment_by_no = 1
		noSeriesLine.Open = true
		must(t, noSeriesLine.InsertOrError(false))
	}
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Quote_nos = "S-QUO"
	salesSetup.Order_nos = "S-ORD"
	salesSetup.Invoice_nos = "S-INV"
	salesSetup.Posted_invoice_nos = "S-INV+"
	salesSetup.Credit_memo_nos = "S-CR"
	salesSetup.Posted_credit_memo_nos = "S-CR+"
	must(t, salesSetup.InsertOrError(false))

	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Name = "Customer C1"
	cust.Customer_posting_group = "DOM"
	cust.Gen_bus_posting_group = "DOM"
	must(t, cust.InsertOrError(false))
	for _, u := range []string{"PCS", "BOX"} {
		unitOfMeasure := gtables.NewUnitOfMeasure(src)
		unitOfMeasure.Code = types.Code(u)
		must(t, unitOfMeasure.InsertOrError(false))
	}
	item := gtables.NewItem(src)
	item.No = "1000"
	item.Description = "Office Chair"
	item.Base_unit_of_measure = "PCS"
	item.Sales_unit_of_measure = "PCS"
	item.Unit_price = dec("10")
	item.Inventory_posting_group = "RESALE"
	item.Gen_prod_posting_group = "RETAIL"
	must(t, item.InsertOrError(false))
	for _, u := range []struct {
		code string
		qty  int64
	}{{"PCS", 1}, {"BOX", 10}} {
		itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(src)
		itemUnitOfMeasure.Item_no = "1000"
		itemUnitOfMeasure.Code = types.Code(u.code)
		itemUnitOfMeasure.Qty_per_unit_of_measure = types.NewDecimalFromInt(u.qty)
		must(t, itemUnitOfMeasure.InsertOrError(false))
	}
	return src
}

func newDocument(t *testing.T, src ftables.Source, documentType gtables.SalesHeaderDocument_type) *gtables.SalesHeader {
	t.Helper()
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = documentType
	must(t, salesHeader.InsertOrError(true))
	must(t, salesHeader.Validate("sell_to_customer_no", types.Code("C1")))
	must(t, salesHeader.ModifyOrError(true))
	return salesHeader
}

func addLine(t *testing.T, salesHeader *gtables.SalesHeader, lineNo int, uom, qty string) *gtables.SalesLine {
	t.Helper()
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = lineNo
	must(t, salesLine.Validate("type", gtables.SalesLine_Type.Item))
	must(t, salesLine.Validate("no", types.Code("1000")))
	must(t, salesLine.Validate("unit_of_measure_code", types.Code(uom)))
	must(t, salesLine.Validate("quantity", dec(qty)))
	must(t, salesLine.InsertOrError(true))
	return salesLine
}

func wantError(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Errorf("error %v, want %q", err, text)
	}
}

// An order line ships and invoices all that is outstanding by default; the quantities to ship
// and invoice are checked against what is outstanding and shipped
func TestSalesOrderLineQuantities(t *testing.T) {
	src := orderCompany(t)
	salesOrder := newDocument(t, src, gtables.SalesHeader_Document_type.Order)
	if salesOrder.No != "SO000001" {
		t.Fatalf("order no. %s", salesOrder.No)
	}
	salesLine := addLine(t, salesOrder, 10000, "BOX", "3")
	wantDec(t, "Qty. to Ship", salesLine.Qty_to_ship, "3")
	wantDec(t, "Qty. to Ship (Base)", salesLine.Qty_to_ship_base, "30")
	wantDec(t, "Qty. to Invoice", salesLine.Qty_to_invoice, "3")
	wantDec(t, "Qty. to Invoice (Base)", salesLine.Qty_to_invoice_base, "30")
	wantDec(t, "Outstanding Quantity", salesLine.Outstanding_quantity, "3")
	wantDec(t, "Outstanding Amount", salesLine.Outstanding_amount, "300")

	// Ship part of it: the quantity to invoice follows
	must(t, salesLine.Validate("qty_to_ship", dec("1")))
	wantDec(t, "Qty. to Ship (Base)", salesLine.Qty_to_ship_base, "10")
	wantDec(t, "Qty. to Invoice", salesLine.Qty_to_invoice, "1")
	wantError(t, salesLine.Validate("qty_to_ship", dec("4")), "cannot ship more than 3 units")
	wantError(t, salesLine.Validate("qty_to_ship", dec("-1")), "cannot ship more than 3 units")
	must(t, salesLine.Validate("qty_to_ship", dec("2")))
	wantError(t, salesLine.Validate("qty_to_invoice", dec("3")), "cannot invoice more than 2 units")
	must(t, salesLine.Validate("qty_to_invoice", dec("1")))
	wantDec(t, "Qty. to Invoice (Base)", salesLine.Qty_to_invoice_base, "10")
	// The base quantities only in the base unit of measure
	wantError(t, salesLine.Validate("qty_to_ship_base", dec("10")), "Qty. per Unit of Measure")

	// What posting will do: 2 shipped, 1 of them invoiced
	salesLine.Quantity_shipped, salesLine.Qty_shipped_base = dec("2"), dec("20")
	salesLine.Quantity_invoiced, salesLine.Qty_invoiced_base = dec("1"), dec("10")
	must(t, salesLine.Validate("quantity", dec("3")))
	wantDec(t, "Outstanding Quantity", salesLine.Outstanding_quantity, "1")
	wantDec(t, "Qty. Shipped Not Invoiced", salesLine.Qty_shipped_not_invoiced, "1")
	wantDec(t, "Qty. Shipped Not Invd. (Base)", salesLine.Qty_shpd_not_invd_base, "10")
	wantDec(t, "Qty. to Ship", salesLine.Qty_to_ship, "1")
	wantDec(t, "Qty. to Invoice", salesLine.Qty_to_invoice, "2") // shipped not invoiced + to ship
	wantDec(t, "Outstanding Amount", salesLine.Outstanding_amount, "100")
	wantDec(t, "Shipped Not Invoiced", salesLine.Shipped_not_invoiced, "100")
	wantDec(t, "Shipped Not Invoiced (LCY)", salesLine.Shipped_not_invoiced_lcy, "100")
	wantError(t, salesLine.Validate("quantity", dec("1")), "must not be less than")
	wantError(t, salesLine.Validate("no", types.Code("1000")), "Qty. Shipped Not Invoiced")
	wantError(t, salesLine.DeleteOrError(true), "Qty. Shipped Not Invoiced")
	must(t, salesLine.ModifyOrError(false))

	// The customer's orders: outstanding and shipped not invoiced
	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C1")))
	cust.CalcFields("outstanding_orders", "shipped_not_invoiced", "outstanding_orders_lcy", "shipped_not_invoiced_lcy")
	wantDec(t, "Outstanding Orders", cust.Outstanding_orders, "100")
	wantDec(t, "Shipped Not Invoiced", cust.Shipped_not_invoiced, "100")
	wantDec(t, "Outstanding Orders (LCY)", cust.Outstanding_orders_lcy, "100")
	wantDec(t, "Shipped Not Invoiced (LCY)", cust.Shipped_not_invoiced_lcy, "100")
}

// Make Order: the order gets a number of the Order Nos., the quote's fields, lines and
// comments; the quote is gone; a quote's lines do not count as orders
func TestSalesQuoteToOrder(t *testing.T) {
	src := orderCompany(t)
	salesQuote := newDocument(t, src, gtables.SalesHeader_Document_type.Quote)
	salesQuote.Your_reference = "REF-1"
	must(t, salesQuote.ModifyOrError(true))
	addLine(t, salesQuote, 10000, "PCS", "4")
	addLine(t, salesQuote, 20000, "BOX", "1")
	comment := gtables.NewSalesCommentLine(src)
	comment.Document_type = gtables.SalesCommentLine_Document_type.Quote
	comment.No = types.Code(salesQuote.No)
	comment.Line_no = 10000
	comment.Comment = "Deliver to the back door"
	must(t, comment.InsertOrError(false))

	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C1")))
	cust.CalcFields("outstanding_orders")
	wantDec(t, "Outstanding Orders of a quote", cust.Outstanding_orders, "0")

	salesOrder, err := salesquotetoorder.Run(salesQuote)
	must(t, err)
	if salesOrder.Document_type != gtables.SalesHeader_Document_type.Order || salesOrder.No != "SO000001" || salesOrder.No_series != "S-ORD" {
		t.Errorf("order %v %s series %s", salesOrder.Document_type, salesOrder.No, salesOrder.No_series)
	}
	stored := gtables.NewSalesHeader(src)
	must(t, stored.GetOrError(map[string]interface{}{"document_type": 1, "no": "SO000001"}))
	if stored.Your_reference != "REF-1" || stored.Sell_to_customer_no != "C1" || stored.Order_date != salesQuote.Order_date || stored.Posting_date.IsZero() {
		t.Errorf("order header %s %s %v %v", stored.Your_reference, stored.Sell_to_customer_no, stored.Order_date, stored.Posting_date)
	}
	salesLine := gtables.NewSalesLine(src)
	salesLine.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	salesLine.SetRange("document_no", types.Code("SO000001"))
	if salesLine.Count() != 2 {
		t.Errorf("order lines %d, want 2", salesLine.Count())
	}
	must(t, salesLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": "SO000001", "line_no": 20000}))
	wantDec(t, "Qty. to Ship of the order line", salesLine.Qty_to_ship, "1")
	wantDec(t, "Quantity (Base)", salesLine.Quantity_base, "10")
	comments := gtables.NewSalesCommentLine(src)
	comments.SetRange("document_type", gtables.SalesCommentLine_Document_type.Order)
	comments.SetRange("no", types.Code("SO000001"))
	if comments.Count() != 1 {
		t.Errorf("order comments %d, want 1", comments.Count())
	}

	quote := gtables.NewSalesHeader(src)
	if quote.Get(map[string]interface{}{"document_type": 0, "no": salesQuote.No}) {
		t.Error("the quote is still there")
	}
	quoteLines := gtables.NewSalesLine(src)
	quoteLines.SetRange("document_type", gtables.SalesLine_Document_type.Quote)
	quoteComments := gtables.NewSalesCommentLine(src)
	quoteComments.SetRange("document_type", gtables.SalesCommentLine_Document_type.Quote)
	if quoteLines.Count() != 0 || quoteComments.Count() != 0 {
		t.Errorf("quote lines %d, comments %d left", quoteLines.Count(), quoteComments.Count())
	}

	cust.CalcFields("outstanding_orders", "outstanding_orders_lcy")
	wantDec(t, "Outstanding Orders", cust.Outstanding_orders, "140")
	wantDec(t, "Outstanding Orders (LCY)", cust.Outstanding_orders_lcy, "140")

	// Only a quote becomes an order
	_, err = salesquotetoorder.Run(salesOrder)
	if err == nil {
		t.Error("an order was made an order")
	}
}
