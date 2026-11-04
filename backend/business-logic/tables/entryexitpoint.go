package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// entryExitPointLogic is the business logic of table 282 Entry_Exit_Point.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/entryexitpoint_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type entryExitPointLogic struct{}

func init() { gtables.RegisterEntryExitPointLogic(entryExitPointLogic{}) }

// OnInsert runs before a new record is inserted
func (entryExitPointLogic) OnInsert(rec *gtables.EntryExitPoint) error {
	return nil
}

// OnModify runs before a record is modified
func (entryExitPointLogic) OnModify(rec *gtables.EntryExitPoint) error {
	return nil
}

// OnDelete runs before a record is deleted
func (entryExitPointLogic) OnDelete(rec *gtables.EntryExitPoint) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (entryExitPointLogic) OnRename(rec *gtables.EntryExitPoint) error {
	return nil
}
