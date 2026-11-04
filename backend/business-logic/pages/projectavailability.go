package pages

import (
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The availability pages' calculated period columns: the capacity after the projects' budgets on
// Order and after those on Quote (classic forms 361/362 for pages 225/226, 363/364 for pages
// 227/231 — the budget line's resource or resource group's capacity in the row's period)
func init() {
	pages.OnAfterGetRecord(225, func(rec *gtables.Resource, page *pages.Control) {
		setCapacityAfter(page, rec.Capacity, rec.Qty_on_order_project, rec.Qty_quoted_project)
	})
	pages.OnAfterGetRecord(226, func(rec *gtables.ResourceGroup, page *pages.Control) {
		setCapacityAfter(page, rec.Capacity, rec.Qty_on_order_project, rec.Qty_quoted_project)
	})
	pages.OnAfterGetRecord(227, func(rec *gtables.ProjectBudgetLine, page *pages.Control) {
		capacity := budgetLineCapacity(rec)
		page.SetValue("capacity", capacity)
		setCapacityAfter(page, capacity, rec.Qty_on_order_project, rec.Qty_quoted_project)
	})
	pages.OnAfterGetRecord(231, func(rec *gtables.ProjectBudgetLine, page *pages.Control) {
		capacity := budgetLineCapacity(rec)
		page.SetValue("capacity", capacity)
		setCapacityAfter(page, capacity, rec.Res_gr_qty_on_order, rec.Res_gr_qty_quoted)
	})
}

// setCapacityAfter: CapacityAfterOrders := Capacity - Qty. on Order; CapacityAfterQuotes :=
// CapacityAfterOrders - Qty. Quoted
func setCapacityAfter(page *pages.Control, capacity, onOrder, quoted types.Decimal) {
	afterOrders := capacity.Sub(onOrder)
	page.SetValue("capacity_after_orders", afterOrders)
	page.SetValue("capacity_after_quotes", afterOrders.Sub(quoted))
}

// budgetLineCapacity: the capacity of a budget line's resource or resource group in the line's
// Date Filter (0 for an item or G/L account line)
func budgetLineCapacity(rec *gtables.ProjectBudgetLine) types.Decimal {
	switch rec.Type {
	case gtables.ProjectBudgetLine_Type.Resource:
		res := gtables.NewResource(rec)
		res.No = rec.No
		_ = res.SetFlowFilter("date_filter", rec.Date_filter)
		res.CalcFields("capacity")
		return res.Capacity
	case gtables.ProjectBudgetLine_Type.GroupResource:
		resGr := gtables.NewResourceGroup(rec)
		resGr.No = rec.No
		_ = resGr.SetFlowFilter("date_filter", rec.Date_filter)
		resGr.CalcFields("capacity")
		return resGr.Capacity
	}
	return types.Decimal{}
}
