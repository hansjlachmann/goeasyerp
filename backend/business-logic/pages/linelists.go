package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 48 Sales Orders and 56 Purchase Orders (classic forms 48/56, Sales › Orders and
// Purchases › Orders on the item): the order lines (SourceTableView Document Type = Order)
func init() {
	pages.OnOpenPage(48, func(rec *gtables.SalesLine, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.SalesLine_Document_type.Order))
		return nil
	})
	pages.OnOpenPage(56, func(rec *gtables.PurchaseLine, page *pages.Open) error {
		page.SetFilter("document_type", int(gtables.PurchaseLine_Document_type.Order))
		return nil
	})
}
