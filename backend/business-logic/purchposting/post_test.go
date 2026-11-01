package purchposting_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/navigate"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchgetreceipt"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func wantDec(t *testing.T, what string, got types.Decimal, want types.Decimal) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

// demoCompany: the demo company (vendors with payment terms, items with their vendors' costs,
// posting setups, the purchase series; the vendor's document number is mandatory), in a
// transaction
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	tx, err := ftables.BeginTransaction(demoDB(t))
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

// demoDB: the demo company's database (no transaction)
func demoDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
	})
	must(t, err)
	return db
}

func newDocument(t *testing.T, src ftables.Source, documentType gtables.PurchaseHeaderDocument_type, vendor string) *gtables.PurchaseHeader {
	t.Helper()
	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.Document_type = documentType
	must(t, purchHeader.InsertOrError(true))
	must(t, purchHeader.Validate("buy_from_vendor_no", types.Code(vendor)))
	must(t, purchHeader.ModifyOrError(true))
	return purchHeader
}

func addLine(t *testing.T, purchHeader *gtables.PurchaseHeader, lineNo int, lineType gtables.PurchaseLineType, no, qty string) *gtables.PurchaseLine {
	t.Helper()
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.Document_type = gtables.PurchaseLineDocument_type(purchHeader.Document_type)
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = lineNo
	must(t, purchLine.Validate("type", lineType))
	must(t, purchLine.Validate("no", types.Code(no)))
	must(t, purchLine.Validate("quantity", dec(qty)))
	must(t, purchLine.InsertOrError(true))
	return purchLine
}

func getLine(t *testing.T, purchHeader *gtables.PurchaseHeader, lineNo int) *gtables.PurchaseLine {
	t.Helper()
	purchLine := gtables.NewPurchaseLine(purchHeader)
	must(t, purchLine.GetOrError(map[string]interface{}{"document_type": int(purchHeader.Document_type), "document_no": purchHeader.No, "line_no": lineNo}))
	return purchLine
}

func reread(t *testing.T, purchHeader *gtables.PurchaseHeader) bool {
	t.Helper()
	return purchHeader.Get(map[string]interface{}{"document_type": int(purchHeader.Document_type), "no": purchHeader.No})
}

func post(t *testing.T, purchHeader *gtables.PurchaseHeader, receive, invoice bool) *purchposting.PurchPost {
	t.Helper()
	purchHeader.Receive = receive
	purchHeader.Invoice = invoice
	purchPost := purchposting.NewPurchPost(purchHeader)
	must(t, purchPost.Run(purchHeader))
	return purchPost
}

// glBalance: the G/L entries of a document add up to zero
func glBalance(t *testing.T, src ftables.Source, documentNo types.Code) {
	t.Helper()
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", documentNo)
	total := types.Decimal{}
	n := 0
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		total = total.Add(glEntry.Amount)
		n++
	}
	if n == 0 || !total.IsZero() {
		t.Errorf("G/L entries of %s: %d, balance %s", documentNo, n, total)
	}
}

// glAmount: the G/L entries of a document on an account
func glAmount(t *testing.T, src ftables.Source, documentNo, account types.Code) types.Decimal {
	t.Helper()
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", documentNo)
	glEntry.SetRange("gl_account_no", account)
	total := types.Decimal{}
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		total = total.Add(glEntry.Amount)
	}
	return total
}

// itemEntries: the item ledger entries of a document — quantity and invoiced quantity
func itemEntries(t *testing.T, src ftables.Source, documentNo types.Code) (types.Decimal, types.Decimal) {
	t.Helper()
	itemLedgEntry := gtables.NewItemLedgerEntry(src)
	itemLedgEntry.SetRange("document_no", documentNo)
	qty, invoiced := types.Decimal{}, types.Decimal{}
	for found := itemLedgEntry.FindSet(); found; found = itemLedgEntry.Next() {
		qty = qty.Add(itemLedgEntry.Quantity)
		invoiced = invoiced.Add(itemLedgEntry.Invoiced_quantity)
	}
	return qty, invoiced
}

func vendEntry(t *testing.T, src ftables.Source, documentNo types.Code) *gtables.VendorLedgerEntry {
	t.Helper()
	vendLedgEntry := gtables.NewVendorLedgerEntry(src)
	vendLedgEntry.SetRange("document_no", documentNo)
	if !vendLedgEntry.FindFirst() {
		t.Fatalf("no vendor entry of %s", documentNo)
	}
	vendLedgEntry.CalcFields("amount", "remaining_amount")
	return vendLedgEntry
}

// An order received in part, the receipt invoiced, then the rest received and invoiced: the
// posted receipts and invoices, the item, vendor and G/L entries, the order's quantities; the
// order is gone when everything is invoiced
func TestPostOrderInParts(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.PurchaseHeader_Document_type.Order, "V00010")
	addLine(t, order, 10000, gtables.PurchaseLine_Type.Item, "1000", "10")
	itemLine := getLine(t, order, 10000)
	must(t, itemLine.Validate("qty_to_receive", dec("4")))
	must(t, itemLine.ModifyOrError(true))
	orderTotal := itemLine.Amount_including_tax

	// Receive 4
	purchPost := post(t, order, true, false)
	if purchPost.ReceiptNo == "" || purchPost.InvoiceNo != "" || purchPost.Deleted {
		t.Fatalf("receive: receipt %q invoice %q deleted %v", purchPost.ReceiptNo, purchPost.InvoiceNo, purchPost.Deleted)
	}
	receipt1 := purchPost.ReceiptNo
	qty, invoiced := itemEntries(t, src, receipt1)
	wantDec(t, "received item quantity", qty, dec("4"))
	wantDec(t, "received item invoiced", invoiced, types.Decimal{})
	itemLine = getLine(t, order, 10000)
	wantDec(t, "Quantity Received", itemLine.Quantity_received, dec("4"))
	wantDec(t, "Outstanding Quantity", itemLine.Outstanding_quantity, dec("6"))
	wantDec(t, "Qty. Rcd. Not Invoiced", itemLine.Qty_rcd_not_invoiced, dec("4"))
	wantDec(t, "Qty. to Receive", itemLine.Qty_to_receive, dec("6"))
	rcptLine := gtables.NewPurchRcptLine(src)
	must(t, rcptLine.GetOrError(map[string]interface{}{"document_no": receipt1, "line_no": 10000}))
	wantDec(t, "receipt line quantity", rcptLine.Quantity, dec("4"))
	wantDec(t, "receipt line not invoiced", rcptLine.Qty_rcd_not_invoiced, dec("4"))
	if rcptLine.Order_no != order.No || rcptLine.Item_rcpt_entry_no == 0 {
		t.Errorf("receipt line order %s, item entry %d", rcptLine.Order_no, rcptLine.Item_rcpt_entry_no)
	}
	if !reread(t, order) || order.Last_receiving_no != receipt1 || order.Receive {
		t.Errorf("order after receiving: last receiving no. %s, receive %v", order.Last_receiving_no, order.Receive)
	}

	// Invoicing needs the vendor's invoice number
	err := purchposting.NewPurchPost(src).Run(func() *gtables.PurchaseHeader { order.Invoice = true; return order }())
	if err == nil || !strings.Contains(err.Error(), "Vendor Invoice No.") {
		t.Errorf("invoice without the vendor's number: %v", err)
	}
	must(t, order.Validate("vendor_invoice_no", types.Code("VINV-1")))
	must(t, order.ModifyOrError(true))

	// Invoice what is received
	purchPost = post(t, order, false, true)
	invoice1 := purchPost.InvoiceNo
	if invoice1 == "" || purchPost.ReceiptNo != "" || purchPost.Deleted {
		t.Fatalf("invoice: %q %q %v", invoice1, purchPost.ReceiptNo, purchPost.Deleted)
	}
	_, invoiced = itemEntries(t, src, invoice1)
	wantDec(t, "invoiced item quantity", invoiced, dec("4"))
	must(t, rcptLine.GetOrError(map[string]interface{}{"document_no": receipt1, "line_no": 10000}))
	wantDec(t, "receipt line invoiced", rcptLine.Quantity_invoiced, dec("4"))
	wantDec(t, "receipt line not invoiced after", rcptLine.Qty_rcd_not_invoiced, types.Decimal{})
	invHeader := gtables.NewPurchInvHeader(src)
	must(t, invHeader.GetOrError(invoice1))
	invHeader.CalcFields("amount", "amount_including_tax")
	entry := vendEntry(t, src, invoice1)
	wantDec(t, "vendor entry", entry.Amount, invHeader.Amount_including_tax.Neg())
	if entry.External_document_no != "VINV-1" {
		t.Errorf("vendor entry external document no. %q", entry.External_document_no)
	}
	glBalance(t, src, invoice1)
	wantDec(t, "purchase account", glAmount(t, src, invoice1, "4110"), invHeader.Amount)
	if invHeader.Order_no != order.No || invHeader.Buy_from_vendor_no != "V00010" {
		t.Errorf("posted invoice order %s vendor %s", invHeader.Order_no, invHeader.Buy_from_vendor_no)
	}
	itemLine = getLine(t, order, 10000)
	wantDec(t, "Quantity Invoiced", itemLine.Quantity_invoiced, dec("4"))

	// Receive and invoice the rest: the order is gone
	must(t, order.Validate("vendor_invoice_no", types.Code("VINV-2")))
	must(t, order.ModifyOrError(true))
	purchPost = post(t, order, true, true)
	if !purchPost.Deleted || purchPost.ReceiptNo == "" || purchPost.InvoiceNo == "" {
		t.Fatalf("rest: %+v", purchPost)
	}
	qty, invoiced = itemEntries(t, src, purchPost.InvoiceNo)
	wantDec(t, "rest item quantity", qty, dec("6"))
	wantDec(t, "rest item invoiced", invoiced, dec("6"))
	glBalance(t, src, purchPost.InvoiceNo)
	if reread(t, order) {
		t.Error("the order is still there")
	}
	lines := gtables.NewPurchaseLine(src)
	lines.SetRange("document_no", order.No)
	if lines.Count() != 0 {
		t.Error("the order's lines are still there")
	}
	second := vendEntry(t, src, purchPost.InvoiceNo)
	wantDec(t, "both invoices", entry.Amount.Add(second.Amount), orderTotal.Neg())

}

// An invoice with an item and a G/L account line, then a credit memo applied to it
func TestPostInvoiceAndCreditMemo(t *testing.T) {
	src := demoCompany(t)
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	addLine(t, invoice, 10000, gtables.PurchaseLine_Type.Item, "1000", "2")
	addLine(t, invoice, 20000, gtables.PurchaseLine_Type.GLAccount, "4130", "1")
	glLine := getLine(t, invoice, 20000)
	must(t, glLine.Validate("direct_unit_cost", dec("100")))
	must(t, glLine.ModifyOrError(true))
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-77")))
	must(t, invoice.ModifyOrError(true))
	purchPost := post(t, invoice, false, false)
	if !purchPost.Deleted || purchPost.InvoiceNo == "" || purchPost.ReceiptNo != "" {
		t.Fatalf("invoice: %+v", purchPost)
	}
	postedInvoice := purchPost.InvoiceNo
	qty, invoiced := itemEntries(t, src, postedInvoice)
	wantDec(t, "invoice item quantity", qty, dec("2"))
	wantDec(t, "invoice item invoiced", invoiced, dec("2"))
	glBalance(t, src, postedInvoice)
	wantDec(t, "G/L account line", glAmount(t, src, postedInvoice, "4130"), dec("100"))
	invLines := gtables.NewPurchInvLine(src)
	invLines.SetRange("document_no", postedInvoice)
	if invLines.Count() != 2 {
		t.Errorf("posted invoice lines %d", invLines.Count())
	}
	invHeader := gtables.NewPurchInvHeader(src)
	must(t, invHeader.GetOrError(postedInvoice))
	if invHeader.Pre_assigned_no != invoice.No {
		t.Errorf("posted invoice pre-assigned no. %s", invHeader.Pre_assigned_no)
	}

	creditMemo := newDocument(t, src, gtables.PurchaseHeader_Document_type.CreditMemo, "V00010")
	addLine(t, creditMemo, 10000, gtables.PurchaseLine_Type.Item, "1000", "1")
	creditMemo.Applies_to_doc_type = gtables.PurchaseHeader_Applies_to_doc_type.Invoice
	creditMemo.Vendor_cr_memo_no = "V-CR-1"
	must(t, creditMemo.ModifyOrError(true))
	must(t, creditMemo.Validate("applies_to_doc_no", postedInvoice))
	must(t, creditMemo.ModifyOrError(true))
	purchPost = post(t, creditMemo, false, false)
	if !purchPost.Deleted || purchPost.CrMemoNo == "" {
		t.Fatalf("credit memo: %+v", purchPost)
	}
	qty, invoiced = itemEntries(t, src, purchPost.CrMemoNo)
	wantDec(t, "credit memo item quantity", qty, dec("-1"))
	wantDec(t, "credit memo item invoiced", invoiced, dec("-1"))
	glBalance(t, src, purchPost.CrMemoNo)
	cmEntry := vendEntry(t, src, purchPost.CrMemoNo)
	if !cmEntry.Amount.IsPositive() || cmEntry.Open {
		t.Errorf("credit memo entry amount %s open %v", cmEntry.Amount, cmEntry.Open)
	}
	invEntry := vendEntry(t, src, postedInvoice)
	wantDec(t, "invoice remaining after the credit memo", invEntry.Remaining_amount, invEntry.Amount.Add(cmEntry.Amount))

	// Nothing to post; a quote cannot be posted; an order needs Receive or Invoice
	empty := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	empty.Vendor_invoice_no = "V-0"
	err := purchposting.NewPurchPost(src).Run(empty)
	if err == nil || !strings.Contains(err.Error(), "nothing to post") {
		t.Errorf("empty invoice: %v", err)
	}
	quote := newDocument(t, src, gtables.PurchaseHeader_Document_type.Quote, "V00010")
	addLine(t, quote, 10000, gtables.PurchaseLine_Type.Item, "1000", "1")
	err = purchposting.NewPurchPost(src).Run(quote)
	if err == nil || !strings.Contains(err.Error(), "cannot be posted") {
		t.Errorf("quote: %v", err)
	}
	order := newDocument(t, src, gtables.PurchaseHeader_Document_type.Order, "V00010")
	addLine(t, order, 10000, gtables.PurchaseLine_Type.Item, "1000", "1")
	err = purchposting.NewPurchPost(src).Run(order)
	if err == nil || !strings.Contains(err.Error(), "Receive") {
		t.Errorf("order without receive/invoice: %v", err)
	}
}

// Use tax: the vendor is not charged the sales tax; the tax owed goes to the jurisdiction's
// purchase tax account and its reverse charge account, and the posting balances
func TestPostUseTax(t *testing.T) {
	src := demoCompany(t)
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	addLine(t, invoice, 10000, gtables.PurchaseLine_Type.Item, "1000", "2")
	line := getLine(t, invoice, 10000)
	if line.Tax_area_code == "" || !line.Tax_liable {
		t.Skipf("the demo vendor's line has no sales tax (area %q liable %v)", line.Tax_area_code, line.Tax_liable)
	}
	must(t, line.Validate("use_tax", true))
	must(t, line.ModifyOrError(true))
	line = getLine(t, invoice, 10000)
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-UT")))
	must(t, invoice.ModifyOrError(true))
	purchPost := post(t, invoice, false, false)
	glBalance(t, src, purchPost.InvoiceNo)
	entry := vendEntry(t, src, purchPost.InvoiceNo)
	wantDec(t, "vendor entry without the use tax", entry.Amount, line.Amount.Neg())
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", purchPost.InvoiceNo)
	if !taxEntry.FindFirst() || !taxEntry.Use_tax || taxEntry.Amount.IsZero() {
		t.Errorf("use tax entry: found %v use tax %v amount %s", taxEntry.Count() > 0, taxEntry.Use_tax, taxEntry.Amount)
	}
}

// printTexts: the texts of a printout run on a document (as from its card)
func printTexts(t *testing.T, src ftables.Source, reportID int, context interface{}) string {
	t.Helper()
	result, err := reports.Execute(reportID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Context: context})
	must(t, err)
	var texts []string
	for _, text := range result.Texts {
		texts = append(texts, text.Text)
	}
	return "|" + strings.Join(texts, "|") + "|"
}

// An order received, its receipt got onto an invoice (Get Receipt Lines) and the invoice posted:
// the receipt's item entry is invoiced, the order line counts it; the printouts of the order,
// the receipt and the invoice count in No. Printed; Navigate finds the posted invoice
func TestPurchaseHistory(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.PurchaseHeader_Document_type.Order, "V00010")
	addLine(t, order, 10000, gtables.PurchaseLine_Type.Item, "1000", "5")
	orderCost := getLine(t, order, 10000).Direct_unit_cost
	joined := printTexts(t, src, reports.PurchaseOrderID, order)
	if !strings.Contains(joined, "|"+order.No.String()+"|") || !strings.Contains(joined, "|Nordisk Papir AS|") {
		t.Errorf("order printout: %s", joined)
	}
	receipt := post(t, order, true, false).ReceiptNo

	// Get Receipt Lines onto an invoice, post it
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	must(t, purchgetreceipt.CreateInvLines(invoice, receipt, "Receipt No. %1:"))
	invLine := getLine(t, invoice, 20000)
	if invLine.Receipt_no != receipt || !invLine.Quantity.Equal(dec("5")) || !invLine.Direct_unit_cost.Equal(orderCost) {
		t.Fatalf("invoice line from the receipt: %s %s at %s", invLine.Receipt_no, invLine.Quantity, invLine.Direct_unit_cost)
	}
	if getLine(t, invoice, 10000).Description != types.Text("Receipt No. "+receipt.String()+":") {
		t.Errorf("description line %q", getLine(t, invoice, 10000).Description)
	}
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-GR-1")))
	must(t, invoice.ModifyOrError(true))
	purchPost := post(t, invoice, false, false)
	if purchPost.ReceiptNo != "" {
		t.Errorf("the invoice received again: %s", purchPost.ReceiptNo)
	}
	qty, invoiced := itemEntries(t, src, purchPost.InvoiceNo)
	wantDec(t, "invoice item quantity", qty, types.Decimal{})
	wantDec(t, "invoice item invoiced", invoiced, dec("5"))
	glBalance(t, src, purchPost.InvoiceNo)
	rcptLine := gtables.NewPurchRcptLine(src)
	must(t, rcptLine.GetOrError(map[string]interface{}{"document_no": receipt, "line_no": 10000}))
	wantDec(t, "receipt line not invoiced", rcptLine.Qty_rcd_not_invoiced, types.Decimal{})
	orderLine := getLine(t, order, 10000)
	wantDec(t, "order line invoiced", orderLine.Quantity_invoiced, dec("5"))
	wantDec(t, "order line received not invoiced", orderLine.Qty_rcd_not_invoiced, types.Decimal{})
	postedLine := gtables.NewPurchInvLine(src)
	must(t, postedLine.GetOrError(map[string]interface{}{"document_no": purchPost.InvoiceNo, "line_no": 20000}))
	if postedLine.Receipt_no != receipt {
		t.Errorf("posted invoice line receipt no. %q", postedLine.Receipt_no)
	}

	// The printouts count in No. Printed
	invHeader := gtables.NewPurchInvHeader(src)
	invHeader.No = purchPost.InvoiceNo
	rcptHeader := gtables.NewPurchRcptHeader(src)
	rcptHeader.No = receipt
	for _, c := range []struct {
		reportID int
		no       types.Code
		context  interface{}
	}{{reports.PurchaseInvoiceID, purchPost.InvoiceNo, invHeader}, {reports.PurchaseReceiptID, receipt, rcptHeader}} {
		joined := printTexts(t, src, c.reportID, c.context)
		if !strings.Contains(joined, "|"+c.no.String()+"|") || !strings.Contains(joined, "|Nordisk Papir AS|") {
			t.Errorf("report %d: %s", c.reportID, joined)
		}
	}
	must(t, invHeader.GetOrError(purchPost.InvoiceNo))
	must(t, rcptHeader.GetOrError(receipt))
	if !reread(t, order) || invHeader.No_printed != 1 || rcptHeader.No_printed != 1 || order.No_printed != 1 {
		t.Errorf("No. Printed: invoice %d, receipt %d, order %d", invHeader.No_printed, rcptHeader.No_printed, order.No_printed)
	}

	// Navigate finds the posted invoice and its entries
	docEntry := gtables.NewDocumentEntry(src)
	must(t, docEntry.InitTemporary(src.GetDB(), src.GetCompany(), src.GetDBType()))
	must(t, navigate.FindRecords(docEntry, purchPost.InvoiceNo.String(), ""))
	found := map[int]string{}
	for ok := docEntry.FindSet(); ok; ok = docEntry.Next() {
		found[docEntry.Table_id] = docEntry.No_filter.String() + "/" + docEntry.Document_no_filter.String()
	}
	if found[gtables.PurchInvHeaderTableID] != purchPost.InvoiceNo.String()+"/" || found[gtables.VendorLedgerEntryTableID] != "/"+purchPost.InvoiceNo.String() {
		t.Errorf("navigate: %v", found)
	}
}

// The quote and the posted credit memo print with their number and vendor
func TestPurchaseQuoteAndCreditMemoPrintouts(t *testing.T) {
	src := demoCompany(t)
	quote := newDocument(t, src, gtables.PurchaseHeader_Document_type.Quote, "V00010")
	addLine(t, quote, 10000, gtables.PurchaseLine_Type.Item, "1000", "2")
	joined := printTexts(t, src, reports.PurchaseQuoteID, quote)
	if !strings.Contains(joined, "|"+quote.No.String()+"|") || !strings.Contains(joined, "|Nordisk Papir AS|") {
		t.Errorf("quote printout: %s", joined)
	}
	creditMemo := newDocument(t, src, gtables.PurchaseHeader_Document_type.CreditMemo, "V00010")
	addLine(t, creditMemo, 10000, gtables.PurchaseLine_Type.Item, "1000", "1")
	must(t, creditMemo.Validate("vendor_cr_memo_no", types.Code("V-CR-9")))
	must(t, creditMemo.ModifyOrError(true))
	crMemoNo := post(t, creditMemo, false, false).CrMemoNo
	crMemoHeader := gtables.NewPurchCrMemoHdr(src)
	crMemoHeader.No = crMemoNo
	joined = printTexts(t, src, reports.PurchaseCrMemoID, crMemoHeader)
	if !strings.Contains(joined, "|"+crMemoNo.String()+"|") || !strings.Contains(joined, "|V-CR-9|") {
		t.Errorf("credit memo printout: %s", joined)
	}
}
