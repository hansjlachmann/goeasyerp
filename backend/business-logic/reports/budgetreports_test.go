package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo budget prints on Trial Balance/Budget (9) against the accounts' net change; Copy G/L
// Budget (96) copies it into another budget, dated on and multiplied
func TestBudgetReports(t *testing.T) {
	src := listCompany(t)
	budget := gtables.NewGLBudgetName(src)
	if !budget.Get(types.Code("2026")) {
		t.Fatal("no demo budget 2026")
	}
	entry := gtables.NewGLBudgetEntry(src)
	entry.SetRange("budget_name", types.Code("2026"))
	count := entry.Count()
	if count == 0 || !entry.FindFirst() {
		t.Fatal("the demo budget has no entries")
	}
	accountNo := entry.Gl_account_no

	// Trial Balance/Budget from the budget: the account's budget of the period
	acc := gtables.NewGLAccount(src)
	if !acc.Get(accountNo) {
		t.Fatalf("account %s", accountNo)
	}
	mustOK(t, acc.SetFlowFilter("budget_filter", "2026"))
	workDate := system.WorkDate()
	mustOK(t, acc.SetFlowFilter("date_filter", types.NewDate(workDate.Year(), 1, 1).String()+".."+workDate.String()))
	acc.CalcFields("budgeted_amount")
	result, err := reports.Execute(reports.TrialBalanceBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(),
		reports.RunOptions{Output: reports.OutputPDF, Context: budget})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	wants := []string{"Trial Balance/Budget", accountNo.String(), "2026"}
	if !acc.Budgeted_amount.IsZero() { // the work date in the demo budget's year
		wants = append(wants, amountText(acc.Budgeted_amount.Abs()))
	}
	for _, want := range wants {
		if !strings.Contains(texts, want) {
			t.Errorf("trial balance/budget does not print %q", want)
		}
	}

	// Copy G/L Budget: into NEXT, a year on, +10 %
	entry.SetRange("budget_name", types.Code("2026"))
	mustOK(t, entry.CalcSums("amount"))
	total := entry.Amount
	_, err = reports.Execute(reports.CopyGLBudgetID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: budget,
		Options: map[string]string{"budget_name": "next", "date_change_formula": "1Y", "amount_change_factor": "1.1"},
	})
	mustOK(t, err)
	if !gtables.NewGLBudgetName(src).Get(types.Code("NEXT")) {
		t.Fatal("Copy G/L Budget did not create budget NEXT")
	}
	copied := gtables.NewGLBudgetEntry(src)
	copied.SetRange("budget_name", types.Code("NEXT"))
	if n := copied.Count(); n != count {
		t.Errorf("%d entries copied, want %d", n, count)
	}
	copied.SetRange("date", types.Date{}, types.NewDate(2026, 12, 31))
	if !copied.IsEmpty() {
		t.Error("copied entries are not dated a year on")
	}
	copied.SetRange("date")
	mustOK(t, copied.CalcSums("amount"))
	want := total.Mul(types.MustDecimal("1.1"))
	if copied.Amount.Sub(want).Abs().GreaterThan(types.NewDecimalFromInt(int64(count))) {
		t.Errorf("copied total %s, want about %s", copied.Amount, want)
	}
}
