package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appBuildLogic is the business logic of table 2000000154 App_Build.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/appbuild_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type appBuildLogic struct{}

func init() { gtables.RegisterAppBuildLogic(appBuildLogic{}) }

// OnInsert runs before a new record is inserted
func (appBuildLogic) OnInsert(rec *gtables.AppBuild) error {
	return nil
}

// OnModify runs before a record is modified
func (appBuildLogic) OnModify(rec *gtables.AppBuild) error {
	return nil
}

// OnDelete runs before a record is deleted
func (appBuildLogic) OnDelete(rec *gtables.AppBuild) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (appBuildLogic) OnRename(rec *gtables.AppBuild) error {
	return nil
}
