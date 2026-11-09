package reports_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// documentsCompany: the demo data with the sales, purchases and fixed assets posted, in the
// database itself (the reports open transactions of their own)
func documentsCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:       func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
		PurchPost:       func(src ftables.Source) demodata.PurchPoster { return purchposting.NewPurchPost(src) },
		GenJnlPostLine:  func(src ftables.Source) demodata.GenJnlPoster { return genjnl.NewGenJnlPostLine(src) },
	})
	mustOK(t, err)
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

func runBatch(t *testing.T, src ftables.Source, id int, options map[string]string, filters map[string][]ftables.FieldFilter) reports.Result {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options, Filters: filters})
	if err != nil {
		t.Fatalf("report %d: %v", id, err)
	}
	return result
}

// Combine Shipments (295) makes one invoice of a customer's shipped orders marked Combine
// Shipments and posts it; the orders, now invoiced completely, are deleted by Delete Invoiced
// Sales Orders (299), which leaves the open orders alone
func TestCombineShipmentsAndDeleteInvoicedOrders(t *testing.T) {
	reports.SalesPost = func(src ftables.Source, salesHeader *gtables.SalesHeader) error {
		return salesposting.NewSalesPost(src).Run(salesHeader)
	}
	src := documentsCompany(t)
	day := types.NewDateFromTime(time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local))

	cust := gtables.NewCustomer(src)
	mustOK(t, cust.GetOrError(types.Code("C00010")))
	cust.Combine_shipments = true
	mustOK(t, cust.ModifyOrError(false))

	openOrders := gtables.NewSalesHeader(src)
	openOrders.SetRange("document_type", gtables.SalesHeader_Document_type.Order)
	ordersBefore := openOrders.Count()

	var orderNos []types.Code
	for i := 0; i < 2; i++ {
		order := gtables.NewSalesHeader(src)
		order.Document_type = gtables.SalesHeader_Document_type.Order
		mustOK(t, order.InsertOrError(true))
		mustOK(t, order.Validate("sell_to_customer_no", cust.No))
		mustOK(t, order.Validate("posting_date", day))
		mustOK(t, order.Validate("location_code", types.Code("MAIN")))
		mustOK(t, order.ModifyOrError(true))
		if !order.Combine_shipments {
			t.Fatal("the order did not take Combine Shipments from the customer")
		}
		line := gtables.NewSalesLine(src)
		line.Document_type = gtables.SalesLine_Document_type.Order
		line.Document_no = order.No
		line.Line_no = 10000
		mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
		mustOK(t, line.Validate("no", types.Code("1000")))
		mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(int64(2+i))))
		mustOK(t, line.InsertOrError(true))

		tx, err := ftables.BeginTransaction(src.GetDB())
		mustOK(t, err)
		txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
		toPost := gtables.NewSalesHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": int(order.Document_type), "no": order.No}))
		toPost.Ship = true
		toPost.Invoice = false
		mustOK(t, salesposting.NewSalesPost(txSrc).Run(toPost))
		mustOK(t, ftables.Commit(tx))
		orderNos = append(orderNos, order.No)
	}

	result := runBatch(t, src, reports.CombineShipmentsID, map[string]string{
		"posting_date": day.String(), "document_date": day.String(), "post_invoices": "true"},
		map[string][]ftables.FieldFilter{"SalesOrderHeader": {{Field: "bill_to_customer_no", Expression: cust.No.String()}}})
	if !strings.Contains(result.Message, "combined") {
		t.Errorf("message: %q", result.Message)
	}
	invoice := gtables.NewSalesInvoiceHeader(src)
	invoice.SetRange("bill_to_customer_no", cust.No)
	invoice.SetRange("posting_date", day)
	if invoice.Count() != 1 {
		t.Fatalf("posted invoices of the combined shipments: %d, want 1", invoice.Count())
	}
	invoice.FindFirst()
	invLine := gtables.NewSalesInvoiceLine(src)
	invLine.SetRange("document_no", invoice.No)
	invLine.SetRange("type", gtables.SalesInvoiceLine_Type.Item)
	if invLine.Count() != 2 {
		t.Errorf("invoice lines: %d, want 2 (one per shipment)", invLine.Count())
	}
	for _, orderNo := range orderNos {
		orderLine := gtables.NewSalesLine(src)
		mustOK(t, orderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order), "document_no": orderNo, "line_no": 10000}))
		if !orderLine.Quantity_invoiced.Equal(orderLine.Quantity) {
			t.Errorf("order %s: invoiced %s of %s", orderNo, orderLine.Quantity_invoiced, orderLine.Quantity)
		}
	}

	runBatch(t, src, reports.CombineShipmentsID, map[string]string{"posting_date": day.String(), "document_date": day.String()}, nil)

	runBatch(t, src, reports.DeleteInvoicedSalesOrdersID, nil, nil)
	for _, orderNo := range orderNos {
		order := gtables.NewSalesHeader(src)
		if order.Get(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": orderNo}) {
			t.Errorf("invoiced order %s was not deleted", orderNo)
		}
		lines := gtables.NewSalesLine(src)
		lines.SetRange("document_no", orderNo)
		if lines.Count() != 0 {
			t.Errorf("order %s: lines left", orderNo)
		}
	}
	if after := openOrders.Count(); after != ordersBefore {
		t.Errorf("orders: %d before, %d after (open orders deleted)", ordersBefore, after)
	}

	// The blanket orders still have outstanding quantities: none deleted
	blanket := gtables.NewSalesHeader(src)
	blanket.SetRange("document_type", gtables.SalesHeader_Document_type.BlanketOrder)
	blanketsBefore := blanket.Count()
	runBatch(t, src, reports.DeleteInvdBlnktSalesOrdersID, nil, nil)
	purchOrders := gtables.NewPurchaseHeader(src)
	purchOrders.SetRange("document_type", gtables.PurchaseHeader_Document_type.Order)
	purchBefore := purchOrders.Count()
	runBatch(t, src, reports.DeleteInvoicedPurchOrdersID, nil, nil)
	runBatch(t, src, reports.DeleteInvdBlnktPurchOrdersID, nil, nil)
	if blanket.Count() != blanketsBefore || blanketsBefore == 0 || purchOrders.Count() != purchBefore {
		t.Errorf("open documents deleted: blanket %d → %d, purchase orders %d → %d", blanketsBefore, blanket.Count(), purchBefore, purchOrders.Count())
	}
}

// Index Fixed Assets (5690) puts the indexed acquisition cost of the assets in a journal of the
// book; Index Insurance (5691) the indexed coverage in the insurance journal
func TestIndexFixedAssetsAndInsurance(t *testing.T) {
	src := documentsCompany(t)
	day := "2026-10-06"
	faSetup := gtables.GetFASetup(src)
	deprBook := gtables.NewDepreciationBook(src)
	mustOK(t, deprBook.GetOrError(faSetup.Default_depr_book))
	// the book's journals (the demo journals' FA Journal Setup)
	tpl := gtables.NewGenJournalTemplate(src)
	if !tpl.Get(types.Code("ASSETS")) {
		tpl.Name = "ASSETS"
		mustOK(t, tpl.Validate("type", gtables.GenJournalTemplate_Type.Assets))
		mustOK(t, tpl.InsertOrError(true))
	}
	batch := gtables.NewGenJournalBatch(src)
	if !batch.Get(tables.GenJnlBatchKey("ASSETS", "DEFAULT")) {
		batch.Journal_template_name = "ASSETS"
		batch.Name = "DEFAULT"
		mustOK(t, batch.InsertOrError(true))
	}
	faJnlSetup := gtables.NewFAJournalSetup(src)
	faJnlSetup.Depreciation_book_code = deprBook.Code
	faJnlSetup.Fa_jnl_template_name, faJnlSetup.Fa_jnl_batch_name = "ASSETS", "DEFAULT"
	faJnlSetup.Gen_jnl_template_name, faJnlSetup.Gen_jnl_batch_name = "ASSETS", "DEFAULT"
	faJnlSetup.Insurance_jnl_template_name, faJnlSetup.Insurance_jnl_batch_name = "INSURANCE", "DEFAULT"
	mustOK(t, faJnlSetup.InsertOrError(true))

	_, err := reports.Execute(reports.IndexFixedAssetsID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"depreciation_book": deprBook.Code.String(), "index_figure": "110", "fa_posting_date": day, "document_no": "IDX1", "acquisition_cost": "true"}})
	if err == nil || !strings.Contains(err.Error(), "Allow Indexation") {
		t.Fatalf("a book without Allow Indexation: %v", err)
	}
	deprBook.Allow_indexation = true
	mustOK(t, deprBook.ModifyOrError(false))
	runBatch(t, src, reports.IndexFixedAssetsID, map[string]string{
		"depreciation_book": deprBook.Code.String(), "index_figure": "110", "fa_posting_date": day, "document_no": "IDX1", "acquisition_cost": "true"}, nil)

	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.SetRange("index_entry", true)
	faJnlLine := gtables.NewFAJournalLine(src)
	faJnlLine.SetRange("index_entry", true)
	if genJnlLine.Count()+faJnlLine.Count() == 0 {
		t.Fatal("no index lines")
	}
	// one asset's line: 10 % of its acquisition cost up to the date
	checked := false
	for found := genJnlLine.FindSetBuffered(); found; found = genJnlLine.Next() {
		faLedgEntry := gtables.NewFALedgerEntry(src)
		faLedgEntry.SetRange("fa_no", genJnlLine.Account_no)
		faLedgEntry.SetRange("depreciation_book_code", deprBook.Code)
		faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
		faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost)
		mustOK(t, faLedgEntry.CalcSums("amount"))
		want := faLedgEntry.Amount.Mul(types.MustDecimal("0.1"))
		if genJnlLine.Amount.Sub(want).Abs().GreaterThan(types.NewDecimalFromInt(1)) || genJnlLine.Document_no != "IDX1" ||
			genJnlLine.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.AcquisitionCost {
			t.Errorf("index line of %s: %s %s %v, want %s", genJnlLine.Account_no, genJnlLine.Amount, genJnlLine.Document_no, genJnlLine.Fa_posting_type, want)
		}
		checked = true
	}
	for found := faJnlLine.FindSetBuffered(); found; found = faJnlLine.Next() {
		if faJnlLine.Document_no != "IDX1" || faJnlLine.Amount.IsZero() {
			t.Errorf("FA index line: %+v", faJnlLine.ToMap())
		}
		checked = true
	}
	if !checked {
		t.Error("no index line checked")
	}

	runBatch(t, src, reports.IndexInsuranceID, map[string]string{"index_figure": "105", "posting_date": day, "document_no": "IDX2"}, nil)
	insJnlLine := gtables.NewInsuranceJournalLine(src)
	insJnlLine.SetRange("index_entry", true)
	if insJnlLine.Count() == 0 {
		t.Fatal("no insurance index lines")
	}
	for found := insJnlLine.FindSetBuffered(); found; found = insJnlLine.Next() {
		coverage := gtables.NewInsCoverageLedgerEntry(src)
		coverage.SetRange("fa_no", insJnlLine.Fa_no)
		coverage.SetRange("insurance_no", insJnlLine.Insurance_no)
		mustOK(t, coverage.CalcSums("amount"))
		want := coverage.Amount.Mul(types.MustDecimal("0.05"))
		if insJnlLine.Amount.Sub(want).Abs().GreaterThan(types.MustDecimal("0.01")) || insJnlLine.Document_no != "IDX2" {
			t.Errorf("insurance index line %s/%s: %s, want %s", insJnlLine.Fa_no, insJnlLine.Insurance_no, insJnlLine.Amount, want)
		}
	}
	_, err = reports.Execute(reports.IndexInsuranceID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"index_figure": "100", "posting_date": day, "document_no": "IDX3"}})
	if err == nil {
		t.Error("Index Figure 100 accepted")
	}
}

// The BOM reports print the items that are components: Where-Used List each with the bills of
// materials it is used in
func TestBOMReports(t *testing.T) {
	src := listCompany(t)
	bomComp := gtables.NewBOMComponent(src)
	bomComp.SetRange("type", gtables.BOMComponent_Type.Item)
	if !bomComp.FindFirst() {
		t.Fatal("no demo BOM component")
	}
	texts := runList(t, src, reports.WhereUsedListID, nil)
	for _, want := range []string{"Where-Used List", bomComp.No.String(), bomComp.Parent_item_no.String()} {
		if !strings.Contains(texts, want) {
			t.Errorf("Where-Used List does not print %q", want)
		}
	}
	// a bill of materials used in another one is a sub-assembly
	subAssembly := gtables.NewBOMComponent(src)
	subAssembly.Parent_item_no = "1000"
	subAssembly.Line_no = 10000
	mustOK(t, subAssembly.Validate("type", gtables.BOMComponent_Type.Item))
	mustOK(t, subAssembly.Validate("no", bomComp.Parent_item_no))
	mustOK(t, subAssembly.Validate("quantity_per", types.NewDecimalFromInt(1)))
	mustOK(t, subAssembly.InsertOrError(true))
	result, err := reports.Execute(reports.SubAssembliesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF})
	mustOK(t, err)
	texts = printedTexts(result, 0)
	if !strings.Contains(texts, "Sub-Assemblies") || !strings.Contains(texts, bomComp.Parent_item_no.String()) {
		t.Errorf("Sub-Assemblies does not print the sub-assembly %s: %s", bomComp.Parent_item_no, texts)
	}
}
