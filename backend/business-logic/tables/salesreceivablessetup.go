package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesReceivablesSetupLogic is the business logic of table 311 Sales_Receivables_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/salesreceivablessetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type salesReceivablesSetupLogic struct{}

func init() { gtables.RegisterSalesReceivablesSetupLogic(salesReceivablesSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (salesReceivablesSetupLogic) OnInsert(rec *gtables.SalesReceivablesSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (salesReceivablesSetupLogic) OnModify(rec *gtables.SalesReceivablesSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (salesReceivablesSetupLogic) OnDelete(rec *gtables.SalesReceivablesSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (salesReceivablesSetupLogic) OnRename(rec *gtables.SalesReceivablesSetup) error {
	return nil
}
