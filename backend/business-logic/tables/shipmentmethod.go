package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// shipmentMethodLogic is the business logic of table 10 Shipment_Method.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/shipmentmethod_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type shipmentMethodLogic struct{}

func init() { gtables.RegisterShipmentMethodLogic(shipmentMethodLogic{}) }

// OnInsert runs before a new record is inserted
func (shipmentMethodLogic) OnInsert(rec *gtables.ShipmentMethod) error {
	return nil
}

// OnModify runs before a record is modified
func (shipmentMethodLogic) OnModify(rec *gtables.ShipmentMethod) error {
	return nil
}

// OnDelete runs before a record is deleted
func (shipmentMethodLogic) OnDelete(rec *gtables.ShipmentMethod) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (shipmentMethodLogic) OnRename(rec *gtables.ShipmentMethod) error {
	return nil
}
