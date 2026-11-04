package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourcesSetupLogic is the business logic of table 314 Resources_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/resourcessetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type resourcesSetupLogic struct{}

func init() { gtables.RegisterResourcesSetupLogic(resourcesSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (resourcesSetupLogic) OnInsert(rec *gtables.ResourcesSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (resourcesSetupLogic) OnModify(rec *gtables.ResourcesSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (resourcesSetupLogic) OnDelete(rec *gtables.ResourcesSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (resourcesSetupLogic) OnRename(rec *gtables.ResourcesSetup) error {
	return nil
}
