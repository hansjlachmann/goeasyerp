package pages

import (
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectStatisticsColumns: Resources, Items, Miscellaneous (G/L accounts) and Total (classic
// arrays 1..4; a resource group's budget counts with the resources)
var projectStatisticsColumns = []string{"res", "item", "misc", "total"}

// Page 222 Project Statistics (OnAfterGetRecord): the budgeted price, cost and profit and the
// realized usage, invoicing and profit of the project per type and in total, in the project's
// FlowFilters
func init() {
	pages.OnAfterGetRecord(222, func(rec *gtables.Project, page *pages.Control) {
		var budgetTotalCost, budgetTotalPrice, usageTotalCost, usageTotalPrice, invTotalPrice [4]types.Decimal
		project := gtables.NewProject(rec)
		project.Copy(rec)

		_ = project.SetFlowFilter("type_filter", "3") // Group (Resource)
		project.CalcFields("budgeted_cost", "budgeted_price")
		budgetTotalCost[0] = project.Budgeted_cost
		budgetTotalPrice[0] = project.Budgeted_price
		for i := 0; i < 3; i++ { // Resource, Item, Account (G/L)
			_ = project.SetFlowFilter("type_filter", string(rune('0'+i)))
			project.CalcFields("budgeted_cost", "budgeted_price", "usage_cost", "usage_price", "invoiced_price")
			budgetTotalCost[i] = budgetTotalCost[i].Add(project.Budgeted_cost)
			budgetTotalCost[3] = budgetTotalCost[3].Add(budgetTotalCost[i])
			budgetTotalPrice[i] = budgetTotalPrice[i].Add(project.Budgeted_price)
			budgetTotalPrice[3] = budgetTotalPrice[3].Add(budgetTotalPrice[i])
			usageTotalCost[i] = project.Usage_cost
			usageTotalCost[3] = usageTotalCost[3].Add(project.Usage_cost)
			usageTotalPrice[i] = project.Usage_price
			usageTotalPrice[3] = usageTotalPrice[3].Add(project.Usage_price)
			invTotalPrice[i] = project.Invoiced_price
			invTotalPrice[3] = invTotalPrice[3].Add(project.Invoiced_price)
		}
		for i, column := range projectStatisticsColumns {
			budgetProfit := budgetTotalPrice[i].Sub(budgetTotalCost[i])
			excessInvoicing := invTotalPrice[i].Sub(usageTotalPrice[i])
			invProfit := invTotalPrice[i].Sub(usageTotalCost[i])
			page.SetValue(column+"_budget_price", budgetTotalPrice[i])
			page.SetValue(column+"_budget_cost", budgetTotalCost[i])
			page.SetValue(column+"_budget_profit", budgetProfit)
			page.SetValue(column+"_budget_profit_pct", calcPercentage(budgetProfit, budgetTotalPrice[i]))
			page.SetValue(column+"_invoiced_price", invTotalPrice[i])
			page.SetValue(column+"_usage_price", usageTotalPrice[i])
			page.SetValue(column+"_excess_invoicing", excessInvoicing)
			page.SetValue(column+"_excess_inv_pct", calcPercentage(invTotalPrice[i], usageTotalPrice[i]))
			page.SetValue(column+"_usage_cost", usageTotalCost[i])
			page.SetValue(column+"_inv_profit", invProfit)
			page.SetValue(column+"_inv_profit_pct", calcPercentage(invProfit, invTotalPrice[i]))
		}
	})
}

// calcPercentage (CalcPercentage): the part as a percentage of the base (0 without a base)
func calcPercentage(partAmount, base types.Decimal) types.Decimal {
	if !base.IsZero() {
		return types.NewDecimalFromInt(100).Mul(partAmount).Div(base)
	}
	return types.Decimal{}
}
