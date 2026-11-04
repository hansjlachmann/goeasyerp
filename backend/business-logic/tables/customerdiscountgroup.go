package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// customerDiscountGroupLogic is the business logic of table 340 Customer_Discount_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/customerdiscountgroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type customerDiscountGroupLogic struct{}

func init() { gtables.RegisterCustomerDiscountGroupLogic(customerDiscountGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (customerDiscountGroupLogic) OnInsert(rec *gtables.CustomerDiscountGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (customerDiscountGroupLogic) OnModify(rec *gtables.CustomerDiscountGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (customerDiscountGroupLogic) OnDelete(rec *gtables.CustomerDiscountGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (customerDiscountGroupLogic) OnRename(rec *gtables.CustomerDiscountGroup) error {
	return nil
}
