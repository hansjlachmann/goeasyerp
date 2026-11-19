package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appBuildSetupLogic is the business logic of table 2000000156 App_Build_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/appbuildsetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type appBuildSetupLogic struct{}

func init() { gtables.RegisterAppBuildSetupLogic(appBuildSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (appBuildSetupLogic) OnInsert(rec *gtables.AppBuildSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (appBuildSetupLogic) OnModify(rec *gtables.AppBuildSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (appBuildSetupLogic) OnDelete(rec *gtables.AppBuildSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (appBuildSetupLogic) OnRename(rec *gtables.AppBuildSetup) error {
	return nil
}
