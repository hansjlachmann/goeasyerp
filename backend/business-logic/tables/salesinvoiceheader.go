package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesInvoiceHeaderLogic is the business logic of table 112 Sales Invoice Header: a posted sales invoice (written by sales
// posting, codeunit 80).
type salesInvoiceHeaderLogic struct{}

func init() { gtables.RegisterSalesInvoiceHeaderLogic(salesInvoiceHeaderLogic{}) }

// OnDelete: the document's lines and comments
func (salesInvoiceHeaderLogic) OnDelete(rec *gtables.SalesInvoiceHeader) error {
	salesInvoiceLine := gtables.NewSalesInvoiceLine(rec)
	salesInvoiceLine.SetRange("document_no", rec.No)
	salesInvoiceLine.DeleteAll()
	salesCommentLine := gtables.NewSalesCommentLine(rec)
	salesCommentLine.SetRange("document_type", gtables.SalesCommentLine_Document_type.PostedInvoice)
	salesCommentLine.SetRange("no", rec.No)
	salesCommentLine.DeleteAll()
	return nil
}
