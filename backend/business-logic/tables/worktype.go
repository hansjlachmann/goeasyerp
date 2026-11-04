package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// workTypeLogic is the business logic of table 200 Work_Type.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/worktype_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type workTypeLogic struct{}

func init() { gtables.RegisterWorkTypeLogic(workTypeLogic{}) }

// OnInsert runs before a new record is inserted
func (workTypeLogic) OnInsert(rec *gtables.WorkType) error {
	return nil
}

// OnModify runs before a record is modified
func (workTypeLogic) OnModify(rec *gtables.WorkType) error {
	return nil
}

// OnDelete runs before a record is deleted
func (workTypeLogic) OnDelete(rec *gtables.WorkType) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (workTypeLogic) OnRename(rec *gtables.WorkType) error {
	return nil
}
