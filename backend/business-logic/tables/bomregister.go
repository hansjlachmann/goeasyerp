package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bOMRegisterLogic is the business logic of table 239 BOM_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/bomregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type bOMRegisterLogic struct{}

func init() { gtables.RegisterBOMRegisterLogic(bOMRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (bOMRegisterLogic) OnInsert(rec *gtables.BOMRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (bOMRegisterLogic) OnModify(rec *gtables.BOMRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (bOMRegisterLogic) OnDelete(rec *gtables.BOMRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (bOMRegisterLogic) OnRename(rec *gtables.BOMRegister) error {
	return nil
}
