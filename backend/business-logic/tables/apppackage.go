package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appPackageLogic is the business logic of table 2000000155 App_Package.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/apppackage_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type appPackageLogic struct{}

func init() { gtables.RegisterAppPackageLogic(appPackageLogic{}) }

// OnInsert runs before a new record is inserted
func (appPackageLogic) OnInsert(rec *gtables.AppPackage) error {
	return nil
}

// OnModify runs before a record is modified
func (appPackageLogic) OnModify(rec *gtables.AppPackage) error {
	return nil
}

// OnDelete runs before a record is deleted
func (appPackageLogic) OnDelete(rec *gtables.AppPackage) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (appPackageLogic) OnRename(rec *gtables.AppPackage) error {
	return nil
}
