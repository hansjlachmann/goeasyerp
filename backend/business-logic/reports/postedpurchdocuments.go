package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchprinted"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The printouts of the posted purchase documents — report 406 Purchase - Invoice, 407 Purchase -
// Credit Memo, 408 Purchase - Receipt — like the posted sales documents: the company, the vendor's
// (buy-from) and the ship-to addresses, the terms, the lines (and the totals of the invoice and
// the credit memo), in copies; a print that is not a preview counts in No. Printed (codeunits
// 319, 320, 318). Run from a posted document it prints that document. Layouts:
// backend/definitions/reports/406-purchase-invoice.yaml, 407-purchase-credit-memo.yaml,
// 408-purchase-receipt.yaml.
const (
	PurchaseInvoiceID = 406
	PurchaseCrMemoID  = 407
	PurchaseReceiptID = 408
)

func init() {
	Register(PurchaseInvoiceID, "purchase-invoice", func() Report { return &purchInvoice{} })
	Register(PurchaseCrMemoID, "purchase-credit-memo", func() Report { return &purchCrMemo{} })
	Register(PurchaseReceiptID, "purchase-receipt", func() Report { return &purchReceipt{} })
}

// Report 406 Purchase - Invoice

type purchInvoice struct {
	postedDocument
	purchInvHeader *gtables.PurchInvHeader
}

func (r *purchInvoice) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.PurchInvHeaderTableName,
		OnInitReport: func(run *Run) error {
			purchInvHeader, ok := run.Context().(*gtables.PurchInvHeader)
			if ok && purchInvHeader.No != "" {
				run.SetRequestFilter("PurchInvHeader", "no", purchInvHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "PurchInvHeader",
			Table:               gtables.PurchInvHeaderTableName,
			RequestFilterFields: []string{"no", "buy_from_vendor_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return purchprinted.RunInvoice(r.purchInvHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "PurchInvLine",
					Table: gtables.PurchInvLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.PurchInvLine, run *Run) error {
						rec.SetRange("document_no", r.purchInvHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.PurchInvLine, run *Run) error {
						run.SetValue("amount_excl_inv_disc", rec.Amount.Add(rec.Inv_discount_amount))
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *purchInvoice) headerOnAfterGetRecord(rec *gtables.PurchInvHeader, run *Run) error {
	r.purchInvHeader = rec
	vendorAddress, err := formataddress.PurchInvBuyFrom(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.PurchInvShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("vendor_address", vendorAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Purchaser_code, rec.Shipment_method_code, rec.Payment_terms_code)
	setCurrencyValue(run, rec, rec.Currency_code)
	purchInvLine := gtables.NewPurchInvLine(rec)
	purchInvLine.SetRange("document_no", rec.No)
	err = purchInvLine.CalcSums("amount", "amount_including_tax", "inv_discount_amount")
	if err != nil {
		return err
	}
	setTotals(run, purchInvLine.Amount, purchInvLine.Amount_including_tax, purchInvLine.Inv_discount_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// Report 407 Purchase - Credit Memo

type purchCrMemo struct {
	postedDocument
	purchCrMemoHeader *gtables.PurchCrMemoHdr
}

func (r *purchCrMemo) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.PurchCrMemoHdrTableName,
		OnInitReport: func(run *Run) error {
			purchCrMemoHeader, ok := run.Context().(*gtables.PurchCrMemoHdr)
			if ok && purchCrMemoHeader.No != "" {
				run.SetRequestFilter("PurchCrMemoHdr", "no", purchCrMemoHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "PurchCrMemoHdr",
			Table:               gtables.PurchCrMemoHdrTableName,
			RequestFilterFields: []string{"no", "buy_from_vendor_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return purchprinted.RunCrMemo(r.purchCrMemoHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "PurchCrMemoLine",
					Table: gtables.PurchCrMemoLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.PurchCrMemoLine, run *Run) error {
						rec.SetRange("document_no", r.purchCrMemoHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.PurchCrMemoLine, run *Run) error {
						run.SetValue("amount_excl_inv_disc", rec.Amount.Add(rec.Inv_discount_amount))
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *purchCrMemo) headerOnAfterGetRecord(rec *gtables.PurchCrMemoHdr, run *Run) error {
	r.purchCrMemoHeader = rec
	vendorAddress, err := formataddress.PurchCrMemoBuyFrom(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.PurchCrMemoShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("vendor_address", vendorAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Purchaser_code, rec.Shipment_method_code, rec.Payment_terms_code)
	setCurrencyValue(run, rec, rec.Currency_code)
	purchCrMemoLine := gtables.NewPurchCrMemoLine(rec)
	purchCrMemoLine.SetRange("document_no", rec.No)
	err = purchCrMemoLine.CalcSums("amount", "amount_including_tax", "inv_discount_amount")
	if err != nil {
		return err
	}
	setTotals(run, purchCrMemoLine.Amount, purchCrMemoLine.Amount_including_tax, purchCrMemoLine.Inv_discount_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// Report 408 Purchase - Receipt

type purchReceipt struct {
	postedDocument
	purchRcptHeader *gtables.PurchRcptHeader
}

func (r *purchReceipt) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.PurchRcptHeaderTableName,
		OnInitReport: func(run *Run) error {
			purchRcptHeader, ok := run.Context().(*gtables.PurchRcptHeader)
			if ok && purchRcptHeader.No != "" {
				run.SetRequestFilter("PurchRcptHeader", "no", purchRcptHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "PurchRcptHeader",
			Table:               gtables.PurchRcptHeaderTableName,
			RequestFilterFields: []string{"no", "buy_from_vendor_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return purchprinted.RunReceipt(r.purchRcptHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "PurchRcptLine",
					Table: gtables.PurchRcptLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.PurchRcptLine, run *Run) error {
						rec.SetRange("document_no", r.purchRcptHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.PurchRcptLine, run *Run) error {
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *purchReceipt) headerOnAfterGetRecord(rec *gtables.PurchRcptHeader, run *Run) error {
	r.purchRcptHeader = rec
	vendorAddress, err := formataddress.PurchRcptBuyFrom(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.PurchRcptShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("vendor_address", vendorAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Purchaser_code, rec.Shipment_method_code, rec.Payment_terms_code)
	r.headerPrinted(run, rec.No)
	return nil
}
