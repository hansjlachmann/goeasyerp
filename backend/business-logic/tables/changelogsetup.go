package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// changeLogSetupLogic is the business logic of table 402 Change_Log_Setup: whether the change
// log records changes (S47; changelog.go) — SUPER users only
type changeLogSetupLogic struct{}

func init() { gtables.RegisterChangeLogSetupLogic(changeLogSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (changeLogSetupLogic) OnInsert(rec *gtables.ChangeLogSetup) error {
	return changeLogSetupChange()
}

// OnModify runs before a record is modified
func (changeLogSetupLogic) OnModify(rec *gtables.ChangeLogSetup) error {
	return changeLogSetupChange()
}

// OnDelete runs before a record is deleted
func (changeLogSetupLogic) OnDelete(rec *gtables.ChangeLogSetup) error {
	return changeLogSetupChange()
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (changeLogSetupLogic) OnRename(rec *gtables.ChangeLogSetup) error {
	return changeLogSetupChange()
}

// changeLogSetupChange: a change of the setup is a SUPER user's, and the change log reads the
// setup again
func changeLogSetupChange() error {
	err := changeLogSuperOnly()
	if err != nil {
		return err
	}
	changeLogSetupChanged()
	return nil
}
