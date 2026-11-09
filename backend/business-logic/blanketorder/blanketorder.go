// Package blanketorder is codeunits 87 Blanket Sales Order to Order and 97 Blanket Purch. Order
// to Order: an order is made from a blanket order — the order gets the blanket order's header
// fields and dates and a line for each of its lines, with the line's Qty. to Ship / Qty. to
// Receive as quantity and the blanket order line it is taken from (Blanket Order No./Line No.);
// the blanket lines' quantities to ship / receive go back to 0 and the blanket order stays.
// Posting the order adds what it ships / receives and invoices to the blanket order lines.
package blanketorder

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesToOrder (codeunit 87 OnRun) makes the order of a sales blanket order and returns it
// (GetSalesOrderHeader). Each blanket line's Qty. to Ship (Base) plus what orders not yet posted
// take from it cannot be more than its outstanding quantity; the blanket line's reservations go
// with the quantity to the order line; with a dialog an order line that takes more of an item than
// is available asks first (codeunit 311). Not migrated: the credit limit check (codeunit 312).
func SalesToOrder(rec *gtables.SalesHeader) (*gtables.SalesHeader, error) {
	return SalesToOrderHideValidationDialog(rec, false)
}

// SalesToOrderHideValidationDialog (SetHideValidationDialog + OnRun): hideValidationDialog
// leaves out the availability question (code that makes orders without asking: demo data)
func SalesToOrderHideValidationDialog(rec *gtables.SalesHeader, hideValidationDialog bool) (*gtables.SalesHeader, error) {
	err := rec.TestField("document_type", gtables.SalesHeader_Document_type.BlanketOrder)
	if err != nil {
		return nil, err
	}
	salesSetup := gtables.GetSalesReceivablesSetup(rec)
	blanketType := gtables.SalesLineDocument_type(rec.Document_type)

	salesOrderHeader := gtables.NewSalesHeader(rec)
	salesOrderHeader.TransferFields(rec)
	salesOrderHeader.Document_type = gtables.SalesHeader_Document_type.Order
	salesOrderHeader.No_printed = 0
	salesOrderHeader.No = ""
	salesOrderLine := gtables.NewSalesLine(rec)
	salesOrderLine.LockTable()
	err = salesOrderHeader.InsertOrError(true)
	if err != nil {
		return nil, err
	}
	salesOrderHeader.Order_date = rec.Order_date
	salesOrderHeader.Posting_date = rec.Posting_date
	salesOrderHeader.Document_date = rec.Document_date
	salesOrderHeader.Shipment_date = rec.Shipment_date
	err = salesOrderHeader.ModifyOrError(false)
	if err != nil {
		return nil, err
	}

	salesBlanketOrderLine := gtables.NewSalesLine(rec)
	salesBlanketOrderLine.SetRange("document_type", blanketType)
	salesBlanketOrderLine.SetRange("document_no", rec.No)
	linesCreated := false
	for found := salesBlanketOrderLine.FindSetBuffered(); found; found = salesBlanketOrderLine.Next() {
		quantityOnOrders := types.Decimal{}
		salesLine := gtables.NewSalesLine(rec)
		salesLine.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		salesLine.SetRange("blanket_order_no", salesBlanketOrderLine.Document_no)
		salesLine.SetRange("blanket_order_line_no", salesBlanketOrderLine.Line_no)
		for ok := salesLine.FindSetBuffered(); ok; ok = salesLine.Next() {
			if salesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo {
				quantityOnOrders = quantityOnOrders.Sub(salesLine.Outstanding_qty_base)
			} else {
				quantityOnOrders = quantityOnOrders.Add(salesLine.Outstanding_qty_base)
			}
		}
		if salesBlanketOrderLine.Qty_to_ship_base.Abs().Add(quantityOnOrders.Abs()).GreaterThan(salesBlanketOrderLine.Outstanding_qty_base.Abs()) ||
			salesBlanketOrderLine.Quantity_base.Mul(salesBlanketOrderLine.Outstanding_qty_base).IsNegative() {
			return nil, apperrors.BlanketOrderQtyToMake(gtables.SalesLineTableName, "qty_to_ship_base", salesBlanketOrderLine.Line_no,
				salesBlanketOrderLine.Outstanding_qty_base.String(), quantityOnOrders.String(),
				salesBlanketOrderLine.Outstanding_qty_base.Sub(quantityOnOrders).String())
		}

		salesOrderLine = gtables.NewSalesLine(rec)
		salesOrderLine.TransferFields(salesBlanketOrderLine)
		resetSalesQuantityFields(salesOrderLine)
		salesOrderLine.Document_type = gtables.SalesLineDocument_type(salesOrderHeader.Document_type)
		salesOrderLine.Document_no = salesOrderHeader.No
		salesOrderLine.Shipment_date = salesOrderHeader.Shipment_date
		salesOrderLine.Blanket_order_no = rec.No
		salesOrderLine.Blanket_order_line_no = salesBlanketOrderLine.Line_no
		if salesOrderLine.No != "" {
			err = salesOrderLine.Validate("quantity", salesBlanketOrderLine.Qty_to_ship)
			if err == nil {
				err = salesOrderLine.Validate("shipment_date", salesBlanketOrderLine.Shipment_date)
			}
			if err == nil {
				err = salesOrderLine.Validate("line_discount_percent", salesBlanketOrderLine.Line_discount_percent)
			}
			if err == nil && (salesOrderLine.Reserve != gtables.SalesLine_Reserve.Always) && !hideValidationDialog && system.GuiAllowed() {
				err = tables.ItemCheckAvailSalesLine(salesOrderLine)
			}
			if err == nil && !salesBlanketOrderLine.Reserved_qty_base.IsZero() {
				err = reservmgt.New(rec).TransferSaleLineToSalesLine(salesBlanketOrderLine, salesOrderLine, salesBlanketOrderLine.Qty_to_ship_base)
			}
			if err != nil {
				return nil, err
			}
		}
		err = salesOrderLine.InsertOrError(false)
		if err != nil {
			return nil, err
		}
		if !salesBlanketOrderLine.Qty_to_ship.IsZero() {
			linesCreated = true
			blanketLine := gtables.NewSalesLine(rec)
			blanketLine.Copy(salesBlanketOrderLine)
			err = blanketLine.Validate("qty_to_ship", types.Decimal{})
			if err == nil {
				err = blanketLine.ModifyOrError(false)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if !linesCreated {
		return nil, apperrors.BlanketOrderNoLines()
	}
	if salesSetup.Copy_comments_blanket_to_order {
		salesCommentLine := gtables.NewSalesCommentLine(rec)
		salesCommentLine.SetRange("document_type", gtables.SalesCommentLineDocument_type(rec.Document_type))
		salesCommentLine.SetRange("no", rec.No)
		for found := salesCommentLine.FindSetBuffered(); found; found = salesCommentLine.Next() {
			salesCommentLine2 := gtables.NewSalesCommentLine(rec)
			salesCommentLine2.TransferFields(salesCommentLine)
			salesCommentLine2.Document_type = gtables.SalesCommentLineDocument_type(salesOrderHeader.Document_type)
			salesCommentLine2.No = salesOrderHeader.No
			err = salesCommentLine2.InsertOrError(false)
			if err != nil {
				return nil, err
			}
		}
	}
	return salesOrderHeader, nil
}

// resetSalesQuantityFields (ResetQuantityFields): a new line has nothing shipped or invoiced
func resetSalesQuantityFields(tempSalesLine *gtables.SalesLine) {
	tempSalesLine.Qty_shipped_not_invoiced = types.Decimal{}
	tempSalesLine.Quantity_shipped = types.Decimal{}
	tempSalesLine.Quantity_invoiced = types.Decimal{}
	tempSalesLine.Qty_shipped_base = types.Decimal{}
	tempSalesLine.Qty_invoiced_base = types.Decimal{}
	tempSalesLine.Reserved_quantity = types.Decimal{}
	tempSalesLine.Reserved_qty_base = types.Decimal{}
}

// PurchToOrder (codeunit 97 OnRun) makes the order of a purchase blanket order and returns it
// (GetPurchOrderHeader); as SalesToOrder, with Qty. to Receive and Expected Receipt Date
func PurchToOrder(rec *gtables.PurchaseHeader) (*gtables.PurchaseHeader, error) {
	err := rec.TestField("document_type", gtables.PurchaseHeader_Document_type.BlanketOrder)
	if err != nil {
		return nil, err
	}
	purchSetup := gtables.GetPurchasesPayablesSetup(rec)
	blanketType := gtables.PurchaseLineDocument_type(rec.Document_type)

	purchOrderHeader := gtables.NewPurchaseHeader(rec)
	purchOrderHeader.TransferFields(rec)
	purchOrderHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
	purchOrderHeader.No_printed = 0
	purchOrderHeader.No = ""
	purchOrderLine := gtables.NewPurchaseLine(rec)
	purchOrderLine.LockTable()
	err = purchOrderHeader.InsertOrError(true)
	if err != nil {
		return nil, err
	}
	purchOrderHeader.Order_date = rec.Order_date
	purchOrderHeader.Posting_date = rec.Posting_date
	purchOrderHeader.Document_date = rec.Document_date
	purchOrderHeader.Expected_receipt_date = rec.Expected_receipt_date
	err = purchOrderHeader.ModifyOrError(false)
	if err != nil {
		return nil, err
	}

	purchBlanketOrderLine := gtables.NewPurchaseLine(rec)
	purchBlanketOrderLine.SetRange("document_type", blanketType)
	purchBlanketOrderLine.SetRange("document_no", rec.No)
	linesCreated := false
	for found := purchBlanketOrderLine.FindSetBuffered(); found; found = purchBlanketOrderLine.Next() {
		quantityOnOrders := types.Decimal{}
		purchLine := gtables.NewPurchaseLine(rec)
		purchLine.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		purchLine.SetRange("blanket_order_no", purchBlanketOrderLine.Document_no)
		purchLine.SetRange("blanket_order_line_no", purchBlanketOrderLine.Line_no)
		for ok := purchLine.FindSetBuffered(); ok; ok = purchLine.Next() {
			if purchLine.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
				quantityOnOrders = quantityOnOrders.Sub(purchLine.Outstanding_qty_base)
			} else {
				quantityOnOrders = quantityOnOrders.Add(purchLine.Outstanding_qty_base)
			}
		}
		if purchBlanketOrderLine.Qty_to_receive_base.Abs().Add(quantityOnOrders.Abs()).GreaterThan(purchBlanketOrderLine.Outstanding_qty_base.Abs()) ||
			purchBlanketOrderLine.Quantity_base.Mul(purchBlanketOrderLine.Outstanding_qty_base).IsNegative() {
			return nil, apperrors.BlanketOrderQtyToMake(gtables.PurchaseLineTableName, "qty_to_receive_base", purchBlanketOrderLine.Line_no,
				purchBlanketOrderLine.Outstanding_qty_base.String(), quantityOnOrders.String(),
				purchBlanketOrderLine.Outstanding_qty_base.Sub(quantityOnOrders).String())
		}

		purchOrderLine = gtables.NewPurchaseLine(rec)
		purchOrderLine.TransferFields(purchBlanketOrderLine)
		resetPurchQuantityFields(purchOrderLine)
		purchOrderLine.Document_type = gtables.PurchaseLineDocument_type(purchOrderHeader.Document_type)
		purchOrderLine.Document_no = purchOrderHeader.No
		purchOrderLine.Expected_receipt_date = purchOrderHeader.Expected_receipt_date
		purchOrderLine.Blanket_order_no = rec.No
		purchOrderLine.Blanket_order_line_no = purchBlanketOrderLine.Line_no
		if purchOrderLine.No != "" {
			err = purchOrderLine.Validate("quantity", purchBlanketOrderLine.Qty_to_receive)
			if err == nil {
				err = purchOrderLine.Validate("expected_receipt_date", purchBlanketOrderLine.Expected_receipt_date)
			}
			if err == nil {
				err = purchOrderLine.Validate("line_discount_percent", purchBlanketOrderLine.Line_discount_percent)
			}
			if err == nil && !purchBlanketOrderLine.Reserved_qty_base.IsZero() {
				err = reservmgt.New(rec).TransferPurchLineToPurchLine(purchBlanketOrderLine, purchOrderLine, purchBlanketOrderLine.Qty_to_receive_base)
			}
			if err != nil {
				return nil, err
			}
		}
		err = purchOrderLine.InsertOrError(false)
		if err != nil {
			return nil, err
		}
		if !purchBlanketOrderLine.Qty_to_receive.IsZero() {
			linesCreated = true
			blanketLine := gtables.NewPurchaseLine(rec)
			blanketLine.Copy(purchBlanketOrderLine)
			err = blanketLine.Validate("qty_to_receive", types.Decimal{})
			if err == nil {
				err = blanketLine.ModifyOrError(false)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if !linesCreated {
		return nil, apperrors.BlanketOrderNoLines()
	}
	if purchSetup.Copy_comments_blanket_to_order {
		purchCommentLine := gtables.NewPurchCommentLine(rec)
		purchCommentLine.SetRange("document_type", gtables.PurchCommentLineDocument_type(rec.Document_type))
		purchCommentLine.SetRange("no", rec.No)
		for found := purchCommentLine.FindSetBuffered(); found; found = purchCommentLine.Next() {
			purchCommentLine2 := gtables.NewPurchCommentLine(rec)
			purchCommentLine2.TransferFields(purchCommentLine)
			purchCommentLine2.Document_type = gtables.PurchCommentLineDocument_type(purchOrderHeader.Document_type)
			purchCommentLine2.No = purchOrderHeader.No
			err = purchCommentLine2.InsertOrError(false)
			if err != nil {
				return nil, err
			}
		}
	}
	return purchOrderHeader, nil
}

// resetPurchQuantityFields (ResetQuantityFields): a new line has nothing received or invoiced
func resetPurchQuantityFields(tempPurchLine *gtables.PurchaseLine) {
	tempPurchLine.Qty_rcd_not_invoiced = types.Decimal{}
	tempPurchLine.Quantity_received = types.Decimal{}
	tempPurchLine.Quantity_invoiced = types.Decimal{}
	tempPurchLine.Qty_rcd_not_invd_base = types.Decimal{}
	tempPurchLine.Qty_received_base = types.Decimal{}
	tempPurchLine.Qty_invoiced_base = types.Decimal{}
	tempPurchLine.Reserved_quantity = types.Decimal{}
	tempPurchLine.Reserved_qty_base = types.Decimal{}
}
