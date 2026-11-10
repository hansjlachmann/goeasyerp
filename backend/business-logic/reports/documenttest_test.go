package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Sales/Purchase Document - Test print the open documents with their warnings: a line with a
// quantity and no number is named; a clean document prints without warnings
func TestDocumentTestReports(t *testing.T) {
	src := documentsCompany(t)

	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.SetRange("document_type", gtables.SalesHeader_Document_type.Order)
	if !salesHeader.FindFirst() {
		t.Fatal("no open sales order")
	}
	result, err := reports.Execute(reports.SalesDocumentTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Context: salesHeader,
		Options: map[string]string{"ship": "true", "invoice": "true"},
	})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	if !strings.Contains(texts, salesHeader.No.String()) || strings.Contains(texts, "Warning!") {
		t.Errorf("clean order %s:\n%s", salesHeader.No, texts)
	}

	// a line with a quantity but no item is a warning
	salesLine := gtables.NewSalesLine(src)
	salesLine.Document_type = gtables.SalesLine_Document_type.Order
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = 990000
	salesLine.Type = gtables.SalesLine_Type.Item
	salesLine.Quantity = types.NewDecimalFromInt(1)
	mustOK(t, salesLine.InsertOrError(false))
	result, err = reports.Execute(reports.SalesDocumentTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Context: salesHeader,
		Options: map[string]string{"ship": "true", "invoice": "true"},
	})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	if !strings.Contains(texts, "Warning!") || !strings.Contains(texts, "must be specified") {
		t.Errorf("order with a line without No.:\n%s", texts)
	}

	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.SetRange("document_type", gtables.PurchaseHeader_Document_type.Invoice)
	if !purchHeader.FindFirst() {
		t.Fatal("no open purchase invoice")
	}
	result, err = reports.Execute(reports.PurchDocumentTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Context: purchHeader,
	})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	if !strings.Contains(texts, purchHeader.No.String()) {
		t.Errorf("purchase invoice %s:\n%s", purchHeader.No, texts)
	}

	// the G/L accounts' Dimension 1/2 Posting rules: the receivables/payables account a header
	// posts to, the sales/purchase account a line posts to
	dimensionRule := func(glAccNo types.Code, rule1, rule2 int) {
		glAcc := gtables.NewGLAccount(src)
		mustOK(t, glAcc.GetOrError(glAccNo))
		glAcc.Dimension_1_posting = gtables.GLAccountDimension_1_posting(rule1)
		glAcc.Dimension_2_posting = gtables.GLAccountDimension_2_posting(rule2)
		mustOK(t, glAcc.ModifyOrError(false))
	}
	codeMandatory := int(gtables.GLAccount_Dimension_1_posting.CodeMandatory)
	noCode := int(gtables.GLAccount_Dimension_1_posting.NoCode)
	salesLine.Delete(false)
	mustOK(t, salesHeader.GetOrError(map[string]interface{}{"document_type": salesHeader.Document_type, "no": salesHeader.No}))
	custPostingGr := gtables.NewCustomerPostingGroup(src)
	mustOK(t, custPostingGr.GetOrError(salesHeader.Customer_posting_group))
	salesHeader.Dimension_1_code = ""
	mustOK(t, salesHeader.ModifyOrError(false))
	dimensionRule(custPostingGr.Receivables_account, codeMandatory, 0)
	result, err = reports.Execute(reports.SalesDocumentTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Context: salesHeader,
		Options: map[string]string{"ship": "true", "invoice": "true"},
	})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	if !strings.Contains(texts, "is mandatory when posting to") || !strings.Contains(texts, custPostingGr.Receivables_account.String()) {
		t.Errorf("order to a receivables account with Code Mandatory:\n%s", texts)
	}

	purchLine := gtables.NewPurchaseLine(src)
	purchLine.SetRange("document_type", purchHeader.Document_type)
	purchLine.SetRange("document_no", purchHeader.No)
	purchLine.SetRange("type", gtables.PurchaseLine_Type.Item)
	if !purchLine.FindFirst() {
		t.Fatal("no item line on the purchase invoice")
	}
	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	mustOK(t, genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": purchLine.Gen_bus_posting_group, "gen_prod_posting_group": purchLine.Gen_prod_posting_group}))
	purchLine.Dimension_2_code = "X"
	mustOK(t, purchLine.ModifyOrError(false))
	dimensionRule(genPostingSetup.Purch_account, 0, noCode)
	result, err = reports.Execute(reports.PurchDocumentTestID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Output: reports.OutputPDF, Context: purchHeader,
	})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	if !strings.Contains(texts, "must be blank when posting to") || !strings.Contains(texts, genPostingSetup.Purch_account.String()) {
		t.Errorf("invoice line to a purchase account with No Code:\n%s", texts)
	}
}

// Batch Post Sales Invoices / Purch. Invoices post every open invoice, each in its own
// transaction, and tell how many were posted
func TestBatchPostInvoices(t *testing.T) {
	reports.SalesPost = func(src ftables.Source, salesHeader *gtables.SalesHeader) error {
		return salesposting.NewSalesPost(src).Run(salesHeader)
	}
	reports.PurchPost = func(src ftables.Source, purchHeader *gtables.PurchaseHeader) error {
		return purchposting.NewPurchPost(src).Run(purchHeader)
	}
	src := documentsCompany(t)

	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.SetRange("document_type", gtables.SalesHeader_Document_type.Invoice)
	open := salesHeader.Count()
	if open == 0 {
		t.Fatal("no open sales invoices")
	}
	result := runBatch(t, src, reports.BatchPostSalesInvoicesID,
		map[string]string{"replace_posting_date": "true", "posting_date": "2026-10-06"}, nil)
	if n := salesHeader.Count(); n != 0 {
		t.Errorf("%d sales invoices left of %d; message %q", n, open, result.Message)
	}
	if !strings.Contains(result.Message, "invoices") {
		t.Errorf("message: %q", result.Message)
	}

	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.SetRange("document_type", gtables.PurchaseHeader_Document_type.Invoice)
	open = purchHeader.Count()
	// the setup's Ext. Doc. No. Mandatory: a document without the vendor's invoice no. is
	// refused and stays (counted as not posted)
	refused := 0
	for found := purchHeader.FindSetBuffered(); found; found = purchHeader.Next() {
		if purchHeader.Vendor_invoice_no == "" {
			refused++
		}
	}
	result = runBatch(t, src, reports.BatchPostPurchInvoicesID, map[string]string{}, nil)
	if n := purchHeader.Count(); n != refused || !strings.Contains(result.Message, "out of a total of") {
		t.Errorf("%d purchase invoices left, want the %d without Vendor Invoice No.; message %q", n, refused, result.Message)
	}
	for found := purchHeader.FindSetBuffered(); found; found = purchHeader.Next() {
		purchHeader.Vendor_invoice_no = "VINV-" + purchHeader.No
		mustOK(t, purchHeader.ModifyOrError(false))
	}
	result = runBatch(t, src, reports.BatchPostPurchInvoicesID, map[string]string{}, nil)
	if n := purchHeader.Count(); n != 0 {
		t.Errorf("%d purchase invoices left of %d; message %q", n, open, result.Message)
	}

	// Replace Posting Date without a date is refused
	_, err := reports.Execute(reports.BatchPostSalesOrdersID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"replace_posting_date": "true", "ship": "true"},
	})
	if err == nil {
		t.Error("Replace Posting Date without a posting date was not refused")
	}
}

// Reminder - Test / Finance Charge Memo - Test print an open reminder and memo with their lines
// and totals; a reminder without posting date is a warning
func TestReminderTestReports(t *testing.T) {
	src := fullDemoCompany(t)

	reminderHeader := gtables.NewReminderHeader(src)
	if !reminderHeader.FindFirst() {
		t.Fatal("no open reminder")
	}
	print := func(id int, context ftables.Table) string {
		result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Context: context})
		mustOK(t, err)
		return printedTexts(result, 0)
	}
	texts := print(reports.ReminderTestID, reminderHeader)
	if !strings.Contains(texts, reminderHeader.No.String()) || !strings.Contains(texts, "Total") || strings.Contains(texts, "Warning!") {
		t.Errorf("reminder %s:\n%s", reminderHeader.No, texts)
	}
	reminderHeader.Posting_date = types.Date{}
	mustOK(t, reminderHeader.ModifyOrError(false))
	texts = print(reports.ReminderTestID, reminderHeader)
	if !strings.Contains(texts, "Warning!") || !strings.Contains(texts, "must be specified") {
		t.Errorf("reminder without posting date:\n%s", texts)
	}

	finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(src)
	if !finChrgMemoHeader.FindFirst() {
		t.Fatal("no open finance charge memo")
	}
	texts = print(reports.FinanceChargeMemoTestID, finChrgMemoHeader)
	if !strings.Contains(texts, finChrgMemoHeader.No.String()) || !strings.Contains(texts, "Total") {
		t.Errorf("finance charge memo %s:\n%s", finChrgMemoHeader.No, texts)
	}
}
