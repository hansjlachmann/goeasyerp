package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectRegisterLogic is the business logic of table 240 Project_Register.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/projectregister_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type projectRegisterLogic struct{}

func init() { gtables.RegisterProjectRegisterLogic(projectRegisterLogic{}) }

// OnInsert runs before a new record is inserted
func (projectRegisterLogic) OnInsert(rec *gtables.ProjectRegister) error {
	return nil
}

// OnModify runs before a record is modified
func (projectRegisterLogic) OnModify(rec *gtables.ProjectRegister) error {
	return nil
}

// OnDelete runs before a record is deleted
func (projectRegisterLogic) OnDelete(rec *gtables.ProjectRegister) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (projectRegisterLogic) OnRename(rec *gtables.ProjectRegister) error {
	return nil
}
