// Package dateformula reads date formulas — "30D", "1M+CM", "-CY", "WD5" — and applies them
// to dates. A date formula is stored in a Code field marked date_formula: true (the generated
// field checks reject a formula Apply cannot read); business logic calculates with it through
// system.CalcDate. Both use this package, so they always agree on what a valid formula is.
package dateformula

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Apply returns the date calculated from a date formula and a date. A formula is a sequence
// of terms, with or without < >:
//
//	[+|-]<number><unit>   e.g. 1M, -10D, +2W — move by days (D), weeks (W), months (M),
//	                       quarters (Q) or years (Y); a month that is too short keeps its last
//	                       day (31 January + 1M = 28/29 February)
//	[+|-]C<unit>          C = current period: +CM the last day of the month, -CM the first
//	                       (CD the day itself, CW Sunday/Monday of the ISO week, CQ, CY)
//	WD<n>                 the next weekday n (1 = Monday … 7 = Sunday), the date itself when
//	                       it is that weekday
//
// Terms apply from left to right: "<1M+CM>" is the last day of next month, "<CM+1D>" the
// first day of next month. A blank date stays blank (the formula is still checked); an
// invalid formula is an error.
func Apply(formula string, d types.Date) (types.Date, error) {
	f := normalize(formula)
	if f == "" {
		return types.Date{}, fmt.Errorf("empty date formula")
	}
	t := d.Time
	if d.IsZero() {
		t = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	for i := 0; i < len(f); {
		sign := 1
		if f[i] == '+' || f[i] == '-' {
			if f[i] == '-' {
				sign = -1
			}
			i++
		}
		switch {
		case i < len(f) && f[i] == 'C':
			i++
			if i >= len(f) {
				return types.Date{}, fmt.Errorf("date formula %q: C needs a unit", formula)
			}
			var ok bool
			if t, ok = currentPeriod(t, f[i], sign); !ok {
				return types.Date{}, fmt.Errorf("date formula %q: unknown unit %q", formula, f[i])
			}
			i++
		case strings.HasPrefix(f[i:], "WD"):
			if sign < 0 {
				return types.Date{}, fmt.Errorf("date formula %q: WD cannot be negative", formula)
			}
			i += 2
			j := i
			for j < len(f) && f[j] >= '0' && f[j] <= '9' {
				j++
			}
			n, err := strconv.Atoi(f[i:j])
			if err != nil || n < 1 || n > 7 {
				return types.Date{}, fmt.Errorf("date formula %q: WD needs a weekday 1-7", formula)
			}
			i = j
			days := (n - isoWeekday(t) + 7) % 7
			t = t.AddDate(0, 0, days)
		default:
			j := i
			for j < len(f) && f[j] >= '0' && f[j] <= '9' {
				j++
			}
			if j == i || j >= len(f) {
				return types.Date{}, fmt.Errorf("date formula %q: expected a number and a unit at %q", formula, f[i:])
			}
			n, _ := strconv.Atoi(f[i:j])
			n *= sign
			switch f[j] {
			case 'D':
				t = t.AddDate(0, 0, n)
			case 'W':
				t = t.AddDate(0, 0, 7*n)
			case 'M':
				t = addMonths(t, n)
			case 'Q':
				t = addMonths(t, 3*n)
			case 'Y':
				t = addMonths(t, 12*n)
			default:
				return types.Date{}, fmt.Errorf("date formula %q: unknown unit %q", formula, f[j])
			}
			i = j + 1
		}
	}
	if d.IsZero() {
		return d, nil
	}
	return types.NewDateFromTime(t), nil
}

// Valid reports whether a date formula can be read; a blank formula is valid (an empty field)
func Valid(formula string) bool {
	if strings.TrimSpace(formula) == "" {
		return true
	}
	_, err := Apply(formula, types.Date{})
	return err == nil
}

// normalize: upper case, without spaces and the < > around the formula
func normalize(formula string) string {
	f := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(formula), " ", ""))
	if strings.HasPrefix(f, "<") && strings.HasSuffix(f, ">") {
		f = f[1 : len(f)-1]
	}
	return f
}

// isoWeekday: Monday 1 … Sunday 7
func isoWeekday(t time.Time) int {
	if wd := int(t.Weekday()); wd != 0 {
		return wd
	}
	return 7
}

// addMonths adds months, keeping the last day of a month that is too short
func addMonths(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(t.Day(), last), 0, 0, 0, 0, time.UTC)
}

// currentPeriod: the end (sign +) or start (sign -) of the current day, week, month, quarter
// or year
func currentPeriod(t time.Time, unit byte, sign int) (time.Time, bool) {
	switch unit {
	case 'D':
		return t, true
	case 'W':
		monday := t.AddDate(0, 0, 1-isoWeekday(t))
		if sign < 0 {
			return monday, true
		}
		return monday.AddDate(0, 0, 6), true
	case 'M', 'Q', 'Y':
		months := map[byte]int{'M': 1, 'Q': 3, 'Y': 12}[unit]
		startMonth := (int(t.Month())-1)/months*months + 1
		start := time.Date(t.Year(), time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
		if sign < 0 {
			return start, true
		}
		return start.AddDate(0, months, -1), true
	}
	return t, false
}
