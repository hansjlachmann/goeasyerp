package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourceRegisterLogic is the business logic of table 240 Resource_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/resourceregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type resourceRegisterLogic struct{}

func init() { gtables.RegisterResourceRegisterLogic(resourceRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (resourceRegisterLogic) OnInsert(rec *gtables.ResourceRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (resourceRegisterLogic) OnModify(rec *gtables.ResourceRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (resourceRegisterLogic) OnDelete(rec *gtables.ResourceRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (resourceRegisterLogic) OnRename(rec *gtables.ResourceRegister) error {
	return nil
}
