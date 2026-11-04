package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maintenanceRegistrationLogic is the business logic of table 5616 Maintenance Registration
type maintenanceRegistrationLogic struct{}

func init() { gtables.RegisterMaintenanceRegistrationLogic(maintenanceRegistrationLogic{}) }

// OnInsert: the fixed asset must exist
func (maintenanceRegistrationLogic) OnInsert(rec *gtables.MaintenanceRegistration) error {
	fa := gtables.NewFixedAsset(rec)
	fa.LockTable()
	return fa.GetOrError(rec.Fa_no)
}
