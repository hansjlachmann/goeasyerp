package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesShipmentLineLogic is the business logic of table 111 Sales Shipment Line: a line of a posted sales shipment (written by
// sales posting, codeunit 80); it has no triggers of its own.
type salesShipmentLineLogic struct{}

func init() { gtables.RegisterSalesShipmentLineLogic(salesShipmentLineLogic{}) }
