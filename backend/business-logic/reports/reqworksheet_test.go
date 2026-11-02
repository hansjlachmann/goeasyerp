package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/reqwksh"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// openWorksheet opens the requisition worksheet page as the page does (OnOpenPage): its
// template and worksheet as the context line's filters (filter group 2)
func openWorksheet(t *testing.T, src ftables.Source, recurring bool) *gtables.RequisitionLine {
	t.Helper()
	reqLine := gtables.NewRequisitionLine(src)
	page := pages.NewOpen(nil)
	mustOK(t, reqwksh.OpenJournalPage(reqLine, page, recurring))
	reqLine.FilterGroup(2)
	for _, f := range page.Filters() {
		reqLine.SetRange(f.Field, f.Value)
	}
	reqLine.FilterGroup(0)
	reqLine.Worksheet_template_name = types.Code(page.Param("template"))
	reqLine.Journal_batch_name = types.Code(page.Param("batch"))
	return reqLine
}

func addReqLine(t *testing.T, worksheet *gtables.RequisitionLine, lineNo int, item types.Code, qty string) *gtables.RequisitionLine {
	t.Helper()
	reqLine := gtables.NewRequisitionLine(worksheet)
	reqLine.Worksheet_template_name = worksheet.Worksheet_template_name
	reqLine.Journal_batch_name = worksheet.Journal_batch_name
	reqLine.Line_no = lineNo
	mustOK(t, reqLine.Validate("type", gtables.RequisitionLine_Type.Item))
	mustOK(t, reqLine.Validate("no", item))
	mustOK(t, reqLine.Validate("quantity", types.MustDecimal(qty)))
	mustOK(t, reqLine.InsertOrError(true))
	return reqLine
}

func runOnWorksheet(t *testing.T, src ftables.Source, id int, worksheet *gtables.RequisitionLine, options map[string]string) reports.Result {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Context: worksheet, Options: options})
	mustOK(t, err)
	return result
}

// The worksheet opens on a new REQ. template and DEFAULT worksheet; an item line takes the
// item's vendor, purchase unit and the vendor's cost; Make Orders makes one purchase order per
// vendor with the lines and empties the worksheet
func TestReqWorksheetMakeOrders(t *testing.T) {
	src := listCompany(t)
	worksheet := openWorksheet(t, src, false)
	if worksheet.Worksheet_template_name != "REQ." || worksheet.Journal_batch_name != "DEFAULT" {
		t.Fatalf("worksheet %s/%s, want REQ./DEFAULT", worksheet.Worksheet_template_name, worksheet.Journal_batch_name)
	}
	reqTemplate := gtables.NewReqWkshTemplate(src)
	mustOK(t, reqTemplate.GetOrError(types.Code("REQ.")))
	if reqTemplate.Page_id != 291 || reqTemplate.Recurring {
		t.Errorf("template page %d recurring %v, want 291 and not recurring", reqTemplate.Page_id, reqTemplate.Recurring)
	}

	item := gtables.NewItem(src)
	mustOK(t, item.GetOrError(types.Code("1000")))
	paper := addReqLine(t, worksheet, 10000, "1000", "20")
	if paper.Vendor_no != item.Vendor_no || paper.Unit_of_measure_code != item.Purch_unit_of_measure || paper.Description != item.Description || paper.Direct_unit_cost.IsZero() {
		t.Errorf("line of 1000: vendor %s, unit %s, %q, cost %s", paper.Vendor_no, paper.Unit_of_measure_code, paper.Description, paper.Direct_unit_cost)
	}
	addReqLine(t, worksheet, 20000, "1020", "5")
	other := gtables.NewItem(src)
	other.SetFilter("vendor_no", "<>"+item.Vendor_no.String())
	found := other.FindSet()
	for found && other.Vendor_no == "" {
		found = other.Next()
	}
	if !found {
		t.Fatal("no item of another vendor")
	}
	addReqLine(t, worksheet, 30000, other.No, "3")

	ordersBefore := gtables.NewPurchaseHeader(src)
	ordersBefore.SetRange("document_type", gtables.PurchaseHeader_Document_type.Order)
	before := ordersBefore.Count()

	result := runOnWorksheet(t, src, reports.MakeOrdersID, worksheet, map[string]string{"order_date": "2026-10-06", "posting_date": "2026-10-06",
		"expected_receipt_date": "2026-10-13", "your_reference": "REQ-1"})
	if !strings.Contains(result.Message, "reports.493.texts.ORDERS_CREATED") && !strings.Contains(result.Message, "created") {
		t.Errorf("message %q", result.Message)
	}
	orders := gtables.NewPurchaseHeader(src)
	orders.SetRange("document_type", gtables.PurchaseHeader_Document_type.Order)
	orders.SetRange("your_reference", types.Text("REQ-1"))
	if n := orders.Count(); n != 2 || ordersBefore.Count() != before+2 {
		for found := orders.FindSet(); found; found = orders.Next() {
			t.Logf("order %s vendor %s", orders.No, orders.Buy_from_vendor_no)
		}
		t.Fatalf("%d orders with the reference, want 2 (one per vendor; %d orders before, %d now; other item %s of %s)", n, before, ordersBefore.Count(), other.No, other.Vendor_no)
	}
	orders.SetRange("buy_from_vendor_no", item.Vendor_no)
	if !orders.FindFirst() {
		t.Fatal("no order of the paper's vendor")
	}
	purchLine := gtables.NewPurchaseLine(src)
	purchLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchLine.SetRange("document_no", orders.No)
	if n := purchLine.Count(); n != 2 {
		t.Errorf("the vendor's order has %d lines, want 2", n)
	}
	purchLine.SetRange("no", types.Code("1000"))
	if !purchLine.FindFirst() || !purchLine.Quantity.Equal(types.MustDecimal("20")) || !purchLine.Direct_unit_cost.Equal(paper.Direct_unit_cost) ||
		paper.Expected_receipt_date.IsZero() || purchLine.Expected_receipt_date != paper.Expected_receipt_date {
		t.Errorf("order line of 1000: qty %s cost %s receipt %s", purchLine.Quantity, purchLine.Direct_unit_cost, purchLine.Expected_receipt_date)
	}

	// (the line's expected receipt date by the item's lead time, the order's from the request)
	if orders.Expected_receipt_date != types.NewDate(2026, 10, 13) {
		t.Errorf("order's expected receipt date %s, want 2026-10-13", orders.Expected_receipt_date)
	}

	// the worksheet's lines are gone, a new empty first line waits
	left := gtables.NewRequisitionLine(src)
	left.SetRange("worksheet_template_name", types.Code("REQ."))
	left.SetRange("journal_batch_name", types.Code("DEFAULT"))
	if n := left.Count(); n != 1 || !left.FindFirst() || left.No != "" {
		t.Errorf("%d lines left (no. %q), want the new empty one", n, left.No)
	}

	// nothing to make: no orders
	runOnWorksheet(t, src, reports.MakeOrdersID, worksheet, nil)
	if ordersBefore.Count() != before+2 {
		t.Errorf("orders made from an empty worksheet")
	}
}

// A recurring worksheet's line stays after Make Orders, its order date moved on by its frequency;
// a line without its recurring method is refused
func TestRecurringReqWorksheet(t *testing.T) {
	src := listCompany(t)
	worksheet := openWorksheet(t, src, true)
	if worksheet.Worksheet_template_name != "RECURRING" {
		t.Fatalf("recurring template %s", worksheet.Worksheet_template_name)
	}
	reqLine := gtables.NewRequisitionLine(src)
	reqLine.Worksheet_template_name = worksheet.Worksheet_template_name
	reqLine.Journal_batch_name = worksheet.Journal_batch_name
	reqLine.Line_no = 10000
	mustOK(t, reqLine.Validate("order_date", system.WorkDate()))
	mustOK(t, reqLine.Validate("type", gtables.RequisitionLine_Type.Item))
	mustOK(t, reqLine.Validate("no", types.Code("1000")))
	mustOK(t, reqLine.Validate("quantity", types.MustDecimal("7")))
	mustOK(t, reqLine.InsertOrError(true))

	_, err := reports.Execute(reports.MakeOrdersID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Context: worksheet})
	if err == nil {
		t.Fatal("a recurring line without Recurring Method was made into an order")
	}

	reqLine.Recurring_method = gtables.RequisitionLine_Recurring_method.Fixed
	reqLine.Recurring_frequency = "1M"
	mustOK(t, reqLine.ModifyOrError(true))
	orderDate := reqLine.Order_date
	runOnWorksheet(t, src, reports.MakeOrdersID, worksheet, map[string]string{"end_order_date": system.WorkDate().String()})
	mustOK(t, reqLine.GetOrError(map[string]interface{}{"worksheet_template_name": reqLine.Worksheet_template_name, "journal_batch_name": reqLine.Journal_batch_name, "line_no": 10000}))
	if reqLine.Order_date != orderDate.AddMonths(1) || !reqLine.Quantity.Equal(types.MustDecimal("7")) {
		t.Errorf("recurring line after Make Orders: order date %s (was %s), qty %s", reqLine.Order_date, orderDate, reqLine.Quantity)
	}
}

// Reorder Inventory Items: an item available below its reorder point gets a line — rounded up to
// its Reorder Quantity, or (Maximum Qty. method) up to its Maximum Inventory
func TestReorderInventoryItems(t *testing.T) {
	src := listCompany(t)
	worksheet := openWorksheet(t, src, false)
	available := func(no types.Code) types.Decimal {
		item := gtables.NewItem(src)
		mustOK(t, item.GetOrError(no))
		item.CalcFields("quantity_on_hand", "qty_on_purch_order", "qty_on_sales_order")
		return item.Quantity_on_hand.Add(item.Qty_on_purch_order).Sub(item.Qty_on_sales_order)
	}
	qty1000 := available("1000")
	qty1020 := available("1020")
	reqMethod := gtables.NewRequisitionMethod(src)
	reqMethod.Code, reqMethod.Reordering_policy, reqMethod.Calc_below_reorder_point = "MAXQTY", gtables.RequisitionMethod_Reordering_policy.MaximumQty, true
	if !reqMethod.Insert(false) {
		mustOK(t, reqMethod.ModifyOrError(false))
	}
	for _, i := range []struct {
		no                      types.Code
		reorderPoint, reorderQt types.Decimal
		maximum                 types.Decimal
		method                  types.Code
	}{
		{"1000", qty1000.Add(types.MustDecimal("5")), types.MustDecimal("4"), types.Decimal{}, ""},
		{"1020", qty1020.Add(types.MustDecimal("1")), types.Decimal{}, qty1020.Add(types.MustDecimal("30")), "MAXQTY"},
	} {
		item := gtables.NewItem(src)
		mustOK(t, item.GetOrError(i.no))
		item.Reorder_point, item.Reorder_quantity, item.Maximum_inventory, item.Requisition_method_code = i.reorderPoint, i.reorderQt, i.maximum, i.method
		mustOK(t, item.ModifyOrError(false))
	}
	runOnWorksheet(t, src, reports.ReorderInventoryItemsID, worksheet, map[string]string{"ending_date": "2026-12-31"})
	got := map[string]string{}
	reqLine := gtables.NewRequisitionLine(src)
	reqLine.SetRange("worksheet_template_name", worksheet.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", worksheet.Journal_batch_name)
	for found := reqLine.FindSet(); found; found = reqLine.Next() {
		got[reqLine.No.String()] = reqLine.Quantity_base.String()
	}
	// 1000: 5 below the point, by 4 → 8; 1020: up to the maximum, 30
	if got["1000"] != "8" || got["1020"] != "30" {
		t.Errorf("reorder lines (base quantities) = %v, want 1000: 8, 1020: 30", got)
	}
}
