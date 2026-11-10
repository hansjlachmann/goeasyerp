package salesposting_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/blanketorder"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A blanket order is called off in orders: Make Order takes each line's Qty. to Ship into a new
// order (the blanket line's Qty. to Ship goes back to 0), posting the order adds the shipped and
// invoiced quantities to the blanket line, and no more than the blanket line's quantity can be
// called off
func TestBlanketSalesOrder(t *testing.T) {
	src := demoCompany(t)
	blanket := newDocument(t, src, gtables.SalesHeader_Document_type.BlanketOrder, "C00010")
	if blanket.No != "SB000001" {
		t.Errorf("blanket order no. %s, want SB000001", blanket.No)
	}
	addLine(t, blanket, 10000, gtables.SalesLine_Type.Item, "1000", "10")
	line := getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", dec("4")))
	must(t, line.ModifyOrError(true))

	order, err := blanketorder.SalesToOrder(blanket)
	must(t, err)
	if order.Document_type != gtables.SalesHeader_Document_type.Order || order.Sell_to_customer_no != "C00010" {
		t.Fatalf("order %v %s for %s", order.Document_type, order.No, order.Sell_to_customer_no)
	}
	orderLine := getLine(t, order, 10000)
	wantDec(t, "order line quantity", orderLine.Quantity, dec("4"))
	if orderLine.Blanket_order_no != blanket.No || orderLine.Blanket_order_line_no != 10000 {
		t.Errorf("order line from blanket %s line %d", orderLine.Blanket_order_no, orderLine.Blanket_order_line_no)
	}
	wantDec(t, "blanket Qty. to Ship after Make Order", getLine(t, blanket, 10000).Qty_to_ship, types.Decimal{})

	// the order shipped and invoiced: the blanket line has shipped and invoiced 4
	post(t, order, true, true)
	line = getLine(t, blanket, 10000)
	wantDec(t, "blanket Quantity Shipped", line.Quantity_shipped, dec("4"))
	wantDec(t, "blanket Quantity Invoiced", line.Quantity_invoiced, dec("4"))
	wantDec(t, "blanket Outstanding Quantity", line.Outstanding_quantity, dec("6"))

	// the posted order is gone; another order of the remaining 6, not posted yet
	line = getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", dec("6")))
	must(t, line.ModifyOrError(true))
	_, err = blanketorder.SalesToOrder(blanket)
	must(t, err)
	// a blanket line an order line takes from is not deleted
	err = getLine(t, blanket, 10000).DeleteOrError(true)
	var appErr *apperrors.AppError
	if err == nil {
		t.Error("a blanket line with an order line was deleted")
	}

	// (refused Make Orders: the codeunit rolls back what they made; here it stays — last)
	// 1 more while the order of 6 is not posted: more than is left
	line = getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", dec("1")))
	must(t, line.ModifyOrError(true))
	_, err = blanketorder.SalesToOrder(blanket)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrBlanketOrderQtyToMake {
		t.Errorf("Make Order of 1 with 6 on an order: %v", err)
	}
	// nothing to ship: no order
	line = getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", types.Decimal{}))
	must(t, line.ModifyOrError(true))
	_, err = blanketorder.SalesToOrder(blanket)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrBlanketOrderNoLines {
		t.Errorf("Make Order with nothing to ship: %v", err)
	}
}

// Make Order checks the order against the customer's credit limit (codeunit 312) before making
// it: the blanket item lines at their Qty. to Ship; without a dialog the warning is a note and
// the order is made, hiding the validation dialog leaves the check out
func TestBlanketSalesOrderCreditLimit(t *testing.T) {
	src := demoCompany(t)
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Credit_warnings = gtables.SalesReceivablesSetup_Credit_warnings.CreditLimit
	must(t, salesSetup.ModifyOrError(false))
	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C00010")))
	cust.Credit_limit_lcy = types.NewDecimalFromInt(1)
	must(t, cust.ModifyOrError(false))

	blanket := newDocument(t, src, gtables.SalesHeader_Document_type.BlanketOrder, "C00010")
	addLine(t, blanket, 10000, gtables.SalesLine_Type.Item, "1000", "10")
	line := getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", dec("4")))
	must(t, line.ModifyOrError(true))

	fcodeunits.StartNotes()
	order, err := blanketorder.SalesToOrder(blanket)
	notes := fcodeunits.TakeNotes()
	must(t, err)
	if len(notes) != 1 || !strings.Contains(notes[0], "credit limit has been exceeded") || !strings.Contains(notes[0], "C00010") {
		t.Errorf("credit limit warning of Make Order: %v", notes)
	}
	wantDec(t, "order line quantity", getLine(t, order, 10000).Quantity, dec("4"))

	line = getLine(t, blanket, 10000)
	must(t, line.Validate("qty_to_ship", dec("2")))
	must(t, line.ModifyOrError(true))
	fcodeunits.StartNotes()
	_, err = blanketorder.SalesToOrderHideValidationDialog(blanket, true)
	notes = fcodeunits.TakeNotes()
	must(t, err)
	if len(notes) != 0 {
		t.Errorf("Make Order with the validation dialog hidden warned: %v", notes)
	}
}
