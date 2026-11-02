// Package docprint is codeunit 229 Document-Print and the PrintRecords functions of the posted
// documents: a document prints with the reports of its usage in Report Selections (table 77),
// one after the other in their sequence, each with its request page.
package docprint

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/purchcalcdisc"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/salescalcdisc"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// DocumentPrintID is codeunit 229 Document-Print
const DocumentPrintID = 229

func init() {
	cu := func(name string, fn functions.Func, table string) {
		functions.Register("codeunit", "229", name, fn, functions.OnTable(table), functions.WithPrint())
	}
	cu("PrintSalesHeader", functions.For(PrintSalesHeader), gtables.SalesHeaderTableName)
	cu("PrintPurchHeader", functions.For(PrintPurchHeader), gtables.PurchaseHeaderTableName)

	// PrintRecords of the posted and issued documents (the Print action of their cards and lists)
	records := func(table string, fn functions.Func) {
		functions.Register("table", table, "PrintRecords", fn, functions.WithPrint())
	}
	records(gtables.SalesShipmentHeaderTableName, functions.For(func(rec *gtables.SalesShipmentHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.SShipment, true)
	}))
	records(gtables.SalesInvoiceHeaderTableName, functions.For(func(rec *gtables.SalesInvoiceHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.SInvoice, true)
	}))
	records(gtables.SalesCrMemoHeaderTableName, functions.For(func(rec *gtables.SalesCrMemoHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.SCrMemo, true)
	}))
	records(gtables.PurchRcptHeaderTableName, functions.For(func(rec *gtables.PurchRcptHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.PReceipt, true)
	}))
	records(gtables.PurchInvHeaderTableName, functions.For(func(rec *gtables.PurchInvHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.PInvoice, true)
	}))
	records(gtables.PurchCrMemoHdrTableName, functions.For(func(rec *gtables.PurchCrMemoHdr) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.PCrMemo, true)
	}))
	records(gtables.IssuedReminderHeaderTableName, functions.For(func(rec *gtables.IssuedReminderHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.Reminder, true)
	}))
	records(gtables.IssuedFinChargeMemoHeaderTableName, functions.For(func(rec *gtables.IssuedFinChargeMemoHeader) (fcodeunits.Result, error) {
		return PrintRecords(rec, gtables.ReportSelections_Usage.FinCharge, true)
	}))
}

// PrintSalesHeader (PrintSalesHeader): a quote, blanket order or order with the reports of its
// usage — the invoice discount calculated first when Sales & Receivables Setup says so
func PrintSalesHeader(salesHeader *gtables.SalesHeader) (fcodeunits.Result, error) {
	var usage gtables.ReportSelectionsUsage
	switch salesHeader.Document_type {
	case gtables.SalesHeader_Document_type.Quote:
		usage = gtables.ReportSelections_Usage.SQuote
	case gtables.SalesHeader_Document_type.BlanketOrder:
		usage = gtables.ReportSelections_Usage.SBlanket
	case gtables.SalesHeader_Document_type.Order:
		usage = gtables.ReportSelections_Usage.SOrder
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	salesSetup := gtables.GetSalesReceivablesSetup(salesHeader)
	if salesSetup.Calc_inv_discount {
		salesLine := gtables.NewSalesLine(salesHeader)
		salesLine.SetRange("document_type", salesHeader.Document_type)
		salesLine.SetRange("document_no", salesHeader.No)
		if salesLine.FindFirst() {
			err := salescalcdisc.Run(salesLine)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			salesHeader.Get(map[string]interface{}{"document_type": int(salesHeader.Document_type), "no": salesHeader.No})
		}
	}
	return PrintRecords(salesHeader, usage, true)
}

// PrintPurchHeader (PrintPurchHeader): a purchase quote, blanket order or order with the reports
// of its usage — the invoice discount calculated first when Purchases & Payables Setup says so
func PrintPurchHeader(purchHeader *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	var usage gtables.ReportSelectionsUsage
	switch purchHeader.Document_type {
	case gtables.PurchaseHeader_Document_type.Quote:
		usage = gtables.ReportSelections_Usage.PQuote
	case gtables.PurchaseHeader_Document_type.BlanketOrder:
		usage = gtables.ReportSelections_Usage.PBlanket
	case gtables.PurchaseHeader_Document_type.Order:
		usage = gtables.ReportSelections_Usage.POrder
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	purchSetup := gtables.GetPurchasesPayablesSetup(purchHeader)
	if purchSetup.Calc_inv_discount {
		purchLine := gtables.NewPurchaseLine(purchHeader)
		purchLine.SetRange("document_type", purchHeader.Document_type)
		purchLine.SetRange("document_no", purchHeader.No)
		if purchLine.FindFirst() {
			err := purchcalcdisc.Run(purchLine)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			purchHeader.Get(map[string]interface{}{"document_type": int(purchHeader.Document_type), "no": purchHeader.No})
		}
	}
	return PrintRecords(purchHeader, usage, true)
}

// PrintRecords runs the reports of the usage on the record (reports.PrintDocuments); the
// printouts are the result
func PrintRecords(rec ftables.Source, usage gtables.ReportSelectionsUsage, requestPage bool) (fcodeunits.Result, error) {
	printouts, err := reports.PrintDocuments(rec, usage, requestPage)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return reports.WithPrintouts(fcodeunits.Result{Success: true}, printouts), nil
}
