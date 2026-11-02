package purchposting_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/blanketorder"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A purchase blanket order is called off in orders: Make Order takes each line's Qty. to Receive
// into a new order, and receiving and invoicing the order counts on the blanket line
func TestBlanketPurchaseOrder(t *testing.T) {
	src := demoCompany(t)
	blanket := newDocument(t, src, gtables.PurchaseHeader_Document_type.BlanketOrder, "V00010")
	if blanket.No != "PB000001" {
		t.Errorf("blanket order no. %s, want PB000001", blanket.No)
	}
	addLine(t, blanket, 10000, gtables.PurchaseLine_Type.Item, "1000", "20")
	line := getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_receive", dec("5")))
	must(t, line.ModifyOrError(true))

	order, err := blanketorder.PurchToOrder(blanket)
	must(t, err)
	orderLine := getLine(t, order, 10000)
	wantDec(t, "order line quantity", orderLine.Quantity, dec("5"))
	if orderLine.Blanket_order_no != blanket.No || orderLine.Blanket_order_line_no != 10000 {
		t.Errorf("order line from blanket %s line %d", orderLine.Blanket_order_no, orderLine.Blanket_order_line_no)
	}
	wantDec(t, "blanket Qty. to Receive after Make Order", getLine(t, blanket, 10000).Qty_to_receive, dec("0"))

	order.Vendor_invoice_no = "VB-1"
	must(t, order.ModifyOrError(false))
	post(t, order, true, false)
	line = getLine(t, blanket, 10000)
	wantDec(t, "blanket Quantity Received", line.Quantity_received, dec("5"))
	wantDec(t, "blanket Quantity Invoiced before the invoice", line.Quantity_invoiced, dec("0"))
	wantDec(t, "blanket Outstanding Quantity", line.Outstanding_quantity, dec("15"))
	post(t, order, false, true)
	wantDec(t, "blanket Quantity Invoiced", getLine(t, blanket, 10000).Quantity_invoiced, dec("5"))
}
