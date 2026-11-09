package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Copy Sales Document (292): a posted invoice copied with its header onto a new credit memo
// applies to it and has its lines as they were; an open order's lines recalculated onto an
// invoice; nothing is copied onto itself
func TestCopySalesDocument(t *testing.T) {
	src := documentsCompany(t)
	run := func(context any, options map[string]string) error {
		_, err := reports.Execute(reports.CopySalesDocumentID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Context: context, Options: options})
		return err
	}

	postedInv := gtables.NewSalesInvoiceHeader(src)
	if !postedInv.FindLast() {
		t.Fatal("no posted sales invoice")
	}
	creditMemo := gtables.NewSalesHeader(src)
	creditMemo.Document_type = gtables.SalesHeader_Document_type.CreditMemo
	mustOK(t, creditMemo.InsertOrError(true))
	mustOK(t, run(creditMemo, map[string]string{"document_type": "7", "document_no": postedInv.No.String(), "include_header": "true", "recalculate_lines": "false"}))
	mustOK(t, creditMemo.GetOrError(map[string]interface{}{"document_type": int(creditMemo.Document_type), "no": creditMemo.No}))
	if creditMemo.Bill_to_customer_no != postedInv.Bill_to_customer_no || creditMemo.Document_type != gtables.SalesHeader_Document_type.CreditMemo {
		t.Errorf("credit memo header: %s %v, want customer %s", creditMemo.Bill_to_customer_no, creditMemo.Document_type, postedInv.Bill_to_customer_no)
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	custLedgEntry.SetRange("document_no", postedInv.No)
	custLedgEntry.SetRange("open", true)
	if custLedgEntry.FindFirst() && (creditMemo.Applies_to_doc_no != postedInv.No || creditMemo.Applies_to_doc_type != gtables.SalesHeader_Applies_to_doc_type.Invoice) {
		t.Errorf("credit memo applies to %v %s, want the open invoice %s", creditMemo.Applies_to_doc_type, creditMemo.Applies_to_doc_no, postedInv.No)
	}
	invLine := gtables.NewSalesInvoiceLine(src)
	invLine.SetRange("document_no", postedInv.No)
	memoLine := gtables.NewSalesLine(src)
	memoLine.SetRange("document_type", gtables.SalesLine_Document_type.CreditMemo)
	memoLine.SetRange("document_no", creditMemo.No)
	if memoLine.Count() != invLine.Count() || memoLine.Count() == 0 {
		t.Fatalf("credit memo lines: %d, invoice lines: %d", memoLine.Count(), invLine.Count())
	}
	for found := memoLine.FindSetBuffered(); found; found = memoLine.Next() {
		if !memoLine.Quantity_invoiced.IsZero() || !memoLine.Qty_to_invoice.Equal(memoLine.Quantity) {
			t.Errorf("credit memo line %d: invoiced %s, to invoice %s of %s", memoLine.Line_no, memoLine.Quantity_invoiced, memoLine.Qty_to_invoice, memoLine.Quantity)
		}
	}

	// an open order recalculated onto a new invoice of its customer
	order := gtables.NewSalesHeader(src)
	order.SetRange("document_type", gtables.SalesHeader_Document_type.Order)
	if !order.FindFirst() {
		t.Fatal("no open sales order")
	}
	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	mustOK(t, invoice.InsertOrError(true))
	mustOK(t, invoice.Validate("sell_to_customer_no", order.Sell_to_customer_no))
	mustOK(t, invoice.ModifyOrError(true))
	mustOK(t, run(invoice, map[string]string{"document_type": "3", "document_no": order.No.String(), "recalculate_lines": "true"}))
	orderLine := gtables.NewSalesLine(src)
	orderLine.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	orderLine.SetRange("document_no", order.No)
	invoiceLine := gtables.NewSalesLine(src)
	invoiceLine.SetRange("document_type", gtables.SalesLine_Document_type.Invoice)
	invoiceLine.SetRange("document_no", invoice.No)
	if invoiceLine.Count() != orderLine.Count() {
		t.Errorf("invoice lines: %d, order lines: %d", invoiceLine.Count(), orderLine.Count())
	}
	mustOK(t, orderLine.CalcSums("quantity"))
	mustOK(t, invoiceLine.CalcSums("quantity"))
	if !invoiceLine.Quantity.Equal(orderLine.Quantity) {
		t.Errorf("invoice quantity %s, order quantity %s", invoiceLine.Quantity, orderLine.Quantity)
	}

	err := run(invoice, map[string]string{"document_type": "4", "document_no": invoice.No.String()})
	if err == nil || !strings.Contains(err.Error(), "cannot be copied onto itself") {
		t.Errorf("copy onto itself: %v", err)
	}
}

// Copy Purchase Document (492): a posted invoice's lines recalculated onto a new invoice of the
// vendor
func TestCopyPurchaseDocument(t *testing.T) {
	src := documentsCompany(t)
	postedInv := gtables.NewPurchInvHeader(src)
	if !postedInv.FindLast() {
		t.Fatal("no posted purchase invoice")
	}
	invoice := gtables.NewPurchaseHeader(src)
	invoice.Document_type = gtables.PurchaseHeader_Document_type.Invoice
	mustOK(t, invoice.InsertOrError(true))
	mustOK(t, invoice.Validate("buy_from_vendor_no", postedInv.Buy_from_vendor_no))
	mustOK(t, invoice.ModifyOrError(true))
	_, err := reports.Execute(reports.CopyPurchDocumentID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: invoice, Options: map[string]string{"document_type": "7", "document_no": postedInv.No.String(), "recalculate_lines": "true"}})
	mustOK(t, err)
	invLine := gtables.NewPurchInvLine(src)
	invLine.SetRange("document_no", postedInv.No)
	invLine.SetFilter("type", "<>0")
	line := gtables.NewPurchaseLine(src)
	line.SetRange("document_type", gtables.PurchaseLine_Document_type.Invoice)
	line.SetRange("document_no", invoice.No)
	line.SetFilter("type", "<>0")
	if line.Count() == 0 || line.Count() != invLine.Count() {
		t.Errorf("invoice lines: %d, posted invoice lines: %d", line.Count(), invLine.Count())
	}
}

// The credit limit (codeunit 312): a quantity the user enters that takes the customer over the
// credit limit gives a warning; the value is kept
func TestCreditLimitWarning(t *testing.T) {
	src := documentsCompany(t)
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Credit_warnings = gtables.SalesReceivablesSetup_Credit_warnings.CreditLimit
	mustOK(t, salesSetup.ModifyOrError(false))
	cust := gtables.NewCustomer(src)
	mustOK(t, cust.GetOrError(types.Code("C00010")))
	cust.Credit_limit_lcy = types.NewDecimalFromInt(1)
	mustOK(t, cust.ModifyOrError(false))

	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", cust.No))
	mustOK(t, order.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = order.No
	line.Line_no = 10000
	mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, line.Validate("no", types.Code("1000")))

	// entered by code: no warning
	fcodeunits.StartNotes()
	mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(5)))
	if notes := fcodeunits.TakeNotes(); len(notes) != 0 {
		t.Errorf("a quantity set by code warned: %v", notes)
	}
	// entered by the user
	fcodeunits.StartNotes()
	fcodeunits.SetCurrFieldNo(line.FieldNo("quantity"))
	err := line.Validate("quantity", types.NewDecimalFromInt(6))
	fcodeunits.SetCurrFieldNo(0)
	notes := fcodeunits.TakeNotes()
	mustOK(t, err)
	if len(notes) != 1 || !strings.Contains(notes[0], "credit limit has been exceeded") || !strings.Contains(notes[0], "C00010") {
		t.Errorf("credit limit warning: %v", notes)
	}
	if !line.Quantity.Equal(types.NewDecimalFromInt(6)) {
		t.Errorf("the quantity was not kept: %s", line.Quantity)
	}
}
