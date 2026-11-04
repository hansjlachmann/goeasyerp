package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 9303 Blanket Sales Orders / 9310 Blanket Purchase Orders: the blanket orders (Document
// Type = Blanket Order). Pages 507 / 509: a new document there is a blanket order. Pages 508 /
// 510 (the lines): a new line takes the type of the line it is entered next to.
func init() {
	pages.OnOpenPage(9303, func(rec *gtables.SalesHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.SalesHeader_Document_type.BlanketOrder))
		return nil
	})
	pages.OnNewRecord(507, func(rec, xRec *gtables.SalesHeader, belowXRec bool) {
		rec.Document_type = gtables.SalesHeader_Document_type.BlanketOrder
	})
	pages.OnNewRecord(508, func(rec, xRec *gtables.SalesLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
	pages.OnOpenPage(9310, func(rec *gtables.PurchaseHeader, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.PurchaseHeader_Document_type.BlanketOrder))
		return nil
	})
	pages.OnNewRecord(509, func(rec, xRec *gtables.PurchaseHeader, belowXRec bool) {
		rec.Document_type = gtables.PurchaseHeader_Document_type.BlanketOrder
	})
	pages.OnNewRecord(510, func(rec, xRec *gtables.PurchaseLine, belowXRec bool) {
		rec.Type = xRec.Type
	})
}
