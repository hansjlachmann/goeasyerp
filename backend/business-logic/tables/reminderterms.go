package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reminderTermsLogic is the business logic of table 292 Reminder_Terms.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reminderterms_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reminderTermsLogic struct{}

func init() { gtables.RegisterReminderTermsLogic(reminderTermsLogic{}) }

// OnInsert runs before a new record is inserted
func (reminderTermsLogic) OnInsert(rec *gtables.ReminderTerms) error {
	return nil
}

// OnModify runs before a record is modified
func (reminderTermsLogic) OnModify(rec *gtables.ReminderTerms) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reminderTermsLogic) OnDelete(rec *gtables.ReminderTerms) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderTermsLogic) OnRename(rec *gtables.ReminderTerms) error {
	return nil
}
