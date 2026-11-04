package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resLedgerEntryLogic is the business logic of table 203 Res_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/resledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type resLedgerEntryLogic struct{}

func init() { gtables.RegisterResLedgerEntryLogic(resLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (resLedgerEntryLogic) OnInsert(rec *gtables.ResLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (resLedgerEntryLogic) OnModify(rec *gtables.ResLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (resLedgerEntryLogic) OnDelete(rec *gtables.ResLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (resLedgerEntryLogic) OnRename(rec *gtables.ResLedgerEntry) error {
	return nil
}
