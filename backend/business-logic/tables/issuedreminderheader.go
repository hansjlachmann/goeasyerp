package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// issuedReminderHeaderLogic is the business logic of table 297 Issued_Reminder_Header.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/issuedreminderheader_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type issuedReminderHeaderLogic struct{}

func init() { gtables.RegisterIssuedReminderHeaderLogic(issuedReminderHeaderLogic{}) }

// OnInsert runs before a new record is inserted
func (issuedReminderHeaderLogic) OnInsert(rec *gtables.IssuedReminderHeader) error {
	return nil
}

// OnModify runs before a record is modified
func (issuedReminderHeaderLogic) OnModify(rec *gtables.IssuedReminderHeader) error {
	return nil
}

// OnDelete runs before a record is deleted
func (issuedReminderHeaderLogic) OnDelete(rec *gtables.IssuedReminderHeader) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (issuedReminderHeaderLogic) OnRename(rec *gtables.IssuedReminderHeader) error {
	return nil
}
