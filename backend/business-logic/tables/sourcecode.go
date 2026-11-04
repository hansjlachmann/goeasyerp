package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// sourceCodeLogic is the business logic of table 230 Source_Code.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/sourcecode_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type sourceCodeLogic struct{}

func init() { gtables.RegisterSourceCodeLogic(sourceCodeLogic{}) }

// OnInsert runs before a new record is inserted
func (sourceCodeLogic) OnInsert(rec *gtables.SourceCode) error {
	return nil
}

// OnModify runs before a record is modified
func (sourceCodeLogic) OnModify(rec *gtables.SourceCode) error {
	return nil
}

// OnDelete runs before a record is deleted
func (sourceCodeLogic) OnDelete(rec *gtables.SourceCode) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (sourceCodeLogic) OnRename(rec *gtables.SourceCode) error {
	return nil
}
