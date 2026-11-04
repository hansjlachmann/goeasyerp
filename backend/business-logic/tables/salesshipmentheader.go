package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesShipmentHeaderLogic is the business logic of table 110 Sales Shipment Header: a posted sales shipment (written by sales
// posting, codeunit 80).
type salesShipmentHeaderLogic struct{}

func init() { gtables.RegisterSalesShipmentHeaderLogic(salesShipmentHeaderLogic{}) }

// OnDelete: the document's lines and comments
func (salesShipmentHeaderLogic) OnDelete(rec *gtables.SalesShipmentHeader) error {
	salesShipmentLine := gtables.NewSalesShipmentLine(rec)
	salesShipmentLine.SetRange("document_no", rec.No)
	salesShipmentLine.DeleteAll()
	salesCommentLine := gtables.NewSalesCommentLine(rec)
	salesCommentLine.SetRange("document_type", gtables.SalesCommentLine_Document_type.Shipment)
	salesCommentLine.SetRange("no", rec.No)
	salesCommentLine.DeleteAll()
	return nil
}
