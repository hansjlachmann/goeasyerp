package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 113 G/L Budget by Periods: a value typed into the budget matrix is the account's Budgeted
// Amount in the cell's period, budget and filters (SetGLAccBudgetedAmount). Page 154 G/L Account
// Balance/Budget: Balance/Budget (%) per period (classic form 350 CalcFormFields). Page 159
// Receivables-Payables: the customers' less the vendors' balances due per period (form 355).
func init() {
	pages.OnMatrixValidate(113, func(rec *gtables.GLAccount, value types.Decimal) error {
		return tables.SetGLAccBudgetedAmount(rec, value)
	})
	pages.OnAfterGetRecord(154, func(rec *gtables.GLAccount, page *pages.Control) {
		budgetPct := types.Decimal{}
		if !rec.Budgeted_amount.IsZero() {
			budgetPct = rec.Net_change.Div(rec.Budgeted_amount).Mul(types.NewDecimalFromInt(100)).Round(1)
		}
		page.SetValue("budget_pct", budgetPct)
	})
	pages.OnAfterGetRecord(159, func(rec *gtables.GeneralLedgerSetup, page *pages.Control) {
		page.SetValue("receivables_payables", rec.Cust_balances_due.Sub(rec.Vendor_balances_due))
	})
}
