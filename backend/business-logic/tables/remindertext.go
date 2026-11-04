package tables

import (
	"fmt"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reminderTextLogic is the business logic of table 294 Reminder_Text.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/remindertext_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reminderTextLogic struct{}

func init() { gtables.RegisterReminderTextLogic(reminderTextLogic{}) }

// OnInsert: the reminder level must exist
func (reminderTextLogic) OnInsert(rec *gtables.ReminderText) error {
	reminderLevel := gtables.NewReminderLevel(rec)
	if !reminderLevel.GetByPK(rec.Reminder_terms_code, rec.Reminder_level) {
		return apperrors.TableRecordNotFound(gtables.ReminderLevelTableName, fmt.Sprintf("%s,%d", rec.Reminder_terms_code, rec.Reminder_level))
	}
	return nil
}

// OnModify runs before a record is modified
func (reminderTextLogic) OnModify(rec *gtables.ReminderText) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reminderTextLogic) OnDelete(rec *gtables.ReminderText) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderTextLogic) OnRename(rec *gtables.ReminderText) error {
	return nil
}
