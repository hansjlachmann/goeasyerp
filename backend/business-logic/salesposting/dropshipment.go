package salesposting

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Drop shipments (codeunit 80): shipping a sales order line linked to a purchase order line
// receives the purchase line at the same time — a purchase item entry (the vendor's delivery) that
// the sale is applied to, the purchase receipt and the purchase order line's received quantity.
// The purchase is invoiced from the purchase order after the sale is invoiced.

// dropShipPostBuffer (Drop Shpt. Post. Buffer, table 223): what a line received on a purchase
// order line
type dropShipPostBuffer struct {
	orderNo         types.Code
	orderLineNo     int
	quantity        types.Decimal
	quantityBase    types.Decimal
	itemShptEntryNo int
}

// prepareDropShipments: shipping drop shipment lines gives each purchase order a receiving no.
func (p *SalesPost) prepareDropShipments() error {
	salesHeader := p.salesHeader
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	salesLine.SetFilter("purch_order_line_no", "<>0")
	if !salesLine.FindFirst() || !salesHeader.Ship {
		return nil
	}
	purchOrderHeader := gtables.NewPurchaseHeader(salesHeader)
	for found := salesLine.FindSetBuffered(); found; found = salesLine.Next() {
		if purchOrderHeader.No == salesLine.Purchase_order_no {
			continue
		}
		err := purchOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseHeader_Document_type.Order), "no": salesLine.Purchase_order_no})
		if err == nil {
			err = purchOrderHeader.TestField("pay_to_vendor_no")
		}
		if err != nil {
			return err
		}
		if purchOrderHeader.Receiving_no == "" {
			err = purchOrderHeader.TestField("receiving_no_series")
			if err != nil {
				return err
			}
			purchOrderHeader.Receiving_no, err = noseries.New(salesHeader).GetNextNo(purchOrderHeader.Receiving_no_series, salesHeader.Posting_date, true)
			if err != nil {
				return err
			}
			err = purchOrderHeader.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// checkDropShipmentLine: a drop shipment is an item; it ships only when it is linked to a
// purchase order line
func checkDropShipmentLine(salesLine *gtables.SalesLine, qtyToShip types.Decimal) error {
	if !salesLine.Drop_shipment {
		return nil
	}
	if salesLine.Type != gtables.SalesLine_Type.Item {
		err := salesLine.TestField("drop_shipment", false)
		if err != nil {
			return err
		}
	}
	if !qtyToShip.IsZero() && salesLine.Purch_order_line_no == 0 {
		return apperrors.DropShptNotAssociated(salesLine.Line_no)
	}
	return nil
}

// postAssocItemJnlLine (PostAssocItemJnlLine): the purchase of what the line ships, an item entry
// of the purchase order line received on the sales document's dates; returns the entry
func (p *SalesPost) postAssocItemJnlLine(salesLine *gtables.SalesLine, qtyToBeShipped, qtyToBeShippedBase types.Decimal) (int, error) {
	salesHeader := p.salesHeader
	purchOrderHeader := gtables.NewPurchaseHeader(salesHeader)
	err := purchOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseHeader_Document_type.Order), "no": salesLine.Purchase_order_no})
	if err != nil {
		return 0, err
	}
	purchOrderLine := gtables.NewPurchaseLine(salesHeader)
	err = purchOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order),
		"document_no": salesLine.Purchase_order_no, "line_no": salesLine.Purch_order_line_no})
	if err != nil {
		return 0, err
	}

	itemJnlLine := gtables.NewItemJournalLine(salesHeader)
	itemJnlLine.Init()
	itemJnlLine.Source_posting_group = purchOrderHeader.Vendor_posting_group
	itemJnlLine.Salespers_purch_code = purchOrderHeader.Purchaser_code
	itemJnlLine.Country_code = purchOrderHeader.Vat_country_code
	itemJnlLine.Reason_code = purchOrderHeader.Reason_code
	itemJnlLine.Posting_no_series = purchOrderHeader.Posting_no_series
	itemJnlLine.Item_no = purchOrderLine.No
	itemJnlLine.Description = purchOrderLine.Description
	itemJnlLine.Dimension_1_code = purchOrderLine.Dimension_1_code
	itemJnlLine.Dimension_2_code = purchOrderLine.Dimension_2_code
	itemJnlLine.Location_code = purchOrderLine.Location_code
	itemJnlLine.Inventory_posting_group = purchOrderLine.Posting_group
	itemJnlLine.Gen_bus_posting_group = purchOrderLine.Gen_bus_posting_group
	itemJnlLine.Gen_prod_posting_group = purchOrderLine.Gen_prod_posting_group
	itemJnlLine.Serial_no = purchOrderLine.Serial_no
	itemJnlLine.Transaction_type = purchOrderLine.Transaction_type
	itemJnlLine.Transport_method = purchOrderLine.Transport_method
	itemJnlLine.Entry_exit_point = purchOrderLine.Entry_point
	itemJnlLine.Area = purchOrderLine.Area
	itemJnlLine.Transaction_specification = purchOrderLine.Transaction_specification
	itemJnlLine.Drop_shipment = purchOrderLine.Drop_shipment
	itemJnlLine.Posting_date = salesHeader.Posting_date
	itemJnlLine.Document_date = salesHeader.Document_date
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.Purchase
	itemJnlLine.Document_no = purchOrderHeader.No
	itemJnlLine.External_document_no = purchOrderHeader.Receiving_no
	itemJnlLine.Quantity = qtyToBeShipped
	itemJnlLine.Quantity_base = qtyToBeShippedBase
	itemJnlLine.Invoiced_quantity = types.Decimal{}
	itemJnlLine.Invoiced_qty_base = types.Decimal{}
	itemJnlLine.Unit_cost = purchOrderLine.Unit_cost_lcy
	itemJnlLine.Unit_amount = purchOrderLine.Unit_cost_lcy
	itemJnlLine.Source_currency_code = salesHeader.Currency_code
	itemJnlLine.Source_currency_unit_cost = purchOrderLine.Unit_cost
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Vendor
	itemJnlLine.Source_no = purchOrderLine.Buy_from_vendor_no
	itemJnlLine.Source_code = p.srcCode
	itemJnlLine.Variant_code = purchOrderLine.Variant_code
	itemJnlLine.Lot_no = purchOrderLine.Lot_no
	itemJnlLine.Bin_code = purchOrderLine.Bin_code
	itemJnlLine.Unit_of_measure_code = purchOrderLine.Unit_of_measure_code
	itemJnlLine.Qty_per_unit_of_measure = purchOrderLine.Qty_per_unit_of_measure
	itemJnlLine.Applies_to_entry = 0
	itemJnlLine.Reserve = gtables.ItemJournalLineReserve(purchOrderLine.Reserve)
	if purchOrderLine.Project_no == "" {
		err = p.itemJnlPost.Run(itemJnlLine)
		if err != nil {
			return 0, err
		}
	}
	return itemJnlLine.Item_shpt_entry_no, nil
}

// insertDropShipReceipts: the purchase receipt of each purchase order the shipment received —
// the order's fields and receiving no., the sales document's dates, a line per purchase order
// line with the quantity received and its item entry
func (p *SalesPost) insertDropShipReceipts() error {
	salesHeader := p.salesHeader
	var orderNos []types.Code
	byOrder := map[types.Code][]*dropShipPostBuffer{}
	for _, buffer := range p.dropShipPostBuffer {
		if _, ok := byOrder[buffer.orderNo]; !ok {
			orderNos = append(orderNos, buffer.orderNo)
		}
		byOrder[buffer.orderNo] = append(byOrder[buffer.orderNo], buffer)
	}
	for _, orderNo := range orderNos {
		purchOrderHeader := gtables.NewPurchaseHeader(salesHeader)
		err := purchOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseHeader_Document_type.Order), "no": orderNo})
		if err != nil {
			return err
		}
		purchRcptHeader := gtables.NewPurchRcptHeader(salesHeader)
		purchRcptHeader.LockTable()
		purchRcptHeader.TransferFields(purchOrderHeader)
		purchRcptHeader.No = purchOrderHeader.Receiving_no
		purchRcptHeader.Order_no = purchOrderHeader.No
		purchRcptHeader.Posting_date = salesHeader.Posting_date
		purchRcptHeader.Document_date = salesHeader.Document_date
		purchRcptHeader.No_series = purchOrderHeader.Receiving_no_series
		purchRcptHeader.Source_code = p.srcCode
		purchRcptHeader.User_id = types.Code(system.UserId())
		purchRcptHeader.No_printed = 0
		err = purchRcptHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		for _, buffer := range byOrder[orderNo] {
			purchOrderLine := gtables.NewPurchaseLine(salesHeader)
			err = purchOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order),
				"document_no": buffer.orderNo, "line_no": buffer.orderLineNo})
			if err != nil {
				return err
			}
			purchRcptLine := gtables.NewPurchRcptLine(salesHeader)
			purchRcptLine.TransferFields(purchOrderLine)
			purchRcptLine.Document_no = purchRcptHeader.No
			purchRcptLine.Quantity = buffer.quantity
			purchRcptLine.Quantity_base = buffer.quantityBase
			purchRcptLine.Quantity_invoiced = types.Decimal{}
			purchRcptLine.Qty_invoiced_base = types.Decimal{}
			purchRcptLine.Qty_rcd_not_invoiced = buffer.quantity
			purchRcptLine.Qty_rcd_not_invd_base = buffer.quantityBase
			purchRcptLine.Order_no = purchOrderLine.Document_no
			purchRcptLine.Order_line_no = purchOrderLine.Line_no
			if !purchRcptLine.Quantity.IsZero() {
				purchRcptLine.Item_rcpt_entry_no = buffer.itemShptEntryNo
			}
			purchRcptLine.Amount = types.Decimal{}
			purchRcptLine.Amount_including_tax = types.Decimal{}
			purchRcptLine.Tax_base_amount = types.Decimal{}
			purchRcptLine.Line_discount_amount = types.Decimal{}
			purchRcptLine.Inv_discount_amount = types.Decimal{}
			err = purchRcptLine.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return p.updateAssocOrder(orderNos, byOrder)
}

// updateAssocOrder (UpdateAssocOrder): the purchase orders keep their receiving no. as the last
// one; their lines count what was received
func (p *SalesPost) updateAssocOrder(orderNos []types.Code, byOrder map[types.Code][]*dropShipPostBuffer) error {
	salesHeader := p.salesHeader
	for _, orderNo := range orderNos {
		purchOrderHeader := gtables.NewPurchaseHeader(salesHeader)
		err := purchOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseHeader_Document_type.Order), "no": orderNo})
		if err != nil {
			return err
		}
		purchOrderHeader.Last_receiving_no = purchOrderHeader.Receiving_no
		purchOrderHeader.Receiving_no = ""
		err = purchOrderHeader.ModifyOrError(false)
		if err != nil {
			return err
		}
		for _, buffer := range byOrder[orderNo] {
			purchOrderLine := gtables.NewPurchaseLine(salesHeader)
			err = purchOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order),
				"document_no": buffer.orderNo, "line_no": buffer.orderLineNo})
			if err != nil {
				return err
			}
			purchOrderLine.Quantity_received = purchOrderLine.Quantity_received.Add(buffer.quantity)
			purchOrderLine.Qty_received_base = purchOrderLine.Qty_received_base.Add(buffer.quantityBase)
			err = tables.InitPurchLineOutstanding(purchOrderLine)
			if err != nil {
				return err
			}
			tables.InitPurchLineQtyToReceive(purchOrderLine)
			err = purchOrderLine.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	p.dropShipPostBuffer = nil
	return nil
}

// updateAssocLines (UpdateAssocLines): a sales order line invoiced completely is no longer linked
// to its purchase order line (nor the purchase line to it)
func updateAssocLines(salesOrderLine *gtables.SalesLine) error {
	purchOrderLine := gtables.NewPurchaseLine(salesOrderLine)
	err := purchOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.Order),
		"document_no": salesOrderLine.Purchase_order_no, "line_no": salesOrderLine.Purch_order_line_no})
	if err != nil {
		return err
	}
	purchOrderLine.Sales_order_no = ""
	purchOrderLine.Sales_order_line_no = 0
	err = purchOrderLine.ModifyOrError(false)
	if err != nil {
		return err
	}
	salesOrderLine.Purchase_order_no = ""
	salesOrderLine.Purch_order_line_no = 0
	return nil
}
