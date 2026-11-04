package tables

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The classic Resource code: numbers from the Resource Nos. series of the Resources Setup,
// Search Name, Unit Cost from Direct Unit Cost and Indirect Cost %, the price/profit
// calculation, the capacity entries following the resource group, and the delete of the
// resource's capacity entries, costs, prices and comments; a price or cost for all resources
// has no code
func TestResourceTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Resource{}, &gtables.ResourceGroup{}, &gtables.ResourcesSetup{}, &gtables.ResCapacityEntry{},
		&gtables.ResourceCost{}, &gtables.ResourcePrice{}, &gtables.CommentLine{}, &gtables.UnitOfMeasure{},
		&gtables.GenProductPostingGroup{}, &gtables.TaxProductPostingGroup{}, &gtables.PostCode{}, &gtables.ResLedgerEntry{}, &gtables.AccountingPeriod{},
		&gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	series := gtables.NewNoSeries(src)
	series.Code, series.Default_nos = "RES", true
	line := gtables.NewNoSeriesLine(src)
	line.InitRecord()
	line.Series_code, line.Line_no, line.Starting_no, line.Increment_by_no = "RES", 10000, "R0010", 10
	resSetup := gtables.NewResourcesSetup(src)
	resSetup.Resource_nos = "RES"
	for _, g := range []types.Code{"G1", "G2"} {
		group := gtables.NewResourceGroup(src)
		group.No = g
		if err := group.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	for _, err := range []error{series.InsertOrError(false), line.InsertOrError(false), resSetup.InsertOrError(false)} {
		if err != nil {
			t.Fatal(err)
		}
	}

	res := gtables.NewResource(src)
	res.InitRecord()
	must(t, res.Validate("name", types.NewText("Anna Berg")))
	must(t, res.InsertOrError(true))
	if res.No != "R0010" || res.No_series != "RES" || res.Search_name != "ANNA BERG" {
		t.Errorf("new resource = %s %s %q, want R0010 from RES, search name ANNA BERG", res.No, res.No_series, res.Search_name)
	}

	// Unit Cost = Direct Unit Cost + Indirect Cost %; Profit % from the price
	must(t, res.Validate("unit_price", types.MustDecimal("200")))
	must(t, res.Validate("direct_unit_cost", types.MustDecimal("80")))
	must(t, res.Validate("indirect_cost_percent", types.MustDecimal("25")))
	if !res.Unit_cost.Equal(types.MustDecimal("100")) || !res.Profit_percent.Equal(types.MustDecimal("50")) {
		t.Errorf("unit cost %s, profit %% %s, want 100 and 50", res.Unit_cost, res.Profit_percent)
	}
	// Price=Cost+Profit: the price from the cost
	must(t, res.Validate("price_profit_calculation", gtables.Resource_Price_profit_calculation.PriceCostProfit))
	must(t, res.Validate("profit_percent", types.MustDecimal("60")))
	if !res.Unit_price.Equal(types.MustDecimal("250")) {
		t.Errorf("unit price %s, want 250", res.Unit_price)
	}
	must(t, res.Validate("resource_group_no", types.Code("G1")))
	must(t, res.ModifyOrError(true))

	// capacity entries follow the group
	for i, d := range []int{1, 2} {
		entry := gtables.NewResCapacityEntry(src)
		entry.Entry_no = i + 1
		entry.Date = types.NewDate(2026, 3, d)
		entry.Capacity = types.MustDecimal("7.5")
		must(t, entry.Validate("resource_no", res.No))
		must(t, entry.InsertOrError(true))
		if entry.Resource_group_no != "G1" {
			t.Errorf("capacity entry group %s, want G1", entry.Resource_group_no)
		}
	}
	must(t, res.Validate("resource_group_no", types.Code("G2")))
	entries := gtables.NewResCapacityEntry(src)
	entries.SetRange("resource_group_no", types.Code("G2"))
	if entries.Count() != 2 {
		t.Errorf("%d capacity entries in G2, want 2", entries.Count())
	}

	// a cost or price for all resources has no code
	resCost := gtables.NewResourceCost(src)
	resCost.Type = gtables.ResourceCost_Type.All
	var appErr *apperrors.AppError
	if err := resCost.Validate("code", types.Code("R0010")); !errors.As(err, &appErr) || appErr.Code != apperrors.ErrResCodeWhenTypeAll {
		t.Errorf("cost for all with a code: %v", err)
	}
	// a cost without work type is a fixed cost
	resCost.Type = gtables.ResourceCost_Type.Resource
	if err := resCost.Validate("cost_type", gtables.ResourceCost_Cost_type.Extra); err == nil {
		t.Error("% Extra cost without work type accepted")
	}
	must(t, resCost.Validate("code", res.No))
	resCost.Cost_type = gtables.ResourceCost_Cost_type.Fixed
	must(t, resCost.InsertOrError(true))
	resPrice := gtables.NewResourcePrice(src)
	resPrice.Code = res.No
	must(t, resPrice.InsertOrError(true))

	// delete: capacity entries, costs and prices go too
	must(t, res.DeleteOrError(true))
	for name, n := range map[string]int{"capacity entries": gtables.NewResCapacityEntry(src).Count(), "costs": gtables.NewResourceCost(src).Count(), "prices": gtables.NewResourcePrice(src).Count()} {
		if n != 0 {
			t.Errorf("%d %s left after the delete", n, name)
		}
	}
}
