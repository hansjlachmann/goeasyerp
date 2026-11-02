package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 794 Adjust Item Costs/Prices: multiplies a cost or price field of the items by an
// adjustment factor (rounded by a rounding method, else to 0.00001) and validates it, so the
// fields that follow it are calculated again. The Average Cost of an item with the Average
// costing method cannot be changed once it has ledger entries.
const AdjustItemCostsPricesID = 794

func init() {
	Register(AdjustItemCostsPricesID, "adjust-item-costs-prices", func() Report { return &adjustItemCostsPrices{} })
}

// The fields the report can adjust (classic Selection; the additional-currency average cost
// is left out — no additional reporting currency)
const (
	adjustItemUnitPrice = iota + 1
	adjustItemProfitPercent
	adjustItemIndirectCostPercent
	adjustItemAverageCost
	adjustItemLastDirectCost
	adjustItemStandardCost
)

type adjustItemCostsPrices struct {
	selection      int
	adjFactor      types.Decimal
	roundingMethod *gtables.RoundingMethod
	roundingCode   types.Code
	window         *system.Window
}

func (r *adjustItemCostsPrices) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "adjust_field", Type: "option", Default: "1",
				Options: []string{"UNIT_PRICE", "PROFIT_PERCENT", "INDIRECT_COST_PERCENT", "AVERAGE_COST", "LAST_DIRECT_COST", "STANDARD_COST"}},
			{Name: "adjustment_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Item", Table: "Item", Key: []string{"no"},
			RequestFilterFields: []string{"no", "vendor_no", "inventory_posting_group", "costing_method"},
			OnPreDataItem:       Trigger(r.itemOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
		}},
	}
}

func (r *adjustItemCostsPrices) onPreReport(run *Run) error {
	r.selection = run.OptionInt("adjust_field")
	r.adjFactor = run.OptionDecimal("adjustment_factor")
	r.roundingCode = types.Code(system.UpperCase(run.Option("rounding_method")))
	r.roundingMethod = gtables.NewRoundingMethod(run)
	return nil
}

func (r *adjustItemCostsPrices) itemOnPreDataItem(item *gtables.Item, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *adjustItemCostsPrices) itemOnAfterGetRecord(item *gtables.Item, run *Run) error {
	r.window.Update(1, item.No)
	if item.Costing_method == gtables.Item_Costing_method.Average && r.selection == adjustItemAverageCost {
		itemLedgEntry := gtables.NewItemLedgerEntry(run)
		itemLedgEntry.SetRange("item_no", item.No)
		if itemLedgEntry.FindFirst() {
			return apperrors.ItemAdjustAverageCost(item.No.String(), item.Costing_method)
		}
	}

	var oldFieldNo types.Decimal
	switch r.selection {
	case adjustItemUnitPrice:
		oldFieldNo = item.Unit_price
	case adjustItemProfitPercent:
		oldFieldNo = item.Profit_percent
	case adjustItemIndirectCostPercent:
		oldFieldNo = item.Indirect_cost_percent
	case adjustItemAverageCost:
		oldFieldNo = item.Average_cost
	case adjustItemLastDirectCost:
		oldFieldNo = item.Last_direct_cost
	case adjustItemStandardCost:
		oldFieldNo = item.Standard_cost
	}
	newFieldNo := oldFieldNo.Mul(r.adjFactor)
	rounded := tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, newFieldNo)
	priceIsRnded := r.roundingCode != "" && roundingMethodRounds(r.roundingMethod, r.roundingCode, newFieldNo)
	newFieldNo = rounded
	if !priceIsRnded {
		newFieldNo = system.Round(newFieldNo, types.MustDecimal("0.00001"))
	}

	var err error
	switch r.selection {
	case adjustItemUnitPrice:
		err = item.Validate("unit_price", newFieldNo)
	case adjustItemProfitPercent:
		err = item.Validate("profit_percent", newFieldNo)
	case adjustItemIndirectCostPercent:
		err = item.Validate("indirect_cost_percent", newFieldNo)
	case adjustItemAverageCost:
		err = item.Validate("average_cost", newFieldNo)
	case adjustItemLastDirectCost:
		err = item.Validate("last_direct_cost", newFieldNo)
	case adjustItemStandardCost:
		err = item.Validate("standard_cost", newFieldNo)
	}
	if err != nil {
		return err
	}
	return item.ModifyOrError(true)
}

// roundingMethodRounds: whether the rounding method's line for the amount rounds it to a
// precision (PriceIsRnded)
func roundingMethodRounds(roundingMethod *gtables.RoundingMethod, code types.Code, amount types.Decimal) bool {
	roundingMethod.Reset()
	roundingMethod.SetRange("code", code)
	roundingMethod.Code = code
	roundingMethod.Minimum_amount = amount
	return roundingMethod.Find("=<") && roundingMethod.Precision.GreaterThan(types.Decimal{})
}
