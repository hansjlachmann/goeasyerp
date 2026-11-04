package reservmgt_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesquotetoorder"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

var postingDate = types.NewDate(2026, 10, 20)

// shipSalesOrder posts the shipment of the line's order on the line's shipment date
func shipSalesOrder(t *testing.T, line *gtables.SalesLine) {
	t.Helper()
	header := gtables.NewSalesHeader(line)
	must(t, header.GetOrError(map[string]interface{}{"document_type": int(line.Document_type), "no": line.Document_no}))
	must(t, header.Validate("posting_date", postingDate))
	header.Ship, header.Invoice = true, false
	must(t, salesposting.NewSalesPost(header).Run(header))
}

// openEntries: the open reservation entries of a line as the demand
func openEntries(src ftables.Source, s reservmgt.Source) *gtables.ReservationEntry {
	entries := gtables.NewReservationEntry(src)
	reservmgt.InitFilterAndSortingLookupFor(entries, true)
	reservmgt.FilterFor(entries, s)
	return entries
}

// Shipping a sales order line that reserved from the inventory applies its item entry to the
// entries it reserved, closes the reservations and frees the line and the entries
func TestShipmentAppliesReservedEntries(t *testing.T) {
	src := demoCompany(t)
	line := salesOrderLine(t, src, "3")
	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	_, err := m.AutoReserve(line.Shipment_date, "", line.Shipment_date)
	must(t, err)
	entries := openEntries(src, reservmgt.SalesLineSource(line))
	if !entries.FindFirst() {
		t.Fatal("not reserved")
	}
	reservedILE := entries.From_ref_no

	reload(t, line)
	shipSalesOrder(t, line)

	if entries.Count() != 0 {
		t.Errorf("%d reservations still open", entries.Count())
	}
	reload(t, line)
	if !line.Reserved_qty_base.IsZero() || !line.Qty_shipped_base.Equal(dec("3")) {
		t.Errorf("line reserved %s, shipped %s", line.Reserved_qty_base, line.Qty_shipped_base)
	}
	ile := gtables.NewItemLedgerEntry(src)
	ile.SetRange("item_no", types.Code("1000"))
	must(t, ile.CalcSums("reserved_quantity"))
	if !ile.Reserved_quantity.IsZero() {
		t.Errorf("item entries still reserved %s", ile.Reserved_quantity)
	}
	sale := gtables.NewItemLedgerEntry(src)
	sale.SetRange("item_no", types.Code("1000"))
	sale.SetRange("entry_type", gtables.ItemLedgerEntry_Entry_type.Sale)
	if !sale.FindLast() || !sale.Quantity.Equal(dec("-3")) {
		t.Fatalf("sale entry %s", sale.Quantity)
	}
	appl := gtables.NewItemApplicationEntry(src)
	appl.SetRange("item_ledger_entry_no", sale.Entry_no)
	appl.SetRange("applied_to_entry_no", reservedILE)
	if appl.Count() == 0 {
		t.Errorf("the sale (entry %d) is not applied to the reserved entry %d", sale.Entry_no, reservedILE)
	}
}

// A sales order line reserved from a purchase order line: the receipt gives the reservation to
// the item entry it posts, the shipment then takes that entry
func TestReceiptMovesReservationToInventory(t *testing.T) {
	src := demoCompany(t)
	vendor := gtables.NewVendor(src)
	if !vendor.FindFirst() {
		t.Fatal("no demo vendor")
	}
	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
	must(t, purchHeader.InsertOrError(true))
	must(t, purchHeader.Validate("buy_from_vendor_no", vendor.No))
	must(t, purchHeader.Validate("posting_date", postingDate))
	must(t, purchHeader.ModifyOrError(true))
	purchLine := gtables.NewPurchaseLine(src)
	purchLine.Document_type = gtables.PurchaseLine_Document_type.Order
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = 10000
	must(t, purchLine.Validate("type", gtables.PurchaseLine_Type.Item))
	must(t, purchLine.Validate("no", types.Code("1000")))
	must(t, purchLine.Validate("location_code", types.Code("MAIN")))
	must(t, purchLine.Validate("quantity", dec("5")))
	must(t, purchLine.Validate("expected_receipt_date", postingDate))
	must(t, purchLine.InsertOrError(true))

	line := salesOrderLine(t, src, "2")
	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	left, err := m.AutoReserveOneLine(reservmgt.SummaryPurchaseOrder, dec("2"), line.Shipment_date, "", line.Shipment_date)
	must(t, err)
	must(t, purchLine.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "document_no": purchLine.Document_no, "line_no": purchLine.Line_no}))
	if !left.IsZero() || !purchLine.Reserved_qty_base.Equal(dec("-2")) {
		t.Fatalf("left %s, purchase line reserved %s", left, purchLine.Reserved_qty_base)
	}

	purchHeader.Receive, purchHeader.Invoice = true, false
	must(t, purchposting.NewPurchPost(purchHeader).Run(purchHeader))

	must(t, purchLine.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "document_no": purchLine.Document_no, "line_no": purchLine.Line_no}))
	if !purchLine.Reserved_qty_base.IsZero() {
		t.Errorf("purchase line still reserved %s", purchLine.Reserved_qty_base)
	}
	entries := openEntries(src, reservmgt.SalesLineSource(line))
	must(t, entries.CalcSums("quantity"))
	if !entries.Quantity.Equal(dec("2")) || !entries.FindFirst() || int(entries.From_type) != reservmgt.TypeItemLedgerEntry {
		t.Fatalf("sales line's reservations %s from %v", entries.Quantity, entries.From_type)
	}
	receipt := gtables.NewItemLedgerEntry(src)
	must(t, receipt.GetOrError(entries.From_ref_no))
	if receipt.Entry_type != gtables.ItemLedgerEntry_Entry_type.Purchase || !receipt.Reserved_quantity.Equal(dec("-2")) {
		t.Errorf("reserved from entry %d (%v), reserved %s", receipt.Entry_no, receipt.Entry_type, receipt.Reserved_quantity)
	}
	reload(t, line)
	if !line.Reserved_qty_base.Equal(dec("2")) {
		t.Errorf("sales line reserved %s", line.Reserved_qty_base)
	}

	shipSalesOrder(t, line)
	must(t, receipt.GetOrError(receipt.Entry_no))
	if !receipt.Reserved_quantity.IsZero() || !receipt.Remaining_quantity.Equal(dec("3")) {
		t.Errorf("receipt after the shipment: reserved %s, remaining %s", receipt.Reserved_quantity, receipt.Remaining_quantity)
	}
}

// A quote line's reservations go to the order line made of it
func TestQuoteToOrderTransfersReservations(t *testing.T) {
	src := demoCompany(t)
	reservSetup := gtables.GetReservationSetup(src)
	reservSetup.Reserve_sales_quote = true
	must(t, reservSetup.ModifyOrError(false))
	cust := gtables.NewCustomer(src)
	if !cust.FindFirst() {
		t.Fatal("no demo customer")
	}
	quote := gtables.NewSalesHeader(src)
	quote.Document_type = gtables.SalesHeader_Document_type.Quote
	must(t, quote.InsertOrError(true))
	must(t, quote.Validate("sell_to_customer_no", cust.No))
	must(t, quote.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Quote
	line.Document_no = quote.No
	line.Line_no = 10000
	must(t, line.Validate("type", gtables.SalesLine_Type.Item))
	must(t, line.Validate("no", types.Code("1000")))
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.Validate("quantity", dec("1")))
	must(t, line.Validate("shipment_date", postingDate))
	must(t, line.InsertOrError(true))
	m := reservmgt.New(src)
	must(t, m.SalesLineSetReservFor(line))
	_, err := m.AutoReserve(line.Shipment_date, "", line.Shipment_date)
	must(t, err)

	order, err := salesquotetoorder.Run(quote)
	must(t, err)
	orderLine := gtables.NewSalesLine(src)
	must(t, orderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order), "document_no": order.No, "line_no": 10000}))
	entries := openEntries(src, reservmgt.SalesLineSource(orderLine))
	must(t, entries.CalcSums("quantity"))
	if !orderLine.Reserved_qty_base.Equal(dec("1")) || !entries.Quantity.Equal(dec("1")) {
		t.Errorf("order line reserved %s, entries %s", orderLine.Reserved_qty_base, entries.Quantity)
	}
	if openEntries(src, reservmgt.SalesLineSource(line)).Count() != 0 {
		t.Error("the quote line keeps reservations")
	}
}
