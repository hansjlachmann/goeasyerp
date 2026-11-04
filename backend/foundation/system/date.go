package system

import (
	"fmt"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/dateformula"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Today returns today's date
func Today() types.Date { return types.Today() }

// WorkDate returns the work date: the current user's (My Settings), else today
func WorkDate() types.Date {
	workDate := codeunits.CurrentWorkDate()
	if !workDate.IsZero() {
		return workDate
	}
	return types.Today()
}

// CurrentDateTime returns the current date and time
func CurrentDateTime() types.DateTime { return types.NewDateTimeFromTime(time.Now()) }

// DMY2Date returns the date of day, month and year; month and year default to the work
// date's. An impossible date (31 February) is an error.
func DMY2Date(day int, monthYear ...int) (types.Date, error) {
	work := WorkDate()
	month, year := int(work.Month()), work.Year()
	if len(monthYear) > 0 {
		month = monthYear[0]
	}
	if len(monthYear) > 1 {
		year = monthYear[1]
	}
	d := types.NewDate(year, time.Month(month), day)
	if month < 1 || month > 12 || day < 1 || d.Day() != day || int(d.Month()) != month {
		return types.Date{}, fmt.Errorf("%d.%d.%d is not a date", day, month, year)
	}
	return d, nil
}

// Date2DMY returns a part of a date: what 1 = day, 2 = month, 3 = year (0 otherwise)
func Date2DMY(d types.Date, what int) int {
	switch what {
	case 1:
		return d.Day()
	case 2:
		return int(d.Month())
	case 3:
		return d.Year()
	}
	return 0
}

// CalcDate returns a date calculated from a date formula and a date (default the work date);
// see dateformula.Apply for the formula syntax ("30D", "1M+CM", "-CY", "WD5"). A blank date
// stays blank; an invalid formula is an error.
func CalcDate(formula string, date ...types.Date) (types.Date, error) {
	d := WorkDate()
	if len(date) > 0 {
		d = date[0]
	}
	return dateformula.Apply(formula, d)
}

// isoWeekday: Monday 1 … Sunday 7
func isoWeekday(t time.Time) int {
	if wd := int(t.Weekday()); wd != 0 {
		return wd
	}
	return 7
}

// Time returns the current time of day
func Time() time.Time {
	now := time.Now()
	return time.Date(0, 1, 1, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), time.Local)
}

// CreateDateTime combines a date and a time of day; a blank date gives a blank date-time
func CreateDateTime(d types.Date, t time.Time) types.DateTime {
	if d.IsZero() {
		return types.DateTime{}
	}
	return types.NewDateTimeFromTime(time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local))
}

// DT2Date returns the date of a date-time (in local time)
func DT2Date(dt types.DateTime) types.Date {
	if dt.IsZero() {
		return types.Date{}
	}
	return types.NewDateFromTime(dt.Local())
}

// DT2Time returns the time of day of a date-time (in local time)
func DT2Time(dt types.DateTime) time.Time {
	l := dt.Local()
	return time.Date(0, 1, 1, l.Hour(), l.Minute(), l.Second(), l.Nanosecond(), time.Local)
}

// Date2DWY returns a part of a date's ISO week: what 1 = weekday (1 Monday … 7 Sunday),
// 2 = week number, 3 = the year the week belongs to (0 otherwise)
func Date2DWY(d types.Date, what int) int {
	year, week := d.ISOWeek()
	switch what {
	case 1:
		return isoWeekday(d.Time)
	case 2:
		return week
	case 3:
		return year
	}
	return 0
}

// DWY2Date returns the date of a weekday (1 Monday … 7 Sunday) in an ISO week and year; week
// and year default to the work date's. A week the year does not have is an error.
func DWY2Date(weekday int, weekYear ...int) (types.Date, error) {
	year, week := WorkDate().ISOWeek()
	if len(weekYear) > 0 {
		week = weekYear[0]
	}
	if len(weekYear) > 1 {
		year = weekYear[1]
	}
	if weekday < 1 || weekday > 7 || week < 1 || week > 53 {
		return types.Date{}, fmt.Errorf("weekday %d of week %d %d is not a date", weekday, week, year)
	}
	// The Monday of week 1 is the Monday of the week with 4 January
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	monday := jan4.AddDate(0, 0, 1-isoWeekday(jan4))
	d := monday.AddDate(0, 0, (week-1)*7+weekday-1)
	if y, w := d.ISOWeek(); y != year || w != week {
		return types.Date{}, fmt.Errorf("%d has no week %d", year, week)
	}
	return types.NewDateFromTime(d), nil
}

// ClosingDate returns the closing date of a date: the period-end date between it and the next
// day that closing entries are posted on ("C2026-12-31"); a blank date stays blank
func ClosingDate(d types.Date) types.Date { return d.Closing() }

// NormalDate returns the normal date of a closing date (the date itself otherwise)
func NormalDate(d types.Date) types.Date { return d.Normal() }
