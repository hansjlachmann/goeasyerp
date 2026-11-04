package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// dateComprRegisterLogic is the business logic of table 87 Date_Compr_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/datecomprregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type dateComprRegisterLogic struct{}

func init() { gtables.RegisterDateComprRegisterLogic(dateComprRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (dateComprRegisterLogic) OnInsert(rec *gtables.DateComprRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (dateComprRegisterLogic) OnModify(rec *gtables.DateComprRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (dateComprRegisterLogic) OnDelete(rec *gtables.DateComprRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (dateComprRegisterLogic) OnRename(rec *gtables.DateComprRegister) error {
	return nil
}
