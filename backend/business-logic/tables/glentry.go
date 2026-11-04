package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gLEntryLogic is the business logic of table 17 G_L_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/glentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type gLEntryLogic struct{}

func init() { gtables.RegisterGLEntryLogic(gLEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (gLEntryLogic) OnInsert(rec *gtables.GLEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (gLEntryLogic) OnModify(rec *gtables.GLEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (gLEntryLogic) OnDelete(rec *gtables.GLEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (gLEntryLogic) OnRename(rec *gtables.GLEntry) error {
	return nil
}
