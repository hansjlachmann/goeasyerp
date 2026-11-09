// Package purchgetdropshpt is codeunit 76 Purch.-Get Drop Shpt. (Get Sales Order on a purchase
// order): the drop shipment lines of a sales order of the purchase order's customer become
// purchase order lines linked to them.
package purchgetdropshpt

import (
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables" // the table triggers
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesOrdersPageID: the list the user picks the sales order from (classic Sales List)
const SalesOrdersPageID = 9305

// SelectSalesOrder: the sales orders of the purchase order's customer on the sales order list in
// lookup mode; the order the user picked
func SelectSalesOrder(purchHeader *gtables.PurchaseHeader) (types.Code, bool, error) {
	err := purchHeader.TestField("document_type", gtables.PurchaseHeader_Document_type.Order)
	if err != nil {
		return "", false, err
	}
	salesOrderHeader := gtables.NewSalesHeader(purchHeader)
	salesOrderHeader.SetCurrentKey("document_type", "sell_to_customer_no")
	salesOrderHeader.SetRange("document_type", gtables.SalesHeader_Document_type.Order)
	salesOrderHeader.SetRange("sell_to_customer_no", purchHeader.Sell_to_customer_no)
	if system.RunModal(SalesOrdersPageID, salesOrderHeader, true) != system.ActionLookupOK || salesOrderHeader.No == "" {
		return "", false, nil
	}
	return salesOrderHeader.No, true, nil
}

// Code (Code): the purchase order gets a line for each drop shipment line of the sales order
// with an outstanding quantity not yet linked — the item, unit of measure and quantity, the
// line's shipment date as expected receipt date, its location, variant, bin, serial and lot
// number — and the sales order line is linked to it and costs what it costs
func Code(purchHeader *gtables.PurchaseHeader, salesOrderNo types.Code) error {
	err := purchHeader.TestField("document_type", gtables.PurchaseHeader_Document_type.Order)
	if err != nil {
		return err
	}
	salesOrderHeader := gtables.NewSalesHeader(purchHeader)
	err = salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": salesOrderNo})
	if err != nil {
		return err
	}
	err = salesOrderHeader.TestField("document_type", gtables.SalesHeader_Document_type.Order)
	if err == nil {
		err = purchHeader.TestField("sell_to_customer_no", salesOrderHeader.Sell_to_customer_no)
	}
	if err == nil {
		err = purchHeader.TestField("ship_to_code", salesOrderHeader.Ship_to_code)
	}
	if err != nil {
		return err
	}

	purchOrderLine := gtables.NewPurchaseLine(purchHeader)
	purchOrderLine.LockTable()
	salesOrderLine := gtables.NewSalesLine(purchHeader)
	salesOrderLine.LockTable()

	purchOrderLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchOrderLine.SetRange("document_no", purchHeader.No)
	nextLineNo := 10000
	if purchOrderLine.FindLast() {
		nextLineNo = purchOrderLine.Line_no + 10000
	}

	salesOrderLine.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	salesOrderLine.SetRange("document_no", salesOrderHeader.No)
	salesOrderLine.SetRange("drop_shipment", true)
	salesOrderLine.SetFilter("outstanding_quantity", "<>0")
	salesOrderLine.SetRange("type", gtables.SalesLine_Type.Item)
	salesOrderLine.SetFilter("no", "<>''")
	salesOrderLine.SetRange("purch_order_line_no", 0)
	var salesOrderLines []*gtables.SalesLine
	for found := salesOrderLine.FindSetBuffered(); found; found = salesOrderLine.Next() {
		line := gtables.NewSalesLine(purchHeader)
		line.Copy(salesOrderLine)
		salesOrderLines = append(salesOrderLines, line)
	}
	if len(salesOrderLines) == 0 {
		return apperrors.DropShptNoLinesToRetrieve(salesOrderHeader.No.String())
	}
	for _, salesOrderLine := range salesOrderLines {
		purchOrderLine := gtables.NewPurchaseLine(purchHeader)
		purchOrderLine.Init()
		purchOrderLine.Document_type = gtables.PurchaseLine_Document_type.Order
		purchOrderLine.Document_no = purchHeader.No
		purchOrderLine.Line_no = nextLineNo
		err = purchOrderLine.Validate("type", gtables.PurchaseLine_Type.Item)
		if err == nil {
			err = purchOrderLine.Validate("no", salesOrderLine.No)
		}
		if err == nil {
			err = purchOrderLine.Validate("unit_of_measure_code", salesOrderLine.Unit_of_measure_code)
		}
		if err == nil {
			err = purchOrderLine.Validate("quantity", salesOrderLine.Outstanding_quantity)
		}
		if err == nil {
			err = purchOrderLine.Validate("direct_unit_cost", purchOrderLine.Direct_unit_cost)
		}
		if err != nil {
			return err
		}
		purchOrderLine.Expected_receipt_date = salesOrderLine.Shipment_date
		purchOrderLine.Location_code = salesOrderLine.Location_code
		purchOrderLine.Serial_no = salesOrderLine.Serial_no
		purchOrderLine.Lot_no = salesOrderLine.Lot_no
		purchOrderLine.Variant_code = salesOrderLine.Variant_code
		purchOrderLine.Bin_code = salesOrderLine.Bin_code
		purchOrderLine.Sales_order_no = salesOrderLine.Document_no
		purchOrderLine.Sales_order_line_no = salesOrderLine.Line_no
		purchOrderLine.Drop_shipment = true
		err = purchOrderLine.InsertOrError(false)
		if err != nil {
			return err
		}
		nextLineNo += 10000

		salesOrderLine.Unit_cost_lcy = purchOrderLine.Unit_cost_lcy
		err = salesOrderLine.Validate("unit_cost_lcy", salesOrderLine.Unit_cost_lcy)
		if err != nil {
			return err
		}
		salesOrderLine.Purchase_order_no = purchOrderLine.Document_no
		salesOrderLine.Purch_order_line_no = purchOrderLine.Line_no
		err = salesOrderLine.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
