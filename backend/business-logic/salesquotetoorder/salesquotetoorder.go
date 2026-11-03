// Package salesquotetoorder is codeunit 86 Sales-Quote to Order: a quote becomes an order — the
// order gets the next number of the Order Nos., the quote's header fields, dates, lines and
// comments, and the quote is deleted.
package salesquotetoorder

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Run (OnRun) makes the order of a quote and returns it (GetSalesOrderHeader); the quote lines'
// reservations go to the order lines. Not migrated: the credit limit check (codeunit 312), the
// availability check (311).
func Run(rec *gtables.SalesHeader) (*gtables.SalesHeader, error) {
	err := rec.TestField("document_type", gtables.SalesHeader_Document_type.Quote)
	if err != nil {
		return nil, err
	}
	cust := gtables.NewCustomer(rec)
	err = cust.GetOrError(rec.Sell_to_customer_no)
	if err != nil {
		return nil, err
	}

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
	if !rec.Posting_date.IsZero() {
		salesOrderHeader.Posting_date = rec.Posting_date
	}
	salesOrderHeader.Document_date = rec.Document_date
	salesOrderHeader.Shipment_date = rec.Shipment_date
	salesOrderHeader.Dimension_1_code = rec.Dimension_1_code
	salesOrderHeader.Dimension_2_code = rec.Dimension_2_code
	salesOrderHeader.Location_code = rec.Location_code
	err = salesOrderHeader.ModifyOrError(false)
	if err != nil {
		return nil, err
	}

	reservMgt := reservmgt.New(rec)
	salesQuoteLine := gtables.NewSalesLine(rec)
	salesQuoteLine.SetRange("document_type", gtables.SalesLineDocument_type(rec.Document_type))
	salesQuoteLine.SetRange("document_no", rec.No)
	for found := salesQuoteLine.FindSet(); found; found = salesQuoteLine.Next() {
		salesOrderLine = gtables.NewSalesLine(rec)
		salesOrderLine.TransferFields(salesQuoteLine)
		if !salesOrderLine.Reserved_qty_base.IsZero() {
			err = salesOrderLine.Validate("reserved_qty_base", types.Decimal{})
			if err != nil {
				return nil, err
			}
		}
		salesOrderLine.Document_type = gtables.SalesLineDocument_type(salesOrderHeader.Document_type)
		salesOrderLine.Document_no = salesOrderHeader.No
		if !salesQuoteLine.Reserved_qty_base.IsZero() {
			err = reservMgt.TransferSaleLineToSalesLine(salesQuoteLine, salesOrderLine, salesQuoteLine.Reserved_qty_base)
			if err == nil {
				err = salesQuoteLine.TestField("reserved_qty_base", types.Decimal{})
			}
			if err != nil {
				return nil, err
			}
		}
		err = salesOrderLine.InsertOrError(false)
		if err != nil {
			return nil, err
		}
	}

	err = moveComments(rec, salesOrderHeader)
	if err != nil {
		return nil, err
	}

	err = rec.DeleteOrError(false)
	if err != nil {
		return nil, err
	}
	salesQuoteLine.DeleteAll()
	return salesOrderHeader, nil
}

// moveComments: the quote's comment lines become the order's
func moveComments(salesHeader, salesOrderHeader *gtables.SalesHeader) error {
	salesCommentLine := gtables.NewSalesCommentLine(salesHeader)
	salesCommentLine.SetRange("document_type", gtables.SalesCommentLineDocument_type(salesHeader.Document_type))
	salesCommentLine.SetRange("no", salesHeader.No)
	for found := salesCommentLine.FindSet(); found; found = salesCommentLine.Next() {
		salesCommentLine2 := gtables.NewSalesCommentLine(salesHeader)
		salesCommentLine2.TransferFields(salesCommentLine)
		salesCommentLine2.Document_type = gtables.SalesCommentLineDocument_type(salesOrderHeader.Document_type)
		salesCommentLine2.No = types.Code(salesOrderHeader.No)
		err := salesCommentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	salesCommentLine.DeleteAll()
	return nil
}
