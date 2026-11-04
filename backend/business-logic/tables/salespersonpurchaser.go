package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salespersonPurchaserLogic is the business logic of table 13 Salesperson_Purchaser.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/salespersonpurchaser_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type salespersonPurchaserLogic struct{}

func init() { gtables.RegisterSalespersonPurchaserLogic(salespersonPurchaserLogic{}) }

// OnInsert runs before a new record is inserted
func (salespersonPurchaserLogic) OnInsert(rec *gtables.SalespersonPurchaser) error {
	return nil
}

// OnModify runs before a record is modified
func (salespersonPurchaserLogic) OnModify(rec *gtables.SalespersonPurchaser) error {
	return nil
}

// OnDelete runs before a record is deleted
func (salespersonPurchaserLogic) OnDelete(rec *gtables.SalespersonPurchaser) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (salespersonPurchaserLogic) OnRename(rec *gtables.SalespersonPurchaser) error {
	return nil
}
