package docprint

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchcalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salescalcdisc"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TestReportPrintID is codeunit 228 Test Report-Print: the Test Report of the journal batch
// lists (PrintGenJnlBatch, PrintItemJnlBatch, …: the template's Test Report ID on the batch, with
// its request page) and of the bank account reconciliation (PrintBankAccRecon: report 1408)
const TestReportPrintID = 228

func init() {
	batch := func(table string, fn functions.Func) {
		functions.Register("table", table, "TestReport", fn, functions.WithPrint())
	}
	batch(gtables.GenJournalBatchTableName, functions.For(PrintGenJnlBatch))
	batch(gtables.ItemJournalBatchTableName, functions.For(PrintItemJnlBatch))
	batch(gtables.BOMJournalBatchTableName, functions.For(PrintBOMJnlBatch))
	batch(gtables.ResJournalBatchTableName, functions.For(PrintResJnlBatch))
	batch(gtables.ProjectJournalBatchTableName, functions.For(PrintProjectJnlBatch))
	batch(gtables.FAJournalBatchTableName, functions.For(PrintFAJnlBatch))
	batch(gtables.InsuranceJournalBatchTableName, functions.For(PrintInsuranceJnlBatch))
	batch(gtables.BankAccReconciliationTableName, functions.For(PrintBankAccRecon))
	functions.Register("table", gtables.BankAccountStatementTableName, "PrintRecords", functions.For(PrintBankAccStmt), functions.WithPrint())
	functions.Register("table", gtables.GenJournalLineTableName, "PrintCheck", functions.For(PrintCheck), functions.WithPrint())
	functions.Register("table", gtables.SalesHeaderTableName, "TestReport", functions.For(PrintTestSalesHeader), functions.WithPrint())
	functions.Register("table", gtables.PurchaseHeaderTableName, "TestReport", functions.For(PrintTestPurchHeader), functions.WithPrint())
}

// testReportRecord is the record a test report runs on
type testReportRecord interface {
	ftables.Table
	ftables.Source
	SetRecFilter()
}

// printTestReport runs the report with its request page on the record (filtered to it,
// SETRECFILTER); the user needs Execute on the report
func printTestReport(reportID int, rec testReportRecord) (fcodeunits.Result, error) {
	if !fcodeunits.CanExecute(session.ObjectReport, reportID) {
		return fcodeunits.Result{}, apperrors.NoPermissionReport(reportID, reports.Caption(reportID, fcodeunits.CurrentLanguage()))
	}
	rec.SetRecFilter()
	result, err := reports.Execute(reportID, rec.GetDB(), rec.GetCompany(), rec.GetDBType(), reports.RunOptions{RequestPage: true, Record: rec})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if result.Cancelled {
		return fcodeunits.Result{Success: true}, nil
	}
	return reports.WithPrintouts(fcodeunits.Result{Success: true}, []reports.Result{result}), nil
}

// PrintGenJnlBatch prints the general journal template's test report on the batch
func PrintGenJnlBatch(genJnlBatch *gtables.GenJournalBatch) (fcodeunits.Result, error) {
	genJnlTemplate := gtables.NewGenJournalTemplate(genJnlBatch)
	err := genJnlTemplate.GetOrError(genJnlBatch.Journal_template_name)
	if err == nil {
		err = genJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(genJnlTemplate.Test_report_id, genJnlBatch)
}

// PrintItemJnlBatch prints the item journal template's test report on the batch
func PrintItemJnlBatch(itemJnlBatch *gtables.ItemJournalBatch) (fcodeunits.Result, error) {
	itemJnlTemplate := gtables.NewItemJournalTemplate(itemJnlBatch)
	err := itemJnlTemplate.GetOrError(itemJnlBatch.Journal_template_name)
	if err == nil {
		err = itemJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(itemJnlTemplate.Test_report_id, itemJnlBatch)
}

// PrintBOMJnlBatch prints the BOM journal template's test report on the batch
func PrintBOMJnlBatch(bomJnlBatch *gtables.BOMJournalBatch) (fcodeunits.Result, error) {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(bomJnlBatch)
	err := bomJnlTemplate.GetOrError(bomJnlBatch.Journal_template_name)
	if err == nil {
		err = bomJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(bomJnlTemplate.Test_report_id, bomJnlBatch)
}

// PrintResJnlBatch prints the resource journal template's test report on the batch
func PrintResJnlBatch(resJnlBatch *gtables.ResJournalBatch) (fcodeunits.Result, error) {
	resJnlTemplate := gtables.NewResJournalTemplate(resJnlBatch)
	err := resJnlTemplate.GetOrError(resJnlBatch.Journal_template_name)
	if err == nil {
		err = resJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(resJnlTemplate.Test_report_id, resJnlBatch)
}

// PrintProjectJnlBatch prints the project journal template's test report on the batch
func PrintProjectJnlBatch(projectJnlBatch *gtables.ProjectJournalBatch) (fcodeunits.Result, error) {
	projectJnlTemplate := gtables.NewProjectJournalTemplate(projectJnlBatch)
	err := projectJnlTemplate.GetOrError(projectJnlBatch.Journal_template_name)
	if err == nil {
		err = projectJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(projectJnlTemplate.Test_report_id, projectJnlBatch)
}

// PrintFAJnlBatch prints the FA journal template's test report on the batch
func PrintFAJnlBatch(faJnlBatch *gtables.FAJournalBatch) (fcodeunits.Result, error) {
	faJnlTemplate := gtables.NewFAJournalTemplate(faJnlBatch)
	err := faJnlTemplate.GetOrError(faJnlBatch.Journal_template_name)
	if err == nil {
		err = faJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(faJnlTemplate.Test_report_id, faJnlBatch)
}

// PrintInsuranceJnlBatch prints the insurance journal template's test report on the batch
func PrintInsuranceJnlBatch(insuranceJnlBatch *gtables.InsuranceJournalBatch) (fcodeunits.Result, error) {
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(insuranceJnlBatch)
	err := insuranceJnlTemplate.GetOrError(insuranceJnlBatch.Journal_template_name)
	if err == nil {
		err = insuranceJnlTemplate.TestField("test_report_id")
	}
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printTestReport(insuranceJnlTemplate.Test_report_id, insuranceJnlBatch)
}

// PrintBankAccRecon prints the reconciliation's test report with the reports of Report
// Selections usage B.Recon.Test (by default 1408 Bank Acc. Recon. - Test), each with its
// request page
func PrintBankAccRecon(bankAccRecon *gtables.BankAccReconciliation) (fcodeunits.Result, error) {
	bankAccRecon.SetRecFilter()
	return PrintRecords(bankAccRecon, gtables.ReportSelections_Usage.BReconTest, true)
}

// PrintBankAccStmt prints a statement with the reports of Report Selections usage B.Stmt (by
// default 1407 Bank Account Statement), each with its request page
func PrintBankAccStmt(bankAccStmt *gtables.BankAccountStatement) (fcodeunits.Result, error) {
	bankAccStmt.SetRecFilter()
	return PrintRecords(bankAccStmt, gtables.ReportSelections_Usage.BStmt, true)
}

// PrintCheck (Payments › Print Check on the payment journal) prints the journal's checks with the
// reports of Report Selections usage B.Check (by default 1401 Check), each with its request page
func PrintCheck(genJnlLine *gtables.GenJournalLine) (fcodeunits.Result, error) {
	result, err := PrintRecords(genJnlLine, gtables.ReportSelections_Usage.BCheck, true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result.Refresh = true
	return result, nil
}

// PrintTestSalesHeader (codeunit 228 PrintSalesHeader): the S.Test reports (202 Sales Document -
// Test) on the document with their request page — the invoice discount calculated first when
// Sales & Receivables Setup says so
func PrintTestSalesHeader(salesHeader *gtables.SalesHeader) (fcodeunits.Result, error) {
	salesSetup := gtables.GetSalesReceivablesSetup(salesHeader)
	if salesSetup.Calc_inv_discount {
		err := salescalcdisc.CalculateInvoiceDiscount(salesHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		salesHeader.Get(map[string]interface{}{"document_type": int(salesHeader.Document_type), "no": salesHeader.No})
	}
	return PrintRecords(salesHeader, gtables.ReportSelections_Usage.STest, true)
}

// PrintTestPurchHeader (codeunit 228 PrintPurchHeader): the P.Test reports (402 Purchase
// Document - Test) on the document with their request page — the invoice discount calculated
// first when Purchases & Payables Setup says so
func PrintTestPurchHeader(purchHeader *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	purchSetup := gtables.GetPurchasesPayablesSetup(purchHeader)
	if purchSetup.Calc_inv_discount {
		err := purchcalcdisc.CalculateInvoiceDiscount(purchHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		purchHeader.Get(map[string]interface{}{"document_type": int(purchHeader.Document_type), "no": purchHeader.No})
	}
	return PrintRecords(purchHeader, gtables.ReportSelections_Usage.PTest, true)
}
