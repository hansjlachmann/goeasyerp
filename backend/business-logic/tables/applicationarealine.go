package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// applicationAreaLineLogic is the business logic of table 346 Application_Area_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/applicationarealine_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type applicationAreaLineLogic struct{}

func init() { gtables.RegisterApplicationAreaLineLogic(applicationAreaLineLogic{}) }

// OnInsert runs before a new record is inserted
func (applicationAreaLineLogic) OnInsert(rec *gtables.ApplicationAreaLine) error {
	return nil
}

// OnModify runs before a record is modified
func (applicationAreaLineLogic) OnModify(rec *gtables.ApplicationAreaLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (applicationAreaLineLogic) OnDelete(rec *gtables.ApplicationAreaLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (applicationAreaLineLogic) OnRename(rec *gtables.ApplicationAreaLine) error {
	return nil
}
