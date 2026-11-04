package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 9300 Sales Quotes: the quotes (Document Type = Quote); new documents there and on the
// Sales Quote card (41) are quotes, the option's first value. Page 95 Sales Quote Subform: a new
// line takes the type of the line it is entered next to (OnNewRecord: Type := xRec.Type). Page 67
// Sales Comment Sheet: the first comment of a document gets the work date (SetUpNewLine).
func init() {
	pages.OnOpenPage(9300, func(rec *gtables.SalesHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.SalesHeader_Document_type.Quote))
		return nil
	})
	pages.OnNewRecord(95, func(rec, xRec *gtables.SalesLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
	pages.OnNewRecord(67, func(rec, xRec *gtables.SalesCommentLine, belowXRec bool) {
		tables.SetUpNewSalesCommentLine(rec)
	})
}
