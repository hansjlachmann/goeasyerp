package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1195 Adjust Resource Costs/Prices: multiplies a cost or price field of the resources
// by an adjustment factor (rounded by a rounding method) and validates it, so the fields that
// follow it are calculated again.
const AdjustResCostsPricesID = 1195

func init() {
	Register(AdjustResCostsPricesID, "adjust-resource-costs-prices", func() Report { return &adjustResCostsPrices{} })
}

// The fields the report can adjust (classic Selection)
const (
	adjustDirectUnitCost = iota + 1
	adjustIndirectCostPercent
	adjustUnitCost
	adjustProfitPercent
	adjustUnitPrice
)

type adjustResCostsPrices struct {
	selection      int
	adjFactor      types.Decimal
	roundingMethod *gtables.RoundingMethod
	roundingCode   types.Code
	window         *system.Window
}

func (r *adjustResCostsPrices) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "adjust_field", Type: "option", Default: "5",
				Options: []string{"DIRECT_UNIT_COST", "INDIRECT_COST_PERCENT", "UNIT_COST", "PROFIT_PERCENT", "UNIT_PRICE"}},
			{Name: "adjustment_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Resource", Table: "Resource", Key: []string{"no"},
			RequestFilterFields: []string{"no", "resource_group_no"},
			OnPreDataItem:       Trigger(r.resourceOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.resourceOnAfterGetRecord),
		}},
	}
}

func (r *adjustResCostsPrices) onPreReport(run *Run) error {
	r.selection = run.OptionInt("adjust_field")
	r.adjFactor = run.OptionDecimal("adjustment_factor")
	r.roundingCode = types.Code(system.UpperCase(run.Option("rounding_method")))
	r.roundingMethod = gtables.NewRoundingMethod(run)
	return nil
}

func (r *adjustResCostsPrices) resourceOnPreDataItem(resource *gtables.Resource, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *adjustResCostsPrices) resourceOnAfterGetRecord(resource *gtables.Resource, run *Run) error {
	r.window.Update(1, resource.No)
	var oldFieldNo types.Decimal
	switch r.selection {
	case adjustDirectUnitCost:
		oldFieldNo = resource.Direct_unit_cost
	case adjustIndirectCostPercent:
		oldFieldNo = resource.Indirect_cost_percent
	case adjustUnitCost:
		oldFieldNo = resource.Unit_cost
	case adjustProfitPercent:
		oldFieldNo = resource.Profit_percent
	case adjustUnitPrice:
		oldFieldNo = resource.Unit_price
	}
	newFieldNo := tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, oldFieldNo.Mul(r.adjFactor))
	var err error
	switch r.selection {
	case adjustDirectUnitCost:
		err = resource.Validate("direct_unit_cost", newFieldNo)
	case adjustIndirectCostPercent:
		err = resource.Validate("indirect_cost_percent", newFieldNo)
	case adjustUnitCost:
		err = resource.Validate("unit_cost", newFieldNo)
	case adjustProfitPercent:
		err = resource.Validate("profit_percent", newFieldNo)
	case adjustUnitPrice:
		err = resource.Validate("unit_price", newFieldNo)
	}
	if err != nil {
		return err
	}
	return resource.ModifyOrError(true)
}
