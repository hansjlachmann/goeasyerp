package salesposting_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/navigate"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/salesgetshipment"
	"github.com/hansjlachmann/openerp/backend/business-logic/salesposting"
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

// demoCompany: the demo company (customers with sales tax, items on inventory, resources,
// posting setups, the posted documents' number series), in a transaction
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db := demoDB(t)
	tx, err := ftables.BeginTransaction(db)
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

func newDocument(t *testing.T, src ftables.Source, documentType gtables.SalesHeaderDocument_type, customer string) *gtables.SalesHeader {
	t.Helper()
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = documentType
	must(t, salesHeader.InsertOrError(true))
	must(t, salesHeader.Validate("sell_to_customer_no", types.Code(customer)))
	must(t, salesHeader.ModifyOrError(true))
	return salesHeader
}

func addLine(t *testing.T, salesHeader *gtables.SalesHeader, lineNo int, lineType gtables.SalesLineType, no, qty string) *gtables.SalesLine {
	t.Helper()
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = lineNo
	must(t, salesLine.Validate("type", lineType))
	must(t, salesLine.Validate("no", types.Code(no)))
	must(t, salesLine.Validate("quantity", dec(qty)))
	must(t, salesLine.InsertOrError(true))
	return salesLine
}

func getLine(t *testing.T, salesHeader *gtables.SalesHeader, lineNo int) *gtables.SalesLine {
	t.Helper()
	salesLine := gtables.NewSalesLine(salesHeader)
	must(t, salesLine.GetOrError(map[string]interface{}{"document_type": int(salesHeader.Document_type), "document_no": salesHeader.No, "line_no": lineNo}))
	return salesLine
}

func reread(t *testing.T, salesHeader *gtables.SalesHeader) bool {
	t.Helper()
	return salesHeader.Get(map[string]interface{}{"document_type": int(salesHeader.Document_type), "no": salesHeader.No})
}

func post(t *testing.T, salesHeader *gtables.SalesHeader, ship, invoice bool) *salesposting.SalesPost {
	t.Helper()
	salesHeader.Ship = ship
	salesHeader.Invoice = invoice
	salesPost := salesposting.NewSalesPost(salesHeader)
	must(t, salesPost.Run(salesHeader))
	return salesPost
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

func custEntry(t *testing.T, src ftables.Source, documentNo types.Code) *gtables.CustomerLedgerEntry {
	t.Helper()
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	custLedgEntry.SetRange("document_no", documentNo)
	if !custLedgEntry.FindFirst() {
		t.Fatalf("no customer entry of %s", documentNo)
	}
	return custLedgEntry
}

// An order shipped in part, the shipment invoiced, then the rest shipped and invoiced: the posted
// shipments and invoices, the item, resource, customer and G/L entries, the order's quantities;
// the order is gone when everything is invoiced
func TestPostOrderInParts(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.SalesHeader_Document_type.Order, "C00010")
	addLine(t, order, 10000, gtables.SalesLine_Type.Item, "1000", "10")
	addLine(t, order, 20000, gtables.SalesLine_Type.Resource, "R0010", "3")
	itemLine := getLine(t, order, 10000)
	must(t, itemLine.Validate("qty_to_ship", dec("4")))
	must(t, itemLine.ModifyOrError(true))
	orderTotal := types.Decimal{}
	for _, lineNo := range []int{10000, 20000} {
		orderTotal = orderTotal.Add(getLine(t, order, lineNo).Amount_including_tax)
	}

	// Ship 4 of the item and the resource
	salesPost := post(t, order, true, false)
	if salesPost.ShipmentNo == "" || salesPost.InvoiceNo != "" || salesPost.Deleted {
		t.Fatalf("ship: shipment %q invoice %q deleted %v", salesPost.ShipmentNo, salesPost.InvoiceNo, salesPost.Deleted)
	}
	shipment1 := salesPost.ShipmentNo
	qty, invoiced := itemEntries(t, src, shipment1)
	wantDec(t, "shipped item quantity", qty, dec("-4"))
	wantDec(t, "shipped item invoiced", invoiced, types.Decimal{})
	itemLine = getLine(t, order, 10000)
	wantDec(t, "Quantity Shipped", itemLine.Quantity_shipped, dec("4"))
	wantDec(t, "Outstanding Quantity", itemLine.Outstanding_quantity, dec("6"))
	wantDec(t, "Qty. Shipped Not Invoiced", itemLine.Qty_shipped_not_invoiced, dec("4"))
	wantDec(t, "Qty. to Ship", itemLine.Qty_to_ship, dec("6"))
	shptLine := gtables.NewSalesShipmentLine(src)
	must(t, shptLine.GetOrError(map[string]interface{}{"document_no": shipment1, "line_no": 10000}))
	wantDec(t, "shipment line quantity", shptLine.Quantity, dec("4"))
	wantDec(t, "shipment line not invoiced", shptLine.Qty_shipped_not_invoiced, dec("4"))
	if shptLine.Order_no != order.No || shptLine.Item_shpt_entry_no == 0 {
		t.Errorf("shipment line order %s, item entry %d", shptLine.Order_no, shptLine.Item_shpt_entry_no)
	}
	if !reread(t, order) || order.Last_shipping_no != shipment1 || order.Ship {
		t.Errorf("order after shipping: last shipping no. %s, ship %v", order.Last_shipping_no, order.Ship)
	}

	// Invoice what is shipped
	salesPost = post(t, order, false, true)
	invoice1 := salesPost.InvoiceNo
	if invoice1 == "" || salesPost.ShipmentNo != "" || salesPost.Deleted {
		t.Fatalf("invoice: %q %q %v", invoice1, salesPost.ShipmentNo, salesPost.Deleted)
	}
	_, invoiced = itemEntries(t, src, invoice1)
	wantDec(t, "invoiced item quantity", invoiced, dec("-4"))
	must(t, shptLine.GetOrError(map[string]interface{}{"document_no": shipment1, "line_no": 10000}))
	wantDec(t, "shipment line invoiced", shptLine.Quantity_invoiced, dec("4"))
	wantDec(t, "shipment line not invoiced after", shptLine.Qty_shipped_not_invoiced, types.Decimal{})
	resLedgEntry := gtables.NewResLedgerEntry(src)
	resLedgEntry.SetRange("document_no", invoice1)
	if !resLedgEntry.FindFirst() {
		t.Fatal("no resource entry")
	}
	wantDec(t, "resource sale quantity", resLedgEntry.Quantity, dec("-3"))
	invHeader := gtables.NewSalesInvoiceHeader(src)
	must(t, invHeader.GetOrError(invoice1))
	invHeader.CalcFields("amount_including_tax")
	entry := custEntry(t, src, invoice1)
	entry.CalcFields("amount")
	wantDec(t, "customer entry", entry.Amount, invHeader.Amount_including_tax)
	glBalance(t, src, invoice1)
	if invHeader.Order_no != order.No || invHeader.Sell_to_customer_no != "C00010" {
		t.Errorf("posted invoice order %s customer %s", invHeader.Order_no, invHeader.Sell_to_customer_no)
	}
	itemLine = getLine(t, order, 10000)
	wantDec(t, "Quantity Invoiced", itemLine.Quantity_invoiced, dec("4"))

	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C00010")))
	cust.CalcFields("outstanding_orders", "shipped_not_invoiced")
	if !cust.Outstanding_orders.IsPositive() || !cust.Shipped_not_invoiced.IsZero() {
		t.Errorf("customer outstanding %s, shipped not invoiced %s", cust.Outstanding_orders, cust.Shipped_not_invoiced)
	}

	// Ship and invoice the rest: the order is gone
	salesPost = post(t, order, true, true)
	if !salesPost.Deleted || salesPost.ShipmentNo == "" || salesPost.InvoiceNo == "" {
		t.Fatalf("rest: %+v", salesPost)
	}
	qty, invoiced = itemEntries(t, src, salesPost.InvoiceNo)
	wantDec(t, "rest item quantity", qty, dec("-6"))
	wantDec(t, "rest item invoiced", invoiced, dec("-6"))
	glBalance(t, src, salesPost.InvoiceNo)
	if reread(t, order) {
		t.Error("the order is still there")
	}
	lines := gtables.NewSalesLine(src)
	lines.SetRange("document_no", order.No)
	if lines.Count() != 0 {
		t.Error("the order's lines are still there")
	}
	invoiceTotal := custEntry(t, src, invoice1)
	invoiceTotal.CalcFields("amount")
	second := custEntry(t, src, salesPost.InvoiceNo)
	second.CalcFields("amount")
	wantDec(t, "both invoices", invoiceTotal.Amount.Add(second.Amount), orderTotal)
	cust.CalcFields("outstanding_orders")
	wantDec(t, "outstanding orders at the end", cust.Outstanding_orders, types.Decimal{})
}

// An invoice with an item and a G/L account line, then a credit memo applied to it
func TestPostInvoiceAndCreditMemo(t *testing.T) {
	src := demoCompany(t)
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "2")
	addLine(t, invoice, 20000, gtables.SalesLine_Type.GLAccount, "3110", "1")
	glLine := getLine(t, invoice, 20000)
	must(t, glLine.Validate("unit_price", dec("100")))
	must(t, glLine.ModifyOrError(true))
	salesPost := post(t, invoice, false, false)
	if !salesPost.Deleted || salesPost.InvoiceNo == "" || salesPost.ShipmentNo != "" {
		t.Fatalf("invoice: %+v", salesPost)
	}
	postedInvoice := salesPost.InvoiceNo
	qty, invoiced := itemEntries(t, src, postedInvoice)
	wantDec(t, "invoice item quantity", qty, dec("-2"))
	wantDec(t, "invoice item invoiced", invoiced, dec("-2"))
	glBalance(t, src, postedInvoice)
	invLines := gtables.NewSalesInvoiceLine(src)
	invLines.SetRange("document_no", postedInvoice)
	if invLines.Count() != 2 {
		t.Errorf("posted invoice lines %d", invLines.Count())
	}
	invEntry := custEntry(t, src, postedInvoice)
	invEntry.CalcFields("amount", "remaining_amount")

	creditMemo := newDocument(t, src, gtables.SalesHeader_Document_type.CreditMemo, "C00020")
	addLine(t, creditMemo, 10000, gtables.SalesLine_Type.Item, "1000", "1")
	creditMemo.Applies_to_doc_type = gtables.SalesHeader_Applies_to_doc_type.Invoice
	must(t, creditMemo.ModifyOrError(true))
	must(t, creditMemo.Validate("applies_to_doc_no", postedInvoice))
	must(t, creditMemo.ModifyOrError(true))
	if !reread(t, creditMemo) {
		t.Fatal("credit memo gone")
	}
	salesPost = post(t, creditMemo, false, false)
	if !salesPost.Deleted || salesPost.CrMemoNo == "" {
		t.Fatalf("credit memo: %+v", salesPost)
	}
	qty, invoiced = itemEntries(t, src, salesPost.CrMemoNo)
	wantDec(t, "credit memo item quantity", qty, dec("1"))
	wantDec(t, "credit memo item invoiced", invoiced, dec("1"))
	glBalance(t, src, salesPost.CrMemoNo)
	cmEntry := custEntry(t, src, salesPost.CrMemoNo)
	cmEntry.CalcFields("amount", "remaining_amount")
	if !cmEntry.Amount.IsNegative() || cmEntry.Open {
		t.Errorf("credit memo entry amount %s open %v", cmEntry.Amount, cmEntry.Open)
	}
	invEntry = custEntry(t, src, postedInvoice)
	invEntry.CalcFields("amount", "remaining_amount")
	wantDec(t, "invoice remaining after the credit memo", invEntry.Remaining_amount, invEntry.Amount.Add(cmEntry.Amount))

	// Nothing to post; a quote cannot be posted
	empty := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	err := salesposting.NewSalesPost(src).Run(empty)
	if err == nil || !strings.Contains(err.Error(), "nothing to post") {
		t.Errorf("empty invoice: %v", err)
	}
	quote := newDocument(t, src, gtables.SalesHeader_Document_type.Quote, "C00020")
	addLine(t, quote, 10000, gtables.SalesLine_Type.Item, "1000", "1")
	err = salesposting.NewSalesPost(src).Run(quote)
	if err == nil || !strings.Contains(err.Error(), "cannot be posted") {
		t.Errorf("quote: %v", err)
	}
	order := newDocument(t, src, gtables.SalesHeader_Document_type.Order, "C00020")
	addLine(t, order, 10000, gtables.SalesLine_Type.Item, "1000", "1")
	err = salesposting.NewSalesPost(src).Run(order)
	if err == nil || !strings.Contains(err.Error(), "Ship") {
		t.Errorf("order without ship/invoice: %v", err)
	}
}

// Invoice rounding: the total is rounded to the precision, the difference goes to the
// customer posting group's rounding account and is a line of the posted invoice
func TestPostInvoiceRounding(t *testing.T) {
	src := demoCompany(t)
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Invoice_rounding = true
	must(t, salesSetup.ModifyOrError(false))
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Inv_rounding_precision_lcy = dec("1")
	must(t, glSetup.ModifyOrError(false))
	cust := gtables.NewCustomer(src)
	must(t, cust.GetOrError(types.Code("C00030")))
	custPostingGr := gtables.NewCustomerPostingGroup(src)
	must(t, custPostingGr.GetOrError(cust.Customer_posting_group))
	custPostingGr.Invoice_rounding_account = "3110"
	must(t, custPostingGr.ModifyOrError(false))

	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00030")
	line := addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "3")
	if line.Amount_including_tax.Equal(line.Amount_including_tax.Round(0)) {
		t.Fatalf("the test needs a total with cents: %s", line.Amount_including_tax)
	}
	salesPost := post(t, invoice, false, false)
	entry := custEntry(t, src, salesPost.InvoiceNo)
	entry.CalcFields("amount")
	if !entry.Amount.Equal(entry.Amount.Round(0)) {
		t.Errorf("customer entry %s not rounded", entry.Amount)
	}
	wantDec(t, "rounded total", entry.Amount, line.Amount_including_tax.Round(0))
	glBalance(t, src, salesPost.InvoiceNo)
	invLines := gtables.NewSalesInvoiceLine(src)
	invLines.SetRange("document_no", salesPost.InvoiceNo)
	if invLines.Count() != 2 {
		t.Errorf("posted invoice lines %d, want the item and the rounding", invLines.Count())
	}
	invHeader := gtables.NewSalesInvoiceHeader(src)
	must(t, invHeader.GetOrError(salesPost.InvoiceNo))
	invHeader.CalcFields("amount_including_tax")
	wantDec(t, "posted invoice total", invHeader.Amount_including_tax, entry.Amount)
}

// History: the posted invoice and shipment printed (No. Printed), an invoice made from the
// order's shipment with Get Shipment Lines and posted (the shipment's item entry invoiced, the
// order line and shipment line updated), Navigate finding the posted documents
func TestPostedDocumentsHistory(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.SalesHeader_Document_type.Order, "C00010")
	addLine(t, order, 10000, gtables.SalesLine_Type.Item, "1000", "5")
	shipment := post(t, order, true, false).ShipmentNo

	// Get Shipment Lines onto an invoice, post it
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00010")
	must(t, salesgetshipment.CreateInvLines(invoice, shipment, "Shipment No. %1:"))
	invLine := getLine(t, invoice, 20000)
	if invLine.Shipment_no != shipment || !invLine.Quantity.Equal(dec("5")) {
		t.Fatalf("invoice line from the shipment: %s %s", invLine.Shipment_no, invLine.Quantity)
	}
	if getLine(t, invoice, 10000).Description != types.Text("Shipment No. "+shipment.String()+":") {
		t.Errorf("description line %q", getLine(t, invoice, 10000).Description)
	}
	salesPost := post(t, invoice, false, false)
	if salesPost.ShipmentNo != "" {
		t.Errorf("the invoice shipped again: %s", salesPost.ShipmentNo)
	}
	qty, invoiced := itemEntries(t, src, salesPost.InvoiceNo)
	wantDec(t, "invoice item quantity", qty, types.Decimal{})
	wantDec(t, "invoice item invoiced", invoiced, dec("-5"))
	glBalance(t, src, salesPost.InvoiceNo)
	shptLine := gtables.NewSalesShipmentLine(src)
	must(t, shptLine.GetOrError(map[string]interface{}{"document_no": shipment, "line_no": 10000}))
	wantDec(t, "shipment line not invoiced", shptLine.Qty_shipped_not_invoiced, types.Decimal{})
	orderLine := getLine(t, order, 10000)
	wantDec(t, "order line invoiced", orderLine.Quantity_invoiced, dec("5"))
	wantDec(t, "order line shipped not invoiced", orderLine.Qty_shipped_not_invoiced, types.Decimal{})

	// The printouts count in No. Printed
	for _, c := range []struct {
		reportID int
		no       types.Code
	}{{reports.SalesInvoiceID, salesPost.InvoiceNo}, {reports.SalesShipmentID, shipment}} {
		var context interface{}
		if c.reportID == reports.SalesInvoiceID {
			h := gtables.NewSalesInvoiceHeader(src)
			h.No = c.no
			context = h
		} else {
			h := gtables.NewSalesShipmentHeader(src)
			h.No = c.no
			context = h
		}
		result, err := reports.Execute(c.reportID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Context: context})
		must(t, err)
		var texts []string
		for _, text := range result.Texts {
			texts = append(texts, text.Text)
		}
		joined := "|" + strings.Join(texts, "|") + "|"
		if !strings.Contains(joined, "|"+c.no.String()+"|") || !strings.Contains(joined, "|Fjordkraft Interiør AS|") {
			t.Errorf("report %d: %s", c.reportID, joined)
		}
	}
	invHeader := gtables.NewSalesInvoiceHeader(src)
	must(t, invHeader.GetOrError(salesPost.InvoiceNo))
	shptHeader := gtables.NewSalesShipmentHeader(src)
	must(t, shptHeader.GetOrError(shipment))
	if invHeader.No_printed != 1 || shptHeader.No_printed != 1 {
		t.Errorf("No. Printed: invoice %d, shipment %d", invHeader.No_printed, shptHeader.No_printed)
	}

	// Navigate finds the posted invoice and its entries
	docEntry := gtables.NewDocumentEntry(src)
	must(t, docEntry.InitTemporary(src.GetDB(), src.GetCompany(), src.GetDBType()))
	must(t, navigate.FindRecords(docEntry, salesPost.InvoiceNo.String(), ""))
	found := map[int]string{}
	for ok := docEntry.FindSet(); ok; ok = docEntry.Next() {
		found[docEntry.Table_id] = docEntry.No_filter.String() + "/" + docEntry.Document_no_filter.String()
	}
	if found[gtables.SalesInvoiceHeaderTableID] != salesPost.InvoiceNo.String()+"/" || found[gtables.CustomerLedgerEntryTableID] != "/"+salesPost.InvoiceNo.String() {
		t.Errorf("navigate: %v", found)
	}
}

// A shipment invoiced later is still the order line's: an order line whose bin changed after
// the shipment is not invoiced from it
func TestInvoiceChecksShipmentLine(t *testing.T) {
	src := demoCompany(t)
	order := newDocument(t, src, gtables.SalesHeader_Document_type.Order, "C00010")
	line := addLine(t, order, 10000, gtables.SalesLine_Type.Item, "1000", "2")
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.ModifyOrError(true))
	reread(t, order)
	post(t, order, true, false)

	line = getLine(t, order, 10000)
	must(t, line.Validate("bin_code", types.Code("A-01")))
	must(t, line.ModifyOrError(true))
	reread(t, order)
	order.Ship, order.Invoice = false, true
	err := salesposting.NewSalesPost(order).Run(order)
	if err == nil || !strings.Contains(err.Error(), "Bin Code") {
		t.Errorf("invoiced from a shipment of another bin: %v", err)
	}
}
