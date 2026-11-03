package reports_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Item costs with euro as the additional reporting currency (11.50): report 86 gives the
// entries and items their euro costs; a purchase at 115 costs 10 euro a piece (entry, item's
// Add.-Curr. Average Cost, the inventory posting in euro); a FIFO sale adjusted by report 795
// takes the purchase's euro cost, report 1002 posts what is left in euro; the G/L balances in
// both currencies
func TestItemCostsAddCurrency(t *testing.T) {
	src := costCompany(t)
	_, err := reports.Execute(reports.AdjustAddReportingCurrencyID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"additional_reporting_currency": "EUR", "document_no": "ACY", "retained_earnings_acc": "2120"}})
	mustOK(t, err)
	item1000 := gtables.NewItem(src)
	mustOK(t, item1000.GetOrError(types.Code("1000")))
	if item1000.Average_cost.IsPositive() && !item1000.Add_curr_average_cost.IsPositive() {
		t.Errorf("item 1000 average cost %s, in euro %s", item1000.Average_cost, item1000.Add_curr_average_cost)
	}
	old := gtables.NewItemLedgerEntry(src)
	old.SetRange("item_no", types.Code("1000"))
	if !old.FindFirst() || old.Adjusted_cost_qty.IsPositive() && !old.Add_currency_adj_cost_qty.IsPositive() {
		t.Errorf("item 1000's first entry: cost %s, in euro %s", old.Adjusted_cost_qty, old.Add_currency_adj_cost_qty)
	}

	postItem(t, src, gtables.ItemJournalLine_Entry_type.Purchase, "10", "115")
	purchase := lastEntry(t, src)
	if !purchase.Additional_currency_unit_cost.Equal(types.MustDecimal("10")) || !purchase.Add_curr_adj_cost_inv_qty.Equal(types.MustDecimal("100")) ||
		!purchase.Add_curr_cost_posted_to_gl.Equal(types.MustDecimal("100")) {
		t.Errorf("purchase in euro: unit cost %s, cost %s, posted %s; want 10, 100, 100",
			purchase.Additional_currency_unit_cost, purchase.Add_curr_adj_cost_inv_qty, purchase.Add_curr_cost_posted_to_gl)
	}
	item := gtables.NewItem(src)
	mustOK(t, item.GetOrError(types.Code("T1")))
	if !item.Add_curr_average_cost.Equal(types.MustDecimal("10")) {
		t.Errorf("T1's Add.-Curr. Average Cost %s, want 10", item.Add_curr_average_cost)
	}

	postItem(t, src, gtables.ItemJournalLine_Entry_type.Sale, "4", "")
	runReport(t, src, reports.AdjustCostItemEntriesID, nil)
	sale := lastEntry(t, src)
	if !sale.Adjusted_cost_qty.Equal(types.MustDecimal("-460")) || !sale.Add_currency_adj_cost_qty.Equal(types.MustDecimal("-40")) ||
		!sale.Add_curr_adj_cost_inv_qty.Equal(types.MustDecimal("-40")) {
		t.Errorf("adjusted sale: %s, in euro %s / %s; want -460, -40", sale.Adjusted_cost_qty, sale.Add_currency_adj_cost_qty, sale.Add_curr_adj_cost_inv_qty)
	}
	_, err = reports.Execute(reports.PostInventoryCostToGLID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"post_level": "2", "posting_date": "2026-10-06"}})
	mustOK(t, err)
	sale = lastEntry(t, src)
	if !sale.Add_curr_cost_posted_to_gl.Equal(types.MustDecimal("-40")) || !sale.Cost_posted_to_gl.Equal(types.MustDecimal("-460")) {
		t.Errorf("sale posted to G/L %s, in euro %s; want -460, -40", sale.Cost_posted_to_gl, sale.Add_curr_cost_posted_to_gl)
	}
	lcy, acy := acyTotals(t, src)
	if !lcy.IsZero() || !acy.IsZero() {
		t.Errorf("G/L totals: LCY %s, euro %s", lcy, acy)
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L check: %+v", r)
	}
}
