package reports_test

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchgetdropshpt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// dropShipSalesOrder: a sales order of customer C00010 with a drop shipment line of item 1000
func dropShipSalesOrder(t *testing.T, src ftables.Source, day types.Date, qty int64) *gtables.SalesHeader {
	t.Helper()
	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, order.Validate("posting_date", day))
	mustOK(t, order.Validate("location_code", types.Code("MAIN")))
	mustOK(t, order.ModifyOrError(true))
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Order
	line.Document_no = order.No
	line.Line_no = 10000
	mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
	mustOK(t, line.Validate("no", types.Code("1000")))
	mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(qty)))
	mustOK(t, line.Validate("drop_shipment", true))
	line.Shipment_date = day
	mustOK(t, line.InsertOrError(true))
	return order
}

// inTransaction runs a posting the way the API does
func inTransaction(t *testing.T, src ftables.Source, post func(txSrc ftables.Source) error) error {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	mustOK(t, err)
	err = post(ftables.DBSource(tx, src.GetCompany(), src.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}

func itemInventory(t *testing.T, src ftables.Source, itemNo string) types.Decimal {
	itemLedgEntry := gtables.NewItemLedgerEntry(src)
	itemLedgEntry.SetRange("item_no", types.Code(itemNo))
	mustOK(t, itemLedgEntry.CalcSums("quantity"))
	return itemLedgEntry.Quantity
}

// A drop shipment from the sales order: Get Sales Order puts the line on a purchase order of the
// customer; shipping the sales order receives the purchase order (receipt, item entries applied
// to each other, inventory unchanged); the purchase is invoiced after the sale
func TestDropShipmentFromSalesOrder(t *testing.T) {
	src := documentsCompany(t)
	day := types.NewDateFromTime(time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local))
	order := dropShipSalesOrder(t, src, day, 3)
	inventoryBefore := itemInventory(t, src, "1000")

	item := gtables.NewItem(src)
	mustOK(t, item.GetOrError(types.Code("1000")))
	purchOrder := gtables.NewPurchaseHeader(src)
	purchOrder.Document_type = gtables.PurchaseHeader_Document_type.Order
	mustOK(t, purchOrder.InsertOrError(true))
	mustOK(t, purchOrder.Validate("buy_from_vendor_no", item.Vendor_no))
	mustOK(t, purchOrder.Validate("posting_date", day))
	mustOK(t, purchOrder.Validate("sell_to_customer_no", types.Code("C00010")))
	if order.Ship_to_code != "" {
		mustOK(t, purchOrder.Validate("ship_to_code", order.Ship_to_code))
	}
	purchOrder.Vendor_invoice_no = "DROP-INV-1"
	mustOK(t, purchOrder.ModifyOrError(true))
	if purchOrder.Ship_to_name == "" {
		t.Error("the purchase order did not take the customer's address")
	}
	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		purchHeader := gtables.NewPurchaseHeader(txSrc)
		mustOK(t, purchHeader.GetOrError(map[string]interface{}{"document_type": 1, "no": purchOrder.No}))
		return purchgetdropshpt.Code(purchHeader, order.No)
	}))
	salesLine := gtables.NewSalesLine(src)
	mustOK(t, salesLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": order.No, "line_no": 10000}))
	if salesLine.Purchase_order_no != purchOrder.No || salesLine.Purch_order_line_no == 0 {
		t.Fatalf("sales line not linked: %s %d", salesLine.Purchase_order_no, salesLine.Purch_order_line_no)
	}
	purchLine := gtables.NewPurchaseLine(src)
	mustOK(t, purchLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": purchOrder.No, "line_no": salesLine.Purch_order_line_no}))
	if !purchLine.Drop_shipment || purchLine.Sales_order_no != order.No || !purchLine.Quantity.Equal(types.NewDecimalFromInt(3)) {
		t.Fatalf("purchase line: drop %v sales order %s qty %s", purchLine.Drop_shipment, purchLine.Sales_order_no, purchLine.Quantity)
	}

	// a linked line keeps its quantity and cannot be deleted
	err := salesLine.Validate("quantity", types.NewDecimalFromInt(5))
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.ErrSalesLineAssocPurchOrder {
		t.Errorf("changing the quantity of a linked line: %v", err)
	}
	mustOK(t, salesLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": order.No, "line_no": 10000}))
	if salesLine.Delete(true) {
		t.Error("a linked sales line was deleted")
	}

	// the sales order ships: the purchase order is received
	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		toPost := gtables.NewSalesHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": 1, "no": order.No}))
		toPost.Ship = true
		toPost.Invoice = false
		return salesposting.NewSalesPost(txSrc).Run(toPost)
	}))
	mustOK(t, purchLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": purchOrder.No, "line_no": purchLine.Line_no}))
	if !purchLine.Quantity_received.Equal(types.NewDecimalFromInt(3)) {
		t.Errorf("purchase line received %s, want 3", purchLine.Quantity_received)
	}
	rcptLine := gtables.NewPurchRcptLine(src)
	rcptLine.SetRange("order_no", purchOrder.No)
	if !rcptLine.FindFirst() || !rcptLine.Quantity.Equal(types.NewDecimalFromInt(3)) || !rcptLine.Drop_shipment || rcptLine.Item_rcpt_entry_no == 0 {
		t.Errorf("purchase receipt line: %s drop %v entry %d", rcptLine.Quantity, rcptLine.Drop_shipment, rcptLine.Item_rcpt_entry_no)
	}
	purchEntry := gtables.NewItemLedgerEntry(src)
	mustOK(t, purchEntry.GetOrError(rcptLine.Item_rcpt_entry_no))
	if !purchEntry.Drop_shipment || !purchEntry.Remaining_quantity.IsZero() {
		t.Errorf("purchase entry: drop %v remaining %s, want closed by the sale", purchEntry.Drop_shipment, purchEntry.Remaining_quantity)
	}
	if inventory := itemInventory(t, src, "1000"); !inventory.Equal(inventoryBefore) {
		t.Errorf("inventory %s after the drop shipment, want %s", inventory, inventoryBefore)
	}

	// the purchase cannot be invoiced before the sale
	err = inTransaction(t, src, func(txSrc ftables.Source) error {
		toPost := gtables.NewPurchaseHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": 1, "no": purchOrder.No}))
		toPost.Receive = false
		toPost.Invoice = true
		return purchposting.NewPurchPost(txSrc).Run(toPost)
	})
	if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.ErrDropShptInvoiceSalesFirst {
		t.Errorf("invoicing the purchase first: %v", err)
	}
	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		toPost := gtables.NewSalesHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": 1, "no": order.No}))
		toPost.Ship = false
		toPost.Invoice = true
		return salesposting.NewSalesPost(txSrc).Run(toPost)
	}))
	mustOK(t, purchLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": purchOrder.No, "line_no": purchLine.Line_no}))
	if purchLine.Sales_order_line_no != 0 {
		t.Errorf("purchase line still linked after the sale was invoiced")
	}
	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		toPost := gtables.NewPurchaseHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": 1, "no": purchOrder.No}))
		toPost.Receive = false
		toPost.Invoice = true
		return purchposting.NewPurchPost(txSrc).Run(toPost)
	}))
	if purchOrder.Get(map[string]interface{}{"document_type": 1, "no": purchOrder.No}) {
		t.Error("the purchase order is still there after it was invoiced completely")
	}
}

// A drop shipment through the requisition worksheet: Get Sales Orders makes a worksheet line,
// Make Orders a purchase order for the customer; receiving it ships the sales order
func TestDropShipmentFromRequisitionWorksheet(t *testing.T) {
	reports.SalesPost = func(src ftables.Source, salesHeader *gtables.SalesHeader) error {
		return salesposting.NewSalesPost(src).Run(salesHeader)
	}
	src := documentsCompany(t)
	day := types.NewDateFromTime(time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local))
	order := dropShipSalesOrder(t, src, day, 2)

	template := gtables.NewReqWkshTemplate(src)
	template.Name = "DROPTEST"
	mustOK(t, template.InsertOrError(true))
	wksh := gtables.NewRequisitionWkshName(src)
	wksh.Worksheet_template_name, wksh.Name = "DROPTEST", "W1"
	mustOK(t, wksh.InsertOrError(true))
	context := gtables.NewRequisitionLine(src)
	context.Worksheet_template_name, context.Journal_batch_name = "DROPTEST", "W1"
	_, err := reports.Execute(reports.GetSalesOrdersID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context,
		Filters: map[string][]ftables.FieldFilter{"Sales_Line": {{Field: "document_no", Expression: order.No.String()}}},
	})
	mustOK(t, err)
	reqLine := gtables.NewRequisitionLine(src)
	reqLine.SetRange("worksheet_template_name", "DROPTEST")
	if !reqLine.FindFirst() || reqLine.Sales_order_no != order.No || reqLine.Sell_to_customer_no != "C00010" {
		t.Fatalf("worksheet line: sales order %s customer %s", reqLine.Sales_order_no, reqLine.Sell_to_customer_no)
	}
	if reqLine.Vendor_no == "" {
		item := gtables.NewItem(src)
		mustOK(t, item.GetOrError(types.Code("1000")))
		mustOK(t, reqLine.Validate("vendor_no", item.Vendor_no))
		mustOK(t, reqLine.ModifyOrError(true))
	}
	_, err = reports.Execute(reports.MakeOrdersID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context,
		Options: map[string]string{"order_date": day.String(), "posting_date": day.String(), "expected_receipt_date": day.String()},
	})
	mustOK(t, err)
	salesLine := gtables.NewSalesLine(src)
	mustOK(t, salesLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": order.No, "line_no": 10000}))
	if salesLine.Purchase_order_no == "" {
		t.Fatal("Make Orders did not link the sales line")
	}
	purchOrder := gtables.NewPurchaseHeader(src)
	mustOK(t, purchOrder.GetOrError(map[string]interface{}{"document_type": 1, "no": salesLine.Purchase_order_no}))
	if purchOrder.Sell_to_customer_no != "C00010" {
		t.Errorf("purchase order customer %q, want C00010", purchOrder.Sell_to_customer_no)
	}

	// receiving the purchase order ships the sales order
	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		toPost := gtables.NewPurchaseHeader(txSrc)
		mustOK(t, toPost.GetOrError(map[string]interface{}{"document_type": 1, "no": purchOrder.No}))
		toPost.Receive = true
		toPost.Invoice = false
		return purchposting.NewPurchPost(txSrc).Run(toPost)
	}))
	mustOK(t, salesLine.GetOrError(map[string]interface{}{"document_type": 1, "document_no": order.No, "line_no": 10000}))
	if !salesLine.Quantity_shipped.Equal(types.NewDecimalFromInt(2)) {
		t.Errorf("sales line shipped %s, want 2", salesLine.Quantity_shipped)
	}
	shptLine := gtables.NewSalesShipmentLine(src)
	shptLine.SetRange("order_no", order.No)
	if !shptLine.FindFirst() || !shptLine.Drop_shipment || shptLine.Item_shpt_entry_no == 0 {
		t.Errorf("sales shipment line: drop %v entry %d", shptLine.Drop_shipment, shptLine.Item_shpt_entry_no)
	}
	saleEntry := gtables.NewItemLedgerEntry(src)
	mustOK(t, saleEntry.GetOrError(shptLine.Item_shpt_entry_no))
	if !saleEntry.Drop_shipment || saleEntry.Applies_to_entry == 0 {
		t.Errorf("sale entry: drop %v applied to %d", saleEntry.Drop_shipment, saleEntry.Applies_to_entry)
	}
}
