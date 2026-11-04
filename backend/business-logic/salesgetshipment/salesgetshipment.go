// Package salesgetshipment is codeunit 64 Sales-Get Shipment: an invoice gets the lines of the
// customer's posted shipments that are not invoiced yet; posting the invoice invoices them.
package salesgetshipment

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// GetShipmentLinesPageID is page 5708 Get Shipment Lines
const GetShipmentLinesPageID = 5708

// Run (OnRun): the shipment the user picks (SelectShipment), its lines onto the invoice
// (CreateInvLines); false when the user picked none
func Run(salesHeader *gtables.SalesHeader, shipmentDescription string) (bool, error) {
	shipmentNo, picked, err := SelectShipment(salesHeader)
	if err != nil || !picked {
		return false, err
	}
	return true, CreateInvLines(salesHeader, shipmentNo, shipmentDescription)
}

// SelectShipment: the bill-to customer's shipment lines in the invoice's currency that are not
// invoiced, on page 5708 in lookup mode; the shipment of the line the user picked
func SelectShipment(salesHeader *gtables.SalesHeader) (types.Code, bool, error) {
	err := salesHeader.TestField("document_type", gtables.SalesHeader_Document_type.Invoice)
	if err != nil {
		return "", false, err
	}
	err = salesHeader.TestField("bill_to_customer_no")
	if err != nil {
		return "", false, err
	}
	salesShptLine := gtables.NewSalesShipmentLine(salesHeader)
	salesShptLine.SetRange("bill_to_customer_no", salesHeader.Bill_to_customer_no)
	salesShptLine.SetFilter("qty_shipped_not_invoiced", "<>0")
	salesShptLine.SetRange("currency_code", salesHeader.Currency_code)
	if system.RunModal(GetShipmentLinesPageID, salesShptLine, true) != system.ActionLookupOK {
		return "", false, nil
	}
	return salesShptLine.Document_no, true, nil
}

// CreateInvLines (CreateInvLines): a description line "Shipment No. …:" and an invoice line for
// each line of the shipment not yet invoiced, after the invoice's last line
func CreateInvLines(salesHeader *gtables.SalesHeader, shipmentNo types.Code, shipmentDescription string) error {
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	nextLineNo := 10000
	if salesLine.FindLast() {
		nextLineNo = salesLine.Line_no + 10000
	}
	salesShptLine := gtables.NewSalesShipmentLine(salesHeader)
	salesShptLine.SetRange("document_no", shipmentNo)
	salesShptLine.SetFilter("qty_shipped_not_invoiced", "<>0")
	var shptLines []*gtables.SalesShipmentLine
	for found := salesShptLine.FindSet(); found; found = salesShptLine.Next() {
		line := gtables.NewSalesShipmentLine(salesHeader)
		line.Copy(salesShptLine)
		shptLines = append(shptLines, line)
	}
	if len(shptLines) == 0 {
		return nil
	}
	descriptionLine := gtables.NewSalesLine(salesHeader)
	descriptionLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	descriptionLine.Document_no = salesHeader.No
	descriptionLine.Line_no = nextLineNo
	descriptionLine.Description = types.Text(system.CopyStr(system.StrSubstNo(shipmentDescription, shipmentNo), 1, descriptionLine.MaxStrLen("description")))
	err := descriptionLine.InsertOrError(false)
	if err != nil {
		return err
	}
	for _, shptLine := range shptLines {
		nextLineNo = nextLineNo + 10000
		err = insertInvLineFromShptLine(salesHeader, shptLine, nextLineNo)
		if err != nil {
			return err
		}
	}
	return nil
}

// insertInvLineFromShptLine (InsertInvLineFromShptLine): the order line the shipment line came
// from, on the invoice for the quantity shipped and not invoiced, with the shipment's number
func insertInvLineFromShptLine(salesHeader *gtables.SalesHeader, salesShptLine *gtables.SalesShipmentLine, lineNo int) error {
	salesOrderLine := gtables.NewSalesLine(salesHeader)
	err := salesOrderLine.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.Order), "document_no": salesShptLine.Order_no, "line_no": salesShptLine.Order_line_no})
	if err != nil {
		return err
	}
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.TransferFields(salesOrderLine)
	salesLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = lineNo
	salesLine.Shipment_no = salesShptLine.Document_no
	salesLine.Shipment_line_no = salesShptLine.Line_no
	salesLine.Quantity_shipped = types.Decimal{}
	salesLine.Qty_shipped_base = types.Decimal{}
	salesLine.Quantity_invoiced = types.Decimal{}
	salesLine.Qty_invoiced_base = types.Decimal{}
	salesLine.Qty_shipped_not_invoiced = types.Decimal{}
	salesLine.Qty_shpd_not_invd_base = types.Decimal{}
	err = salesLine.Validate("quantity", salesShptLine.Qty_shipped_not_invoiced)
	if err != nil {
		return err
	}
	return salesLine.InsertOrError(false)
}
