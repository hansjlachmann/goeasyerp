package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// sourceCodeSetupLogic is the business logic of table 242 Source_Code_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/sourcecodesetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type sourceCodeSetupLogic struct{}

func init() { gtables.RegisterSourceCodeSetupLogic(sourceCodeSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (sourceCodeSetupLogic) OnInsert(rec *gtables.SourceCodeSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (sourceCodeSetupLogic) OnModify(rec *gtables.SourceCodeSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (sourceCodeSetupLogic) OnDelete(rec *gtables.SourceCodeSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (sourceCodeSetupLogic) OnRename(rec *gtables.SourceCodeSetup) error {
	return nil
}
