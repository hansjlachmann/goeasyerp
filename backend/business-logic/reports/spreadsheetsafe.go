package reports

import (
	"regexp"
	"strings"
)

// spreadsheetNumber: a number as a file holds it (a sign, digits, separators) — it starts with
// - or + but is no formula
var spreadsheetNumber = regexp.MustCompile(`^[+-]?[\d.,'\s]*\d[\d.,'\s]*%?$`)

// spreadsheetSafe is a text as a spreadsheet takes it as text: a value starting with = + - @
// (or a tab or line break) would run as a formula when the file is opened in a spreadsheet — a
// description =HYPERLINK(…) —, so it gets a leading ' (the spreadsheet's mark for text, not
// shown). Numbers stay as they are. spreadsheetText reads such a value back.
func spreadsheetSafe(value string) string {
	if value == "" || !strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return value
	}
	if spreadsheetNumber.MatchString(value) {
		return value
	}
	return "'" + value
}

// spreadsheetText is the value spreadsheetSafe wrote: the leading ' it added taken off
func spreadsheetText(value string) string {
	rest, found := strings.CutPrefix(value, "'")
	if !found || rest == "" || !strings.ContainsRune("=+-@\t\r", rune(rest[0])) {
		return value
	}
	if spreadsheetNumber.MatchString(rest) {
		return value
	}
	return rest
}
