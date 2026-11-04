package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAClassLogic is the business logic of table 5607 FA_Class.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/faclass_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type fAClassLogic struct{}

func init() { gtables.RegisterFAClassLogic(fAClassLogic{}) }

// OnInsert runs before a new record is inserted
func (fAClassLogic) OnInsert(rec *gtables.FAClass) error {
	return nil
}

// OnModify runs before a record is modified
func (fAClassLogic) OnModify(rec *gtables.FAClass) error {
	return nil
}

// OnDelete runs before a record is deleted
func (fAClassLogic) OnDelete(rec *gtables.FAClass) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (fAClassLogic) OnRename(rec *gtables.FAClass) error {
	return nil
}
