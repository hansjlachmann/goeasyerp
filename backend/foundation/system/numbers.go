package system

import (
	"math"

	"github.com/shopspring/decimal"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Round rounds a decimal to a precision (default 0.01) in a direction: "=" to the nearest
// (half away from zero, default), "<" down, ">" up — Round(x, 0.01), Round(x, 1, ">").
func Round(d types.Decimal, args ...any) types.Decimal {
	precision := decimal.New(1, -2)
	direction := "="
	for _, a := range args {
		switch v := a.(type) {
		case types.Decimal:
			precision = v.Decimal
		case float64:
			precision = decimal.NewFromFloat(v)
		case int:
			precision = decimal.NewFromInt(int64(v))
		case string:
			direction = v
		}
	}
	if precision.IsZero() || precision.IsNegative() {
		precision = decimal.New(1, -2)
	}
	q := d.Decimal.Div(precision)
	switch direction {
	case "<":
		q = q.Floor()
	case ">":
		q = q.Ceil()
	default:
		if q.IsNegative() {
			q = q.Neg().Add(decimal.New(5, -1)).Floor().Neg()
		} else {
			q = q.Add(decimal.New(5, -1)).Floor()
		}
	}
	return types.Decimal{Decimal: q.Mul(precision)}
}

// Abs returns the absolute value of a decimal
func Abs(d types.Decimal) types.Decimal { return d.Abs() }

// Power raises a number to a power: exact for whole exponents, through float64 otherwise
func Power(base, exponent types.Decimal) types.Decimal {
	if exponent.Equal(types.Decimal{Decimal: exponent.Truncate(0)}) {
		return types.Decimal{Decimal: base.Pow(exponent.Decimal)}
	}
	return types.NewDecimal(math.Pow(base.Float64(), exponent.Float64()))
}
