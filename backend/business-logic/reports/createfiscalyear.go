package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 93 Create Fiscal Year: creates the accounting periods of a fiscal year (a number of
// periods of a length) and the first period of the next one — before or after the existing
// fiscal years. A year created before a closed one is closed at once (a processing-only report,
// run from the Accounting Periods page).
const CreateFiscalYearID = 93

func init() {
	Register(CreateFiscalYearID, "create-fiscal-year", func() Report { return &createFiscalYear{} })
}

type createFiscalYear struct{}

func (r *createFiscalYear) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date", Required: true},
			{Name: "no_of_periods", Type: "number", Default: "12", Required: true},
			{Name: "period_length", Type: "text", Default: "1M", Required: true},
		},
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
	}
}

// The new year starts where the last one's last period does
func (r *createFiscalYear) onInitReport(run *Run) error {
	accountingPeriod := gtables.NewAccountingPeriod(run)
	if accountingPeriod.FindLast() {
		run.SetOption("starting_date", accountingPeriod.Starting_date.String())
	}
	return nil
}

func (r *createFiscalYear) onPreReport(run *Run) error {
	fiscalYearStartDate := run.OptionDate("starting_date")
	noOfPeriods := run.OptionInt("no_of_periods")
	periodLength := run.Option("period_length")

	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.Starting_date = fiscalYearStartDate
	err := accountingPeriod.TestField("starting_date")
	if err != nil {
		return err
	}

	var firstPeriodStartDate, lastPeriodStartDate types.Date
	firstPeriodLocked := false
	if accountingPeriod.FindFirst() {
		firstPeriodStartDate = accountingPeriod.Starting_date
		firstPeriodLocked = accountingPeriod.Date_locked
		if fiscalYearStartDate.Before(firstPeriodStartDate) && firstPeriodLocked {
			if system.GuiAllowed() && !system.Confirm(run.Text("CLOSE_NEW_YEAR")+run.Text("CONFIRM_CREATE_AND_CLOSE")) {
				return Quit()
			}
		}
		accountingPeriod.FindLast()
		lastPeriodStartDate = accountingPeriod.Starting_date
	} else {
		if system.GuiAllowed() && !system.Confirm(run.Text("CANNOT_CHANGE_START")+run.Text("CONFIRM_CREATE")) {
			return Quit()
		}
	}

	for i := 1; i <= noOfPeriods+1; i++ {
		if !fiscalYearStartDate.After(firstPeriodStartDate) && i == noOfPeriods+1 {
			return nil
		}
		if !firstPeriodStartDate.IsZero() {
			if !fiscalYearStartDate.Before(firstPeriodStartDate) && fiscalYearStartDate.Before(lastPeriodStartDate) {
				return system.Error(run.Text("ONLY_BEFORE_OR_AFTER"))
			}
		}
		accountingPeriod.Init()
		err = accountingPeriod.Validate("starting_date", fiscalYearStartDate)
		if err != nil {
			return err
		}
		if i == 1 || i == noOfPeriods+1 {
			accountingPeriod.New_fiscal_year = true
		}
		if firstPeriodStartDate.IsZero() && i == 1 {
			accountingPeriod.Date_locked = true
		}
		if accountingPeriod.Starting_date.Before(firstPeriodStartDate) && firstPeriodLocked {
			accountingPeriod.Closed = true
			accountingPeriod.Date_locked = true
		}
		if !accountingPeriod.Find("=") {
			err = accountingPeriod.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		fiscalYearStartDate, err = system.CalcDate(periodLength, fiscalYearStartDate)
		if err != nil {
			return err
		}
	}
	return nil
}
