package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// issuedReminderLineLogic is the business logic of table 298 Issued_Reminder_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/issuedreminderline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type issuedReminderLineLogic struct{}

func init() { gtables.RegisterIssuedReminderLineLogic(issuedReminderLineLogic{}) }

// OnInsert runs before a new record is inserted
func (issuedReminderLineLogic) OnInsert(rec *gtables.IssuedReminderLine) error {
	return nil
}

// OnModify runs before a record is modified
func (issuedReminderLineLogic) OnModify(rec *gtables.IssuedReminderLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (issuedReminderLineLogic) OnDelete(rec *gtables.IssuedReminderLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (issuedReminderLineLogic) OnRename(rec *gtables.IssuedReminderLine) error {
	return nil
}
