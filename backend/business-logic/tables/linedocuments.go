package tables

import (
	"fmt"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Show Document on the line lists (forms 516/518 Sales/Purchase Lines, 525–530 the posted lines,
// 48/56 Sales/Purchase Orders): the line's document opens on its card — a sales or purchase
// document on the card of its type, a posted line on its posted document's card. Esc goes back
// to the list.

// The document cards by document type (Quote, Order, Invoice, Credit Memo, Blanket Order)
var (
	salesDocumentCardIDs = map[gtables.SalesHeaderDocument_type]int{
		gtables.SalesHeader_Document_type.Quote:        41,
		gtables.SalesHeader_Document_type.Order:        42,
		gtables.SalesHeader_Document_type.Invoice:      43,
		gtables.SalesHeader_Document_type.CreditMemo:   44,
		gtables.SalesHeader_Document_type.BlanketOrder: 507,
	}
	purchDocumentCardIDs = map[gtables.PurchaseHeaderDocument_type]int{
		gtables.PurchaseHeader_Document_type.Quote:        49,
		gtables.PurchaseHeader_Document_type.Order:        50,
		gtables.PurchaseHeader_Document_type.Invoice:      51,
		gtables.PurchaseHeader_Document_type.CreditMemo:   52,
		gtables.PurchaseHeader_Document_type.BlanketOrder: 509,
	}
)

// The posted documents' cards
const (
	postedSalesShipmentCardID = 130
	postedSalesInvoiceCardID  = 132
	postedSalesCrMemoCardID   = 134
	postedPurchReceiptCardID  = 136
	postedPurchInvoiceCardID  = 138
	postedPurchCrMemoCardID   = 140
)

func init() {
	register := func(table string, fn functions.Func) {
		functions.Register("table", table, "ShowDocument", fn)
	}
	register(gtables.SalesLineTableName, functions.For(SalesLineShowDocument))
	register(gtables.PurchaseLineTableName, functions.For(PurchLineShowDocument))
	register(gtables.SalesShipmentLineTableName, functions.For(func(rec *gtables.SalesShipmentLine) (fcodeunits.Result, error) {
		return showDocument(postedSalesShipmentCardID, rec.Document_no.String()), nil
	}))
	register(gtables.SalesInvoiceLineTableName, functions.For(func(rec *gtables.SalesInvoiceLine) (fcodeunits.Result, error) {
		return showDocument(postedSalesInvoiceCardID, rec.Document_no.String()), nil
	}))
	register(gtables.SalesCrMemoLineTableName, functions.For(func(rec *gtables.SalesCrMemoLine) (fcodeunits.Result, error) {
		return showDocument(postedSalesCrMemoCardID, rec.Document_no.String()), nil
	}))
	register(gtables.PurchRcptLineTableName, functions.For(func(rec *gtables.PurchRcptLine) (fcodeunits.Result, error) {
		return showDocument(postedPurchReceiptCardID, rec.Document_no.String()), nil
	}))
	register(gtables.PurchInvLineTableName, functions.For(func(rec *gtables.PurchInvLine) (fcodeunits.Result, error) {
		return showDocument(postedPurchInvoiceCardID, rec.Document_no.String()), nil
	}))
	register(gtables.PurchCrMemoLineTableName, functions.For(func(rec *gtables.PurchCrMemoLine) (fcodeunits.Result, error) {
		return showDocument(postedPurchCrMemoCardID, rec.Document_no.String()), nil
	}))
}

// showDocument opens a card on a record, Esc going back to the page the function ran from
func showDocument(pageID int, key string) fcodeunits.Result {
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: pageID, Key: key, Return: true}}
}

// SalesLineShowDocument: the line's sales document on the card of its type (it must exist)
func SalesLineShowDocument(salesLine *gtables.SalesLine) (fcodeunits.Result, error) {
	salesHeader := gtables.NewSalesHeader(salesLine)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(salesLine.Document_type), "no": salesLine.Document_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	pageID := salesDocumentCardIDs[salesHeader.Document_type]
	return showDocument(pageID, fmt.Sprintf("%d,%s", int(salesHeader.Document_type), salesHeader.No)), nil
}

// PurchLineShowDocument: the line's purchase document on the card of its type (it must exist)
func PurchLineShowDocument(purchLine *gtables.PurchaseLine) (fcodeunits.Result, error) {
	purchHeader := gtables.NewPurchaseHeader(purchLine)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "no": purchLine.Document_no})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	pageID := purchDocumentCardIDs[purchHeader.Document_type]
	return showDocument(pageID, fmt.Sprintf("%d,%s", int(purchHeader.Document_type), purchHeader.No)), nil
}
