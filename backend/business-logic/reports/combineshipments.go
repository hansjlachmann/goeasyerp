package reports

import (
	"errors"
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salescalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesgetshipment"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 295 Combine Shipments: the posted shipments of the sales orders marked Combine
// Shipments (from the customer) that are not invoiced become one sales invoice per bill-to
// customer and currency — a "Shipment No. …:" line and the shipment's lines (codeunit 64's
// CreateInvLines) —, dated with the request page's posting and document dates; with Calc. Inv.
// Discount the invoice discount is calculated, with Post Invoices the invoices are posted
// (codeunit 80; one that fails is counted and stays). Each invoice is made in a transaction of
// its own and posted in another, as the classic commits after each.
const CombineShipmentsID = 295

// SalesPost posts a sales document in src (codeunit 80 Sales-Post); set by the codeunits
// package — the reports cannot import the posting (the item journal's tests run reports)
var SalesPost func(src ftables.Source, salesHeader *gtables.SalesHeader) error

func init() {
	Register(CombineShipmentsID, "combine-shipments", func() Report { return &combineShipments{} })
}

type combineShipments struct {
	salesOrderHeader *gtables.SalesHeader
	window           *system.Window
	invoices         []*combinedInvoice
}

// combinedInvoice: the shipments one invoice combines (a bill-to customer and currency)
type combinedInvoice struct {
	billToCustomerNo types.Code
	currencyCode     types.Code
	shipmentNos      []types.Code
}

func (r *combineShipments) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_date", Type: "date", Required: true},
			{Name: "calc_inv_discount", Type: "bool", Default: "false"},
			{Name: "post_invoices", Type: "bool", Default: "false"},
		},
		OnInitReport: r.onInitReport,
		OnPostReport: r.onPostReport,
		DataItems: []*DataItem{{
			Name: "SalesOrderHeader", Table: gtables.SalesHeaderTableName,
			Key: []string{"document_type", "combine_shipments", "bill_to_customer_no", "currency_code"},
			TableView: []ftables.FieldFilter{
				{Field: "document_type", Expression: strconv.Itoa(int(gtables.SalesHeader_Document_type.Order))},
				{Field: "combine_shipments", Expression: "true"},
			},
			RequestFilterFields: []string{"sell_to_customer_no", "bill_to_customer_no"},
			OnPreDataItem:       r.salesOrderHeaderOnPreDataItem,
			OnAfterGetRecord:    Trigger(r.salesOrderHeaderOnAfterGetRecord),
			OnPostDataItem: func(rec any, run *Run) error {
				r.window.Close()
				return nil
			},
			DataItems: []*DataItem{{
				Name: "Sales_Shipment_Header", Table: gtables.SalesShipmentHeaderTableName, Key: []string{"order_no"},
				Link:                map[string]string{"order_no": "no"},
				RequestFilterFields: []string{"posting_date"},
				OnAfterGetRecord:    Trigger(r.salesShipmentHeaderOnAfterGetRecord),
			}},
		}},
	}
}

// onInitReport (OnOpenForm): the dates the work date, Calc. Inv. Discount from the setup
func (r *combineShipments) onInitReport(run *Run) error {
	workDate := system.WorkDate().String()
	run.SetOption("posting_date", workDate)
	run.SetOption("document_date", workDate)
	salesSetup := gtables.GetSalesReceivablesSetup(run)
	run.SetOption("calc_inv_discount", strconv.FormatBool(salesSetup.Calc_inv_discount))
	return nil
}

func (r *combineShipments) salesOrderHeaderOnPreDataItem(rec any, run *Run) error {
	if run.OptionDate("posting_date").IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	if run.OptionDate("document_date").IsZero() {
		return system.Error(run.Text("ENTER_DOCUMENT_DATE"))
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *combineShipments) salesOrderHeaderOnAfterGetRecord(salesOrderHeader *gtables.SalesHeader, run *Run) error {
	r.salesOrderHeader = salesOrderHeader
	r.window.Update(1, salesOrderHeader.Bill_to_customer_no)
	r.window.Update(2, salesOrderHeader.No)
	return nil
}

// salesShipmentHeaderOnAfterGetRecord: a shipment with lines not invoiced goes onto the invoice
// of the order's bill-to customer and currency (a new one where they change)
func (r *combineShipments) salesShipmentHeaderOnAfterGetRecord(salesShptHeader *gtables.SalesShipmentHeader, run *Run) error {
	r.window.Update(3, salesShptHeader.No)
	salesShptLine := gtables.NewSalesShipmentLine(run)
	salesShptLine.SetRange("document_no", salesShptHeader.No)
	salesShptLine.SetFilter("qty_shipped_not_invoiced", "<>0")
	if !salesShptLine.FindFirst() {
		return nil
	}
	var salesInv *combinedInvoice
	if len(r.invoices) > 0 {
		salesInv = r.invoices[len(r.invoices)-1]
	}
	if salesInv == nil || r.salesOrderHeader.Bill_to_customer_no != salesInv.billToCustomerNo ||
		r.salesOrderHeader.Currency_code != salesInv.currencyCode {
		salesInv = &combinedInvoice{billToCustomerNo: r.salesOrderHeader.Bill_to_customer_no, currencyCode: r.salesOrderHeader.Currency_code}
		r.invoices = append(r.invoices, salesInv)
	}
	salesInv.shipmentNos = append(salesInv.shipmentNos, salesShptHeader.No)
	return nil
}

// onPostReport (OnPostDataItem): the invoices made and, with Post Invoices, posted
func (r *combineShipments) onPostReport(run *Run) error {
	if len(r.invoices) == 0 {
		run.Message(run.Text("NOTHING_TO_COMBINE"))
		return nil
	}
	salesSetup := gtables.GetSalesReceivablesSetup(run)
	calcInvDisc := run.OptionBool("calc_inv_discount") || salesSetup.Calc_inv_discount
	noOfSalesInvErrors := 0
	for _, salesInv := range r.invoices {
		salesInvNo, err := r.insertSalesInvoice(run, salesInv, calcInvDisc)
		if err != nil {
			return err
		}
		if run.OptionBool("post_invoices") {
			err = r.postSalesInvoice(run, salesInvNo)
			if err != nil {
				noOfSalesInvErrors++
			}
		}
	}
	if noOfSalesInvErrors == 0 {
		run.Message(run.Text("SHIPMENTS_COMBINED"))
	} else {
		run.Message(run.Text("NOT_ALL_POSTED", noOfSalesInvErrors))
	}
	return nil
}

// insertSalesInvoice (InsertSalesInvHeader, the shipments' lines, FinalizeSalesInvHeader's
// invoice discount) in a transaction of its own; the invoice's number
func (r *combineShipments) insertSalesInvoice(run *Run, salesInv *combinedInvoice, calcInvDisc bool) (types.Code, error) {
	tx, err := ftables.BeginTransaction(run.GetDB())
	if err != nil {
		return "", err
	}
	src := ftables.DBSource(tx, run.GetCompany(), run.GetDBType())
	salesInvNo, err := insertCombinedInvoice(run, src, salesInv, calcInvDisc)
	if err != nil {
		_ = ftables.Rollback(tx)
		return "", err
	}
	return salesInvNo, ftables.Commit(tx)
}

func insertCombinedInvoice(run *Run, src ftables.Source, salesInv *combinedInvoice, calcInvDisc bool) (types.Code, error) {
	salesInvHeader := gtables.NewSalesHeader(src)
	salesInvHeader.Document_type = gtables.SalesHeader_Document_type.Invoice
	salesInvHeader.No = ""
	err := salesInvHeader.InsertOrError(true)
	if err != nil {
		return "", err
	}
	err = salesInvHeader.Validate("sell_to_customer_no", salesInv.billToCustomerNo)
	if err != nil {
		return "", err
	}
	if salesInvHeader.Bill_to_customer_no != salesInvHeader.Sell_to_customer_no {
		err = salesInvHeader.Validate("bill_to_customer_no", salesInv.billToCustomerNo)
		if err != nil {
			return "", err
		}
	}
	err = salesInvHeader.Validate("posting_date", run.OptionDate("posting_date"))
	if err != nil {
		return "", err
	}
	err = salesInvHeader.Validate("document_date", run.OptionDate("document_date"))
	if err != nil {
		return "", err
	}
	err = salesInvHeader.Validate("currency_code", salesInv.currencyCode)
	if err != nil {
		return "", err
	}
	err = salesInvHeader.ModifyOrError(false)
	if err != nil {
		return "", err
	}

	for _, shipmentNo := range salesInv.shipmentNos {
		err = salesgetshipment.CreateInvLines(salesInvHeader, shipmentNo, run.Text("SHIPMENT_NO"))
		if err != nil {
			return "", err
		}
	}
	if calcInvDisc {
		err = salescalcdisc.CalculateInvoiceDiscount(salesInvHeader)
		if err != nil {
			return "", err
		}
	}
	return salesInvHeader.No, nil
}

// postSalesInvoice (FinalizeSalesInvHeader's posting): codeunit 80 in a transaction of its own
func (r *combineShipments) postSalesInvoice(run *Run, salesInvNo types.Code) error {
	if SalesPost == nil {
		return errors.New("combine shipments: no sales posting")
	}
	tx, err := ftables.BeginTransaction(run.GetDB())
	if err != nil {
		return err
	}
	src := ftables.DBSource(tx, run.GetCompany(), run.GetDBType())
	salesInvHeader := gtables.NewSalesHeader(src)
	err = salesInvHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Invoice), "no": salesInvNo})
	if err == nil {
		err = SalesPost(src, salesInvHeader)
	}
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}
