package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gLBudgetNameLogic is the business logic of table 95 G_L_Budget_Name.
type gLBudgetNameLogic struct{}

func init() { gtables.RegisterGLBudgetNameLogic(gLBudgetNameLogic{}) }

// OnDelete deletes the budget's entries
func (gLBudgetNameLogic) OnDelete(rec *gtables.GLBudgetName) error {
	glBudgetEntry := gtables.NewGLBudgetEntry(rec)
	glBudgetEntry.SetCurrentKey("budget_name", "gl_account_no", "date")
	glBudgetEntry.SetRange("budget_name", rec.Name)
	glBudgetEntry.DeleteAll()
	return nil
}
