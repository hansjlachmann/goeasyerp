package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchCommentLineLogic is the business logic of table 43 Purch. Comment Line
type purchCommentLineLogic struct{}

func init() { gtables.RegisterPurchCommentLineLogic(purchCommentLineLogic{}) }

// SetUpNewPurchCommentLine (SetUpNewLine): the first comment of a document gets the work date
func SetUpNewPurchCommentLine(rec *gtables.PurchCommentLine) {
	purchCommentLine := gtables.NewPurchCommentLine(rec)
	purchCommentLine.SetRange("document_type", rec.Document_type)
	purchCommentLine.SetRange("no", rec.No)
	if !purchCommentLine.FindFirst() {
		rec.Date = system.WorkDate()
	}
}
