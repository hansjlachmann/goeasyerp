package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 211 Drop Shipment Status (the classic local report 10051, renumbered; Sales › Reports):
// the drop shipment lines of the sales orders by sales order, customer or item (Report By) — per
// group a heading (the order's or customer's name and phone, the item's description), per line
// the shipment date, quantity and what is invoiced, and the purchase order line delivering it
// (vendor, expected receipt date, quantities ordered and invoiced) with a status: Order Expected
// Late, Pending Vendor Shipment Confirmation, Not Yet Ordered, No Purchase Order Issued, Order
// Line Complete.
const DropShipmentStatusID = 211

func init() {
	Register(DropShipmentStatusID, "drop-shipment-status", func() Report { return &dropShipmentStatus{} })
}

type dropShipmentStatus struct {
	language                 string
	xCustomer, xItem, xOrder types.Code
	firstPass                bool
}

const (
	dropShipSortSalesOrder = 1
	dropShipSortCustomer   = 2
	dropShipSortItem       = 3
)

func (r *dropShipmentStatus) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{{Name: "report_by", Type: "option", Default: "1",
			Options: []string{"SALES_ORDER", "CUSTOMER", "ITEM"}}},
		OnPreReport: func(run *Run) error {
			r.language = fcodeunits.CurrentLanguage()
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Sales_Line", Table: gtables.SalesLineTableName, Key: []string{"document_type", "document_no", "line_no"},
			TableView: []ftables.FieldFilter{
				{Field: "document_type", Expression: system.Format(int(gtables.SalesLine_Document_type.Order))},
				{Field: "drop_shipment", Expression: "true"},
				{Field: "type", Expression: system.Format(int(gtables.SalesLine_Type.Item))},
			},
			RequestFilterFields: []string{"bill_to_customer_no", "document_no", "no", "shipment_date"},
			OnPreDataItem: Trigger(func(salesLine *gtables.SalesLine, run *Run) error {
				setFilterText(run, salesLine)
				r.firstPass = true
				switch clampSort(run.OptionInt("report_by")) {
				case dropShipSortCustomer:
					salesLine.SetCurrentKey("document_type", "bill_to_customer_no", "document_no", "line_no")
				case dropShipSortItem:
					salesLine.SetCurrentKey("document_type", "type", "no", "document_no", "line_no")
				default:
					salesLine.SetCurrentKey("document_type", "document_no", "line_no")
				}
				run.SetValue("subtitle", run.Text("REPORT_BY_"+[]string{"SALES_ORDER", "CUSTOMER", "ITEM"}[clampSort(run.OptionInt("report_by"))-1]))
				return nil
			}),
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
		}},
	}
}

// clampSort: the Report By option (1 sales order, 2 customer, 3 item; else by sales order)
func clampSort(sortOption int) int {
	if sortOption < dropShipSortSalesOrder || sortOption > dropShipSortItem {
		return dropShipSortSalesOrder
	}
	return sortOption
}

func (r *dropShipmentStatus) onAfterGetRecord(salesLine *gtables.SalesLine, run *Run) error {
	sortOption := clampSort(run.OptionInt("report_by"))
	newGroup := r.firstPass ||
		sortOption == dropShipSortCustomer && salesLine.Bill_to_customer_no != r.xCustomer ||
		sortOption == dropShipSortItem && salesLine.No != r.xItem ||
		sortOption == dropShipSortSalesOrder && salesLine.Document_no != r.xOrder
	run.SetValue("group_customer", newGroup && sortOption == dropShipSortCustomer)
	run.SetValue("group_item", newGroup && sortOption == dropShipSortItem)
	run.SetValue("group_order", newGroup && sortOption == dropShipSortSalesOrder)
	if newGroup {
		customer := gtables.NewCustomer(run)
		if !customer.Get(salesLine.Bill_to_customer_no) {
			customer.Name = ""
			customer.Contact = ""
			customer.Phone_no = ""
		}
		run.SetValue("customer_name", customer.Name)
		run.SetValue("customer_phone", customer.Phone_no)
		run.SetValue("customer_contact", customer.Contact)
		itemDescription := types.Text("")
		item := gtables.NewItem(run)
		if item.Get(salesLine.No) {
			itemDescription = item.Description
		}
		run.SetValue("item_description", itemDescription)
		r.xCustomer = salesLine.Bill_to_customer_no
		r.xItem = salesLine.No
		r.xOrder = salesLine.Document_no
	}
	r.firstPass = false

	statusLine1, statusLine2 := "", ""
	pendingVendConfirm := run.Text("PENDING_VEND_CONFIRM")
	custInvQty := ""
	if salesLine.Quantity_invoiced.LessThan(salesLine.Quantity) {
		if !salesLine.Quantity_invoiced.IsZero() {
			custInvQty = formatValue(salesLine.Quantity_invoiced, "0:5", r.language)
		}
	} else {
		custInvQty = run.Text("COMPLETE")
	}
	vendInvQty, vendOrdQty := "", ""
	purchaseLine := gtables.NewPurchaseLine(run)
	hasPurchLine := salesLine.Purch_order_line_no != 0 && purchaseLine.Get(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order),
		"document_no": salesLine.Purchase_order_no, "line_no": salesLine.Purch_order_line_no})
	if hasPurchLine {
		if salesLine.Shipment_date.Before(purchaseLine.Expected_receipt_date) {
			statusLine1 = run.Text("ORDER_EXPECTED_LATE")
		}
		if purchaseLine.Quantity_invoiced.LessThan(purchaseLine.Quantity) {
			statusLine2 = pendingVendConfirm
			if !purchaseLine.Quantity_invoiced.IsZero() {
				vendInvQty = formatValue(purchaseLine.Quantity_invoiced, "0:5", r.language)
			}
		} else {
			vendInvQty = run.Text("COMPLETE")
		}
		if purchaseLine.Quantity.LessThan(salesLine.Quantity.Sub(salesLine.Quantity_invoiced)) {
			statusLine1 = run.Text("WARNING_ENTIRE_QTY")
			statusLine2 = run.Text("NOT_YET_ORDERED")
			if !purchaseLine.Quantity.IsZero() {
				vendOrdQty = formatValue(purchaseLine.Quantity, "0:5", r.language)
			}
		} else {
			vendOrdQty = run.Text("COMPLETE")
		}
	} else {
		if salesLine.Quantity_invoiced.LessThan(salesLine.Quantity) {
			statusLine1 = run.Text("WARNING")
		} else {
			statusLine1 = run.Text("ORDER_LINE_COMPLETE")
		}
	}
	if statusLine1 == "" && statusLine2 != "" {
		statusLine1 = statusLine2
		statusLine2 = ""
	}
	run.SetValue("status_line_1", statusLine1)
	run.SetValue("status_line_2", statusLine2)
	run.SetValue("cust_inv_qty", custInvQty)
	run.SetValue("has_purch_line", hasPurchLine)
	run.SetValue("no_purch_order", !hasPurchLine && salesLine.Quantity_invoiced.LessThan(salesLine.Quantity))
	run.SetValue("purch_order_no", purchaseLine.Document_no)
	run.SetValue("vendor_no", purchaseLine.Buy_from_vendor_no)
	run.SetValue("expected_receipt_date", purchaseLine.Expected_receipt_date)
	run.SetValue("purch_unit_of_measure", purchaseLine.Unit_of_measure_code)
	run.SetValue("vend_ord_qty", vendOrdQty)
	run.SetValue("vend_inv_qty", vendInvQty)
	return nil
}
