package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reasonCodeLogic is the business logic of table 231 Reason_Code.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reasoncode_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reasonCodeLogic struct{}

func init() { gtables.RegisterReasonCodeLogic(reasonCodeLogic{}) }

// OnInsert runs before a new record is inserted
func (reasonCodeLogic) OnInsert(rec *gtables.ReasonCode) error {
	return nil
}

// OnModify runs before a record is modified
func (reasonCodeLogic) OnModify(rec *gtables.ReasonCode) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reasonCodeLogic) OnDelete(rec *gtables.ReasonCode) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reasonCodeLogic) OnRename(rec *gtables.ReasonCode) error {
	return nil
}
