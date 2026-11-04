package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// physInventoryLedgerEntryLogic is the business logic of table 281 Phys_Inventory_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/physinventoryledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type physInventoryLedgerEntryLogic struct{}

func init() { gtables.RegisterPhysInventoryLedgerEntryLogic(physInventoryLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (physInventoryLedgerEntryLogic) OnInsert(rec *gtables.PhysInventoryLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (physInventoryLedgerEntryLogic) OnModify(rec *gtables.PhysInventoryLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (physInventoryLedgerEntryLogic) OnDelete(rec *gtables.PhysInventoryLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (physInventoryLedgerEntryLogic) OnRename(rec *gtables.PhysInventoryLedgerEntry) error {
	return nil
}
