package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fASubclassLogic is the business logic of table 5608 FA_Subclass.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/fasubclass_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type fASubclassLogic struct{}

func init() { gtables.RegisterFASubclassLogic(fASubclassLogic{}) }

// OnInsert runs before a new record is inserted
func (fASubclassLogic) OnInsert(rec *gtables.FASubclass) error {
	return nil
}

// OnModify runs before a record is modified
func (fASubclassLogic) OnModify(rec *gtables.FASubclass) error {
	return nil
}

// OnDelete runs before a record is deleted
func (fASubclassLogic) OnDelete(rec *gtables.FASubclass) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (fASubclassLogic) OnRename(rec *gtables.FASubclass) error {
	return nil
}
