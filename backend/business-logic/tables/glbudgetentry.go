package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gLBudgetEntryLogic is the business logic of table 96 G_L_Budget_Entry.
type gLBudgetEntryLogic struct{}

func init() { gtables.RegisterGLBudgetEntryLogic(gLBudgetEntryLogic{}) }

// OnInsert: an entry has a date and a budget; the user who entered it
func (gLBudgetEntryLogic) OnInsert(rec *gtables.GLBudgetEntry) error {
	err := rec.TestField("date")
	if err != nil {
		return err
	}
	err = rec.TestField("budget_name")
	if err != nil {
		return err
	}
	rec.User_id = types.Code(system.UserId())
	return nil
}
