package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 9306 Purchase Quotes: the quotes (Document Type = Quote); new documents there and on the
// Purchase Quote card (49) are quotes, the option's first value. Page 97 Purchase Quote Subform:
// a new line takes the type of the line it is entered next to (OnNewRecord: Type := xRec.Type).
// Page 66 Purch. Comment Sheet: the first comment of a document gets the work date
// (SetUpNewLine).
func init() {
	pages.OnOpenPage(9306, func(rec *gtables.PurchaseHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.PurchaseHeader_Document_type.Quote))
		return nil
	})
	pages.OnNewRecord(97, func(rec, xRec *gtables.PurchaseLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
	pages.OnNewRecord(66, func(rec, xRec *gtables.PurchCommentLine, belowXRec bool) {
		tables.SetUpNewPurchCommentLine(rec)
	})
}
