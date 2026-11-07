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

// OnDelete deletes the terms' levels and their texts
func (reminderTermsLogic) OnDelete(rec *gtables.ReminderTerms) error {
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", rec.Code)
	reminderLevel.DeleteAll()

	reminderText := gtables.NewReminderText(rec)
	reminderText.SetRange("reminder_terms_code", rec.Code)
	reminderText.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderTermsLogic) OnRename(rec *gtables.ReminderTerms) error {
	return nil
}
