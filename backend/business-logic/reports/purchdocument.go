package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchpost"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchprinted"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// purchDocument is the printout of a purchase document of one type — report 404 Purchase - Quote,
// 405 Order — sent to the vendor: the company (with its logo), the vendor's (buy-from) and the
// ship-to addresses, the document's terms, its lines and totals, in copies; a print that is not a
// preview counts in the document's No. Printed (codeunit 317). Run from a document it prints that
// document. Layouts: backend/definitions/reports/404-purchase-quote.yaml, 405-order.yaml.
const (
	PurchaseQuoteID = 404
	PurchaseOrderID = 405
)

func init() {
	Register(PurchaseQuoteID, "purchase-quote", func() Report {
		return &purchDocument{documentType: gtables.PurchaseHeader_Document_type.Quote}
	})
	Register(PurchaseOrderID, "order", func() Report {
		return &purchDocument{documentType: gtables.PurchaseHeader_Document_type.Order}
	})
}

type purchDocument struct {
	postedDocument
	documentType gtables.PurchaseHeaderDocument_type
	purchHeader  *gtables.PurchaseHeader
}

func (r *purchDocument) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.PurchaseHeaderTableName,
		OnInitReport: func(run *Run) error {
			purchHeader, ok := run.Context().(*gtables.PurchaseHeader)
			if ok && purchHeader.No != "" {
				run.SetRequestFilter("PurchaseHeader", "no", purchHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "PurchaseHeader",
			Table:               gtables.PurchaseHeaderTableName,
			Key:                 []string{"document_type", "no"},
			RequestFilterFields: []string{"no", "buy_from_vendor_no", "no_printed"},
			OnPreDataItem: Trigger(func(rec *gtables.PurchaseHeader, run *Run) error {
				rec.FilterGroup(2)
				rec.SetRange("document_type", r.documentType)
				rec.FilterGroup(0)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return purchprinted.Run(r.purchHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "PurchaseLine",
					Table: gtables.PurchaseLineTableName,
					Key:   []string{"document_type", "document_no", "line_no"},
					OnPreDataItem: Trigger(func(rec *gtables.PurchaseLine, run *Run) error {
						rec.SetRange("document_type", gtables.PurchaseLineDocument_type(r.purchHeader.Document_type))
						rec.SetRange("document_no", r.purchHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.PurchaseLine, run *Run) error {
						run.SetValue("amount_excl_inv_disc", rec.Amount.Add(rec.Inv_discount_amount))
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *purchDocument) headerOnAfterGetRecord(rec *gtables.PurchaseHeader, run *Run) error {
	r.purchHeader = rec
	vendorAddress, err := formataddress.PurchHeaderBuyFrom(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.PurchHeaderShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("vendor_address", vendorAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Purchaser_code, rec.Shipment_method_code, rec.Payment_terms_code)
	totals, err := purchpost.SumPurchLines(rec, purchpost.General)
	if err != nil {
		return err
	}
	totalPurchLine := totals.TotalPurchLine
	run.SetValue("subtotal", totalPurchLine.Amount.Add(totalPurchLine.Inv_discount_amount))
	run.SetValue("invoice_discount", totalPurchLine.Inv_discount_amount.Neg())
	run.SetValue("tax_amount", totals.TaxAmount)
	run.SetValue("total", totalPurchLine.Amount_including_tax)
	setCurrencyValue(run, rec, rec.Currency_code)
	r.headerPrinted(run, rec.No)
	return nil
}
