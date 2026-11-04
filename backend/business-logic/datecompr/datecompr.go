// Package datecompr is codeunit 356 DateComprMgt: the period a date belongs to for a date
// compression — within the compression's starting and ending date, its fiscal year (closed:
// the fiscal year after it is date locked) and the period length's day, week, month, quarter,
// accounting period or year.
package datecompr

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// DateComprMgt keeps the fiscal year and accounting period found last (the codeunit's globals)
type DateComprMgt struct {
	accountingPeriod     *gtables.AccountingPeriod
	fiscYearDate         [2]types.Date
	accountingPeriodDate [2]types.Date
}

// New opens the codeunit in src's company
func New(src ftables.Source) *DateComprMgt {
	return &DateComprMgt{accountingPeriod: gtables.NewAccountingPeriod(src)}
}

// GetDateFilter (GetDateFilter): the date filter of the period the date belongs to for the
// compression dateComprReg (its Starting/Ending Date and Period Length) — a blank date: itself;
// checkFiscYearEnd: the date's fiscal year must be closed (the next fiscal year's first period
// date locked)
func (m *DateComprMgt) GetDateFilter(date types.Date, dateComprReg *gtables.DateComprRegister, checkFiscYearEnd bool) (string, error) {
	date1, date2, err := m.GetDateRange(date, dateComprReg, checkFiscYearEnd)
	if err != nil {
		return "", err
	}
	if date1 == date2 {
		return date1.String(), nil
	}
	return system.StrSubstNo("%1..%2", date1.String(), date2.String()), nil
}

// GetDateRange is GetDateFilter's period as its first and last date (the first is the new
// entry's date: GETRANGEMIN of the filter)
func (m *DateComprMgt) GetDateRange(date types.Date, dateComprReg *gtables.DateComprRegister, checkFiscYearEnd bool) (types.Date, types.Date, error) {
	if date.IsZero() {
		return date, date, nil
	}

	if date.Before(m.fiscYearDate[0]) || date.After(m.fiscYearDate[1]) || m.fiscYearDate[1].IsZero() {
		accountingPeriod := m.accountingPeriod
		accountingPeriod.Reset()
		accountingPeriod.SetRange("new_fiscal_year", true)
		accountingPeriod.Starting_date = date
		if !accountingPeriod.Find("=<") {
			return types.Date{}, types.Date{}, apperrors.DateComprNoPeriod(date)
		}
		m.fiscYearDate[0] = accountingPeriod.Starting_date
		accountingPeriod.Starting_date = date
		if !accountingPeriod.Find(">") {
			return types.Date{}, types.Date{}, apperrors.DateComprNoPeriod(date)
		}
		m.fiscYearDate[1] = accountingPeriod.Starting_date.AddDays(-1)
		accountingPeriod.SetRange("new_fiscal_year")
		if checkFiscYearEnd {
			err := accountingPeriod.TestField("date_locked", true)
			if err != nil {
				return types.Date{}, types.Date{}, err
			}
		}
	}

	if dateComprReg.Period_length == gtables.DateComprRegister_Period_length.Day {
		return date, date, nil
	}

	date1 := dateComprReg.Starting_date
	date2 := dateComprReg.Ending_date
	maximize(&date1, m.fiscYearDate[0])
	minimize(&date2, m.fiscYearDate[1])

	err := m.limit(&date1, &date2, "CY+1D-1Y", "CY", date)
	if err != nil {
		return types.Date{}, types.Date{}, err
	}
	if dateComprReg.Period_length == gtables.DateComprRegister_Period_length.Year {
		return date1, date2, nil
	}

	if date.Before(m.accountingPeriodDate[0]) || date.After(m.accountingPeriodDate[1]) || m.accountingPeriodDate[1].IsZero() {
		accountingPeriod := m.accountingPeriod
		accountingPeriod.Reset()
		accountingPeriod.Starting_date = date
		if !accountingPeriod.Find("=<") {
			return types.Date{}, types.Date{}, apperrors.DateComprNoPeriod(date)
		}
		m.accountingPeriodDate[0] = accountingPeriod.Starting_date
		if !accountingPeriod.Next() {
			return types.Date{}, types.Date{}, apperrors.DateComprNoPeriod(date)
		}
		m.accountingPeriodDate[1] = accountingPeriod.Starting_date.AddDays(-1)
	}

	maximize(&date1, m.accountingPeriodDate[0])
	minimize(&date2, m.accountingPeriodDate[1])
	if dateComprReg.Period_length == gtables.DateComprRegister_Period_length.Period {
		return date1, date2, nil
	}

	err = m.limit(&date1, &date2, "CQ+1D-1Q", "CQ", date)
	if err != nil {
		return types.Date{}, types.Date{}, err
	}
	if dateComprReg.Period_length == gtables.DateComprRegister_Period_length.Quarter {
		return date1, date2, nil
	}

	err = m.limit(&date1, &date2, "CM+1D-1M", "CM", date)
	if err != nil {
		return types.Date{}, types.Date{}, err
	}
	if dateComprReg.Period_length == gtables.DateComprRegister_Period_length.Month {
		return date1, date2, nil
	}

	err = m.limit(&date1, &date2, "CW+1D-1W", "CW", date)
	if err != nil {
		return types.Date{}, types.Date{}, err
	}
	return date1, date2, nil
}

// limit: Maximize(Date1, CALCDATE(from, Date)), Minimize(Date2, CALCDATE(to, Date))
func (m *DateComprMgt) limit(date1, date2 *types.Date, from, to string, date types.Date) error {
	start, err := system.CalcDate(from, date)
	if err != nil {
		return err
	}
	end, err := system.CalcDate(to, date)
	if err != nil {
		return err
	}
	maximize(date1, start)
	minimize(date2, end)
	return nil
}

// maximize (Maximize): the later of the two dates
func maximize(date *types.Date, newDate types.Date) {
	if date.Before(newDate) {
		*date = newDate
	}
}

// minimize (Minimize): the earlier of the two dates
func minimize(date *types.Date, newDate types.Date) {
	if date.After(newDate) {
		*date = newDate
	}
}
