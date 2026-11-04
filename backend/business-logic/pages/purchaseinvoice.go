package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 9308 Purchase Invoices and 9309 Purchase Credit Memos: the documents of their type.
// Pages 51 Purchase Invoice and 52 Purchase Credit Memo: a new document there is of their type.
// Pages 55 Purch. Invoice Subform and 98 Purch. Cr. Memo Subform: a new line takes the type of
// the line it is entered next to (OnNewRecord: Type := xRec.Type).
func init() {
	for _, page := range []struct {
		list, card, subform int
		documentType        gtables.PurchaseHeaderDocument_type
	}{
		{9308, 51, 55, gtables.PurchaseHeader_Document_type.Invoice},
		{9309, 52, 98, gtables.PurchaseHeader_Document_type.CreditMemo},
	} {
		documentType := page.documentType
		pages.OnOpenPage(page.list, func(rec *gtables.PurchaseHeader, page *pages.Open) error {
			page.SetFilter("document_type", int(documentType))
			return nil
		})
		pages.OnNewRecord(page.card, func(rec, xRec *gtables.PurchaseHeader, belowXRec bool) {
			rec.Document_type = documentType
		})
		pages.OnNewRecord(page.subform, func(rec, xRec *gtables.PurchaseLine, belowXRec bool) {
			rec.Type = xRec.Type
		})
	}
}
