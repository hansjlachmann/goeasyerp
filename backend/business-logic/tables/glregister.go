package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gLRegisterLogic is the business logic of table 45 G_L_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/glregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type gLRegisterLogic struct{}

func init() { gtables.RegisterGLRegisterLogic(gLRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (gLRegisterLogic) OnInsert(rec *gtables.GLRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (gLRegisterLogic) OnModify(rec *gtables.GLRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (gLRegisterLogic) OnDelete(rec *gtables.GLRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (gLRegisterLogic) OnRename(rec *gtables.GLRegister) error {
	return nil
}
