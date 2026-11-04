package demodata

import (
	"fmt"
	"time"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insertDemoBudget: a G/L budget for this year (named by the year) — for each income statement
// posting account a monthly amount of last twelve months' net change plus 5 %, rounded to
// hundreds, for every month of the year
func insertDemoBudget(src ftables.Source, today time.Time, counts *Counts) error {
	glBudget := gtables.NewGLBudgetName(src)
	glBudget.Name = types.Code(fmt.Sprint(today.Year()))
	glBudget.Description = types.Text(fmt.Sprintf("Budget %d", today.Year()))
	err := glBudget.InsertOrError(true)
	if err != nil {
		return err
	}

	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local)
	from := types.NewDateFromTime(monthStart.AddDate(-1, 0, 0))
	to := types.NewDateFromTime(monthStart.AddDate(0, 0, -1))
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	glAcc.SetRange("income_balance", gtables.GLAccount_Income_balance.IncomeStatement)
	err = glAcc.SetFlowFilter("date_filter", from.String()+".."+to.String())
	if err != nil {
		return err
	}
	hundred := types.NewDecimalFromInt(100)
	var entries []*gtables.GLBudgetEntry
	for found := glAcc.FindSetBuffered(); found; found = glAcc.Next() {
		glAcc.CalcFields("net_change")
		monthly := glAcc.Net_change.Mul(types.MustDecimal("1.05")).Div(types.NewDecimalFromInt(12)).Div(hundred).Round(0).Mul(hundred)
		if monthly.IsZero() {
			continue
		}
		for month := 1; month <= 12; month++ {
			entry := gtables.NewGLBudgetEntry(src)
			entry.Entry_no = len(entries) + 1
			entry.Budget_name = glBudget.Name
			entry.Gl_account_no = glAcc.No
			entry.Date = types.NewDate(today.Year(), time.Month(month), 1)
			entry.Amount = monthly
			entry.Description = glAcc.Name
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	err = gtables.NewGLBudgetEntry(src).InsertAll(entries, true)
	if err != nil {
		return err
	}
	counts.GLBudgetEntries = len(entries)
	return nil
}
