// Package format prints dates and decimals for the user: in the user's date and number formats
// (My Settings), else the language's — printouts and messages that name a date or an amount.
package format

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// numberFormat: the thousands and decimal separators and the date layout of a language
type numberFormat struct {
	thousands string
	decimal   string
	date      string // Go layout of a date
}

var numberFormats = map[string]numberFormat{
	"en-US": {",", ".", "01/02/2006"},
	"nb-NO": {" ", ",", "02.01.2006"},
	"da-DK": {".", ",", "02.01.2006"},
}

// regionalDateLayouts and regionalNumberFormats: the user's own formats (My Settings) as Go date
// layouts and separators
var (
	regionalDateLayouts = map[string]string{
		"dd.mm.yy": "02.01.06", "dd.mm.yyyy": "02.01.2006", "dd/mm/yyyy": "02/01/2006",
		"mm/dd/yyyy": "01/02/2006", "yyyy-mm-dd": "2006-01-02",
	}
	regionalNumberFormats = map[string][2]string{
		"1,234.56": {",", "."}, "1.234,56": {".", ","}, "1 234,56": {" ", ","}, "1234.56": {"", "."},
	}
)

// formatOf is the format used: the language's (en-US for languages without one),
// with the user's date and number formats of My Settings where set
func formatOf(language string) numberFormat {
	f, ok := numberFormats[language]
	if !ok {
		f = numberFormats["en-US"]
	}
	regional := fcodeunits.CurrentRegional()
	layout, ok := regionalDateLayouts[regional.DateFormat]
	if ok {
		f.date = layout
	}
	separators, ok := regionalNumberFormats[regional.NumberFormat]
	if ok {
		f.thousands, f.decimal = separators[0], separators[1]
	}
	return f
}

// Decimal prints a decimal in the language with decimal places "min:max" (blank: 0:5),
// rounded half away from zero at max
func Decimal(d types.Decimal, places string, language string) string {
	min, max := 0, 5
	minText, maxText, ok := strings.Cut(places, ":")
	if ok {
		min, _ = strconv.Atoi(minText)
		max, _ = strconv.Atoi(maxText)
	}
	text := d.Round(int32(max)).StringFixed(int32(max))
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign, text = "-", text[1:]
	}
	intPart, frac, _ := strings.Cut(text, ".")
	frac = strings.TrimRight(frac, "0")
	for len(frac) < min {
		frac += "0"
	}
	if strings.Trim(intPart+frac, "0") == "" {
		sign = ""
	}
	f := formatOf(language)
	var grouped strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			grouped.WriteString(f.thousands)
		}
		grouped.WriteRune(c)
	}
	if frac == "" {
		return sign + grouped.String()
	}
	return sign + grouped.String() + f.decimal + frac
}

// Date prints a date in the language (blank for a blank date)
func Date(d types.Date, language string) string {
	if d.IsZero() {
		return ""
	}
	return d.Time.Format(formatOf(language).date)
}
