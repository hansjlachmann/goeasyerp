package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemRegisterLogic is the business logic of table 46 Item_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/itemregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type itemRegisterLogic struct{}

func init() { gtables.RegisterItemRegisterLogic(itemRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (itemRegisterLogic) OnInsert(rec *gtables.ItemRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (itemRegisterLogic) OnModify(rec *gtables.ItemRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (itemRegisterLogic) OnDelete(rec *gtables.ItemRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (itemRegisterLogic) OnRename(rec *gtables.ItemRegister) error {
	return nil
}
