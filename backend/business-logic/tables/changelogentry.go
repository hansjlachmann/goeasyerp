package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// changeLogEntryLogic is the business logic of table 405 Change_Log_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/changelogentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type changeLogEntryLogic struct{}

func init() { gtables.RegisterChangeLogEntryLogic(changeLogEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (changeLogEntryLogic) OnInsert(rec *gtables.ChangeLogEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (changeLogEntryLogic) OnModify(rec *gtables.ChangeLogEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (changeLogEntryLogic) OnDelete(rec *gtables.ChangeLogEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (changeLogEntryLogic) OnRename(rec *gtables.ChangeLogEntry) error {
	return nil
}
