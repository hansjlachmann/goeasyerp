package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// userTimeRegisterLogic is the business logic of table 51 User_Time_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/usertimeregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userTimeRegisterLogic struct{}

func init() { gtables.RegisterUserTimeRegisterLogic(userTimeRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (userTimeRegisterLogic) OnInsert(rec *gtables.UserTimeRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (userTimeRegisterLogic) OnModify(rec *gtables.UserTimeRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (userTimeRegisterLogic) OnDelete(rec *gtables.UserTimeRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userTimeRegisterLogic) OnRename(rec *gtables.UserTimeRegister) error {
	return nil
}
