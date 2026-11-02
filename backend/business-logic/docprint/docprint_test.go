package docprint_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/docprint"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// quoteCompany: a company with the global Printer Selection table, customer C1 and quote SQ1
func quoteCompany(t *testing.T) (ftables.Source, *gtables.SalesHeader) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "print", database.DBTypeSQLite))
	mustOK(t, gtables.NewPrinterSelection(ftables.DBSource(db, "", database.DBTypeSQLite)).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "print", database.DBTypeSQLite)

	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Name = "Fjord Møbler AS"
	cust.Customer_posting_group = "DOM"
	cust.Gen_bus_posting_group = "DOM"
	mustOK(t, cust.InsertOrError(false))
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = gtables.SalesHeader_Document_type.Quote
	salesHeader.No = "SQ1"
	mustOK(t, salesHeader.InsertOrError(true))
	mustOK(t, salesHeader.Validate("sell_to_customer_no", types.Code("C1")))
	mustOK(t, salesHeader.ModifyOrError(true))
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.Document_type = gtables.SalesLine_Document_type.Quote
	salesLine.Document_no = "SQ1"
	salesLine.Line_no = 10000
	mustOK(t, salesLine.Validate("description", types.Text("Delivered in October")))
	mustOK(t, salesLine.InsertOrError(true))
	return src, salesHeader
}

func insertReportSelection(t *testing.T, src ftables.Source, usage gtables.ReportSelectionsUsage, sequence types.Code, reportID int) {
	t.Helper()
	reportSelection := gtables.NewReportSelections(src)
	reportSelection.Usage = usage
	reportSelection.Sequence = sequence
	reportSelection.Report_id = reportID
	mustOK(t, reportSelection.InsertOrError(true))
}

func TestUsageReports(t *testing.T) {
	src, _ := quoteCompany(t)
	sQuote := gtables.ReportSelections_Usage.SQuote

	// no line for the usage: its default report; a usage without a default: an error
	ids, err := reports.UsageReports(src, sQuote)
	mustOK(t, err)
	if len(ids) != 1 || ids[0] != reports.SalesQuoteID {
		t.Errorf("default S.Quote reports %v, want [204]", ids)
	}
	_, err = reports.UsageReports(src, gtables.ReportSelections_Usage.STest)
	if err == nil {
		t.Error("S.Test has no report: want an error")
	}

	// the lines in their sequence, Report ID 0 left out
	insertReportSelection(t, src, sQuote, "2", reports.OrderConfirmationID)
	insertReportSelection(t, src, sQuote, "1", reports.SalesQuoteID)
	insertReportSelection(t, src, sQuote, "3", 0)
	ids, err = reports.UsageReports(src, sQuote)
	mustOK(t, err)
	if len(ids) != 2 || ids[0] != reports.SalesQuoteID || ids[1] != reports.OrderConfirmationID {
		t.Errorf("S.Quote reports %v, want [204 205]", ids)
	}

	// lines that all have Report ID 0: nothing to print
	insertReportSelection(t, src, gtables.ReportSelections_Usage.SOrder, "1", 0)
	_, err = reports.UsageReports(src, gtables.ReportSelections_Usage.SOrder)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrReportSelectionNone {
		t.Errorf("S.Order with Report ID 0 only: %v, want ERR_REPORT_SELECTION_NONE", err)
	}
}

func TestNewReportSelectionSequence(t *testing.T) {
	src, _ := quoteCompany(t)
	rec := gtables.NewReportSelections(src)
	rec.Usage = gtables.ReportSelections_Usage.SInvoice
	tables.NewReportSelection(rec)
	if rec.Sequence != "1" {
		t.Errorf("first sequence %q, want 1", rec.Sequence)
	}
	insertReportSelection(t, src, gtables.ReportSelections_Usage.SInvoice, "1", reports.SalesInvoiceID)
	insertReportSelection(t, src, gtables.ReportSelections_Usage.SInvoice, "2", reports.SalesInvoiceID)
	tables.NewReportSelection(rec)
	if rec.Sequence != "3" {
		t.Errorf("next sequence %q, want 3", rec.Sequence)
	}
}

func TestPrintRecordsPrinterSelection(t *testing.T) {
	src, salesHeader := quoteCompany(t)
	fcodeunits.SetCurrentContext("ANNA", "Anna", "print", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)

	// no Printer Selection: printed without a request page to the print dialog
	result, err := docprint.PrintRecords(salesHeader, gtables.ReportSelections_Usage.SQuote, false)
	mustOK(t, err)
	if result.Data["output"] != reports.OutputPrint || !strings.HasSuffix(result.Data["filename"].(string), "SQ1.pdf") {
		t.Errorf("printout %v %v, want print of SQ1", result.Data["output"], result.Data["filename"])
	}

	// all users and reports: PDF; the user's line for the report: preview; another user's: no
	printerSelection := func(user types.Code, reportID int, printer gtables.PrinterSelectionPrinter_name) {
		rec := gtables.NewPrinterSelection(ftables.DBSource(src.GetDB(), "", src.GetDBType()))
		rec.User_id = user
		rec.Report_id = reportID
		rec.Printer_name = printer
		mustOK(t, rec.InsertOrError(false))
	}
	printerSelection("", 0, gtables.PrinterSelection_Printer_name.PDF)
	if got := reports.PrinterOutput(src, reports.SalesQuoteID, reports.OutputPrint); got != reports.OutputPDF {
		t.Errorf("all users, all reports: %q, want pdf", got)
	}
	printerSelection("BOB", reports.SalesQuoteID, gtables.PrinterSelection_Printer_name.Print)
	printerSelection("ANNA", reports.SalesQuoteID, gtables.PrinterSelection_Printer_name.Preview)
	if got := reports.PrinterOutput(src, reports.SalesQuoteID, reports.OutputPrint); got != reports.OutputPreview {
		t.Errorf("the user's line for the report: %q, want preview", got)
	}
	if got := reports.PrinterOutput(src, reports.SalesInvoiceID, reports.OutputPrint); got != reports.OutputPDF {
		t.Errorf("another report: %q, want pdf (all users, all reports)", got)
	}

	// two reports in the usage: both printouts in the result
	insertReportSelection(t, src, gtables.ReportSelections_Usage.SQuote, "1", reports.SalesQuoteID)
	insertReportSelection(t, src, gtables.ReportSelections_Usage.SQuote, "2", reports.SalesQuoteID)
	result, err = docprint.PrintRecords(salesHeader, gtables.ReportSelections_Usage.SQuote, false)
	mustOK(t, err)
	others, _ := result.Data["printouts"].([]map[string]interface{})
	if result.Data["output"] != reports.OutputPreview || len(others) != 1 {
		t.Errorf("output %v, %d more printouts; want preview and 1", result.Data["output"], len(others))
	}
}
