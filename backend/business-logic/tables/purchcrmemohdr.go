package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchCrMemoHdrLogic is the business logic of table 124 Purch. Cr. Memo Hdr.: a posted purchase credit memo (written by
// purchase posting, codeunit 90).
type purchCrMemoHdrLogic struct{}

func init() { gtables.RegisterPurchCrMemoHdrLogic(purchCrMemoHdrLogic{}) }

// OnDelete: the document's lines and comments
func (purchCrMemoHdrLogic) OnDelete(rec *gtables.PurchCrMemoHdr) error {
	purchCrMemoLine := gtables.NewPurchCrMemoLine(rec)
	purchCrMemoLine.SetRange("document_no", rec.No)
	purchCrMemoLine.DeleteAll()
	purchCommentLine := gtables.NewPurchCommentLine(rec)
	purchCommentLine.SetRange("document_type", gtables.PurchCommentLine_Document_type.PostedCreditMemo)
	purchCommentLine.SetRange("no", rec.No)
	purchCommentLine.DeleteAll()
	return nil
}
