package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// sMTPSetupLogic is the business logic of table 409 SMTP_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/smtpsetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type sMTPSetupLogic struct{}

func init() { gtables.RegisterSMTPSetupLogic(sMTPSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (sMTPSetupLogic) OnInsert(rec *gtables.SMTPSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (sMTPSetupLogic) OnModify(rec *gtables.SMTPSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (sMTPSetupLogic) OnDelete(rec *gtables.SMTPSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (sMTPSetupLogic) OnRename(rec *gtables.SMTPSetup) error {
	return nil
}
