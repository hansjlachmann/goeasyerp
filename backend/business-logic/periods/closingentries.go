package periods

import (
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ClosingEntries is the balance views' Closing Entries option (classic ClosingEntryFilter
// 'Include,Exclude')
type ClosingEntries int

const (
	IncludeClosingEntries ClosingEntries = iota
	ExcludeClosingEntries
)

// ClosingEntriesDateFilter is the Date Filter of a balance view's period (classic SetDateFilter
// of forms 154, 408–416, 422): Net Change the period, Balance at Date up to its end — the
// period's end as its closing date (classic Period End), so Include takes the closing entries
// of a fiscal year ending with the period; Exclude leaves out the closing date of every fiscal
// year that ends within the filter (the day before an accounting period with New Fiscal Year),
// the filter cut into ranges around them.
func ClosingEntriesDateFilter(src ftables.Source, period Period, balanceAtDate bool, closingEntries ClosingEntries) string {
	start := period.Start
	if balanceAtDate {
		start = types.Date{}
	}
	end := period.End
	if closingEntries == IncludeClosingEntries {
		return dateRange(start, system.ClosingDate(end).String())
	}
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("new_fiscal_year", true)
	if start.IsZero() {
		accountingPeriod.SetRange("starting_date", types.Date{}, end.AddDays(1))
	} else {
		accountingPeriod.SetRange("starting_date", start.AddDays(1), end.AddDays(1))
	}
	var ranges []string
	from := start
	closedEnd := false
	for found := accountingPeriod.FindSet(); found; found = accountingPeriod.Next() {
		// the fiscal year before this one ends the day before: its closing date is left out
		yearEnd := accountingPeriod.Starting_date.AddDays(-1)
		ranges = append(ranges, dateRange(from, yearEnd.String()))
		from = accountingPeriod.Starting_date
		closedEnd = yearEnd.Equal(end)
	}
	if !closedEnd {
		ranges = append(ranges, dateRange(from, system.ClosingDate(end).String()))
	}
	return strings.Join(ranges, "|")
}

// dateRange: from..to, without a start (blank date) ..to
func dateRange(from types.Date, to string) string {
	if from.IsZero() {
		return ".." + to
	}
	return from.String() + ".." + to
}
