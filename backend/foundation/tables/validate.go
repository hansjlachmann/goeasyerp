package tables

import (
	"fmt"
	"unicode/utf8"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/dateformula"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Field checks for the tables' Validate triggers. The errors name the field, so the user
// sees its caption in their own language ("Name cannot exceed 50 characters").

// CheckRequired fails when a field that must have a value is empty.
func CheckRequired(table, field string, empty bool) error {
	if empty {
		return apperrors.FieldRequired(table, field)
	}
	return nil
}

// CheckMaxLength fails when value has more than max characters (characters, not bytes:
// "Ørsta" is 5 long, like the database's VARCHAR counts it).
func CheckMaxLength(table, field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return apperrors.FieldTooLong(table, field, max)
	}
	return nil
}

// CheckRange fails when value is outside min..max.
func CheckRange[T ~int | ~int64 | ~float64](table, field string, value, min, max T) error {
	if value < min || value > max {
		return apperrors.FieldOutOfRange(table, field, fmt.Sprint(min), fmt.Sprint(max))
	}
	return nil
}

// CheckMin fails when value is below min (a range with only a lower bound: MinValue=0).
func CheckMin[T ~int | ~int64 | ~float64](table, field string, value, min T) error {
	if value < min {
		return apperrors.FieldBelowMin(table, field, fmt.Sprint(min))
	}
	return nil
}

// CheckMax fails when value is above max (a range with only an upper bound).
func CheckMax[T ~int | ~int64 | ~float64](table, field string, value, max T) error {
	if value > max {
		return apperrors.FieldAboveMax(table, field, fmt.Sprint(max))
	}
	return nil
}

// CheckDecimalRange fails when a decimal is outside min..max; an empty bound is open
// ("0", "" = MinValue=0). The bounds come from the definition, so they are valid numbers.
func CheckDecimalRange(table, field string, value types.Decimal, min, max string) error {
	below := min != "" && value.LessThan(types.MustDecimal(min))
	above := max != "" && value.GreaterThan(types.MustDecimal(max))
	switch {
	case (below || above) && min != "" && max != "":
		return apperrors.FieldOutOfRange(table, field, min, max)
	case below:
		return apperrors.FieldBelowMin(table, field, min)
	case above:
		return apperrors.FieldAboveMax(table, field, max)
	}
	return nil
}

// CheckDateFormula fails when a date formula field (date_formula: true) holds a value that is
// not a date formula; a blank field is fine.
func CheckDateFormula(table, field, value string) error {
	if !dateformula.Valid(value) {
		return apperrors.DateFormulaInvalid(table, field, value)
	}
	return nil
}
