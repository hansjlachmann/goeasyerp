package tables

import (
	"strings"

	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// G/L budgets: table 95 G/L Budget Name, 96 G/L Budget Entry, the G/L Account's Budgeted Amount
// (codeunit 7 GLBudget-Open, entering a budgeted amount in the budget matrix)

type gLBudgetNameLogic struct{}

type gLBudgetEntryLogic struct{}

func init() {
	gtables.RegisterGLBudgetNameLogic(gLBudgetNameLogic{})
	gtables.RegisterGLBudgetEntryLogic(gLBudgetEntryLogic{})
}

// OnDelete deletes the budget's entries
func (gLBudgetNameLogic) OnDelete(rec *gtables.GLBudgetName) error {
	glBudgetEntry := gtables.NewGLBudgetEntry(rec)
	glBudgetEntry.SetCurrentKey("budget_name", "gl_account_no", "date")
	glBudgetEntry.SetRange("budget_name", rec.Name)
	glBudgetEntry.DeleteAll()
	return nil
}

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

// GLBudgetOpen is codeunit 7 GLBudget-Open: the budget the account's Budget Filter names, else
// the first budget — DEFAULT created when there is none — goes into the Budget Filter
func GLBudgetOpen(rec *gtables.GLAccount) error {
	glBudget := gtables.NewGLBudgetName(rec)
	searchForName := false
	if rec.Budget_filter == "" {
		searchForName = true
	} else {
		glBudget.SetFilter("name", rec.Budget_filter)
		searchForName = !glBudget.FindFirst()
		glBudget.SetRange("name")
	}
	if searchForName {
		if !glBudget.FindFirst() {
			glBudget.Init()
			glBudget.Name = "DEFAULT"
			glBudget.Description = "Default Budget"
			err := glBudget.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		return rec.SetFlowFilter("budget_filter", glBudget.Name.String())
	}
	return nil
}

// SetGLAccBudgetedAmount enters the account's Budgeted Amount in its FlowFilters (classic: a
// value typed into the FlowField): a G/L Budget Entry with the difference to the amount budgeted
// now, for the budget, department and project of the filters and the first date of the Date
// Filter. Each filter used must be one value (a blank department or project filter: none).
func SetGLAccBudgetedAmount(rec *gtables.GLAccount, budgetedAmount types.Decimal) error {
	err := CheckGLAcc(rec)
	if err != nil {
		return err
	}
	budgetName, ok := oneFilterValue(rec.Budget_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.GLAccountTableName, "budget_filter")
	}
	departmentCode, ok := oneFilterValue(rec.Department_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.GLAccountTableName, "department_filter")
	}
	projectCode, ok := oneFilterValue(rec.Project_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.GLAccountTableName, "project_filter")
	}
	date, ok := dateFilterMin(rec.Date_filter)
	if !ok {
		return apperrors.BudgetNotOneValue(gtables.GLAccountTableName, "date_filter")
	}
	rec.CalcFields("budgeted_amount")
	if budgetedAmount.Equal(rec.Budgeted_amount) {
		return nil
	}
	glBudgetEntry := gtables.NewGLBudgetEntry(rec)
	glBudgetEntry.LockTable()
	entryNo := 1
	if glBudgetEntry.FindLast() {
		entryNo = glBudgetEntry.Entry_no + 1
	}
	glBudgetEntry.Init()
	glBudgetEntry.Entry_no = entryNo
	glBudgetEntry.Budget_name = types.Code(budgetName)
	glBudgetEntry.Gl_account_no = rec.No
	glBudgetEntry.Date = date
	glBudgetEntry.Department_code = types.Code(departmentCode)
	glBudgetEntry.Project_code = types.Code(projectCode)
	glBudgetEntry.Amount = budgetedAmount.Sub(rec.Budgeted_amount)
	err = glBudgetEntry.InsertOrError(true)
	if err != nil {
		return err
	}
	rec.Budgeted_amount = budgetedAmount
	return nil
}

// oneFilterValue: the value of a filter on one value (blank: no filter); false for a range,
// alternatives or wildcards
func oneFilterValue(filter string) (string, bool) {
	filter = strings.TrimSpace(filter)
	if strings.ContainsAny(filter, "|*<>&") || strings.Contains(filter, "..") {
		return "", false
	}
	return strings.ToUpper(filter), true
}

// dateFilterMin: the first date of a date filter (classic GETRANGEMIN): the date, or the start
// of a range a..b; false without a start (blank, ..b) or for other filters
func dateFilterMin(filter string) (types.Date, bool) {
	filter = strings.TrimSpace(filter)
	if strings.ContainsAny(filter, "|*<>&") {
		return types.Date{}, false
	}
	fromText, _, _ := strings.Cut(filter, "..")
	date, err := types.NewDateFromString(strings.TrimSpace(fromText))
	if err != nil || date.IsZero() {
		return types.Date{}, false
	}
	return date, true
}
