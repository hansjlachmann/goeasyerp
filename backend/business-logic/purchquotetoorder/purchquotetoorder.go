// Package purchquotetoorder is codeunit 96 Purch.-Quote to Order: a quote becomes an order — the
// order gets the next number of the Order Nos., the quote's header fields, dates, lines and
// comments, and the quote is deleted.
package purchquotetoorder

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Run (OnRun) makes the order of a quote and returns it (GetPurchOrderHeader); the quote lines'
// reservations go to the order lines.
func Run(rec *gtables.PurchaseHeader) (*gtables.PurchaseHeader, error) {
	err := rec.TestField("document_type", gtables.PurchaseHeader_Document_type.Quote)
	if err != nil {
		return nil, err
	}
	vend := gtables.NewVendor(rec)
	err = vend.GetOrError(rec.Buy_from_vendor_no)
	if err != nil {
		return nil, err
	}

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
	if !rec.Posting_date.IsZero() {
		purchOrderHeader.Posting_date = rec.Posting_date
	}
	purchOrderHeader.Document_date = rec.Document_date
	// a quote has no expected receipt date (or posting date): the order keeps its own then
	if !rec.Expected_receipt_date.IsZero() {
		purchOrderHeader.Expected_receipt_date = rec.Expected_receipt_date
	}
	purchOrderHeader.Dimension_1_code = rec.Dimension_1_code
	purchOrderHeader.Dimension_2_code = rec.Dimension_2_code
	purchOrderHeader.Location_code = rec.Location_code
	err = purchOrderHeader.ModifyOrError(false)
	if err != nil {
		return nil, err
	}

	purchQuoteLine := gtables.NewPurchaseLine(rec)
	purchQuoteLine.SetRange("document_type", gtables.PurchaseLineDocument_type(rec.Document_type))
	purchQuoteLine.SetRange("document_no", rec.No)
	reservMgt := reservmgt.New(rec)
	for found := purchQuoteLine.FindSet(); found; found = purchQuoteLine.Next() {
		purchOrderLine = gtables.NewPurchaseLine(rec)
		purchOrderLine.TransferFields(purchQuoteLine)
		if !purchOrderLine.Reserved_qty_base.IsZero() {
			err = purchOrderLine.Validate("reserved_qty_base", types.Decimal{})
			if err != nil {
				return nil, err
			}
		}
		purchOrderLine.Document_type = gtables.PurchaseLineDocument_type(purchOrderHeader.Document_type)
		purchOrderLine.Document_no = purchOrderHeader.No
		purchOrderLine.Expected_receipt_date = purchOrderHeader.Expected_receipt_date
		if !purchQuoteLine.Reserved_qty_base.IsZero() {
			err = reservMgt.TransferPurchLineToPurchLine(purchQuoteLine, purchOrderLine, purchQuoteLine.Reserved_qty_base)
			if err == nil {
				err = purchQuoteLine.TestField("reserved_qty_base", types.Decimal{})
			}
			if err != nil {
				return nil, err
			}
		}
		err = purchOrderLine.InsertOrError(false)
		if err != nil {
			return nil, err
		}
	}

	err = moveComments(rec, purchOrderHeader)
	if err != nil {
		return nil, err
	}

	err = rec.DeleteOrError(false)
	if err != nil {
		return nil, err
	}
	purchQuoteLine.DeleteAll()
	return purchOrderHeader, nil
}

// moveComments: the quote's comment lines become the order's
func moveComments(purchHeader, purchOrderHeader *gtables.PurchaseHeader) error {
	purchCommentLine := gtables.NewPurchCommentLine(purchHeader)
	purchCommentLine.SetRange("document_type", gtables.PurchCommentLineDocument_type(purchHeader.Document_type))
	purchCommentLine.SetRange("no", purchHeader.No)
	for found := purchCommentLine.FindSet(); found; found = purchCommentLine.Next() {
		purchCommentLine2 := gtables.NewPurchCommentLine(purchHeader)
		purchCommentLine2.TransferFields(purchCommentLine)
		purchCommentLine2.Document_type = gtables.PurchCommentLineDocument_type(purchOrderHeader.Document_type)
		purchCommentLine2.No = types.Code(purchOrderHeader.No)
		err := purchCommentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	purchCommentLine.DeleteAll()
	return nil
}
