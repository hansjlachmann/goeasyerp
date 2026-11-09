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
