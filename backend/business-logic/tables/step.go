package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// stepLogic is the business logic of table 163 Step.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/step_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type stepLogic struct{}

func init() { gtables.RegisterStepLogic(stepLogic{}) }

// OnInsert runs before a new record is inserted
func (stepLogic) OnInsert(rec *gtables.Step) error {
	return nil
}

// OnModify runs before a record is modified
func (stepLogic) OnModify(rec *gtables.Step) error {
	return nil
}

// OnDelete runs before a record is deleted
func (stepLogic) OnDelete(rec *gtables.Step) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (stepLogic) OnRename(rec *gtables.Step) error {
	return nil
}
