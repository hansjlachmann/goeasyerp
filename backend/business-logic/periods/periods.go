// Package periods is codeunit 359 PeriodFormManagement: the periods of the period pages (Item
// Availability by Periods, Item Turnover …) — days, weeks, months, quarters, years or the
// accounting periods —, found around a date and stepped through, with their names.
package periods

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PeriodType is the length of the periods (classic 'Day,Week,Month,Quarter,Year,Accounting
// Period')
type PeriodType int

const (
	Day PeriodType = iota
	Week
	Month
	Quarter
	Year
	AccountingPeriod
)

// Period is a row of the classic Date table (2000000007): its start, end and name
type Period struct {
	Start types.Date
	End   types.Date
	Name  string // accounting periods: the period's name; other types: CreatePeriodFormat
}

// FindDate (FindDate): the period around a date — searchString "=" the period containing it,
// "<" the one before, ">" the one after; several characters are tried in order ("=<>", the
// default for "" and "=><"). A blank date is the work date. Accounting periods come from the
// Accounting Period table; false when there is none.
func FindDate(src ftables.Source, searchString string, date types.Date, periodType PeriodType) (Period, bool) {
	if date.IsZero() {
		date = system.WorkDate()
	}
	if searchString == "" || searchString == "=><" {
		searchString = "=<>"
	}
	if periodType == AccountingPeriod {
		accountingPeriod := gtables.NewAccountingPeriod(src)
		accountingPeriod.Starting_date = date
		for _, c := range searchString {
			found := false
			switch c {
			case '=':
				// the period containing the date: the last one starting on or before it
				accountingPeriod.SetFilter("starting_date", ".."+date.String())
				found = accountingPeriod.FindLast()
				accountingPeriod.SetRange("starting_date")
			case '<':
				accountingPeriod.SetFilter("starting_date", ".."+date.AddDays(-1).String())
				found = accountingPeriod.FindLast()
				accountingPeriod.SetRange("starting_date")
			case '>':
				accountingPeriod.SetFilter("starting_date", date.AddDays(1).String()+"..")
				found = accountingPeriod.FindFirst()
				accountingPeriod.SetRange("starting_date")
			}
			if found {
				return copyAccountingPeriod(src, accountingPeriod), true
			}
		}
		return Period{}, false
	}
	for _, c := range searchString {
		switch c {
		case '=':
			return calendarPeriod(date, periodType), true
		case '<':
			return calendarPeriod(periodStart(date, periodType).AddDays(-1), periodType), true
		case '>':
			return calendarPeriod(calendarPeriod(date, periodType).End.AddDays(1), periodType), true
		}
	}
	return Period{}, false
}

// NextDate (NextDate): the period steps after (negative: before) a period; it returns the
// period reached and how many steps were taken (fewer at the end of the accounting periods)
func NextDate(src ftables.Source, nextStep int, period Period, periodType PeriodType) (Period, int) {
	if periodType == AccountingPeriod {
		accountingPeriod := gtables.NewAccountingPeriod(src)
		if !accountingPeriod.Get(period.Start) {
			return period, 0
		}
		steps := 0
		for steps != nextStep {
			direction := 1
			if nextStep < 0 {
				direction = -1
			}
			if !accountingPeriod.Next(direction) {
				break
			}
			steps += direction
		}
		if steps == 0 {
			return period, 0
		}
		return copyAccountingPeriod(src, accountingPeriod), steps
	}
	steps := 0
	for steps != nextStep {
		if nextStep > 0 {
			period = calendarPeriod(period.End.AddDays(1), periodType)
			steps++
		} else {
			period = calendarPeriod(period.Start.AddDays(-1), periodType)
			steps--
		}
	}
	return period, steps
}

// CreatePeriodFormat (CreatePeriodFormat): the name of a period — weeks "41.2026" (the week a
// date is in, a week starting in the year before by its Sunday), quarters "4/2026", years
// "2026". Days and months are named by the page in the user's language (the start date, the
// month's name), so their name is empty; accounting periods have their own.
func CreatePeriodFormat(periodType PeriodType, date types.Date) string {
	switch periodType {
	case Week:
		if system.Date2DWY(date, 2) == 1 {
			date = date.AddDays(7 - system.Date2DWY(date, 1))
		}
		return fmt.Sprintf("%d.%d", system.Date2DWY(date, 2), system.Date2DWY(date, 3))
	case Quarter:
		return fmt.Sprintf("%d/%d", (int(date.Month())-1)/3+1, date.Year())
	case Year:
		return fmt.Sprintf("%d", date.Year())
	}
	return ""
}

// calendarPeriod is the day, week (Monday to Sunday), month, quarter or year containing a date
func calendarPeriod(date types.Date, periodType PeriodType) Period {
	start := periodStart(date, periodType)
	var end types.Date
	switch periodType {
	case Day:
		end = start
	case Week:
		end = start.AddDays(6)
	case Month:
		end = start.AddMonths(1).AddDays(-1)
	case Quarter:
		end = start.AddMonths(3).AddDays(-1)
	case Year:
		end = start.AddYears(1).AddDays(-1)
	}
	return Period{Start: start, End: end, Name: CreatePeriodFormat(periodType, start)}
}

// periodStart is the first day of the period containing a date
func periodStart(date types.Date, periodType PeriodType) types.Date {
	switch periodType {
	case Week:
		return date.AddDays(1 - system.Date2DWY(date, 1))
	case Month:
		return types.NewDate(date.Year(), date.Month(), 1)
	case Quarter:
		return types.NewDate(date.Year(), date.Month()-(date.Month()-1)%3, 1)
	case Year:
		return types.NewDate(date.Year(), 1, 1)
	}
	return types.NewDate(date.Year(), date.Month(), date.Day())
}

// copyAccountingPeriod (CopyAccountingPeriod): an accounting period as a period — it ends the
// day before the next one starts (the last one never ends)
func copyAccountingPeriod(src ftables.Source, accountingPeriod *gtables.AccountingPeriod) Period {
	period := Period{Start: accountingPeriod.Starting_date, Name: accountingPeriod.Name.String()}
	next := gtables.NewAccountingPeriod(src)
	next.SetFilter("starting_date", accountingPeriod.Starting_date.AddDays(1).String()+"..")
	if next.FindFirst() {
		period.End = next.Starting_date.AddDays(-1)
	} else {
		period.End, _ = system.DMY2Date(31, 12, 9999)
	}
	return period
}
