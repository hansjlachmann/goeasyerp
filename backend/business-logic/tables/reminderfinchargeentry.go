package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reminderFinChargeEntryLogic is the business logic of table 300 Reminder_Fin_Charge_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reminderfinchargeentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reminderFinChargeEntryLogic struct{}

func init() { gtables.RegisterReminderFinChargeEntryLogic(reminderFinChargeEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (reminderFinChargeEntryLogic) OnInsert(rec *gtables.ReminderFinChargeEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (reminderFinChargeEntryLogic) OnModify(rec *gtables.ReminderFinChargeEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reminderFinChargeEntryLogic) OnDelete(rec *gtables.ReminderFinChargeEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reminderFinChargeEntryLogic) OnRename(rec *gtables.ReminderFinChargeEntry) error {
	return nil
}
