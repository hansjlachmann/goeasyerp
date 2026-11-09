package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// currencyForReminderLevelLogic is the business logic of table 329 Currency_for_Reminder_Level.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/currencyforreminderlevel_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type currencyForReminderLevelLogic struct{}

func init() { gtables.RegisterCurrencyForReminderLevelLogic(currencyForReminderLevelLogic{}) }

// OnInsert runs before a new record is inserted
func (currencyForReminderLevelLogic) OnInsert(rec *gtables.CurrencyForReminderLevel) error {
	return nil
}

// OnModify runs before a record is modified
func (currencyForReminderLevelLogic) OnModify(rec *gtables.CurrencyForReminderLevel) error {
	return nil
}

// OnDelete runs before a record is deleted
func (currencyForReminderLevelLogic) OnDelete(rec *gtables.CurrencyForReminderLevel) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (currencyForReminderLevelLogic) OnRename(rec *gtables.CurrencyForReminderLevel) error {
	return nil
}
