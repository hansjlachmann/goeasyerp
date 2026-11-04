package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// installedAppLogic is the business logic of table 2000000153 Installed_App.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/installedapp_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type installedAppLogic struct{}

func init() { gtables.RegisterInstalledAppLogic(installedAppLogic{}) }

// OnInsert runs before a new record is inserted
func (installedAppLogic) OnInsert(rec *gtables.InstalledApp) error {
	return nil
}

// OnModify runs before a record is modified
func (installedAppLogic) OnModify(rec *gtables.InstalledApp) error {
	return nil
}

// OnDelete runs before a record is deleted
func (installedAppLogic) OnDelete(rec *gtables.InstalledApp) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (installedAppLogic) OnRename(rec *gtables.InstalledApp) error {
	return nil
}
