package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resCapacityEntryLogic is the business logic of table 160 Res. Capacity Entry.
type resCapacityEntryLogic struct{}

func init() { gtables.RegisterResCapacityEntryLogic(resCapacityEntryLogic{}) }

// OnValidate_Resource_no: the entry belongs to the resource's group
func (resCapacityEntryLogic) OnValidate_Resource_no(rec *gtables.ResCapacityEntry) error {
	res := gtables.NewResource(rec)
	err := res.GetOrError(rec.Resource_no)
	if err != nil {
		return err
	}
	rec.Resource_group_no = res.Resource_group_no
	return nil
}
