package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fALocationLogic is the business logic of table 5609 FA_Location.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/falocation_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type fALocationLogic struct{}

func init() { gtables.RegisterFALocationLogic(fALocationLogic{}) }

// OnInsert runs before a new record is inserted
func (fALocationLogic) OnInsert(rec *gtables.FALocation) error {
	return nil
}

// OnModify runs before a record is modified
func (fALocationLogic) OnModify(rec *gtables.FALocation) error {
	return nil
}

// OnDelete runs before a record is deleted
func (fALocationLogic) OnDelete(rec *gtables.FALocation) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (fALocationLogic) OnRename(rec *gtables.FALocation) error {
	return nil
}
