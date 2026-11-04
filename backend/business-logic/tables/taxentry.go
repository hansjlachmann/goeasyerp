package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxEntryLogic is the business logic of table 254 Tax_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxEntryLogic struct{}

func init() { gtables.RegisterTaxEntryLogic(taxEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (taxEntryLogic) OnInsert(rec *gtables.TaxEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (taxEntryLogic) OnModify(rec *gtables.TaxEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taxEntryLogic) OnDelete(rec *gtables.TaxEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxEntryLogic) OnRename(rec *gtables.TaxEntry) error {
	return nil
}
