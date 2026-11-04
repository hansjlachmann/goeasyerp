package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userSetupLogic is the business logic of table 91 User_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/usersetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userSetupLogic struct{}

func init() { gtables.RegisterUserSetupLogic(userSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (userSetupLogic) OnInsert(rec *gtables.UserSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (userSetupLogic) OnModify(rec *gtables.UserSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (userSetupLogic) OnDelete(rec *gtables.UserSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userSetupLogic) OnRename(rec *gtables.UserSetup) error {
	return nil
}
