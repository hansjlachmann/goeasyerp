package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// locationLogic is the business logic of table 14 Location.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/location_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type locationLogic struct{}

func init() { gtables.RegisterLocationLogic(locationLogic{}) }

// OnInsert runs before a new record is inserted
func (locationLogic) OnInsert(rec *gtables.Location) error {
	return nil
}

// OnModify runs before a record is modified
func (locationLogic) OnModify(rec *gtables.Location) error {
	return nil
}

// OnDelete: the location's bins go with it
func (locationLogic) OnDelete(rec *gtables.Location) error {
	bin := gtables.NewBin(rec)
	bin.SetRange("location_code", rec.Code)
	bin.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (locationLogic) OnRename(rec *gtables.Location) error {
	return nil
}
