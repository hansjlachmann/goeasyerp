package purchposting

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Drop shipments (codeunit 90): receiving a purchase order line linked to a sales order line
// ships the sales line at the same time — a sale item entry applied to the purchase entry, the
// sales shipment and the sales order line's shipped quantity. A drop shipment is received and
// invoiced in two postings, and the purchase is invoiced after the sale.

// dropShipPostBuffer (Drop Shpt. Post. Buffer, table 223): what a line shipped on a sales order
// line
type dropShipPostBuffer struct {
	orderNo         types.Code
	orderLineNo     int
	quantity        types.Decimal
	quantityBase    types.Decimal
	itemShptEntryNo int
}

// checkDropShipments: invoicing — a line linked to a sales order line is not received and
// invoiced at once, and leaves at least what the sale still has to invoice; receiving — each
// sales order gets a shipping no.
func (p *PurchPost) checkDropShipments() error {
	purchHeader := p.purchHeader
	purchLine := gtables.NewPurchaseLine(purchHeader)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	purchLine.SetFilter("sales_order_line_no", "<>0")
	if !purchLine.FindFirst() {
		return nil
	}
	if purchHeader.Invoice {
		for found := purchLine.FindSetBuffered(); found; found = purchLine.Next() {
			salesOrderLine := gtables.NewSalesLine(purchHeader)
			err := salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
				"document_no": purchLine.Sales_order_no, "line_no": purchLine.Sales_order_line_no})
			if err != nil {
				return err
			}
			qtyToReceive, _, qtyToInvoice, _ := p.qtysToPost(purchLine)
			if purchHeader.Receive && purchHeader.Invoice && !qtyToInvoice.IsZero() && !qtyToReceive.IsZero() {
				return apperrors.DropShptReceiveAndInvoice()
			}
			if purchLine.Quantity_received.Sub(purchLine.Quantity_invoiced).Abs().LessThan(qtyToInvoice.Abs()) {
				qtyToInvoice = purchLine.Quantity_received.Sub(purchLine.Quantity_invoiced)
			}
			if purchLine.Quantity.Sub(qtyToInvoice.Add(purchLine.Quantity_invoiced)).Abs().LessThan(salesOrderLine.Quantity.Sub(salesOrderLine.Quantity_invoiced).Abs()) {
				return apperrors.DropShptInvoiceSalesFirst(purchLine.Sales_order_no.String())
			}
		}
	}
	if !purchHeader.Receive {
		return nil
	}
	salesOrderHeader := gtables.NewSalesHeader(purchHeader)
	for found := purchLine.FindSetBuffered(); found; found = purchLine.Next() {
		if salesOrderHeader.No == purchLine.Sales_order_no {
			continue
		}
		err := salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": purchLine.Sales_order_no})
		if err == nil {
			err = salesOrderHeader.TestField("bill_to_customer_no")
		}
		if err != nil {
			return err
		}
		if salesOrderHeader.Shipping_no == "" {
			err = salesOrderHeader.TestField("shipping_no_series")
			if err != nil {
				return err
			}
			salesOrderHeader.Shipping_no, err = noseries.New(purchHeader).GetNextNo(salesOrderHeader.Shipping_no_series, purchHeader.Posting_date, true)
			if err != nil {
				return err
			}
			err = salesOrderHeader.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// postAssocItemJnlLine (PostAssocItemJnlLine): the sale of what the line receives, an item entry
// of the sales order line shipped on the purchase document's dates, applied to the purchase
// entry; returns the entry
func (p *PurchPost) postAssocItemJnlLine(purchLine *gtables.PurchaseLine, qtyToBeShipped, qtyToBeShippedBase types.Decimal, itemLedgShptEntryNo int) (int, error) {
	purchHeader := p.purchHeader
	salesOrderHeader := gtables.NewSalesHeader(purchHeader)
	err := salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": purchLine.Sales_order_no})
	if err != nil {
		return 0, err
	}
	salesOrderLine := gtables.NewSalesLine(purchHeader)
	err = salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
		"document_no": purchLine.Sales_order_no, "line_no": purchLine.Sales_order_line_no})
	if err != nil {
		return 0, err
	}

	itemJnlLine := gtables.NewItemJournalLine(purchHeader)
	itemJnlLine.Init()
	itemJnlLine.Source_posting_group = salesOrderHeader.Customer_posting_group
	itemJnlLine.Salespers_purch_code = salesOrderHeader.Salesperson_code
	itemJnlLine.Country_code = salesOrderHeader.Vat_country_code
	itemJnlLine.Reason_code = salesOrderHeader.Reason_code
	itemJnlLine.Posting_no_series = salesOrderHeader.Posting_no_series
	itemJnlLine.Item_no = salesOrderLine.No
	itemJnlLine.Description = salesOrderLine.Description
	itemJnlLine.Dimension_1_code = salesOrderLine.Dimension_1_code
	itemJnlLine.Dimension_2_code = salesOrderLine.Dimension_2_code
	itemJnlLine.Location_code = salesOrderLine.Location_code
	itemJnlLine.Inventory_posting_group = salesOrderLine.Posting_group
	itemJnlLine.Gen_bus_posting_group = salesOrderLine.Gen_bus_posting_group
	itemJnlLine.Gen_prod_posting_group = salesOrderLine.Gen_prod_posting_group
	itemJnlLine.Serial_no = salesOrderLine.Serial_no
	itemJnlLine.Transaction_type = salesOrderLine.Transaction_type
	itemJnlLine.Transport_method = salesOrderLine.Transport_method
	itemJnlLine.Entry_exit_point = salesOrderLine.Exit_point
	itemJnlLine.Area = salesOrderLine.Area
	itemJnlLine.Transaction_specification = salesOrderLine.Transaction_specification
	itemJnlLine.Drop_shipment = salesOrderLine.Drop_shipment
	itemJnlLine.Posting_date = purchHeader.Posting_date
	itemJnlLine.Document_date = purchHeader.Document_date
	itemJnlLine.Entry_type = gtables.ItemJournalLine_Entry_type.Sale
	itemJnlLine.Document_no = salesOrderHeader.Shipping_no
	itemJnlLine.External_document_no = salesOrderHeader.External_document_no
	itemJnlLine.Quantity = qtyToBeShipped
	itemJnlLine.Quantity_base = qtyToBeShippedBase
	itemJnlLine.Invoiced_quantity = types.Decimal{}
	itemJnlLine.Invoiced_qty_base = types.Decimal{}
	itemJnlLine.Unit_cost = salesOrderLine.Unit_cost_lcy
	itemJnlLine.Unit_amount = salesOrderLine.Unit_price
	itemJnlLine.Source_currency_code = purchHeader.Currency_code
	itemJnlLine.Source_currency_unit_cost = salesOrderLine.Unit_cost
	itemJnlLine.Source_type = gtables.ItemJournalLine_Source_type.Customer
	itemJnlLine.Source_no = salesOrderLine.Sell_to_customer_no
	itemJnlLine.Source_code = p.srcCode
	itemJnlLine.Variant_code = salesOrderLine.Variant_code
	itemJnlLine.Lot_no = salesOrderLine.Lot_no
	itemJnlLine.Bin_code = salesOrderLine.Bin_code
	itemJnlLine.Unit_of_measure_code = salesOrderLine.Unit_of_measure_code
	itemJnlLine.Qty_per_unit_of_measure = salesOrderLine.Qty_per_unit_of_measure
	itemJnlLine.Applies_to_entry = itemLedgShptEntryNo
	itemJnlLine.Reserve = gtables.ItemJournalLineReserve(salesOrderLine.Reserve)
	if salesOrderLine.Appl_to_project_entry == 0 && salesOrderLine.Project_applies_to_id == "" {
		err = p.itemJnlPost.Run(itemJnlLine)
		if err != nil {
			return 0, err
		}
	}
	return itemJnlLine.Item_shpt_entry_no, nil
}

// insertDropShipShipments: the sales shipment of each sales order the receipt shipped — the
// order's fields and shipping no., the purchase document's dates, a line per sales order line
// with the quantity shipped and its item entry
func (p *PurchPost) insertDropShipShipments() error {
	purchHeader := p.purchHeader
	var orderNos []types.Code
	byOrder := map[types.Code][]*dropShipPostBuffer{}
	for _, buffer := range p.dropShipPostBuffer {
		if _, ok := byOrder[buffer.orderNo]; !ok {
			orderNos = append(orderNos, buffer.orderNo)
		}
		byOrder[buffer.orderNo] = append(byOrder[buffer.orderNo], buffer)
	}
	for _, orderNo := range orderNos {
		salesOrderHeader := gtables.NewSalesHeader(purchHeader)
		err := salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": orderNo})
		if err != nil {
			return err
		}
		salesShptHeader := gtables.NewSalesShipmentHeader(purchHeader)
		salesShptHeader.LockTable()
		salesShptHeader.TransferFields(salesOrderHeader)
		salesShptHeader.No = salesOrderHeader.Shipping_no
		salesShptHeader.Order_no = salesOrderHeader.No
		salesShptHeader.Posting_date = purchHeader.Posting_date
		salesShptHeader.Document_date = purchHeader.Document_date
		salesShptHeader.No_series = salesOrderHeader.Shipping_no_series
		salesShptHeader.Source_code = p.srcCode
		salesShptHeader.User_id = types.Code(system.UserId())
		salesShptHeader.No_printed = 0
		err = salesShptHeader.InsertOrError(false)
		if err != nil {
			return err
		}
		for _, buffer := range byOrder[orderNo] {
			salesOrderLine := gtables.NewSalesLine(purchHeader)
			err = salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
				"document_no": buffer.orderNo, "line_no": buffer.orderLineNo})
			if err != nil {
				return err
			}
			salesShptLine := gtables.NewSalesShipmentLine(purchHeader)
			salesShptLine.TransferFields(salesOrderLine)
			salesShptLine.Document_no = salesShptHeader.No
			salesShptLine.Quantity = buffer.quantity
			salesShptLine.Quantity_base = buffer.quantityBase
			salesShptLine.Quantity_invoiced = types.Decimal{}
			salesShptLine.Qty_invoiced_base = types.Decimal{}
			salesShptLine.Qty_shipped_not_invoiced = buffer.quantity
			salesShptLine.Qty_shpd_not_invd_base = buffer.quantityBase
			salesShptLine.Order_no = salesOrderLine.Document_no
			salesShptLine.Order_line_no = salesOrderLine.Line_no
			if !salesShptLine.Quantity.IsZero() {
				salesShptLine.Item_shpt_entry_no = buffer.itemShptEntryNo
			}
			salesShptLine.Amount = types.Decimal{}
			salesShptLine.Amount_including_tax = types.Decimal{}
			salesShptLine.Tax_base_amount = types.Decimal{}
			salesShptLine.Line_discount_amount = types.Decimal{}
			salesShptLine.Inv_discount_amount = types.Decimal{}
			err = salesShptLine.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return p.updateAssocOrder(orderNos, byOrder)
}

// updateAssocOrder (UpdateAssocOrder): the sales orders keep their shipping no. as the last one;
// their lines count what was shipped
func (p *PurchPost) updateAssocOrder(orderNos []types.Code, byOrder map[types.Code][]*dropShipPostBuffer) error {
	purchHeader := p.purchHeader
	for _, orderNo := range orderNos {
		salesOrderHeader := gtables.NewSalesHeader(purchHeader)
		err := salesOrderHeader.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesHeader_Document_type.Order), "no": orderNo})
		if err != nil {
			return err
		}
		salesOrderHeader.Last_shipping_no = salesOrderHeader.Shipping_no
		salesOrderHeader.Shipping_no = ""
		err = salesOrderHeader.ModifyOrError(false)
		if err != nil {
			return err
		}
		for _, buffer := range byOrder[orderNo] {
			salesOrderLine := gtables.NewSalesLine(purchHeader)
			err = salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
				"document_no": buffer.orderNo, "line_no": buffer.orderLineNo})
			if err != nil {
				return err
			}
			salesOrderLine.Quantity_shipped = salesOrderLine.Quantity_shipped.Add(buffer.quantity)
			salesOrderLine.Qty_shipped_base = salesOrderLine.Qty_shipped_base.Add(buffer.quantityBase)
			err = tables.InitSalesLineOutstanding(salesOrderLine)
			if err != nil {
				return err
			}
			tables.InitSalesLineQtyToShip(salesOrderLine)
			err = salesOrderLine.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	p.dropShipPostBuffer = nil
	return nil
}

// checkDropShipmentReceiptInvoice: invoicing a received drop shipment leaves at least what the
// sale still has to invoice
func checkDropShipmentReceiptInvoice(purchOrderLine *gtables.PurchaseLine) error {
	if purchOrderLine.Sales_order_line_no == 0 {
		return nil
	}
	salesOrderLine := gtables.NewSalesLine(purchOrderLine)
	err := salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
		"document_no": purchOrderLine.Sales_order_no, "line_no": purchOrderLine.Sales_order_line_no})
	if err != nil {
		return err
	}
	if purchOrderLine.Quantity.Sub(purchOrderLine.Quantity_invoiced).Abs().LessThan(salesOrderLine.Quantity.Sub(salesOrderLine.Quantity_invoiced).Abs()) {
		return apperrors.DropShptInvoiceSalesFirst(purchOrderLine.Sales_order_no.String())
	}
	return nil
}
