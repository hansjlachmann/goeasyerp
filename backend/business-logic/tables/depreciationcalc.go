package tables

import (
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Date calculations of fixed asset depreciation (codeunits 5616 Depreciation Calculation and
// 5617 FA Date Calculation): a year of 360 days, a month of 30.

// DeprDays is the number of depreciation days from a date to a date, both included (a month
// counts 30 days; the 31st and a month's last day count as the 30th)
func DeprDays(startingDate, endingDate types.Date) int {
	if endingDate.Before(startingDate) {
		return 0
	}
	if startingDate.IsZero() || endingDate.IsZero() {
		return 0
	}
	startingDay := startingDate.Day()
	endingDay := endingDate.Day()
	if startingDate.Day() == 31 {
		startingDay = 30
	}
	if endingDate.AddDays(1).Day() == 1 {
		endingDay = 30
	}
	return 1 + endingDay - startingDay + 30*(int(endingDate.Month())-int(startingDate.Month())) +
		360*(endingDate.Year()-startingDate.Year())
}

// Yesterday is the day before a date in the depreciation calendar (the 31st counts as the
// 30th)
func Yesterday(thisDate types.Date) types.Date {
	if thisDate.IsZero() {
		return thisDate
	}
	if thisDate.Day() == 31 {
		thisDate = thisDate.AddDays(-1)
	}
	return thisDate.AddDays(-1)
}

// CalculateDate is the date a number of depreciation days after a date (FA Date Calculation)
func CalculateDate(startingDate types.Date, numberOfDays int) types.Date {
	if numberOfDays <= 0 {
		return startingDate
	}
	years := startingDate.Year()
	months := int(startingDate.Month())
	days := startingDate.Day()
	if startingDate.AddDays(1).Day() == 1 {
		days = 30
	}
	days = days + numberOfDays
	months = months + days/30
	days = days % 30
	if days == 0 {
		days = 30
		months = months - 1
	}
	years = years + months/12
	months = months % 12
	if months == 0 {
		months = 12
		years = years - 1
	}
	if months == 2 && days > 28 {
		days = 28
		if types.NewDate(years, time.February, 28).AddDays(1).Day() == 29 {
			days = 29
		}
	}
	switch months {
	case 1, 3, 5, 7, 8, 10, 12:
		if days == 30 {
			days = 31
		}
	}
	return types.NewDate(years, time.Month(months), days)
}
