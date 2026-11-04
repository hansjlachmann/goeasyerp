package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// accountingPeriodLogic is the business logic of table 50 Accounting Period: periods of a closed
// (date locked) fiscal year cannot be changed, and no period may be inserted before them
type accountingPeriodLogic struct{}

func init() { gtables.RegisterAccountingPeriodLogic(accountingPeriodLogic{}) }

// OnInsert: the period after the new one must not be date locked
func (accountingPeriodLogic) OnInsert(rec *gtables.AccountingPeriod) error {
	accountingPeriod2 := gtables.NewAccountingPeriod(rec)
	accountingPeriod2.Starting_date = rec.Starting_date
	if accountingPeriod2.Find(">") {
		err := accountingPeriod2.TestField("date_locked", false)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnDelete: a date locked period cannot be deleted
func (accountingPeriodLogic) OnDelete(rec *gtables.AccountingPeriod) error {
	return rec.TestField("date_locked", false)
}

// OnRename: a date locked period cannot be renamed, and the period after it must not be
// date locked
func (accountingPeriodLogic) OnRename(rec *gtables.AccountingPeriod) error {
	err := rec.TestField("date_locked", false)
	if err != nil {
		return err
	}
	accountingPeriod2 := gtables.NewAccountingPeriod(rec)
	accountingPeriod2.Starting_date = rec.Starting_date
	if accountingPeriod2.Find(">") {
		err = accountingPeriod2.TestField("date_locked", false)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Starting_date: the period's name is its month
func (accountingPeriodLogic) OnValidate_Starting_date(rec *gtables.AccountingPeriod) error {
	rec.Name = types.NewText(rec.Starting_date.Month().String())
	return nil
}

// OnValidate_New_fiscal_year: not in a date locked period
func (accountingPeriodLogic) OnValidate_New_fiscal_year(rec *gtables.AccountingPeriod) error {
	return rec.TestField("date_locked", false)
}
