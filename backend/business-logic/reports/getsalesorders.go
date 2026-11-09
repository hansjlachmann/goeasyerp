package reports

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 698 Get Sales Orders (Drop Shipment › Get Sales Orders on the requisition worksheet):
// the drop shipment lines of the sales orders in the filters that are not linked to a purchase
// order and have an outstanding quantity become worksheet lines — the item, unit of measure,
// quantity, the shipment date as expected receipt date, location, variant, bin, serial and lot
// number, the sales order line, its customer and ship-to address; Make Orders buys them with a
// purchase order per customer and ship-to address.
const GetSalesOrdersID = 698

func init() {
	Register(GetSalesOrdersID, "get-sales-orders", func() Report { return &getSalesOrders{} })
}

type getSalesOrders struct {
	reqLine   *gtables.RequisitionLine
	nextLine  int
	lineCount int
}

func (r *getSalesOrders) Definition() *Definition {
	return &Definition{
		Transaction:  true,
		ContextTable: gtables.RequisitionLineTableName,
		OnInitReport: func(run *Run) error {
			reqLine, ok := run.Context().(*gtables.RequisitionLine)
			if !ok {
				return system.Error(run.Text("NO_WORKSHEET"))
			}
			r.reqLine = reqLine
			if r.reqLine.Worksheet_template_name == "" {
				r.reqLine.Worksheet_template_name = reqLineFilter(r.reqLine, "worksheet_template_name")
			}
			if r.reqLine.Journal_batch_name == "" {
				r.reqLine.Journal_batch_name = reqLineFilter(r.reqLine, "journal_batch_name")
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			reqTemplate := gtables.NewReqWkshTemplate(run)
			err := reqTemplate.GetOrError(r.reqLine.Worksheet_template_name)
			if err != nil {
				return err
			}
			reqWkshName := gtables.NewRequisitionWkshName(run)
			err = reqWkshName.GetOrError(map[string]interface{}{"worksheet_template_name": r.reqLine.Worksheet_template_name, "name": r.reqLine.Journal_batch_name})
			if err != nil {
				return err
			}
			reqLine := gtables.NewRequisitionLine(run)
			reqLine.SetRange("worksheet_template_name", r.reqLine.Worksheet_template_name)
			reqLine.SetRange("journal_batch_name", r.reqLine.Journal_batch_name)
			reqLine.LockTable()
			if reqLine.FindLast() {
				r.nextLine = reqLine.Line_no
			}
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Sales_Line", Table: gtables.SalesLineTableName, Key: []string{"document_type", "document_no", "line_no"},
			TableView: []ftables.FieldFilter{
				{Field: "document_type", Expression: system.Format(int(gtables.SalesLine_Document_type.Order))},
				{Field: "type", Expression: system.Format(int(gtables.SalesLine_Type.Item))},
				{Field: "drop_shipment", Expression: "true"},
				{Field: "purch_order_line_no", Expression: "0"},
				{Field: "outstanding_quantity", Expression: "<>0"},
			},
			RequestFilterFields: []string{"document_no", "sell_to_customer_no", "no"},
			OnAfterGetRecord: Trigger(func(salesLine *gtables.SalesLine, run *Run) error {
				r.lineCount++
				return r.insertReqWkshLine(run, salesLine)
			}),
			OnPostDataItem: Trigger(func(salesLine *gtables.SalesLine, run *Run) error {
				if r.lineCount == 0 {
					return apperrors.DropShptNoSalesLines()
				}
				run.Refresh()
				return nil
			}),
		}},
	}
}

// insertReqWkshLine (InsertReqWkshLine): the worksheet line of a drop shipment sales line
func (r *getSalesOrders) insertReqWkshLine(run *Run, salesOrderLine *gtables.SalesLine) error {
	reqLine := gtables.NewRequisitionLine(run)
	reqLine.Init()
	reqLine.Worksheet_template_name = r.reqLine.Worksheet_template_name
	reqLine.Journal_batch_name = r.reqLine.Journal_batch_name
	r.nextLine += 10000
	reqLine.Line_no = r.nextLine
	err := reqLine.Validate("type", gtables.RequisitionLine_Type.Item)
	if err == nil {
		err = reqLine.Validate("no", salesOrderLine.No)
	}
	if err == nil {
		err = reqLine.Validate("unit_of_measure_code", salesOrderLine.Unit_of_measure_code)
	}
	if err != nil {
		return err
	}
	reqLine.Lot_no = salesOrderLine.Lot_no
	reqLine.Variant_code = salesOrderLine.Variant_code
	reqLine.Bin_code = salesOrderLine.Bin_code
	reqLine.Qty_per_unit_of_measure = salesOrderLine.Qty_per_unit_of_measure
	err = reqLine.Validate("quantity", salesOrderLine.Outstanding_quantity)
	if err != nil {
		return err
	}
	reqLine.Expected_receipt_date = salesOrderLine.Shipment_date
	reqLine.Location_code = salesOrderLine.Location_code
	reqLine.Serial_no = salesOrderLine.Serial_no
	reqLine.Sales_order_no = salesOrderLine.Document_no
	reqLine.Sales_order_line_no = salesOrderLine.Line_no
	reqLine.Sell_to_customer_no = salesOrderLine.Sell_to_customer_no
	salesOrderHeader := gtables.NewSalesHeader(run)
	err = salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": salesOrderLine.Document_no})
	if err != nil {
		return err
	}
	reqLine.Ship_to_code = salesOrderHeader.Ship_to_code
	return reqLine.InsertOrError(false)
}
