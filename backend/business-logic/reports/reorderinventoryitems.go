package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 699 Reorder Inventory Items: worksheet lines for the items whose quantity available up
// to the Ending Date — inventory + on purchase orders − on sales orders — is below their Reorder
// Point, the reorder quantity of their Requisition Method or reorder fields (codeunit 5400
// GetItemReorderQty); an item with a Reorder Cycle is calculated per cycle up to the Ending Date,
// each line expected at the cycle's end. Run from the worksheet page.
const ReorderInventoryItemsID = 699

func init() {
	Register(ReorderInventoryItemsID, "reorder-inventory-items", func() Report { return &reorderInventoryItems{} })
}

type reorderInventoryItems struct {
	reqLine          *gtables.RequisitionLine
	endingDate       types.Date
	periodEndingDate types.Date
	window           *system.Window
}

func (r *reorderInventoryItems) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "ending_date", Type: "date", Required: true},
		},
		Transaction:  true,
		ContextTable: gtables.RequisitionLineTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Item", Table: gtables.ItemTableName, Key: []string{"vendor_no"},
			RequestFilterFields: []string{"no", "vendor_no", "bill_of_materials"},
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.itemOnPostDataItem),
		}},
	}
}

// onInitReport (SetReqWkshLine): the worksheet of the page
func (r *reorderInventoryItems) onInitReport(run *Run) error {
	reqLine, ok := run.Context().(*gtables.RequisitionLine)
	if !ok {
		return system.Error(run.Text("NO_WORKSHEET"))
	}
	r.reqLine = gtables.NewRequisitionLine(run)
	r.reqLine.Worksheet_template_name = reqLine.Worksheet_template_name
	r.reqLine.Journal_batch_name = reqLine.Journal_batch_name
	if r.reqLine.Worksheet_template_name == "" {
		r.reqLine.Worksheet_template_name = reqLineFilter(reqLine, "worksheet_template_name")
	}
	if r.reqLine.Journal_batch_name == "" {
		r.reqLine.Journal_batch_name = reqLineFilter(reqLine, "journal_batch_name")
	}
	return nil
}

// onPreReport: the Ending Date is needed; new lines follow the worksheet's last line
func (r *reorderInventoryItems) onPreReport(run *Run) error {
	r.endingDate = run.OptionDate("ending_date")
	if r.endingDate.IsZero() {
		return system.Error(run.Text("ENTER_ENDING_DATE"))
	}
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
	reqLine := r.reqLine
	reqLine.SetRange("worksheet_template_name", reqLine.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", reqLine.Journal_batch_name)
	reqLine.LockTable()
	if reqLine.FindLast() {
		reqLine.Init()
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *reorderInventoryItems) itemOnAfterGetRecord(item *gtables.Item, run *Run) error {
	r.window.Update(1, item.No)
	err := item.SetFlowFilter("date_filter", ".."+r.endingDate.String())
	if err != nil {
		return err
	}
	r.periodEndingDate = types.Date{}

	if item.Reorder_cycle == "" {
		return r.calcItemAvailable(item)
	}
	periodEndingDate, err := system.CalcDate(item.Reorder_cycle.String(), system.WorkDate())
	if err != nil {
		return err
	}
	r.periodEndingDate = periodEndingDate.AddDays(-1)
	for r.periodEndingDate.Before(r.endingDate) {
		err = r.calcItemAvailable(item)
		if err != nil {
			return err
		}
		periodEndingDate, err = system.CalcDate(item.Reorder_cycle.String(), r.periodEndingDate.AddDays(1))
		if err != nil {
			return err
		}
		r.periodEndingDate = periodEndingDate.AddDays(-1)
	}
	return nil
}

func (r *reorderInventoryItems) itemOnPostDataItem(item *gtables.Item, run *Run) error {
	r.window.Close()
	return nil
}

// calcItemAvailable (CalcItemAvailable): an item available below its Reorder Point gets a line
// of its reorder quantity (there is no production: no scheduled receipts or needs)
func (r *reorderInventoryItems) calcItemAvailable(item *gtables.Item) error {
	item.CalcFields("quantity_on_hand", "qty_on_purch_order", "qty_on_sales_order")
	qtyAvailable := item.Quantity_on_hand.Add(item.Qty_on_purch_order).Sub(item.Qty_on_sales_order)
	if qtyAvailable.LessThan(item.Reorder_point) {
		reorderQty, err := tables.GetItemReorderQty(item, item.Requisition_method_code, qtyAvailable)
		if err != nil {
			return err
		}
		return r.insertReqWkshLine(item, reorderQty)
	}
	return nil
}

// insertReqWkshLine (InsertReqWkshLine): a worksheet line of the item and quantity (in its
// purchase unit), expected at the cycle's end
func (r *reorderInventoryItems) insertReqWkshLine(item *gtables.Item, reorderQty types.Decimal) error {
	reqLine := r.reqLine
	reqLine.Line_no += 10000
	reqLine.Type = gtables.RequisitionLine_Type.Item
	err := reqLine.Validate("no", item.No)
	if err != nil {
		return err
	}
	err = reqLine.TestField("qty_per_unit_of_measure")
	if err != nil {
		return err
	}
	err = reqLine.Validate("quantity", system.Round(reorderQty.Div(reqLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001")))
	if err != nil {
		return err
	}
	if !r.periodEndingDate.IsZero() {
		reqLine.Expected_receipt_date = r.periodEndingDate
	}
	return reqLine.InsertOrError(true)
}
