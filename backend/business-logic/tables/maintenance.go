package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maintenanceLogic is the business logic of table 5626 Maintenance.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/maintenance_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type maintenanceLogic struct{}

func init() { gtables.RegisterMaintenanceLogic(maintenanceLogic{}) }

// OnInsert runs before a new record is inserted
func (maintenanceLogic) OnInsert(rec *gtables.Maintenance) error {
	return nil
}

// OnModify runs before a record is modified
func (maintenanceLogic) OnModify(rec *gtables.Maintenance) error {
	return nil
}

// OnDelete runs before a record is deleted
func (maintenanceLogic) OnDelete(rec *gtables.Maintenance) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (maintenanceLogic) OnRename(rec *gtables.Maintenance) error {
	return nil
}
