package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// accSchedColumnLayoutNameLogic is the business logic of table 333 Acc. Sched. Column Layout Name.
type accSchedColumnLayoutNameLogic struct{}

func init() { gtables.RegisterAccSchedColumnLayoutNameLogic(accSchedColumnLayoutNameLogic{}) }

// OnDelete deletes the layout's columns
func (accSchedColumnLayoutNameLogic) OnDelete(rec *gtables.AccSchedColumnLayoutName) error {
	accSchedColumn := gtables.NewAccSchedColumn(rec)
	accSchedColumn.SetRange("column_layout_name", rec.Name)
	accSchedColumn.DeleteAll()
	return nil
}
