package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reminderCommentLineLogic is the business logic of table 299 Reminder_Comment_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/remindercommentline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reminderCommentLineLogic struct{}

func init() { gtables.RegisterReminderCommentLineLogic(reminderCommentLineLogic{}) }

// OnInsert runs before a new record is inserted
func (reminderCommentLineLogic) OnInsert(rec *gtables.ReminderCommentLine) error {
	return nil
}

// OnModify runs before a record is modified
func (reminderCommentLineLogic) OnModify(rec *gtables.ReminderCommentLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reminderCommentLineLogic) OnDelete(rec *gtables.ReminderCommentLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderCommentLineLogic) OnRename(rec *gtables.ReminderCommentLine) error {
	return nil
}
