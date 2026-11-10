package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesShipmentHeaderLogic is the business logic of table 110 Sales Shipment Header: a posted sales shipment (written by sales
// posting, codeunit 80).
type salesShipmentHeaderLogic struct{}

func init() {
	gtables.RegisterSalesShipmentHeaderLogic(salesShipmentHeaderLogic{})
	functions.Register("table", gtables.SalesShipmentHeaderTableName, "StartTrackingSite", functions.For(SalesShipmentHeaderStartTrackingSite), functions.WithDialog())
}

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

// SalesShipmentHeaderStartTrackingSite is StartTrackingSite (Track Package on the posted
// shipment): the shipping agent's tracking page for the package opens in the browser
func SalesShipmentHeaderStartTrackingSite(rec *gtables.SalesShipmentHeader) (fcodeunits.Result, error) {
	err := rec.TestField("shipping_agent_code")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = rec.TestField("package_tracking_no")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	shippingAgent := gtables.NewShippingAgent(rec)
	err = shippingAgent.GetOrError(rec.Shipping_agent_code)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = shippingAgent.TestField("internet_address")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = system.Hyperlink(GetTrackingInternetAddr(shippingAgent, rec.Package_tracking_no))
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true}, nil
}
