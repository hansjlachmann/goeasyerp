package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// noSeriesLineLogic is the business logic of table 309 No_Series_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/noseriesline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type noSeriesLineLogic struct{}

func init() { gtables.RegisterNoSeriesLineLogic(noSeriesLineLogic{}) }

// OnInsert runs before a new record is inserted
func (noSeriesLineLogic) OnInsert(rec *gtables.NoSeriesLine) error {
	return nil
}

// OnModify runs before a record is modified
func (noSeriesLineLogic) OnModify(rec *gtables.NoSeriesLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (noSeriesLineLogic) OnDelete(rec *gtables.NoSeriesLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (noSeriesLineLogic) OnRename(rec *gtables.NoSeriesLine) error {
	return nil
}

// OnValidate_Starting_no gives the line's numbers the new number's format
func (noSeriesLineLogic) OnValidate_Starting_no(rec *gtables.NoSeriesLine) error {
	return noseries.UpdateNoSeriesLine(rec, rec.Starting_no, "starting_no")
}

// OnValidate_Ending_no: no ending number, no warning number; the line is open until its
// ending number is used
func (noSeriesLineLogic) OnValidate_Ending_no(rec *gtables.NoSeriesLine) error {
	if rec.Ending_no == "" {
		rec.Warning_no = ""
	}
	err := noseries.UpdateNoSeriesLine(rec, rec.Ending_no, "ending_no")
	if err != nil {
		return err
	}
	return rec.Validate("open", rec.Open)
}

// OnValidate_Warning_no needs an ending number
func (noSeriesLineLogic) OnValidate_Warning_no(rec *gtables.NoSeriesLine) error {
	err := rec.TestField("ending_no")
	if err != nil {
		return err
	}
	return noseries.UpdateNoSeriesLine(rec, rec.Warning_no, "warning_no")
}

// OnValidate_Last_no_used gives the line's numbers its format and recalculates Open
func (noSeriesLineLogic) OnValidate_Last_no_used(rec *gtables.NoSeriesLine) error {
	err := noseries.UpdateNoSeriesLine(rec, rec.Last_no_used, "last_no_used")
	if err != nil {
		return err
	}
	return rec.Validate("open", rec.Open)
}

// OnValidate_Open: open while the ending number is not used
func (noSeriesLineLogic) OnValidate_Open(rec *gtables.NoSeriesLine) error {
	rec.Open = rec.Ending_no == "" || rec.Ending_no != rec.Last_no_used
	return nil
}
