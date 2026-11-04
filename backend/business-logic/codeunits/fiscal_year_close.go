package codeunits

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FiscalYearClose - Codeunit 6 "Fiscal Year-Close": closes the oldest open fiscal year — its
// periods are closed and date locked, after asking. The next fiscal year must exist. Run from
// the Accounting Periods page (Close Year).
const FiscalYearCloseID = 6
const FiscalYearCloseName = "fiscal-year-close"

func init() {
	Register(FiscalYearCloseID, FiscalYearCloseName, NewFiscalYearClose)
}

type FiscalYearClose struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewFiscalYearClose creates a new instance of the codeunit
func NewFiscalYearClose(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &FiscalYearClose{db: db, company: company, dbType: dbType}
}

func (c *FiscalYearClose) ID() int                    { return FiscalYearCloseID }
func (c *FiscalYearClose) Name() string               { return FiscalYearCloseName }
func (c *FiscalYearClose) SourceTable() string        { return gtables.AccountingPeriodTableName }
func (c *FiscalYearClose) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *FiscalYearClose) GetDB() database.Executor   { return c.db }
func (c *FiscalYearClose) GetCompany() string         { return c.company }
func (c *FiscalYearClose) GetDBType() database.DBType { return c.dbType }

// Run closes the fiscal year
func (c *FiscalYearClose) Run(record interface{}) (fcodeunits.Result, error) {
	result, err := c.code()
	return result, err
}

func (c *FiscalYearClose) code() (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	accountingPeriod2 := gtables.NewAccountingPeriod(c)
	accountingPeriod2.SetRange("closed", false)
	if !accountingPeriod2.FindFirst() {
		return fcodeunits.Result{}, system.Error(ts.Message("FISCAL_YEAR_NO_OPEN_PERIOD", lang))
	}
	fiscalYearStartDate := accountingPeriod2.Starting_date
	err := accountingPeriod2.TestField("new_fiscal_year", true)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	accountingPeriod2.SetRange("new_fiscal_year", true)
	if !accountingPeriod2.Find(">") {
		return fcodeunits.Result{}, system.Error(ts.Message("FISCAL_YEAR_CREATE_NEW", lang))
	}
	nextYearStartDate := accountingPeriod2.Starting_date
	fiscalYearEndDate := nextYearStartDate.AddDays(-1)
	accountingPeriod2.SetRange("new_fiscal_year")
	accountingPeriod2.Find("<")
	lastPeriodStartDate := accountingPeriod2.Starting_date

	if !system.Confirm(ts.Message("FISCAL_YEAR_CLOSE_CONFIRM", lang), fiscalYearStartDate, fiscalYearEndDate) {
		return fcodeunits.Result{Success: true}, nil
	}

	accountingPeriod := gtables.NewAccountingPeriod(c)
	accountingPeriod.SetRange("starting_date", fiscalYearStartDate, lastPeriodStartDate)
	accountingPeriod.ModifyAll("closed", true)
	accountingPeriod.SetRange("starting_date", fiscalYearStartDate, nextYearStartDate)
	accountingPeriod.ModifyAll("date_locked", true)
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
