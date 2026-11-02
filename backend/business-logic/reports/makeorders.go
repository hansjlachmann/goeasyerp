package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/reqwksh"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 493 Make Orders: the requisition worksheet's lines become purchase orders (codeunit
// 333) with the order, posting and expected receipt dates and Your Reference of the request page
// — on a recurring worksheet the lines up to the Expiration Date's order date; with Print
// Purchase Orders each order prints with the P.Order reports of Report Selections. Run from the
// worksheet page (its line, with the page's template and worksheet).
const MakeOrdersID = 493

func init() {
	Register(MakeOrdersID, "make-orders", func() Report { return &makeOrders{} })
}

type makeOrders struct {
	reqLine     *gtables.RequisitionLine
	reqTemplate *gtables.ReqWkshTemplate
}

func (r *makeOrders) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "end_order_date", Type: "date"},
			{Name: "order_date", Type: "date"},
			{Name: "posting_date", Type: "date"},
			{Name: "expected_receipt_date", Type: "date"},
			{Name: "your_reference", Type: "text"},
			{Name: "print_purch_orders", Type: "bool", Default: "false"},
		},
		Transaction:  true,
		ContextTable: gtables.RequisitionLineTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
	}
}

// onInitReport (SetReqWkshLine, OnOpenForm): the worksheet of the page; the dates the work date,
// the Expiration Date only on a recurring worksheet
func (r *makeOrders) onInitReport(run *Run) error {
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
	r.reqTemplate = gtables.NewReqWkshTemplate(run)
	err := r.reqTemplate.GetOrError(r.reqLine.Worksheet_template_name)
	if err != nil {
		return err
	}
	workDate := system.WorkDate().String()
	run.SetOption("order_date", workDate)
	run.SetOption("posting_date", workDate)
	run.SetOption("expected_receipt_date", workDate)
	if r.reqTemplate.Recurring {
		run.SetOption("end_order_date", workDate)
	}
	return nil
}

// onPreReport (UseOneJnl): the orders of the worksheet, the message, the printouts and the
// worksheet the user is in afterwards
func (r *makeOrders) onPreReport(run *Run) error {
	reqLine := r.reqLine
	if r.reqTemplate.Recurring && reqLine.GetFilter("order_date") != "" {
		return reqLine.FieldError("order_date", run.Text("ORDER_DATE_FILTERED"))
	}
	purchOrderHeader := gtables.NewPurchaseHeader(run)
	purchOrderHeader.Order_date = run.OptionDate("order_date")
	purchOrderHeader.Posting_date = run.OptionDate("posting_date")
	purchOrderHeader.Expected_receipt_date = run.OptionDate("expected_receipt_date")
	purchOrderHeader.Your_reference = types.Text(run.Option("your_reference"))
	endOrderDate := types.Date{}
	if r.reqTemplate.Recurring {
		endOrderDate = run.OptionDate("end_order_date")
	}
	tempJnlBatchName := reqLine.Journal_batch_name
	reqWkshMakeOrders := &reqwksh.MakeOrder{}
	reqWkshMakeOrders.Set(purchOrderHeader, endOrderDate)
	err := reqWkshMakeOrders.Run(reqLine)
	if err != nil {
		return err
	}

	if reqLine.Line_no == 0 {
		run.Message(run.Text("NOTHING_TO_CREATE"))
	} else {
		if tempJnlBatchName == reqLine.Journal_batch_name {
			run.Message(run.Text("ORDERS_CREATED"))
		} else {
			run.Message(run.Text("ORDERS_CREATED_NEW_WORKSHEET", reqLine.Journal_batch_name))
			run.OpenPage(&fcodeunits.PageLink{PageID: r.reqTemplate.Page_id,
				Params: map[string]string{"template": reqLine.Worksheet_template_name.String(), "batch": reqLine.Journal_batch_name.String()}})
		}
	}

	// Print Purchase Orders (FinalizeOrderHeader): each order with the P.Order reports, straight
	// to their printer
	if run.OptionBool("print_purch_orders") {
		for _, purchOrderHeader2 := range reqWkshMakeOrders.Orders {
			printouts, err := PrintDocuments(purchOrderHeader2, gtables.ReportSelections_Usage.POrder, false)
			if err != nil {
				return err
			}
			run.AddPrintouts(printouts...)
		}
	}
	return nil
}

// reqLineFilter: the worksheet line's filter on a field in filter group 2 (the page's template
// and worksheet)
func reqLineFilter(rec *gtables.RequisitionLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
