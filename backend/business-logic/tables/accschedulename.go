package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// accScheduleNameLogic is the business logic of table 84 Acc. Schedule Name.
type accScheduleNameLogic struct{}

func init() { gtables.RegisterAccScheduleNameLogic(accScheduleNameLogic{}) }

// OnDelete deletes the schedule's lines
func (accScheduleNameLogic) OnDelete(rec *gtables.AccScheduleName) error {
	accSchedLine := gtables.NewAccScheduleLine(rec)
	accSchedLine.SetRange("schedule_name", rec.Name)
	accSchedLine.DeleteAll()
	return nil
}
