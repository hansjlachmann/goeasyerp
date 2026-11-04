package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insCoverageLedgerEntryLogic is the business logic of table 5629 Ins_Coverage_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/inscoverageledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type insCoverageLedgerEntryLogic struct{}

func init() { gtables.RegisterInsCoverageLedgerEntryLogic(insCoverageLedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (insCoverageLedgerEntryLogic) OnInsert(rec *gtables.InsCoverageLedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (insCoverageLedgerEntryLogic) OnModify(rec *gtables.InsCoverageLedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (insCoverageLedgerEntryLogic) OnDelete(rec *gtables.InsCoverageLedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (insCoverageLedgerEntryLogic) OnRename(rec *gtables.InsCoverageLedgerEntry) error {
	return nil
}
