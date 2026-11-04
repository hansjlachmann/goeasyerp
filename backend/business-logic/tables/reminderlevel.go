package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reminderLevelLogic is the business logic of table 293 Reminder_Level.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reminderlevel_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reminderLevelLogic struct{}

func init() { gtables.RegisterReminderLevelLogic(reminderLevelLogic{}) }

// OnInsert runs before a new record is inserted
func (reminderLevelLogic) OnInsert(rec *gtables.ReminderLevel) error {
	return nil
}

// OnModify runs before a record is modified
func (reminderLevelLogic) OnModify(rec *gtables.ReminderLevel) error {
	return nil
}

// OnDelete deletes the level's beginning and ending texts
func (reminderLevelLogic) OnDelete(rec *gtables.ReminderLevel) error {
	reminderText := gtables.NewReminderText(rec)
	reminderText.SetRange("reminder_terms_code", rec.Reminder_terms_code)
	reminderText.SetRange("reminder_level", rec.No)
	reminderText.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderLevelLogic) OnRename(rec *gtables.ReminderLevel) error {
	return nil
}

// NewReminderLevel gives a new level the next No. of its reminder terms (NewRecord)
func NewReminderLevel(rec *gtables.ReminderLevel) {
	reminderLevel := gtables.NewReminderLevel(rec)
	reminderLevel.SetRange("reminder_terms_code", rec.Reminder_terms_code)
	reminderLevel.FindLast()
	rec.No = reminderLevel.No + 1
}
