package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 9305 Sales Orders: the orders (Document Type = Order). Page 42 Sales Order: a new
// document there is an order. Page 46 Sales Order Subform: a new line takes the type of the
// line it is entered next to (OnNewRecord: Type := xRec.Type).
func init() {
	pages.OnOpenPage(9305, func(rec *gtables.SalesHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.SalesHeader_Document_type.Order))
		return nil
	})
	pages.OnNewRecord(42, func(rec, xRec *gtables.SalesHeader, belowXRec bool) {
		rec.Document_type = gtables.SalesHeader_Document_type.Order
	})
	pages.OnNewRecord(46, func(rec, xRec *gtables.SalesLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
}
