package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchInvHeaderLogic is the business logic of table 122 Purch. Inv. Header: a posted purchase invoice (written by
// purchase posting, codeunit 90).
type purchInvHeaderLogic struct{}

func init() { gtables.RegisterPurchInvHeaderLogic(purchInvHeaderLogic{}) }

// OnDelete: the document's lines and comments
func (purchInvHeaderLogic) OnDelete(rec *gtables.PurchInvHeader) error {
	purchInvLine := gtables.NewPurchInvLine(rec)
	purchInvLine.SetRange("document_no", rec.No)
	purchInvLine.DeleteAll()
	purchCommentLine := gtables.NewPurchCommentLine(rec)
	purchCommentLine.SetRange("document_type", gtables.PurchCommentLine_Document_type.PostedInvoice)
	purchCommentLine.SetRange("no", rec.No)
	purchCommentLine.DeleteAll()
	return nil
}
