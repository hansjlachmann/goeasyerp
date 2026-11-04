package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchRcptHeaderLogic is the business logic of table 120 Purch. Rcpt. Header: a posted purchase receipt (written by
// purchase posting, codeunit 90).
type purchRcptHeaderLogic struct{}

func init() { gtables.RegisterPurchRcptHeaderLogic(purchRcptHeaderLogic{}) }

// OnDelete: the document's lines and comments
func (purchRcptHeaderLogic) OnDelete(rec *gtables.PurchRcptHeader) error {
	purchRcptLine := gtables.NewPurchRcptLine(rec)
	purchRcptLine.SetRange("document_no", rec.No)
	purchRcptLine.DeleteAll()
	purchCommentLine := gtables.NewPurchCommentLine(rec)
	purchCommentLine.SetRange("document_type", gtables.PurchCommentLine_Document_type.Receipt)
	purchCommentLine.SetRange("no", rec.No)
	purchCommentLine.DeleteAll()
	return nil
}
