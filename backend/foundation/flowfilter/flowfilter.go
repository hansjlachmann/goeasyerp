// Package flowfilter turns FlowFilter expressions into SQL conditions.
//
// A FlowFilter field (FieldClass FlowFilter) is not stored: it holds a filter the user
// sets, e.g. a Date Filter "2026-01-01..2026-03-31", and FlowFields apply it to a field of
// their source table (Posting Date = FIELD(Date Filter)). Clause builds the SQL condition
// for one column from such an expression.
//
// Syntax: alternatives separated by '|'; each alternative is a value, a range "a..b",
// an open range "..b" or "a..", "<>value", or a comparison "<value", "<=value", ">value",
// ">=value". Text and Code accept '*' wildcards and ” for the blank value ("”|MAIN"). Booleans accept Yes/No, true/false, 1/0.
// Dates are ISO (YYYY-MM-DD), a closing date with a C (C2026-12-31: after the date, so
// "..2026-12-31" leaves it out) — the frontend converts the user's local date format before
// sending. Decimals accept a decimal point or comma ("1500,50"), no thousands separators.
//
// The same syntax filters FlowFields in lists (Customer Balance ">10000"): there the
// "column" is the FlowField's SQL expression (ClauseFor).
package flowfilter

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Kind is the data type of a FlowFilter field.
type Kind string

const (
	KindDate Kind = "date"
	KindBool Kind = "bool"
	KindInt  Kind = "int"
	KindText Kind = "text"
	KindCode Kind = "code"
	// KindDecimal: FlowField sums (filtering a list on e.g. Balance)
	KindDecimal Kind = "decimal"
)

// Clause returns the SQL condition (with ? placeholders) and its arguments that apply expr
// to column. An empty expression means no filter: "" and no arguments.
func Clause(column string, kind Kind, expr string) (string, []interface{}, error) {
	return ClauseFor(column, nil, kind, expr)
}

// ClauseFor is Clause for a column that is an SQL expression with its own arguments (a
// FlowField's subquery): the expression is repeated in each alternative, so its arguments
// are repeated before that alternative's values.
func ClauseFor(column string, columnArgs []interface{}, kind Kind, expr string) (string, []interface{}, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", nil, nil
	}
	var ors []string
	var args []interface{}
	for _, part := range strings.Split(expr, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", nil, fmt.Errorf("empty value in filter %q", expr)
		}
		cond, a, err := alternative(column, kind, part)
		if err != nil {
			return "", nil, err
		}
		ors = append(ors, cond)
		args = append(args, columnArgs...)
		args = append(args, a...)
	}
	if len(ors) == 1 {
		return ors[0], args, nil
	}
	return "(" + strings.Join(ors, " OR ") + ")", args, nil
}

// UpperLimit returns the filter up to the upper end of expr, from the start (UpperLimit:
// "Balance at Date" sums the entries up to the Date Filter's last date): "2026-01-01..2026-03-31"
// gives "..2026-03-31", "2026-03-31" too. An expression without an upper end ("2026-01-01..",
// "<>x", wildcards) or an invalid one gives "" — no filter. Of several alternatives the highest
// upper end counts.
func UpperLimit(kind Kind, expr string) string {
	if strings.TrimSpace(expr) == "" || Validate(kind, expr) != nil {
		return ""
	}
	best, bestValue, strict := "", interface{}(nil), false
	for _, part := range strings.Split(expr, "|") {
		part = strings.TrimSpace(part)
		end, isStrict := "", false
		switch {
		case strings.HasPrefix(part, "<>") || strings.HasPrefix(part, ">") || strings.Contains(part, "*"):
			return ""
		case strings.HasPrefix(part, "<="):
			end = strings.TrimSpace(part[2:])
		case strings.HasPrefix(part, "<"):
			end, isStrict = strings.TrimSpace(part[1:]), true
		case strings.Contains(part, ".."):
			_, end, _ = strings.Cut(part, "..")
			end = strings.TrimSpace(end)
			if end == "" {
				return ""
			}
		default:
			end = part
		}
		v, err := value(kind, end)
		if err != nil {
			return ""
		}
		if c := compare(v, bestValue); bestValue == nil || c > 0 || (c == 0 && !isStrict) {
			best, bestValue, strict = end, v, isStrict
		}
	}
	if strict {
		return "<" + best
	}
	return ".." + best
}

// compare orders two filter values of the same kind (-1, 0, 1); nil is lowest
func compare(a, b interface{}) int {
	if b == nil {
		return 1
	}
	switch x := a.(type) {
	case int64:
		return cmp.Compare(x, b.(int64))
	case float64:
		return cmp.Compare(x, b.(float64))
	case string:
		return cmp.Compare(x, b.(string)) // dates are ISO: their text sorts by date
	case bool:
		return 0
	}
	return 0
}

// Validate reports whether expr is a valid filter for kind.
func Validate(kind Kind, expr string) error {
	_, _, err := Clause("x", kind, expr)
	return err
}

func alternative(column string, kind Kind, part string) (string, []interface{}, error) {
	if rest, ok := strings.CutPrefix(part, "<>"); ok {
		v, err := value(kind, strings.TrimSpace(rest))
		if err != nil {
			return "", nil, err
		}
		return column + " <> ?", []interface{}{v}, nil
	}
	if kind != KindBool {
		for _, op := range []string{"<=", ">=", "<", ">"} {
			if rest, ok := strings.CutPrefix(part, op); ok {
				v, err := value(kind, strings.TrimSpace(rest))
				if err != nil {
					return "", nil, err
				}
				return column + " " + op + " ?", []interface{}{v}, nil
			}
		}
	}

	if kind != KindBool {
		if from, to, isRange := strings.Cut(part, ".."); isRange {
			from, to = strings.TrimSpace(from), strings.TrimSpace(to)
			switch {
			case from == "" && to == "":
				return "", nil, fmt.Errorf("range %q has no start or end", part)
			case from == "":
				v, err := value(kind, to)
				if err != nil {
					return "", nil, err
				}
				return column + " <= ?", []interface{}{v}, nil
			case to == "":
				v, err := value(kind, from)
				if err != nil {
					return "", nil, err
				}
				return column + " >= ?", []interface{}{v}, nil
			default:
				a, err := value(kind, from)
				if err != nil {
					return "", nil, err
				}
				b, err := value(kind, to)
				if err != nil {
					return "", nil, err
				}
				return column + " BETWEEN ? AND ?", []interface{}{a, b}, nil
			}
		}
	}

	if (kind == KindText || kind == KindCode) && strings.Contains(part, "*") {
		pattern := part
		if kind == KindCode {
			pattern = strings.ToUpper(pattern)
		}
		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(pattern)
		return column + ` LIKE ? ESCAPE '\'`, []interface{}{strings.ReplaceAll(escaped, "*", "%")}, nil
	}

	v, err := value(kind, part)
	if err != nil {
		return "", nil, err
	}
	return column + " = ?", []interface{}{v}, nil
}

// value parses one filter value of the given kind into its SQL argument.
func value(kind Kind, s string) (interface{}, error) {
	if s == "" {
		return nil, fmt.Errorf("missing value")
	}
	if s == "''" && (kind == KindText || kind == KindCode) {
		return "", nil
	}
	switch kind {
	case KindDate:
		// ISO, a closing date with its C ("C2026-12-31"): as the database keeps it
		d, err := types.NewDateFromString(s)
		if err != nil || d.IsZero() || len(s) < 10 {
			return nil, fmt.Errorf("%q is not a date", s)
		}
		return d.StorageString(), nil
	case KindInt:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", s)
		}
		return n, nil
	case KindDecimal:
		// A decimal comma (nb-NO, da-DK) is read as the decimal point
		if !strings.Contains(s, ".") {
			s = strings.Replace(s, ",", ".", 1)
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || strings.ContainsAny(s, "eEnN") {
			return nil, fmt.Errorf("%q is not a number", s)
		}
		return f, nil
	case KindBool:
		switch strings.ToLower(s) {
		case "yes", "true", "1", "ja":
			return true, nil
		case "no", "false", "0", "nei", "nej":
			return false, nil
		}
		return nil, fmt.Errorf("%q is not Yes or No", s)
	case KindCode:
		return strings.ToUpper(s), nil
	default:
		return s, nil
	}
}
