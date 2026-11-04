package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// areaLogic is the business logic of table 284 Area.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/area_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type areaLogic struct{}

func init() { gtables.RegisterAreaLogic(areaLogic{}) }

// OnInsert runs before a new record is inserted
func (areaLogic) OnInsert(rec *gtables.Area) error {
	return nil
}

// OnModify runs before a record is modified
func (areaLogic) OnModify(rec *gtables.Area) error {
	return nil
}

// OnDelete runs before a record is deleted
func (areaLogic) OnDelete(rec *gtables.Area) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (areaLogic) OnRename(rec *gtables.Area) error {
	return nil
}
