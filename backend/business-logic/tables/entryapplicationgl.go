package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// entryApplicationGLLogic is the business logic of table 906 Entry_Application_GL.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/entryapplicationgl_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type entryApplicationGLLogic struct{}

func init() { gtables.RegisterEntryApplicationGLLogic(entryApplicationGLLogic{}) }

// OnInsert runs before a new record is inserted
func (entryApplicationGLLogic) OnInsert(rec *gtables.EntryApplicationGL) error {
	return nil
}

// OnModify runs before a record is modified
func (entryApplicationGLLogic) OnModify(rec *gtables.EntryApplicationGL) error {
	return nil
}

// OnDelete runs before a record is deleted
func (entryApplicationGLLogic) OnDelete(rec *gtables.EntryApplicationGL) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (entryApplicationGLLogic) OnRename(rec *gtables.EntryApplicationGL) error {
	return nil
}
