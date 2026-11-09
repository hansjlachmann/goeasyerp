package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Drop shipments (classic): a sales order line marked Drop Shipment is bought from the vendor
// and delivered by the vendor straight to the customer. The purchase order line that buys it
// (Get Sales Order, Make Orders) links the two lines — Sales Line 71/72 Purchase Order No. /
// Purch. Order Line No., Purchase Line 71/72/73 Sales Order No. / Sales Order Line No. / Drop
// Shipment — and posting the shipment of one posts the receipt of the other. A linked line
// keeps its type, item, location, quantity, variant, bin and unit of measure; the purchase
// order's Sell-to Customer No. and Ship-to Code are the customer's address it delivers to.

// testSalesLineNotAssociated (IF "Purch. Order Line No." <> 0 THEN ERROR): a sales order line
// linked to a purchase order line keeps the field
func testSalesLineNotAssociated(rec *gtables.SalesLine, field string) error {
	if rec.Purch_order_line_no != 0 {
		return apperrors.SalesLineAssocPurchOrder(gtables.SalesLineTableName, field, rec.Purchase_order_no.String())
	}
	return nil
}

// testPurchLineNotDropShipment (IF "Drop Shipment" THEN ERROR): a drop shipment purchase line
// keeps the field
func testPurchLineNotDropShipment(rec *gtables.PurchaseLine, field string) error {
	if rec.Drop_shipment {
		return apperrors.PurchLineAssocSalesOrder(gtables.PurchaseLineTableName, field, rec.Sales_order_no.String())
	}
	return nil
}

// OnValidate_Drop_shipment: only an item line of an order not shipped yet and not for a project;
// not once it is linked to a purchase order line; a reserved line cannot be a drop shipment
func (salesLineLogic) OnValidate_Drop_shipment(rec *gtables.SalesLine) error {
	err := rec.TestField("document_type", gtables.SalesLine_Document_type.Order)
	if err == nil {
		err = rec.TestField("type", gtables.SalesLine_Type.Item)
	}
	if err == nil {
		err = rec.TestField("quantity_shipped", types.Decimal{})
	}
	if err == nil {
		err = rec.TestField("project_no", "")
	}
	if err != nil {
		return err
	}
	err = testSalesLineNotAssociated(rec, "drop_shipment")
	if err != nil {
		return err
	}
	err = dropShipmentReservedSalesLine(rec)
	if err != nil {
		return err
	}
	if rec.Reserve != gtables.SalesLine_Reserve.Always {
		return checkSalesLineItemAvailable(rec)
	}
	return nil
}

// dropShipmentReservedSalesLine (SalesLineVerifyChange): a reserved line cannot be a drop
// shipment
func dropShipmentReservedSalesLine(rec *gtables.SalesLine) error {
	if rec.Drop_shipment && !rec.Reserved_qty_base.IsZero() {
		return apperrors.DropShptReserved(gtables.SalesLineTableName, "drop_shipment")
	}
	return nil
}

// dropShipmentReservedPurchLine (PurchLineVerifyChange): a reserved line cannot be a drop
// shipment
func dropShipmentReservedPurchLine(rec *gtables.PurchaseLine) error {
	if rec.Reserved_qty_base.IsZero() {
		return nil
	}
	if rec.Sales_order_no != "" {
		return apperrors.DropShptReserved(gtables.PurchaseLineTableName, "sales_order_no")
	}
	if rec.Sales_order_line_no != 0 {
		return apperrors.DropShptReserved(gtables.PurchaseLineTableName, "sales_order_line_no")
	}
	if rec.Drop_shipment {
		return apperrors.DropShptReserved(gtables.PurchaseLineTableName, "drop_shipment")
	}
	return nil
}

// unlinkDropShipmentSalesLine (Purchase Line OnDelete): the sales order line of a deleted drop
// shipment purchase line is no longer linked
func unlinkDropShipmentSalesLine(rec *gtables.PurchaseLine) error {
	if rec.Sales_order_line_no == 0 {
		return nil
	}
	rec.LockTable()
	salesOrderLine := gtables.NewSalesLine(rec)
	salesOrderLine.LockTable()
	err := salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
		"document_no": rec.Sales_order_no, "line_no": rec.Sales_order_line_no})
	if err != nil {
		return err
	}
	salesOrderLine.Purchase_order_no = ""
	salesOrderLine.Purch_order_line_no = 0
	return salesOrderLine.ModifyOrError(false)
}

// updatePurchLineSalesCost (UpdateSalesCost): the sales order line of a drop shipment costs
// what the purchase line costs; saved when the user changed the purchase line
func updatePurchLineSalesCost(rec *gtables.PurchaseLine) error {
	if rec.Sales_order_line_no == 0 {
		return nil
	}
	salesOrderLine := gtables.NewSalesLine(rec)
	err := salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order),
		"document_no": rec.Sales_order_no, "line_no": rec.Sales_order_line_no})
	if err != nil {
		return err
	}
	salesOrderLine.Unit_cost_lcy = rec.Unit_cost_lcy
	salesOrderLine.Unit_cost = rec.Unit_cost
	err = salesOrderLine.Validate("unit_cost_lcy", salesOrderLine.Unit_cost_lcy)
	if err != nil {
		return err
	}
	if fcodeunits.CurrFieldNo() != 0 {
		return salesOrderLine.ModifyOrError(false)
	}
	return nil
}

// purchHeaderDropShipmentLinesExist: an order with lines delivering sales orders keeps its
// customer and ship-to address
func testPurchHeaderNoDropShipments(rec *gtables.PurchaseHeader, field string) error {
	if rec.Document_type != gtables.PurchaseHeader_Document_type.Order {
		return nil
	}
	purchLine := gtables.NewPurchaseLine(rec)
	purchLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchLine.SetRange("document_no", rec.No)
	purchLine.SetFilter("sales_order_line_no", "<>0")
	if purchLine.FindFirst() {
		return apperrors.PurchHeaderAssocSales(gtables.PurchaseHeaderTableName, field)
	}
	return nil
}

// OnValidate_Sell_to_customer_no: blank — the goods come to the company (its ship-to address
// and location); a customer — the customer's address (Ship-to Code blank)
func (purchaseHeaderLogic) OnValidate_Sell_to_customer_no(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.Sell_to_customer_no != xRec.Sell_to_customer_no {
		err := testPurchHeaderNoDropShipments(rec, "sell_to_customer_no")
		if err != nil {
			return err
		}
	}
	if rec.Sell_to_customer_no == "" {
		rec.Ship_to_code = ""
		setPurchShipToCompany(rec)
		return nil
	}
	return rec.Validate("ship_to_code", "")
}

// OnValidate_Ship_to_code: the customer's ship-to address (blank: the customer's own address),
// its shipment method and location
func (purchaseHeaderLogic) OnValidate_Ship_to_code(rec *gtables.PurchaseHeader) error {
	xRec := rec.XRec()
	if rec.Ship_to_code != xRec.Ship_to_code {
		err := testPurchHeaderNoDropShipments(rec, "ship_to_code")
		if err != nil {
			return err
		}
	}
	if rec.Ship_to_code != "" {
		shipToAddr := gtables.NewShipToAddress(rec)
		err := shipToAddr.GetOrError(map[string]interface{}{"customer_no": rec.Sell_to_customer_no, "code": rec.Ship_to_code})
		if err != nil {
			return err
		}
		rec.Ship_to_name = shipToAddr.Name
		rec.Ship_to_name_2 = shipToAddr.Name_2
		rec.Ship_to_address = shipToAddr.Address
		rec.Ship_to_address_2 = shipToAddr.Address_2
		rec.Ship_to_city = shipToAddr.City
		rec.Ship_to_post_code = shipToAddr.Post_code
		rec.Ship_to_state = shipToAddr.State
		rec.Ship_to_country_code = shipToAddr.Country_code
		rec.Ship_to_contact = shipToAddr.Contact
		rec.Shipment_method_code = shipToAddr.Shipment_method_code
		if rec.Location_code != shipToAddr.Location_code {
			return rec.Validate("location_code", shipToAddr.Location_code)
		}
	} else {
		err := rec.TestField("sell_to_customer_no")
		if err != nil {
			return err
		}
		cust := gtables.NewCustomer(rec)
		err = cust.GetOrError(rec.Sell_to_customer_no)
		if err != nil {
			return err
		}
		rec.Ship_to_name = cust.Name
		rec.Ship_to_name_2 = cust.Name_2
		rec.Ship_to_address = cust.Address
		rec.Ship_to_address_2 = cust.Address_2
		rec.Ship_to_city = cust.City
		rec.Ship_to_post_code = cust.Post_code
		rec.Ship_to_state = cust.State
		rec.Ship_to_country_code = cust.Country_code
		rec.Ship_to_contact = cust.Contact
		rec.Shipment_method_code = cust.Shipment_method_code
		if rec.Location_code != cust.Location_code {
			return rec.Validate("location_code", cust.Location_code)
		}
	}
	return nil
}
