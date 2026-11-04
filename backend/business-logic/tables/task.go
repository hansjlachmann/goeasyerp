package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taskLogic is the business logic of table 162 Task.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/task_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taskLogic struct{}

func init() { gtables.RegisterTaskLogic(taskLogic{}) }

// OnInsert runs before a new record is inserted
func (taskLogic) OnInsert(rec *gtables.Task) error {
	return nil
}

// OnModify runs before a record is modified
func (taskLogic) OnModify(rec *gtables.Task) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taskLogic) OnDelete(rec *gtables.Task) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taskLogic) OnRename(rec *gtables.Task) error {
	return nil
}
