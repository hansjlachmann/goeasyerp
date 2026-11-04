package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectJournalQuantityLogic is the business logic of table 278 Project_Journal_Quantity.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/projectjournalquantity_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type projectJournalQuantityLogic struct{}

func init() { gtables.RegisterProjectJournalQuantityLogic(projectJournalQuantityLogic{}) }

// OnInsert runs before a new record is inserted
func (projectJournalQuantityLogic) OnInsert(rec *gtables.ProjectJournalQuantity) error {
	return nil
}

// OnModify runs before a record is modified
func (projectJournalQuantityLogic) OnModify(rec *gtables.ProjectJournalQuantity) error {
	return nil
}

// OnDelete runs before a record is deleted
func (projectJournalQuantityLogic) OnDelete(rec *gtables.ProjectJournalQuantity) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (projectJournalQuantityLogic) OnRename(rec *gtables.ProjectJournalQuantity) error {
	return nil
}
