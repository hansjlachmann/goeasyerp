package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesCrMemoHeaderLogic is the business logic of table 114 Sales Cr.Memo Header: a posted sales credit memo (written by sales
// posting, codeunit 80).
type salesCrMemoHeaderLogic struct{}

func init() { gtables.RegisterSalesCrMemoHeaderLogic(salesCrMemoHeaderLogic{}) }

// OnDelete: the document's lines and comments
func (salesCrMemoHeaderLogic) OnDelete(rec *gtables.SalesCrMemoHeader) error {
	salesCrMemoLine := gtables.NewSalesCrMemoLine(rec)
	salesCrMemoLine.SetRange("document_no", rec.No)
	salesCrMemoLine.DeleteAll()
	salesCommentLine := gtables.NewSalesCommentLine(rec)
	salesCommentLine.SetRange("document_type", gtables.SalesCommentLine_Document_type.PostedCreditMemo)
	salesCommentLine.SetRange("no", rec.No)
	salesCommentLine.DeleteAll()
	return nil
}
