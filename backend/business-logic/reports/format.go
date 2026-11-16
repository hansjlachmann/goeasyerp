package reports

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// formatDecimal prints a decimal in the language with decimal places "min:max" (blank: 0:5)
func formatDecimal(d types.Decimal, places string, language string) string {
	return format.Decimal(d, places, language)
}

// formatDate prints a date in the language (blank for a blank date)
func formatDate(d types.Date, language string) string {
	return format.Date(d, language)
}

// formats is how a printout prints numbers and dates: in the user's language with the user's
// formats (My Settings), or — a document in its customer's language (Run.SetLanguage) — in that
// language's own formats
type formats struct {
	language string
	document bool // the document's language: its own formats, not the user's
}

func (f formats) decimal(d types.Decimal, places string) string {
	if f.document {
		return format.DecimalInLanguage(d, places, f.language)
	}
	return format.Decimal(d, places, f.language)
}

func (f formats) date(d types.Date) string {
	if f.document {
		return format.DateInLanguage(d, f.language)
	}
	return format.Date(d, f.language)
}

// formatValue prints a report value: texts as they are, numbers and dates in the language,
// Yes/No
func formatValue(v interface{}, places string, language string) string {
	return formats{language: language}.value(v, places)
}

// value prints a report value: texts as they are, numbers and dates in the formats, Yes/No in
// the language
func (f formats) value(v interface{}, places string) string {
	language := f.language
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case types.Code:
		return value.String()
	case types.Text:
		return value.String()
	case types.Decimal:
		if places == "" {
			places = "2:2"
		}
		return f.decimal(value, places)
	case types.Date:
		return f.date(value)
	case int:
		return strconv.Itoa(value)
	case bool:
		key := "terms.no"
		if value {
			key = "terms.yes"
		}
		return i18n.GetInstance().CommonTranslation(key, language)
	}
	lines := linesOf(v)
	if lines != nil {
		return strings.Join(lines, "\n")
	}
	return fmt.Sprint(v)
}

// isSet: a value a section's or element's print_if takes as set — a text that is not blank, a
// number that is not 0, true, lines or a picture
func isSet(v interface{}) bool {
	switch value := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case types.Code:
		return value != ""
	case types.Text:
		return strings.TrimSpace(value.String()) != ""
	case types.Decimal:
		return !value.IsZero()
	case types.Date:
		return !value.IsZero()
	case int:
		return value != 0
	case bool:
		return value
	case []byte:
		return len(value) > 0
	}
	lines := linesOf(v)
	if lines != nil {
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				return true
			}
		}
		return false
	}
	return true
}

// isZero: a value blank_zero leaves out
func isZero(v interface{}) bool {
	switch value := v.(type) {
	case types.Decimal:
		return value.IsZero()
	case int:
		return value == 0
	}
	return false
}
