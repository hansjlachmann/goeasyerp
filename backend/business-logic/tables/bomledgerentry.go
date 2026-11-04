package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bOMLedgerEntryLogic is the business logic of table 238 BOM_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/bomledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type bOMLedgerEntryLogic struct{}

func init() { gtables.RegisterBOMLedgerEntryLogic(bOMLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (bOMLedgerEntryLogic) OnInsert(rec *gtables.BOMLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (bOMLedgerEntryLogic) OnModify(rec *gtables.BOMLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (bOMLedgerEntryLogic) OnDelete(rec *gtables.BOMLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (bOMLedgerEntryLogic) OnRename(rec *gtables.BOMLedgerEntry) error {
	return nil
}
