package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemApplicationEntryLogic is the business logic of table 339 Item_Application_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/itemapplicationentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type itemApplicationEntryLogic struct{}

func init() { gtables.RegisterItemApplicationEntryLogic(itemApplicationEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (itemApplicationEntryLogic) OnInsert(rec *gtables.ItemApplicationEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (itemApplicationEntryLogic) OnModify(rec *gtables.ItemApplicationEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (itemApplicationEntryLogic) OnDelete(rec *gtables.ItemApplicationEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (itemApplicationEntryLogic) OnRename(rec *gtables.ItemApplicationEntry) error {
	return nil
}
