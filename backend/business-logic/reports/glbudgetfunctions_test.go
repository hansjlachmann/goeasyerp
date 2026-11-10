package reports_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The G/L budget functions on a budget of their own: Amount by Period spreads a total over the
// periods (growing by a percentage, the periods add up to the total), Budget from History takes
// each month's net change, Import/Export writes the entries as a file and reads them back
func TestGLBudgetFunctions(t *testing.T) {
	src := listCompany(t)
	budget := gtables.NewGLBudgetName(src)
	budget.Name = "TEST"
	mustOK(t, budget.InsertOrError(true))
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	glAcc.SetRange("income_balance", gtables.GLAccount_Income_balance.IncomeStatement)
	found := glAcc.FindSet()
	for found {
		mustOK(t, glAcc.SetFlowFilter("date_filter", "2026-01-01..2026-12-31"))
		glAcc.CalcFields("net_change")
		if !glAcc.Net_change.IsZero() {
			break
		}
		found = glAcc.Next()
	}
	if !found {
		t.Fatal("no income statement account with entries in 2026")
	}
	accountFilter := map[string][]ftables.FieldFilter{"GLAccount": {{Field: "no", Expression: glAcc.No.String()}}}
	entries := func(date string) (int, types.Decimal) {
		e := gtables.NewGLBudgetEntry(src)
		e.SetRange("budget_name", "TEST")
		e.SetRange("gl_account_no", glAcc.No)
		if date != "" {
			e.SetFilter("date", date)
		}
		mustOK(t, e.CalcSums("amount"))
		return e.Count(), e.Amount
	}

	// 81: 1,200 over 12 months, 10 % more each month, whole numbers
	_, err := reports.Execute(reports.BudgetAmountByPeriodID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: budget, Filters: accountFilter,
		Options: map[string]string{"budget_beginning_date": "2027-01-01", "budget_amount": "1200", "period_percent_change": "10", "no_of_periods": "12"},
	})
	mustOK(t, err)
	n, total := entries("2027-01-01..2027-12-31")
	diff := total.Sub(types.NewDecimalFromInt(1200)).Abs()
	if n != 12 || diff.GreaterThan(types.NewDecimalFromInt(6)) {
		t.Errorf("amount by period: %d entries, total %s, want 12 and about 1200", n, total)
	}
	_, first := entries("2027-01-01")
	_, last := entries("2027-12-01")
	if !last.GreaterThan(first) {
		t.Errorf("first month %s, last month %s: the periods do not grow", first, last)
	}

	// 82: the account's net change of each month of 2026 as the budget of 2028, unchanged
	_, err = reports.Execute(reports.BudgetFromHistoryID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: budget, Filters: accountFilter,
		Options: map[string]string{"history_beginning_date": "2026-01-01", "budget_beginning_date": "2028-01-01", "no_of_periods": "12", "round_to": "0"},
	})
	mustOK(t, err)
	history := gtables.NewGLAccount(src)
	mustOK(t, history.GetOrError(glAcc.No))
	mustOK(t, history.SetFlowFilter("date_filter", "2026-01-01..2026-12-31"))
	history.CalcFields("net_change")
	n, total = entries("2028-01-01..2028-12-31")
	if n != 12 || total.Sub(history.Net_change).Abs().GreaterThan(types.MustDecimal("0.6")) {
		t.Errorf("budget from history: %d entries, total %s, want 12 and %s", n, total, history.Net_change)
	}

	// 83: export the budget, import the file into another budget
	result, err := reports.Execute(reports.ImportExportGLBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: budget, Options: map[string]string{"direction": "1"},
	})
	mustOK(t, err)
	if !strings.HasPrefix(string(result.File), "gl_account_no,date") || strings.Count(string(result.File), "\n") != 25 {
		t.Fatalf("export file:\n%s", result.File)
	}
	copyBudget := gtables.NewGLBudgetName(src)
	copyBudget.Name = "COPY"
	mustOK(t, copyBudget.InsertOrError(true))
	_, err = reports.Execute(reports.ImportExportGLBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: copyBudget, Options: map[string]string{"direction": "0", "file": base64.StdEncoding.EncodeToString(result.File)},
	})
	mustOK(t, err)
	imported := gtables.NewGLBudgetEntry(src)
	imported.SetRange("budget_name", "COPY")
	mustOK(t, imported.CalcSums("amount"))
	_, all := entries("")
	if imported.Count() != 24 || !imported.Amount.Equal(all) {
		t.Errorf("imported %d entries, %s; want 24, %s", imported.Count(), imported.Amount, all)
	}

	// a line with an account that does not exist refuses the file, naming the line
	bad := "gl_account_no,date,dimension_1_code,dimension_2_code,amount,description\nNOSUCH,2029-01-01,,,10,\n"
	_, err = reports.Execute(reports.ImportExportGLBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: copyBudget, Options: map[string]string{"direction": "0", "file": base64.StdEncoding.EncodeToString([]byte(bad))},
	})
	if err == nil || !strings.Contains(err.Error(), "2") {
		t.Errorf("a file with an unknown account: %v", err)
	}
}
