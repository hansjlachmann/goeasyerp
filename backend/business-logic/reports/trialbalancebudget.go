package reports

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 9 Trial Balance/Budget: each G/L account's net change in the period against its budget
// (Budget Filter), and its balance at the period's end against the budget up to then, with the
// net change / balance as % of the budget (heading and total accounts in bold). Without a date
// filter the period is the work date's fiscal year up to the work date.
const TrialBalanceBudgetID = 9

func init() {
	Register(TrialBalanceBudgetID, "trial-balance-budget", func() Report { return &trialBalanceBudget{} })
}

type trialBalanceBudget struct {
	glAcc2 *gtables.GLAccount
}

func (r *trialBalanceBudget) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.GLBudgetNameTableName,
		OnInitReport: func(run *Run) error {
			glBudget, ok := run.Context().(*gtables.GLBudgetName)
			if ok && glBudget.Name != "" {
				run.SetRequestFilter("GLAccount", "budget_filter", glBudget.Name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error { setReportHeadValues(run); return nil },
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "account_type", "date_filter", "budget_filter", "department_filter", "project_filter"},
			OnPreDataItem:       Trigger(r.glAccOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.glAccOnAfterGetRecord),
		}},
	}
}

// glAccOnPreDataItem: the accounts' filters for the budget up to the period's end (GLAcc2, from
// the start of the fiscal year the period begins in)
func (r *trialBalanceBudget) glAccOnPreDataItem(rec *gtables.GLAccount, run *Run) error {
	if rec.Date_filter == "" {
		dateFilter, _ := createFiscalYearToDate(run, system.WorkDate())
		err := rec.SetFlowFilter("date_filter", dateFilter)
		if err != nil {
			return err
		}
	}
	setFilterText(run, rec)
	setPeriodText(run, rec.Date_filter)
	run.SetValue("budget_filter_text", rec.Budget_filter)

	r.glAcc2 = gtables.NewGLAccount(run)
	r.glAcc2.Copy(rec)
	startDate, endDate := dateFilterRange(rec.Date_filter)
	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.SetRange("new_fiscal_year", true)
	accountingPeriod.Starting_date = startDate
	if accountingPeriod.Find("=<") {
		startDate = accountingPeriod.Starting_date
	}
	return r.glAcc2.SetFlowFilter("date_filter", startDate.String()+".."+endDate.String())
}

func (r *trialBalanceBudget) glAccOnAfterGetRecord(rec *gtables.GLAccount, run *Run) error {
	rec.CalcFields("net_change", "budgeted_amount", "balance_at_date")
	r.glAcc2.No = rec.No
	r.glAcc2.Totaling = rec.Totaling
	r.glAcc2.CalcFields("budget_at_date")
	netDebit, netCredit := debitCredit(rec.Net_change)
	balDebit, balCredit := debitCredit(rec.Balance_at_date)
	run.SetValue("name", indentedName(rec))
	run.SetValue("is_total", isTotalAccount(rec))
	run.SetValue("net_change_debit", netDebit)
	run.SetValue("net_change_credit", netCredit)
	run.SetValue("diff_pct", budgetPct(rec.Net_change, rec.Budgeted_amount))
	run.SetValue("budgeted_amount", rec.Budgeted_amount.Abs())
	run.SetValue("balance_debit", balDebit)
	run.SetValue("balance_credit", balCredit)
	run.SetValue("diff_at_date_pct", budgetPct(rec.Balance_at_date, r.glAcc2.Budget_at_date))
	run.SetValue("budget_at_date", r.glAcc2.Budget_at_date.Abs())
	return nil
}

// budgetPct: an amount as % of its budget (0 without a budget)
func budgetPct(amount, budget types.Decimal) types.Decimal {
	if budget.IsZero() {
		return types.Decimal{}
	}
	return amount.Div(budget).Mul(types.NewDecimalFromInt(100)).Round(1)
}

// createFiscalYearToDate: the date filter from the start of a date's fiscal year (its accounting
// period with New Fiscal Year; without accounting periods its calendar year) to the date
func createFiscalYearToDate(run *Run, date types.Date) (string, types.Date) {
	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.SetRange("new_fiscal_year", true)
	accountingPeriod.Starting_date = date
	startDate := types.NewDate(date.Year(), 1, 1)
	if accountingPeriod.Find("=<") {
		startDate = accountingPeriod.Starting_date
	}
	return startDate.String() + ".." + date.String(), startDate
}
