package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesCommentLineLogic is the business logic of table 44 Sales Comment Line
type salesCommentLineLogic struct{}

func init() { gtables.RegisterSalesCommentLineLogic(salesCommentLineLogic{}) }

// SetUpNewSalesCommentLine (SetUpNewLine): the first comment of a document gets the work date
func SetUpNewSalesCommentLine(rec *gtables.SalesCommentLine) {
	salesCommentLine := gtables.NewSalesCommentLine(rec)
	salesCommentLine.SetRange("document_type", rec.Document_type)
	salesCommentLine.SetRange("no", rec.No)
	if !salesCommentLine.FindFirst() {
		rec.Date = system.WorkDate()
	}
}
