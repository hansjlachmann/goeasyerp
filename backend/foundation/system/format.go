package system

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Format returns a value as text in the Go-native, language-independent format: dates
// 2026-10-20 (a blank date gives ""), date-times RFC 3339, decimals 1234.5, booleans
// true/false, options their value name. With a length the text is padded with spaces or cut
// to it: a positive length is left-aligned, a negative one right-aligned.
func Format(value any, length ...int) string {
	s := format(value)
	if len(length) > 0 && length[0] != 0 {
		return PadStr(s, length[0])
	}
	return s
}

func format(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case types.Code:
		return string(v)
	case types.Text:
		return string(v)
	case types.Date:
		return v.String() // ISO, a closing date with C ("C2026-12-31")
	case types.DateTime:
		if v.IsZero() {
			return ""
		}
		return v.Format(time.RFC3339)
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.Format(time.RFC3339)
	case types.Decimal:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case fmt.Stringer: // option values, …
		return v.String()
	case error:
		return v.Error()
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.String:
		return rv.String()
	}
	return fmt.Sprint(value)
}

// Evaluate reads text into the variable target points to, in the Go-native formats Format
// writes (dates 2026-10-20, date-times RFC 3339, decimals 1234.5, booleans true/false/1/0);
// Code values are trimmed and upper-cased. It returns false — and leaves the variable as it
// was — when the text is not a value of that type.
func Evaluate(target any, text string) bool {
	text = strings.TrimSpace(text)
	switch p := target.(type) {
	case *string:
		*p = text
	case *types.Text:
		*p = types.Text(text)
	case *types.Code:
		*p = types.Code(strings.ToUpper(text))
	case *types.Date:
		d, err := types.NewDateFromString(text)
		if err != nil {
			return false
		}
		*p = d
	case *types.DateTime:
		if text == "" {
			*p = types.DateTime{}
			return true
		}
		t, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return false
		}
		*p = types.NewDateTimeFromTime(t)
	case *time.Time:
		t, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return false
		}
		*p = t
	case *types.Decimal:
		d, err := types.NewDecimalFromString(text)
		if err != nil {
			return false
		}
		*p = d
	case *bool:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return false
		}
		*p = b
	case *float64:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return false
		}
		*p = f
	default:
		// Integers of any kind, also option types (their index)
		rv := reflect.ValueOf(target)
		if rv.Kind() != reflect.Pointer || rv.IsNil() {
			return false
		}
		e := rv.Elem()
		switch e.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(text, 10, e.Type().Bits())
			if err != nil {
				return false
			}
			e.SetInt(n)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(text, 10, e.Type().Bits())
			if err != nil {
				return false
			}
			e.SetUint(n)
		default:
			return false
		}
	}
	return true
}
