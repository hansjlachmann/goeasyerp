package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// priceGroupLogic is the business logic of table 6 Price_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/pricegroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type priceGroupLogic struct{}

func init() { gtables.RegisterPriceGroupLogic(priceGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (priceGroupLogic) OnInsert(rec *gtables.PriceGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (priceGroupLogic) OnModify(rec *gtables.PriceGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (priceGroupLogic) OnDelete(rec *gtables.PriceGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (priceGroupLogic) OnRename(rec *gtables.PriceGroup) error {
	return nil
}
