package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemLedgerEntryLogic is the business logic of table 32 Item_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/itemledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type itemLedgerEntryLogic struct{}

func init() { gtables.RegisterItemLedgerEntryLogic(itemLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (itemLedgerEntryLogic) OnInsert(rec *gtables.ItemLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (itemLedgerEntryLogic) OnModify(rec *gtables.ItemLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (itemLedgerEntryLogic) OnDelete(rec *gtables.ItemLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (itemLedgerEntryLogic) OnRename(rec *gtables.ItemLedgerEntry) error {
	return nil
}
