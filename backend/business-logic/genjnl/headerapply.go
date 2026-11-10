package genjnl

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunits 401 Sales Header Apply and 402 Purchase Header Apply (Functions › Apply Entries on the
// sales and purchase documents): Apply Customer / Vendor Entries opens over the bill-to
// customer's / pay-to vendor's open entries with the document's Applies-to ID (its No. when
// blank) — F9 marks the entries with it; after OK the document keeps the ID when entries carry
// it (and its Applies-to Doc. Type/No. are cleared), else the ID is taken off. Posting applies
// the document to the marked entries (codeunit 12 by Applies-to ID).
func init() {
	functions.Register("table", gtables.SalesHeaderTableName, "ApplyEntries", functions.For(SalesHeaderApply), functions.WithDialog())
	functions.Register("table", gtables.PurchaseHeaderTableName, "ApplyEntries", functions.For(PurchHeaderApply), functions.WithDialog())
}

// headerApplyPage: the Apply Entries page in lookup mode with the document's ID, amount (as the
// customer's or vendor's entry will have it), posting date, document type and currency
func headerApplyPage(pageID int, appliesToID types.Code, amount types.Decimal, postingDate types.Date, docType gtables.GenJournalLineDocument_type, currencyCode types.Code) *system.Page {
	page := system.NewPage(pageID)
	page.LookupMode(true)
	page.SetParam("applies_to_id", appliesToID.String())
	page.SetParam("amount", amount.String())
	page.SetParam("posting_date", postingDate.String())
	page.SetParam("document_type", itoa(int(docType)))
	page.SetParam("currency_code", currencyCode.String())
	return page
}

// SalesHeaderApply (codeunit 401 Sales Header Apply)
func SalesHeaderApply(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	salesHeader := rec
	bilToCustNo := salesHeader.Bill_to_customer_no
	custLedgEntry := gtables.NewCustomerLedgerEntry(salesHeader)
	custLedgEntry.SetCurrentKey("customer_no", "open", "positive")
	custLedgEntry.SetRange("customer_no", bilToCustNo)
	custLedgEntry.SetRange("open", true)
	if salesHeader.Applies_to_id == "" {
		salesHeader.Applies_to_id = salesHeader.No
	}
	if salesHeader.Applies_to_id == "" {
		return fcodeunits.Result{}, apperrors.JnlSpecifyOne(gtables.SalesHeaderTableName, "no", "applies_to_id")
	}
	// the document's amount as its customer entry will have it: an invoice debits, a credit memo credits
	salesHeader.CalcFields("amount_including_tax")
	amount := salesHeader.Amount_including_tax
	docType := gtables.GenJournalLine_Document_type.Invoice
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
		amount = amount.Neg()
		docType = gtables.GenJournalLine_Document_type.CreditMemo
	}
	page := headerApplyPage(ApplyCustEntriesPageID, salesHeader.Applies_to_id, amount, salesHeader.Posting_date, docType, salesHeader.Currency_code)
	page.SetTableView(custLedgEntry)
	if page.RunModal() != system.ActionLookupOK {
		return fcodeunits.Result{Success: true}, nil
	}
	custLedgEntry = gtables.NewCustomerLedgerEntry(salesHeader)
	custLedgEntry.SetCurrentKey("customer_no", "open", "positive")
	custLedgEntry.SetRange("customer_no", bilToCustNo)
	custLedgEntry.SetRange("open", true)
	custLedgEntry.SetRange("applies_to_id", salesHeader.Applies_to_id)
	if custLedgEntry.FindFirst() {
		salesHeader.Applies_to_doc_type = gtables.SalesHeader_Applies_to_doc_type.Blank
		salesHeader.Applies_to_doc_no = ""
	} else {
		salesHeader.Applies_to_id = ""
	}
	err := salesHeader.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// PurchHeaderApply (codeunit 402 Purchase Header Apply)
func PurchHeaderApply(rec *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	purchHeader := rec
	payToVendorNo := purchHeader.Pay_to_vendor_no
	vendLedgEntry := gtables.NewVendorLedgerEntry(purchHeader)
	vendLedgEntry.SetCurrentKey("vendor_no", "open", "positive")
	vendLedgEntry.SetRange("vendor_no", payToVendorNo)
	vendLedgEntry.SetRange("open", true)
	if purchHeader.Applies_to_id == "" {
		purchHeader.Applies_to_id = purchHeader.No
	}
	if purchHeader.Applies_to_id == "" {
		return fcodeunits.Result{}, apperrors.JnlSpecifyOne(gtables.PurchaseHeaderTableName, "no", "applies_to_id")
	}
	// the document's amount as its vendor entry will have it: an invoice credits, a credit memo debits
	purchHeader.CalcFields("amount_including_tax")
	amount := purchHeader.Amount_including_tax.Neg()
	docType := gtables.GenJournalLine_Document_type.Invoice
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
		amount = amount.Neg()
		docType = gtables.GenJournalLine_Document_type.CreditMemo
	}
	page := headerApplyPage(ApplyVendEntriesPageID, purchHeader.Applies_to_id, amount, purchHeader.Posting_date, docType, purchHeader.Currency_code)
	page.SetTableView(vendLedgEntry)
	if page.RunModal() != system.ActionLookupOK {
		return fcodeunits.Result{Success: true}, nil
	}
	vendLedgEntry = gtables.NewVendorLedgerEntry(purchHeader)
	vendLedgEntry.SetCurrentKey("vendor_no", "open", "positive")
	vendLedgEntry.SetRange("vendor_no", payToVendorNo)
	vendLedgEntry.SetRange("open", true)
	vendLedgEntry.SetRange("applies_to_id", purchHeader.Applies_to_id)
	if vendLedgEntry.FindFirst() {
		purchHeader.Applies_to_doc_type = gtables.PurchaseHeader_Applies_to_doc_type.Blank
		purchHeader.Applies_to_doc_no = ""
	} else {
		purchHeader.Applies_to_id = ""
	}
	err := purchHeader.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
