package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxGroupLogic is the business logic of table 321 Tax_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxgroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxGroupLogic struct{}

func init() { gtables.RegisterTaxGroupLogic(taxGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (taxGroupLogic) OnInsert(rec *gtables.TaxGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (taxGroupLogic) OnModify(rec *gtables.TaxGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taxGroupLogic) OnDelete(rec *gtables.TaxGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxGroupLogic) OnRename(rec *gtables.TaxGroup) error {
	return nil
}
