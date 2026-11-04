package codeunits

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Preview Posting on the sales orders, invoices and credit memos (cards and lists)
func init() {
	functions.Register("table", gtables.SalesHeaderTableName, "PreviewPosting", functions.For(SalesPostPreview), functions.WithDialog())
}

// SalesPostPreview posts the document as Post would (an order asks what to post), in a
// transaction that is rolled back, and opens Posting Preview on the entries it would write;
// nothing is saved. A posting whose G/L entries do not balance gives the balance check's error
// with the entries (Show Entries).
func SalesPostPreview(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	ship, invoice := false, false
	if rec.Document_type == gtables.SalesHeader_Document_type.Order {
		var ok bool
		ship, invoice, ok = askShipInvoice()
		if !ok {
			return fcodeunits.Result{Success: true}, nil
		}
	}
	preview, err := postingpreview.Run(rec.GetDB(), rec.CompanyName(), rec.GetDBType(), fcodeunits.CurrentUserID(), func(src ftables.Source) error {
		_, err := postSalesDocument(src, rec, ship, invoice)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return preview.Result()
}
