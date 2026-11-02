package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// setupChecklistLineLogic is the business logic of table 344 Setup_Checklist_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/setupchecklistline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type setupChecklistLineLogic struct{}

func init() { gtables.RegisterSetupChecklistLineLogic(setupChecklistLineLogic{}) }

// OnInsert runs before a new record is inserted
func (setupChecklistLineLogic) OnInsert(rec *gtables.SetupChecklistLine) error {
	return nil
}

// OnModify runs before a record is modified
func (setupChecklistLineLogic) OnModify(rec *gtables.SetupChecklistLine) error {
	return nil
}

// OnDelete deletes the table's comments
func (setupChecklistLineLogic) OnDelete(rec *gtables.SetupChecklistLine) error {
	commentLine := gtables.NewSetupChecklistComment(rec)
	commentLine.SetRange("table_id", rec.Table_id)
	commentLine.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (setupChecklistLineLogic) OnRename(rec *gtables.SetupChecklistLine) error {
	return nil
}

// OnValidate_Starting_date: a new Starting Date moves the Ending Date by as many days
func (setupChecklistLineLogic) OnValidate_Starting_date(rec *gtables.SetupChecklistLine) error {
	xRec := rec.XRec()
	if !rec.Starting_date.IsZero() && !xRec.Starting_date.IsZero() && !rec.Ending_date.IsZero() {
		rec.Ending_date = rec.Ending_date.AddDays(xRec.Starting_date.DaysBetween(rec.Starting_date))
	}
	return nil
}
