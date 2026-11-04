package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// noSeriesLogic is the business logic of table 308 No_Series.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/noseries_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type noSeriesLogic struct{}

func init() { gtables.RegisterNoSeriesLogic(noSeriesLogic{}) }

// OnInsert runs before a new record is inserted
func (noSeriesLogic) OnInsert(rec *gtables.NoSeries) error {
	return nil
}

// OnModify runs before a record is modified
func (noSeriesLogic) OnModify(rec *gtables.NoSeries) error {
	return nil
}

// OnDelete deletes the series' lines and its relationships (both as the series and as a
// related series)
func (noSeriesLogic) OnDelete(rec *gtables.NoSeries) error {
	noSeriesLine := gtables.NewNoSeriesLine(rec)
	noSeriesLine.SetRange("series_code", rec.Code)
	noSeriesLine.DeleteAll()
	noSeriesRelationship := gtables.NewNoSeriesRelationship(rec)
	noSeriesRelationship.SetRange("code", rec.Code)
	noSeriesRelationship.DeleteAll()
	noSeriesRelationship.SetRange("code")
	noSeriesRelationship.SetRange("series_code", rec.Code)
	noSeriesRelationship.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (noSeriesLogic) OnRename(rec *gtables.NoSeries) error {
	return nil
}
