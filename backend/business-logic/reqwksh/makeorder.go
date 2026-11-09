package reqwksh

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// MakeOrder is codeunit 333 Req. Wksh.-Make Order: the lines of a worksheet with a number,
// vendor and quantity become purchase orders — one per vendor, order address and currency —
// with the dates and reference of the request (Set). A worksheet's lines are deleted afterwards
// (a numbered worksheet moves on to the next number when it is empty); a recurring worksheet's
// lines stay, their order date moved on by the frequency, a Variable line without quantity.
type MakeOrder struct {
	// Set: the order, posting and expected receipt dates and Your Reference of the orders, the
	// last order date of the recurring lines to take
	purchOrderHeader *gtables.PurchaseHeader
	endOrderDate     types.Date

	// Orders are the purchase orders created
	Orders []*gtables.PurchaseHeader

	reqTemplate      *gtables.ReqWkshTemplate
	reqLine          *gtables.RequisitionLine
	orderDateReq     types.Date
	postingDateReq   types.Date
	receiveDateReq   types.Date
	referenceReq     types.Text
	orderCounter     int
	lineCount        int
	orderLineCounter int
	nextLineNo       int
	window           *system.Window
}

// Set (Set): the dates and reference of the orders, the last order date of the recurring lines
func (m *MakeOrder) Set(newPurchOrderHeader *gtables.PurchaseHeader, newEndingOrderDate types.Date) {
	m.purchOrderHeader = newPurchOrderHeader
	m.endOrderDate = newEndingOrderDate
}

// Run (OnRun) makes the orders of the worksheet of reqLine (its template and worksheet). After
// it reqLine's Line No. is the number of orders made (0: nothing to make) and its Journal Batch
// Name the worksheet the user is in now.
func (m *MakeOrder) Run(reqLine *gtables.RequisitionLine) error {
	m.reqLine = gtables.NewRequisitionLine(reqLine)
	m.reqLine.Copy(reqLine)
	err := m.code()
	if err != nil {
		return err
	}
	reqLine.Copy(m.reqLine)
	return nil
}

func (m *MakeOrder) code() error {
	language := fcodeunits.CurrentLanguage()
	reqLine := m.reqLine
	m.orderDateReq = m.purchOrderHeader.Order_date
	m.postingDateReq = m.purchOrderHeader.Posting_date
	m.receiveDateReq = m.purchOrderHeader.Expected_receipt_date
	m.referenceReq = m.purchOrderHeader.Your_reference
	m.purchOrderHeader = nil

	reqLine.SetRange("worksheet_template_name", reqLine.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", reqLine.Journal_batch_name)
	reqLine.LockTable()

	m.reqTemplate = gtables.NewReqWkshTemplate(reqLine)
	err := m.reqTemplate.GetOrError(reqLine.Worksheet_template_name)
	if err != nil {
		return err
	}

	if m.reqTemplate.Recurring {
		reqLine.SetRange("order_date", types.Date{}, m.endOrderDate)
		reqLine.SetFilter("expiration_date", system.StrSubstNo("''|%1..", system.Format(system.WorkDate())))
	}

	if !reqLine.FindFirst() {
		reqLine.Line_no = 0
		return nil
	}

	if m.reqTemplate.Recurring {
		m.window = system.OpenWindow(i18n.GetInstance().Message("REQ_WKSH_MAKE_ORDER_WINDOW_RECURRING", language))
	} else {
		m.window = system.OpenWindow(i18n.GetInstance().Message("REQ_WKSH_MAKE_ORDER_WINDOW", language))
	}
	defer m.window.Close()
	m.window.Update(1, reqLine.Journal_batch_name)

	// Check lines
	m.lineCount = 0
	reqLine2 := gtables.NewRequisitionLine(reqLine)
	reqLine2.CopyFilters(reqLine)
	for found := reqLine2.FindSet(); found; found = reqLine2.Next() {
		m.lineCount++
		m.window.Update(2, m.lineCount)
		err = m.checkRecurringLine(reqLine2)
		if err != nil {
			return err
		}
		err = checkReqWkshLine(reqLine2)
		if err != nil {
			return err
		}
	}

	// Create lines
	m.lineCount = 0
	m.orderCounter = 0
	m.orderLineCounter = 0
	purchOrderHeader := gtables.NewPurchaseHeader(reqLine)
	reqLine.SetCurrentKey("worksheet_template_name", "journal_batch_name", "vendor_no", "sell_to_customer_no", "ship_to_code", "order_address_code", "currency_code")
	var lines []*gtables.RequisitionLine
	for found := reqLine.FindSet(); found; found = reqLine.Next() {
		line := gtables.NewRequisitionLine(reqLine)
		line.Copy(reqLine)
		lines = append(lines, line)
	}
	for _, line := range lines {
		if purchOrderHeader.Buy_from_vendor_no != "" &&
			(purchOrderHeader.Buy_from_vendor_no != line.Vendor_no || purchOrderHeader.Currency_code != line.Currency_code) ||
			purchOrderHeader.Sell_to_customer_no != line.Sell_to_customer_no ||
			purchOrderHeader.Ship_to_code != line.Ship_to_code ||
			purchOrderHeader.Order_address_code != line.Order_address_code {
			err = m.finalizeOrderHeader(purchOrderHeader)
			if err != nil {
				return err
			}
		}
		m.makeRecurringText(line)
		purchOrderHeader, err = m.insertPurchOrderLine(line, purchOrderHeader)
		if err != nil {
			return err
		}
	}
	if purchOrderHeader.Buy_from_vendor_no != "" {
		err = m.finalizeOrderHeader(purchOrderHeader)
		if err != nil {
			return err
		}
	}

	// The number of orders made and the worksheet the user is in now
	reqLine.Init()
	reqLine.Line_no = m.orderCounter

	if m.orderCounter != 0 {
		if !m.reqTemplate.Recurring {
			// Not a recurring worksheet
			reqLine2 = gtables.NewRequisitionLine(reqLine)
			reqLine2.CopyFilters(reqLine)
			reqLine2.SetFilter("vendor_no", "<>''")
			reqLine2.FindFirst() // remember the last line: a new first line takes its type and order date
			reqLine.DeleteAll()

			reqLine3 := gtables.NewRequisitionLine(reqLine)
			reqLine3.SetRange("worksheet_template_name", reqLine.Worksheet_template_name)
			reqLine3.SetRange("journal_batch_name", reqLine.Journal_batch_name)
			if !reqLine3.FindLast() {
				nextName := system.IncStr(reqLine.Journal_batch_name.String())
				if nextName != "" {
					reqWkshName := gtables.NewRequisitionWkshName(reqLine)
					err = reqWkshName.GetOrError(map[string]interface{}{"worksheet_template_name": reqLine.Worksheet_template_name, "name": reqLine.Journal_batch_name})
					if err != nil {
						return err
					}
					err = reqWkshName.DeleteOrError(false)
					if err != nil {
						return err
					}
					reqWkshName.Name = types.Code(nextName)
					reqWkshName.Insert(false)
					reqLine.Journal_batch_name = reqWkshName.Name
				}
			}

			reqLine3.SetRange("journal_batch_name", reqLine.Journal_batch_name)
			if !reqLine3.FindLast() {
				reqLine3.Init()
				reqLine3.Worksheet_template_name = reqLine.Worksheet_template_name
				reqLine3.Journal_batch_name = reqLine.Journal_batch_name
				reqLine3.Line_no = 10000
				reqLine3.Type = reqLine2.Type
				reqLine3.Order_date = reqLine2.Order_date
				err = reqLine3.InsertOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkReqWkshLine (CheckReqWkshLine): a line that is not empty has a number, a vendor and a
// quantity
func checkReqWkshLine(reqLine2 *gtables.RequisitionLine) error {
	if reqLine2.No != "" || reqLine2.Vendor_no != "" || !reqLine2.Quantity.IsZero() {
		err := reqLine2.TestField("no")
		if err != nil {
			return err
		}
		err = reqLine2.TestField("vendor_no")
		if err != nil {
			return err
		}
		return reqLine2.TestField("quantity")
	}
	return nil
}

// insertPurchOrderLine (InsertPurchOrderLine): the line as a line of the purchase order of its
// vendor, order address and currency — a new order when the line's differ from the current one
func (m *MakeOrder) insertPurchOrderLine(reqLine2 *gtables.RequisitionLine, purchOrderHeader *gtables.PurchaseHeader) (*gtables.PurchaseHeader, error) {
	if reqLine2.No == "" || reqLine2.Vendor_no == "" || reqLine2.Quantity.IsZero() {
		return purchOrderHeader, nil
	}

	if purchOrderHeader.Buy_from_vendor_no != reqLine2.Vendor_no ||
		purchOrderHeader.Sell_to_customer_no != reqLine2.Sell_to_customer_no ||
		purchOrderHeader.Ship_to_code != reqLine2.Ship_to_code ||
		purchOrderHeader.Order_address_code != reqLine2.Order_address_code ||
		purchOrderHeader.Currency_code != reqLine2.Currency_code {
		var err error
		purchOrderHeader, err = m.insertHeader(reqLine2)
		if err != nil {
			return nil, err
		}
		m.lineCount = 0
		m.nextLineNo = 0
	}

	m.lineCount++
	m.window.Update(4, m.lineCount)

	err := reqLine2.TestField("currency_code", purchOrderHeader.Currency_code)
	if err != nil {
		return nil, err
	}
	purchOrderLine := gtables.NewPurchaseLine(reqLine2)
	purchOrderLine.Init()
	purchOrderLine.Document_type = gtables.PurchaseLine_Document_type.Order
	purchOrderLine.Buy_from_vendor_no = reqLine2.Vendor_no
	purchOrderLine.Document_no = purchOrderHeader.No
	m.nextLineNo += 10000
	purchOrderLine.Line_no = m.nextLineNo
	err = purchOrderLine.Validate("type", purchLineType(reqLine2.Type))
	if err != nil {
		return nil, err
	}
	err = purchOrderLine.Validate("no", reqLine2.No)
	if err != nil {
		return nil, err
	}
	err = purchOrderLine.Validate("unit_of_measure_code", reqLine2.Unit_of_measure_code)
	if err != nil {
		return nil, err
	}
	purchOrderLine.Qty_per_unit_of_measure = reqLine2.Qty_per_unit_of_measure
	err = purchOrderLine.Validate("quantity", reqLine2.Quantity)
	if err != nil {
		return nil, err
	}
	err = purchOrderLine.Validate("direct_unit_cost", reqLine2.Direct_unit_cost)
	if err != nil {
		return nil, err
	}
	if !reqLine2.Expected_receipt_date.IsZero() {
		purchOrderLine.Expected_receipt_date = reqLine2.Expected_receipt_date
	}
	err = purchOrderLine.Validate("quantity_disc_percent", reqLine2.Quantity_disc_percent)
	if err != nil {
		return nil, err
	}
	purchOrderLine.Vendor_item_no = reqLine2.Vendor_item_no
	purchOrderLine.Dimension_1_code = reqLine2.Dimension_1_code
	purchOrderLine.Dimension_2_code = reqLine2.Dimension_2_code
	purchOrderLine.Location_code = reqLine2.Location_code
	purchOrderLine.Description = reqLine2.Description
	purchOrderLine.Description_2 = reqLine2.Description_2
	purchOrderLine.Variant_code = reqLine2.Variant_code
	purchOrderLine.Bin_code = reqLine2.Bin_code
	purchOrderLine.Serial_no = reqLine2.Serial_no
	purchOrderLine.Lot_no = reqLine2.Lot_no
	purchOrderLine.Sales_order_no = reqLine2.Sales_order_no
	purchOrderLine.Sales_order_line_no = reqLine2.Sales_order_line_no
	purchOrderLine.Drop_shipment = reqLine2.Sales_order_line_no != 0
	purchOrderLine.Reserve = gtables.PurchaseLineReserve(reqLine2.Reserve)
	if !reqLine2.Reserved_qty_base.IsZero() {
		err = reservmgt.New(reqLine2).TransferReqLineToPurchLine(reqLine2, purchOrderLine)
		if err == nil {
			err = reqLine2.ModifyOrError(false)
		}
		if err != nil {
			return nil, err
		}
	}
	err = purchOrderLine.InsertOrError(true)
	if err != nil {
		return nil, err
	}
	if purchOrderLine.Drop_shipment {
		err = linkDropShipment(reqLine2, purchOrderLine)
		if err != nil {
			return nil, err
		}
	}
	return purchOrderHeader, nil
}

// linkDropShipment: the sales order line a drop shipment line buys for — still the worksheet
// line's (customer, ship-to address, item, outstanding quantity, location, unit of measure,
// variant, bin) and not linked yet — gets the purchase order line and its cost
func linkDropShipment(reqLine2 *gtables.RequisitionLine, purchOrderLine *gtables.PurchaseLine) error {
	salesOrderLine := gtables.NewSalesLine(reqLine2)
	salesOrderLine.LockTable()
	salesOrderHeader := gtables.NewSalesHeader(reqLine2)
	salesOrderHeader.LockTable()
	err := salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": reqLine2.Sales_order_no})
	if err == nil {
		err = reqLine2.TestField("ship_to_code", salesOrderHeader.Ship_to_code)
	}
	if err == nil {
		err = salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
			"document_no": reqLine2.Sales_order_no, "line_no": reqLine2.Sales_order_line_no})
	}
	if err == nil {
		err = salesOrderLine.TestField("drop_shipment", true)
	}
	if err == nil {
		err = salesOrderLine.TestField("type", gtables.SalesLine_Type.Item)
	}
	if err != nil {
		return err
	}
	if salesOrderLine.Purch_order_line_no != 0 {
		return apperrors.DropShptAlreadyAssociated(salesOrderLine.No.String(), salesOrderLine.Document_no.String(), salesOrderLine.Purchase_order_no.String())
	}
	checks := []struct {
		field string
		value interface{}
	}{
		{"sell_to_customer_no", salesOrderLine.Sell_to_customer_no},
		{"type", gtables.RequisitionLine_Type.Item},
		{"quantity", salesOrderLine.Outstanding_quantity},
		{"no", salesOrderLine.No},
		{"location_code", salesOrderLine.Location_code},
		{"unit_of_measure_code", salesOrderLine.Unit_of_measure_code},
		{"variant_code", salesOrderLine.Variant_code},
		{"bin_code", salesOrderLine.Bin_code},
		{"qty_per_unit_of_measure", salesOrderLine.Qty_per_unit_of_measure},
	}
	for _, check := range checks {
		err = reqLine2.TestField(check.field, check.value)
		if err != nil {
			return err
		}
	}
	salesOrderLine.Unit_cost_lcy = purchOrderLine.Unit_cost_lcy
	err = salesOrderLine.Validate("unit_cost_lcy", salesOrderLine.Unit_cost_lcy)
	if err != nil {
		return err
	}
	salesOrderLine.Purchase_order_no = purchOrderLine.Document_no
	salesOrderLine.Purch_order_line_no = purchOrderLine.Line_no
	return salesOrderLine.ModifyOrError(false)
}

// purchLineType: the purchase line type of a worksheet line's type (Account (G/L) → G/L Account)
func purchLineType(reqLineType gtables.RequisitionLineType) gtables.PurchaseLineType {
	switch reqLineType {
	case gtables.RequisitionLine_Type.AccountGL:
		return gtables.PurchaseLine_Type.GLAccount
	case gtables.RequisitionLine_Type.Item:
		return gtables.PurchaseLine_Type.Item
	}
	return gtables.PurchaseLine_Type.Blank
}

// insertHeader (InsertHeader): a new purchase order (Order Nos. of Purchases & Payables Setup) of
// the line's vendor, order address and currency with the request's dates and reference
func (m *MakeOrder) insertHeader(reqLine2 *gtables.RequisitionLine) (*gtables.PurchaseHeader, error) {
	m.orderCounter++
	m.window.Update(3, m.orderCounter)
	purchSetup := gtables.GetPurchasesPayablesSetup(reqLine2)
	err := purchSetup.TestField("order_nos")
	if err != nil {
		return nil, err
	}
	purchOrderHeader := gtables.NewPurchaseHeader(reqLine2)
	purchOrderHeader.Init()
	purchOrderHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
	purchOrderHeader.No = ""
	purchOrderHeader.Posting_date = m.postingDateReq
	err = purchOrderHeader.InsertOrError(true)
	if err != nil {
		return nil, err
	}
	purchOrderHeader.Your_reference = m.referenceReq
	purchOrderHeader.Order_date = m.orderDateReq
	purchOrderHeader.Expected_receipt_date = m.receiveDateReq
	err = purchOrderHeader.Validate("buy_from_vendor_no", reqLine2.Vendor_no)
	if err != nil {
		return nil, err
	}
	if reqLine2.Order_address_code != "" {
		err = purchOrderHeader.Validate("order_address_code", reqLine2.Order_address_code)
		if err != nil {
			return nil, err
		}
	}
	if reqLine2.Sell_to_customer_no != "" {
		err = purchOrderHeader.Validate("sell_to_customer_no", reqLine2.Sell_to_customer_no)
		if err != nil {
			return nil, err
		}
	}
	err = purchOrderHeader.Validate("currency_code", reqLine2.Currency_code)
	if err != nil {
		return nil, err
	}
	if reqLine2.Ship_to_code != "" {
		err = purchOrderHeader.Validate("ship_to_code", reqLine2.Ship_to_code)
		if err != nil {
			return nil, err
		}
	}
	err = purchOrderHeader.ModifyOrError(true)
	if err != nil {
		return nil, err
	}
	m.Orders = append(m.Orders, purchOrderHeader)
	return purchOrderHeader, nil
}

// finalizeOrderHeader (FinalizeOrderHeader): the order's worksheet lines are deleted — on a
// recurring worksheet moved on to their next order date (Variable lines without quantity)
func (m *MakeOrder) finalizeOrderHeader(purchOrderHeader *gtables.PurchaseHeader) error {
	reqLine2 := gtables.NewRequisitionLine(m.reqLine)
	reqLine2.CopyFilters(m.reqLine)
	reqLine2.SetCurrentKey("worksheet_template_name", "journal_batch_name", "vendor_no", "sell_to_customer_no", "ship_to_code", "order_address_code", "currency_code")
	reqLine2.SetRange("vendor_no", purchOrderHeader.Buy_from_vendor_no)
	reqLine2.SetRange("sell_to_customer_no", purchOrderHeader.Sell_to_customer_no)
	reqLine2.SetRange("ship_to_code", purchOrderHeader.Ship_to_code)
	reqLine2.SetRange("order_address_code", purchOrderHeader.Order_address_code)
	reqLine2.SetRange("currency_code", purchOrderHeader.Currency_code)
	if m.reqTemplate.Recurring {
		// Recurring worksheet
		for found := reqLine2.FindSetBuffered(); found; found = reqLine2.Next() {
			m.orderLineCounter++
			m.window.Update(5, m.orderLineCounter)
			if !reqLine2.Order_date.IsZero() {
				nextOrderDate, err := tables.NextRecurringDate(reqLine2.Order_date, reqLine2.Recurring_frequency)
				if err != nil {
					return err
				}
				err = reqLine2.Validate("order_date", nextOrderDate)
				if err != nil {
					return err
				}
				err = tables.SetReqLineCurrency(reqLine2, purchOrderHeader.Currency_code)
				if err != nil {
					return err
				}
			}
			if reqLine2.Recurring_method == gtables.RequisitionLine_Recurring_method.Variable && reqLine2.No != "" {
				reqLine2.Quantity = types.Decimal{}
				reqLine2.Quantity_disc_percent = types.Decimal{}
			}
			err := reqLine2.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	} else {
		// Not a recurring worksheet
		m.orderLineCounter += m.lineCount
		m.window.Update(5, m.orderLineCounter)
		reqLine2.DeleteAll()
	}
	return nil
}

// checkRecurringLine (CheckRecurringLine): a recurring worksheet's line needs its method and
// frequency (Variable: a quantity); another worksheet's line has neither
func (m *MakeOrder) checkRecurringLine(reqLine2 *gtables.RequisitionLine) error {
	if reqLine2.No == "" {
		return nil
	}
	if m.reqTemplate.Recurring {
		err := reqLine2.TestField("recurring_method")
		if err != nil {
			return err
		}
		err = reqLine2.TestField("recurring_frequency")
		if err != nil {
			return err
		}
		if reqLine2.Recurring_method == gtables.RequisitionLine_Recurring_method.Variable {
			return reqLine2.TestField("quantity")
		}
		return nil
	}
	err := reqLine2.TestField("recurring_method", gtables.RequisitionLine_Recurring_method.Blank)
	if err != nil {
		return err
	}
	return reqLine2.TestField("recurring_frequency", types.Code(""))
}

// makeRecurringText (MakeRecurringText): %1–%5 in a recurring line's description (day, week,
// month, month name, accounting period of its order date)
func (m *MakeOrder) makeRecurringText(reqLine2 *gtables.RequisitionLine) {
	if reqLine2.No != "" && reqLine2.Recurring_method != gtables.RequisitionLine_Recurring_method.Blank && !reqLine2.Order_date.IsZero() {
		_, reqLine2.Description = tables.RecurringText(reqLine2, reqLine2.Order_date, "", reqLine2.Description, 0, reqLine2.MaxStrLen("description"))
	}
}
