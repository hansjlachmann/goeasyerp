package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 9307 Purchase Orders: the orders (Document Type = Order). Page 50 Purchase Order: a new
// document there is an order. Page 54 Purchase Order Subform: a new line takes the type of the
// line it is entered next to (OnNewRecord: Type := xRec.Type).
func init() {
	pages.OnOpenPage(9307, func(rec *gtables.PurchaseHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.PurchaseHeader_Document_type.Order))
		return nil
	})
	pages.OnNewRecord(50, func(rec, xRec *gtables.PurchaseHeader, belowXRec bool) {
		rec.Document_type = gtables.PurchaseHeader_Document_type.Order
	})
	pages.OnNewRecord(54, func(rec, xRec *gtables.PurchaseLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
}
