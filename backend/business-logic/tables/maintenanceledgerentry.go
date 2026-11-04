package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maintenanceLedgerEntryLogic is the business logic of table 5625 Maintenance_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/maintenanceledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type maintenanceLedgerEntryLogic struct{}

func init() { gtables.RegisterMaintenanceLedgerEntryLogic(maintenanceLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (maintenanceLedgerEntryLogic) OnInsert(rec *gtables.MaintenanceLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (maintenanceLedgerEntryLogic) OnModify(rec *gtables.MaintenanceLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (maintenanceLedgerEntryLogic) OnDelete(rec *gtables.MaintenanceLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (maintenanceLedgerEntryLogic) OnRename(rec *gtables.MaintenanceLedgerEntry) error {
	return nil
}
