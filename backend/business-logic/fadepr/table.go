package fadepr

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// deprTableBuffer is a period of a depreciation table (table 5646 Depreciation Table Buffer)
type deprTableBuffer struct {
	noOfDaysInPeriod  int
	periodDeprPercent types.Decimal
}

// getTablePercent is codeunit 5618 Table Depr. Calculation's GetTablePercent: the part (0..1)
// of the depreciation table's percentages that falls between two dates — the table's periods
// (month 30 days, quarter 90, year the book's fiscal year, or the accounting periods) counted from
// the First User-Defined Depr. Date, a period's percentage by the days of it inside the dates;
// after the table's last line its period repeats (classic: the buffer record keeps its values)
func getTablePercent(src ftables.Source, deprBook *gtables.DepreciationBook, deprTableCode types.Code, firstUserDefinedDeprDate, startingDate, endingDate types.Date) (types.Decimal, error) {
	if startingDate.IsZero() || endingDate.IsZero() || startingDate.After(endingDate) || firstUserDefinedDeprDate.After(startingDate) {
		return types.Decimal{}, nil
	}
	daysInFiscalYear := deprBook.No_of_days_in_fiscal_year
	if daysInFiscalYear == 0 {
		daysInFiscalYear = 360
	}
	deprTableHeader := gtables.NewDepreciationTableHeader(src)
	err := deprTableHeader.GetOrError(deprTableCode)
	if err != nil {
		return types.Decimal{}, err
	}
	startingLimit := DeprDays(firstUserDefinedDeprDate, startingDate)
	endingLimit := DeprDays(firstUserDefinedDeprDate, endingDate)
	if startingDate.Month() == 2 && startingDate.AddDays(1).Day() == 1 {
		startingLimit = startingLimit - (30 - startingDate.Day())
	}
	buffer, err := createTableBuffer(src, deprTableHeader, firstUserDefinedDeprDate, endingLimit, daysInFiscalYear)
	if err != nil {
		return types.Decimal{}, err
	}
	// CalculatePercent
	lastPointer := 0
	percentage := types.Decimal{}
	for _, b := range buffer {
		firstPointer := lastPointer + 1
		lastPointer = firstPointer + b.noOfDaysInPeriod - 1
		numberOfDays := 0
		if !(startingLimit > lastPointer || endingLimit < firstPointer) {
			switch {
			case startingLimit < firstPointer && endingLimit <= lastPointer:
				numberOfDays = endingLimit - firstPointer + 1
			case startingLimit < firstPointer && endingLimit > lastPointer:
				numberOfDays = b.noOfDaysInPeriod
			case startingLimit >= firstPointer && endingLimit <= lastPointer:
				numberOfDays = endingLimit - startingLimit + 1
			case startingLimit >= firstPointer && endingLimit > lastPointer:
				numberOfDays = lastPointer - startingLimit + 1
			}
			percentage = percentage.Add(b.periodDeprPercent.Mul(decInt(numberOfDays)).Div(decInt(b.noOfDaysInPeriod)))
		}
	}
	return percentage.Div(dec100), nil
}

// createTableBuffer (CreateTableBuffer): the table's periods (an unbroken sequence of period
// nos.), with their days and percentages (Total No. of Units: the period's units of the total),
// then the last period again up to the ending limit
func createTableBuffer(src ftables.Source, header *gtables.DepreciationTableHeader, firstUserDefinedDeprDate types.Date, endingLimit, daysInFiscalYear int) ([]deprTableBuffer, error) {
	deprTableLine := gtables.NewDepreciationTableLine(src)
	deprTableLine.SetRange("depreciation_table_code", header.Code)
	if !deprTableLine.FindFirst() {
		return nil, apperrors.FADeprTableNoLines(header.Code.String())
	}
	accountingPeriod := gtables.NewAccountingPeriod(src)
	if header.Period_length == gtables.DepreciationTableHeader_Period_length.Period {
		accountingPeriod.SetFilter("starting_date", ">="+firstUserDefinedDeprDate.String())
		accountingPeriod.FindFirst()
		if !accountingPeriod.Starting_date.Equal(firstUserDefinedDeprDate) {
			return nil, apperrors.FADeprTableFirstDate(firstUserDefinedDeprDate.String(), accountingPeriod.Starting_date.String())
		}
	}
	daysInPeriod := 0
	switch header.Period_length {
	case gtables.DepreciationTableHeader_Period_length.Month:
		daysInPeriod = 30
	case gtables.DepreciationTableHeader_Period_length.Quarter:
		daysInPeriod = 90
	case gtables.DepreciationTableHeader_Period_length.Year:
		daysInPeriod = daysInFiscalYear
	}
	var buffer []deprTableBuffer
	totalNoOfDays := 0
	periodNo := 0
	for found := deprTableLine.FindSet(); found; found = deprTableLine.Next() {
		periodNo++
		if periodNo != deprTableLine.Period_no {
			return nil, apperrors.FieldErrorText(gtables.DepreciationTableLineTableName, "period_no", "≠ "+decInt(periodNo).String())
		}
		if header.Period_length == gtables.DepreciationTableHeader_Period_length.Period {
			periodStart := accountingPeriod.Starting_date
			if accountingPeriod.Next() {
				daysInPeriod = DeprDays(periodStart, Yesterday(accountingPeriod.Starting_date))
			}
			if daysInPeriod == 0 {
				return nil, apperrors.FADeprTablePeriodMissing()
			}
			if daysInPeriod <= 5 {
				return nil, apperrors.FADeprTablePeriodTooShort()
			}
		}
		totalNoOfDays += daysInPeriod
		b := deprTableBuffer{noOfDaysInPeriod: daysInPeriod, periodDeprPercent: deprTableLine.Period_depreciation_percent}
		if header.Total_no_of_units.IsPositive() {
			b.periodDeprPercent = deprTableLine.No_of_units_in_period.Mul(dec100).Div(header.Total_no_of_units)
		}
		buffer = append(buffer, b)
		if totalNoOfDays > endingLimit {
			break
		}
	}
	last := buffer[len(buffer)-1]
	for totalNoOfDays < endingLimit {
		buffer = append(buffer, last)
		totalNoOfDays += daysInPeriod
	}
	return buffer, nil
}
