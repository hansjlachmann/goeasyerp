package types

import (
	"database/sql/driver"
	"fmt"
	"math"
	"strings"
	"time"
)

// Date represents a date without time component (Date type)
// Stored in database as "YYYY-MM-DD" format
//
// A closing date (ClosingDate) lies between a date and the next day — the period-end date
// that closing entries are posted on. It is kept as the date at 23:59:59, so it sorts after
// the date and before the next day; it reads and prints as "C2026-12-31". The database keeps
// it as "2026-12-31 23:59:59" (a TEXT column on SQLite, a TIMESTAMP column on PostgreSQL;
// normal dates as "2026-12-31", midnight): it sorts between the date and the next day, a range
// up to the date ("..2026-12-31") leaves it out and one up to the closing date
// ("..C2026-12-31") takes it in.
type Date struct {
	time.Time
}

// closingPrefix marks a closing date in text: "C2026-12-31"
const closingPrefix = "C"

// closingTime is the time of day a closing date is stored with
const closingTime = "23:59:59"

// Closing returns the closing date of d (ClosingDate); a blank date stays blank
func (d Date) Closing() Date {
	if d.IsZero() {
		return d
	}
	return Date{time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.UTC)}
}

// Normal returns the normal date of a closing date, d itself otherwise (NormalDate)
func (d Date) Normal() Date {
	if d.IsZero() {
		return d
	}
	return NewDate(d.Year(), d.Month(), d.Day())
}

// IsClosing reports whether d is a closing date
func (d Date) IsClosing() bool {
	return !d.IsZero() && d.Hour() == 23 && d.Minute() == 59 && d.Second() == 59
}

// NewDate creates a new Date from year, month, day
func NewDate(year int, month time.Month, day int) Date {
	return Date{time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// NewDateFromTime creates a Date from a time.Time (strips time component)
func NewDateFromTime(t time.Time) Date {
	return Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

// NewDateFromString parses a date string in "YYYY-MM-DD" format
func NewDateFromString(value string) (Date, error) {
	if value == "" {
		return Date{}, nil
	}
	if rest, ok := strings.CutPrefix(strings.ToUpper(value), closingPrefix); ok {
		d, err := NewDateFromString(rest)
		if err != nil || d.IsZero() {
			return Date{}, fmt.Errorf("invalid closing date %q, expected CYYYY-MM-DD", value)
		}
		return d.Closing(), nil
	}

	datePart, timePart := value, ""
	if len(value) > 10 && (value[10] == 'T' || value[10] == ' ') {
		// Stored with a time: a TIMESTAMP column read into a string on PostgreSQL
		// ("2026-10-21T00:00:00Z"), a closing date ("2026-12-31 23:59:59")
		datePart, timePart = value[:10], value[11:]
	}
	t, err := time.Parse("2006-01-02", datePart)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	}
	d := Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
	if strings.HasPrefix(timePart, closingTime) {
		return d.Closing(), nil
	}
	return d, nil
}

// MustDate creates a Date from a string, panics on error
func MustDate(value string) Date {
	d, err := NewDateFromString(value)
	if err != nil {
		panic(fmt.Sprintf("invalid date value: %s", value))
	}
	return d
}

// Today returns today's date
func Today() Date {
	now := time.Now()
	return Date{time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}
}

// ZeroDate returns a zero Date value (0001-01-01)
func ZeroDate() Date {
	return Date{time.Time{}}
}

// String returns the date in "YYYY-MM-DD" format (ISO 8601)
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	if d.IsClosing() {
		return closingPrefix + d.Time.Format("2006-01-02")
	}
	return d.Time.Format("2006-01-02")
}

// Format returns the date formatted according to the layout
func (d Date) Format(layout string) string {
	return d.Time.Format(layout)
}

// IsZero returns true if the date is the zero value
func (d Date) IsZero() bool {
	return d.Time.IsZero()
}

// Equal checks if two dates are the same day (a closing date only equals itself)
func (d Date) Equal(other Date) bool {
	return d.Year() == other.Year() &&
		d.Month() == other.Month() &&
		d.Day() == other.Day() &&
		d.IsClosing() == other.IsClosing()
}

// Before checks if d is before other
func (d Date) Before(other Date) bool {
	return d.Time.Before(other.Time)
}

// After checks if d is after other
func (d Date) After(other Date) bool {
	return d.Time.After(other.Time)
}

// AddDays adds the specified number of days to the date
func (d Date) AddDays(days int) Date {
	newDate := d.Time.AddDate(0, 0, days)
	return Date{newDate}
}

// AddMonths adds the specified number of months to the date
func (d Date) AddMonths(months int) Date {
	newDate := d.Time.AddDate(0, months, 0)
	return Date{newDate}
}

// AddYears adds the specified number of years to the date
func (d Date) AddYears(years int) Date {
	newDate := d.Time.AddDate(years, 0, 0)
	return Date{newDate}
}

// DaysBetween returns the number of days between two dates
func (d Date) DaysBetween(other Date) int {
	duration := other.Time.Sub(d.Time)
	return int(math.Round(duration.Hours() / 24)) // a daylight saving change makes a day 23 or 25 hours
}

// UnmarshalText implements encoding.TextUnmarshaler for JSON/form input
func (d *Date) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		d.Time = time.Time{}
		return nil
	}

	parsed, err := NewDateFromString(string(text))
	if err != nil {
		return err
	}

	d.Time = parsed.Time
	return nil
}

// MarshalText implements encoding.TextMarshaler for JSON output
func (d Date) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return []byte(""), nil
	}
	return []byte(d.String()), nil
}

// Scan implements sql.Scanner for database scanning
func (d *Date) Scan(value interface{}) error {
	if value == nil {
		d.Time = time.Time{}
		return nil
	}

	switch v := value.(type) {
	case string:
		parsed, err := NewDateFromString(v)
		if err != nil {
			return err
		}
		d.Time = parsed.Time
	case []byte:
		parsed, err := NewDateFromString(string(v))
		if err != nil {
			return err
		}
		d.Time = parsed.Time
	case time.Time:
		d.Time = time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
		if v.Hour() == 23 && v.Minute() == 59 && v.Second() == 59 {
			d.Time = d.Closing().Time
		}
	default:
		return fmt.Errorf("unsupported type for Date: %T", v)
	}

	return nil
}

// Value implements driver.Valuer for database storage: "YYYY-MM-DD", a closing date
// "YYYY-MM-DD 23:59:59"
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.StorageString(), nil
}

// StorageString is the date as the database keeps it and compares it: "2026-12-31", a closing
// date "2026-12-31 23:59:59" ("" for a blank date)
func (d Date) StorageString() string {
	if d.IsZero() {
		return ""
	}
	if d.IsClosing() {
		return d.Time.Format("2006-01-02") + " " + closingTime
	}
	return d.Time.Format("2006-01-02")
}

// FilterArg is a date of a filter expression as the database compares it: a date
// ("2026-12-31", "C2026-12-31") as StorageString, other text unchanged (the database tells)
func FilterArg(value string) string {
	d, err := NewDateFromString(strings.TrimSpace(value))
	if err != nil || d.IsZero() {
		return value
	}
	return d.StorageString()
}

// BlankKeyDate is how a blank date is stored in a primary key column: a key column cannot hold
// NULL (PostgreSQL) and NULL never equals itself in a key lookup. It reads back as the blank
// date (the zero time) and sorts before every date.
const BlankKeyDate = "0001-01-01"

// KeyDate is the database value of a primary key field: a blank date becomes BlankKeyDate, other
// values are unchanged
func KeyDate(v interface{}) interface{} {
	d, ok := v.(Date)
	if ok && d.IsZero() {
		return BlankKeyDate
	}
	return v
}
