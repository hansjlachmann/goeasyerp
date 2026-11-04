package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxProductPostingGroupLogic is the business logic of table 324 Tax_Product_Posting_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxproductpostinggroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxProductPostingGroupLogic struct{}

func init() { gtables.RegisterTaxProductPostingGroupLogic(taxProductPostingGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (taxProductPostingGroupLogic) OnInsert(rec *gtables.TaxProductPostingGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (taxProductPostingGroupLogic) OnModify(rec *gtables.TaxProductPostingGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taxProductPostingGroupLogic) OnDelete(rec *gtables.TaxProductPostingGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxProductPostingGroupLogic) OnRename(rec *gtables.TaxProductPostingGroup) error {
	return nil
}
