package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// phaseLogic is the business logic of table 161 Phase.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/phase_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type phaseLogic struct{}

func init() { gtables.RegisterPhaseLogic(phaseLogic{}) }

// OnInsert runs before a new record is inserted
func (phaseLogic) OnInsert(rec *gtables.Phase) error {
	return nil
}

// OnModify runs before a record is modified
func (phaseLogic) OnModify(rec *gtables.Phase) error {
	return nil
}

// OnDelete runs before a record is deleted
func (phaseLogic) OnDelete(rec *gtables.Phase) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (phaseLogic) OnRename(rec *gtables.Phase) error {
	return nil
}
