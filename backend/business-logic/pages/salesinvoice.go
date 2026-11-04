package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 9301 Sales Invoices and 9302 Sales Credit Memos: the documents of their type. Pages 43
// Sales Invoice and 44 Sales Credit Memo: a new document there is of their type. Pages 47 Sales
// Invoice Subform and 96 Sales Cr. Memo Subform: a new line takes the type of the line it is
// entered next to (OnNewRecord: Type := xRec.Type).
func init() {
	for _, page := range []struct {
		list, card, subform int
		documentType        gtables.SalesHeaderDocument_type
	}{
		{9301, 43, 47, gtables.SalesHeader_Document_type.Invoice},
		{9302, 44, 96, gtables.SalesHeader_Document_type.CreditMemo},
	} {
		documentType := page.documentType
		pages.OnOpenPage(page.list, func(rec *gtables.SalesHeader, page *pages.Open) error {
			page.SetFilter("document_type", int(documentType))
			return nil
		})
		pages.OnNewRecord(page.card, func(rec, xRec *gtables.SalesHeader, belowXRec bool) {
			rec.Document_type = documentType
		})
		pages.OnNewRecord(page.subform, func(rec, xRec *gtables.SalesLine, belowXRec bool) {
			rec.Type = xRec.Type
		})
	}
}
