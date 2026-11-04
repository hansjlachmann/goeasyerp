package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxBusinessPostingGroupLogic is the business logic of table 323 Tax_Business_Posting_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxbusinesspostinggroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxBusinessPostingGroupLogic struct{}

func init() { gtables.RegisterTaxBusinessPostingGroupLogic(taxBusinessPostingGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (taxBusinessPostingGroupLogic) OnInsert(rec *gtables.TaxBusinessPostingGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (taxBusinessPostingGroupLogic) OnModify(rec *gtables.TaxBusinessPostingGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taxBusinessPostingGroupLogic) OnDelete(rec *gtables.TaxBusinessPostingGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxBusinessPostingGroupLogic) OnRename(rec *gtables.TaxBusinessPostingGroup) error {
	return nil
}
