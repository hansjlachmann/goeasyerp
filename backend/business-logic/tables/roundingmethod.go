package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// roundingMethodLogic is the business logic of table 42 Rounding Method.
type roundingMethodLogic struct{}

func init() { gtables.RegisterRoundingMethodLogic(roundingMethodLogic{}) }

// ApplyRoundingMethod rounds an amount by a rounding method (the block the price reports
// share): the method's line with the highest Minimum Amount up to the amount adds Amount
// Added Before, rounds to its Precision (nearest, up or down) and adds Amount Added After. A
// blank code leaves the amount.
func ApplyRoundingMethod(roundingMethod *gtables.RoundingMethod, code types.Code, amount types.Decimal) types.Decimal {
	if code == "" {
		return amount
	}
	roundingMethod.Reset()
	roundingMethod.SetRange("code", code)
	roundingMethod.Code = code
	roundingMethod.Minimum_amount = amount
	if roundingMethod.Find("=<") {
		amount = amount.Add(roundingMethod.Amount_added_before)
		if roundingMethod.Precision.GreaterThan(types.Decimal{}) {
			amount = system.Round(amount, roundingMethod.Precision, system.CopyStr("=><", int(roundingMethod.Type)+1, 1))
		}
		amount = amount.Add(roundingMethod.Amount_added_after)
	}
	return amount
}
