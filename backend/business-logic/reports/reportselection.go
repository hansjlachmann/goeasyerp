package reports

import (
	"encoding/base64"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report Selections (table 77): the reports a document prints with, per usage in their sequence
// — Document-Print (business-logic/docprint), Post and Print, Make Orders.

// DefaultReports are the reports of a usage while Report Selections has no line for it (the
// reports codeunit 2 Company-Initialize enters; usages whose report does not exist yet have none)
var DefaultReports = map[gtables.ReportSelectionsUsage]int{
	gtables.ReportSelections_Usage.SQuote:    SalesQuoteID,
	gtables.ReportSelections_Usage.SBlanket:  BlanketSalesOrderID,
	gtables.ReportSelections_Usage.SOrder:    OrderConfirmationID,
	gtables.ReportSelections_Usage.SInvoice:  SalesInvoiceID,
	gtables.ReportSelections_Usage.SCrMemo:   SalesCrMemoID,
	gtables.ReportSelections_Usage.SShipment: SalesShipmentID,
	gtables.ReportSelections_Usage.PQuote:    PurchaseQuoteID,
	gtables.ReportSelections_Usage.PBlanket:  BlanketPurchaseOrderID,
	gtables.ReportSelections_Usage.POrder:    PurchaseOrderID,
	gtables.ReportSelections_Usage.PInvoice:  PurchaseInvoiceID,
	gtables.ReportSelections_Usage.PCrMemo:   PurchaseCrMemoID,
	gtables.ReportSelections_Usage.PReceipt:  PurchaseReceiptID,
	gtables.ReportSelections_Usage.Reminder:  ReminderPrintID,
	gtables.ReportSelections_Usage.FinCharge: FinanceChargeMemoPrintID,
}

// UsageReports returns the reports of a usage in their sequence (Report ID not 0); the default
// report while Report Selections has no line for the usage. None: an error.
func UsageReports(src ftables.Source, usage gtables.ReportSelectionsUsage) ([]int, error) {
	reportSelection := gtables.NewReportSelections(src)
	reportSelection.SetRange("usage", usage)
	if reportSelection.IsEmpty() {
		if id, ok := DefaultReports[usage]; ok {
			return []int{id}, nil
		}
		return nil, apperrors.ReportSelectionNone(usage)
	}
	reportSelection.SetFilter("report_id", "<>0")
	var ids []int
	for found := reportSelection.FindSet(); found; found = reportSelection.Next() {
		ids = append(ids, reportSelection.Report_id)
	}
	if len(ids) == 0 {
		return nil, apperrors.ReportSelectionNone(usage)
	}
	return ids, nil
}

// PrintDocuments runs the reports of the usage on the record, one after the other: with their
// request pages, or (requestPage false) straight to the output of their Printer Selection, the
// browser's print dialog by default. Execute on each report is needed.
func PrintDocuments(rec ftables.Source, usage gtables.ReportSelectionsUsage, requestPage bool) ([]Result, error) {
	ids, err := UsageReports(rec, usage)
	if err != nil {
		return nil, err
	}
	var printouts []Result
	for _, id := range ids {
		if !fcodeunits.CanExecute(session.ObjectReport, id) {
			return nil, apperrors.NoPermissionReport(id, Caption(id, fcodeunits.CurrentLanguage()))
		}
		opts := RunOptions{RequestPage: requestPage, Context: rec}
		if !requestPage {
			opts.Output = PrinterOutput(rec, id, OutputPrint)
		}
		result, err := Execute(id, rec.GetDB(), rec.GetCompany(), rec.GetDBType(), opts)
		if err != nil {
			return nil, err
		}
		if !result.Cancelled {
			printouts = append(printouts, result)
		}
	}
	return printouts, nil
}

// WithPrintouts puts the printouts into a result's data: the first printout's PDF (pdf,
// filename, output), the others in printouts — the browser shows, prints or downloads each
func WithPrintouts(result fcodeunits.Result, printouts []Result) fcodeunits.Result {
	var list []map[string]interface{}
	for _, r := range printouts {
		if r.PDF == nil {
			continue
		}
		list = append(list, map[string]interface{}{
			"pdf":      base64.StdEncoding.EncodeToString(r.PDF),
			"filename": r.FileName,
			"output":   r.Output,
		})
	}
	if len(list) == 0 {
		return result
	}
	if result.Data == nil {
		result.Data = map[string]interface{}{}
	}
	for key, value := range list[0] {
		result.Data[key] = value
	}
	if len(list) > 1 {
		result.Data["printouts"] = list[1:]
	}
	return result
}
