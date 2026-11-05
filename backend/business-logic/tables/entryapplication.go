package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// entryApplicationLogic is the business logic of table 905 Entry_Application.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/entryapplication_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type entryApplicationLogic struct{}

func init() { gtables.RegisterEntryApplicationLogic(entryApplicationLogic{}) }

// OnInsert runs before a new record is inserted
func (entryApplicationLogic) OnInsert(rec *gtables.EntryApplication) error {
	return nil
}

// OnModify runs before a record is modified
func (entryApplicationLogic) OnModify(rec *gtables.EntryApplication) error {
	return nil
}

// OnDelete runs before a record is deleted
func (entryApplicationLogic) OnDelete(rec *gtables.EntryApplication) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (entryApplicationLogic) OnRename(rec *gtables.EntryApplication) error {
	return nil
}
