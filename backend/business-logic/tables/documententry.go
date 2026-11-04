package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// documentEntryLogic is the business logic of table 265 Document_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/documententry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type documentEntryLogic struct{}

func init() { gtables.RegisterDocumentEntryLogic(documentEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (documentEntryLogic) OnInsert(rec *gtables.DocumentEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (documentEntryLogic) OnModify(rec *gtables.DocumentEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (documentEntryLogic) OnDelete(rec *gtables.DocumentEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (documentEntryLogic) OnRename(rec *gtables.DocumentEntry) error {
	return nil
}
