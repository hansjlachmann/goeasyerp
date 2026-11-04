package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// noSeriesRelationshipLogic is the business logic of table 310 No_Series_Relationship.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/noseriesrelationship_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type noSeriesRelationshipLogic struct{}

func init() { gtables.RegisterNoSeriesRelationshipLogic(noSeriesRelationshipLogic{}) }

// OnInsert runs before a new record is inserted
func (noSeriesRelationshipLogic) OnInsert(rec *gtables.NoSeriesRelationship) error {
	return nil
}

// OnModify runs before a record is modified
func (noSeriesRelationshipLogic) OnModify(rec *gtables.NoSeriesRelationship) error {
	return nil
}

// OnDelete runs before a record is deleted
func (noSeriesRelationshipLogic) OnDelete(rec *gtables.NoSeriesRelationship) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (noSeriesRelationshipLogic) OnRename(rec *gtables.NoSeriesRelationship) error {
	return nil
}
