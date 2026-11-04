package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesprinted"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The printouts of the posted sales documents — report 206 Sales - Invoice, 207 Sales - Credit
// Memo, 208 Sales - Shipment — like the quote and order confirmation (salesDocument): the
// company, the bill-to and ship-to addresses, the terms, the lines (and the totals of the invoice
// and the credit memo), in copies; a print that is not a preview counts in No. Printed
// (codeunits 315, 316, 314). Run from a posted document it prints that document. Layouts:
// backend/definitions/reports/206-sales-invoice.yaml, 207-sales-credit-memo.yaml,
// 208-sales-shipment.yaml.
const (
	SalesInvoiceID  = 206
	SalesCrMemoID   = 207
	SalesShipmentID = 208
)

func init() {
	Register(SalesInvoiceID, "sales-invoice", func() Report { return &salesInvoice{} })
	Register(SalesCrMemoID, "sales-credit-memo", func() Report { return &salesCrMemo{} })
	Register(SalesShipmentID, "sales-shipment", func() Report { return &salesShipment{} })
}

// postedDocumentOptions: the options of the posted documents' printouts
func postedDocumentOptions() []Option {
	return []Option{
		{Name: "no_of_copies", Type: "number", Default: "0"},
		{Name: "print_company_address", Type: "bool", Default: "true"},
	}
}

// postedDocument is what the three printouts share: the copies and the documents printed
type postedDocument struct {
	noOfLoops int
	printed   int
}

func (r *postedDocument) copyLoopOnPreDataItem(rec *Integer, run *Run) error {
	rec.SetRange("number", 1, r.noOfLoops)
	return nil
}

func (r *postedDocument) copyLoopOnAfterGetRecord(rec *Integer, run *Run) error {
	run.SetPageNo(1)
	if rec.Number > 1 {
		run.SetValue("copy_text", run.Text("COPY"))
	} else {
		run.SetValue("copy_text", "")
	}
	return nil
}

// headerPrinted: a document's values are set — several documents go into one file named as
// the report
func (r *postedDocument) headerPrinted(run *Run, no types.Code) {
	r.noOfLoops = noOfLoops(run)
	r.printed++
	if r.printed == 1 {
		run.SetFileName(run.Text("FILE_NAME", no))
	} else {
		run.SetFileName("")
	}
}

// setTotals: the totals of the posted lines — subtotal before the invoice discount, the
// invoice discount, the tax and the total
func setTotals(run *Run, amount, amountInclTax, invDiscount types.Decimal) {
	run.SetValue("subtotal", amount.Add(invDiscount))
	run.SetValue("invoice_discount", invDiscount.Neg())
	run.SetValue("tax_amount", amountInclTax.Sub(amount))
	run.SetValue("total", amountInclTax)
}

// Report 206 Sales - Invoice

type salesInvoice struct {
	postedDocument
	salesInvHeader *gtables.SalesInvoiceHeader
}

func (r *salesInvoice) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.SalesInvoiceHeaderTableName,
		OnInitReport: func(run *Run) error {
			salesInvHeader, ok := run.Context().(*gtables.SalesInvoiceHeader)
			if ok && salesInvHeader.No != "" {
				run.SetRequestFilter("SalesInvoiceHeader", "no", salesInvHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "SalesInvoiceHeader",
			Table:               gtables.SalesInvoiceHeaderTableName,
			RequestFilterFields: []string{"no", "sell_to_customer_no", "bill_to_customer_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return salesprinted.RunInvoice(r.salesInvHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "SalesInvoiceLine",
					Table: gtables.SalesInvoiceLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.SalesInvoiceLine, run *Run) error {
						rec.SetRange("document_no", r.salesInvHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.SalesInvoiceLine, run *Run) error {
						run.SetValue("amount_excl_inv_disc", rec.Amount.Add(rec.Inv_discount_amount))
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *salesInvoice) headerOnAfterGetRecord(rec *gtables.SalesInvoiceHeader, run *Run) error {
	r.salesInvHeader = rec
	billToAddress, err := formataddress.SalesInvBillTo(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.SalesInvShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("bill_to_address", billToAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Salesperson_code, rec.Shipment_method_code, rec.Payment_terms_code)
	setCurrencyValue(run, rec, rec.Currency_code)
	salesInvLine := gtables.NewSalesInvoiceLine(rec)
	salesInvLine.SetRange("document_no", rec.No)
	err = salesInvLine.CalcSums("amount", "amount_including_tax", "inv_discount_amount")
	if err != nil {
		return err
	}
	setTotals(run, salesInvLine.Amount, salesInvLine.Amount_including_tax, salesInvLine.Inv_discount_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// Report 207 Sales - Credit Memo

type salesCrMemo struct {
	postedDocument
	salesCrMemoHeader *gtables.SalesCrMemoHeader
}

func (r *salesCrMemo) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.SalesCrMemoHeaderTableName,
		OnInitReport: func(run *Run) error {
			salesCrMemoHeader, ok := run.Context().(*gtables.SalesCrMemoHeader)
			if ok && salesCrMemoHeader.No != "" {
				run.SetRequestFilter("SalesCrMemoHeader", "no", salesCrMemoHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "SalesCrMemoHeader",
			Table:               gtables.SalesCrMemoHeaderTableName,
			RequestFilterFields: []string{"no", "sell_to_customer_no", "bill_to_customer_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return salesprinted.RunCrMemo(r.salesCrMemoHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "SalesCrMemoLine",
					Table: gtables.SalesCrMemoLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.SalesCrMemoLine, run *Run) error {
						rec.SetRange("document_no", r.salesCrMemoHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.SalesCrMemoLine, run *Run) error {
						run.SetValue("amount_excl_inv_disc", rec.Amount.Add(rec.Inv_discount_amount))
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *salesCrMemo) headerOnAfterGetRecord(rec *gtables.SalesCrMemoHeader, run *Run) error {
	r.salesCrMemoHeader = rec
	billToAddress, err := formataddress.SalesCrMemoBillTo(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.SalesCrMemoShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("bill_to_address", billToAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Salesperson_code, rec.Shipment_method_code, rec.Payment_terms_code)
	setCurrencyValue(run, rec, rec.Currency_code)
	salesCrMemoLine := gtables.NewSalesCrMemoLine(rec)
	salesCrMemoLine.SetRange("document_no", rec.No)
	err = salesCrMemoLine.CalcSums("amount", "amount_including_tax", "inv_discount_amount")
	if err != nil {
		return err
	}
	setTotals(run, salesCrMemoLine.Amount, salesCrMemoLine.Amount_including_tax, salesCrMemoLine.Inv_discount_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// Report 208 Sales - Shipment

type salesShipment struct {
	postedDocument
	salesShptHeader *gtables.SalesShipmentHeader
}

func (r *salesShipment) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.SalesShipmentHeaderTableName,
		OnInitReport: func(run *Run) error {
			salesShptHeader, ok := run.Context().(*gtables.SalesShipmentHeader)
			if ok && salesShptHeader.No != "" {
				run.SetRequestFilter("SalesShipmentHeader", "no", salesShptHeader.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "SalesShipmentHeader",
			Table:               gtables.SalesShipmentHeaderTableName,
			RequestFilterFields: []string{"no", "sell_to_customer_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return salesprinted.RunShipment(r.salesShptHeader)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "SalesShipmentLine",
					Table: gtables.SalesShipmentLineTableName,
					OnPreDataItem: Trigger(func(rec *gtables.SalesShipmentLine, run *Run) error {
						rec.SetRange("document_no", r.salesShptHeader.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.SalesShipmentLine, run *Run) error {
						run.SetValue("description_2", rec.Description_2.String())
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *salesShipment) headerOnAfterGetRecord(rec *gtables.SalesShipmentHeader, run *Run) error {
	r.salesShptHeader = rec
	billToAddress, err := formataddress.SalesShptBillTo(rec)
	if err != nil {
		return err
	}
	shipToAddress, err := formataddress.SalesShptShipTo(rec)
	if err != nil {
		return err
	}
	run.SetValue("bill_to_address", billToAddress)
	run.SetValue("ship_to_address", shipToAddress)
	setTermsValues(run, rec, rec.Salesperson_code, rec.Shipment_method_code, rec.Payment_terms_code)
	r.headerPrinted(run, rec.No)
	return nil
}
