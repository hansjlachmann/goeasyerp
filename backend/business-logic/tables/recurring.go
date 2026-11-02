package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The recurring journals (general, item, BOM, resource, fixed asset) share these pieces of their
// post batch codeunits: the lines that are due, the line check, the placeholders of the
// document no. and description, and the next date.

// SetRecurringFilters: a recurring journal posts the lines dated up to the work date that have
// not expired (dateField: the line's posting date field)
func SetRecurringFilters(rec interface{ SetFilter(field, expr string) }, dateField string) {
	workDate := system.WorkDate().String()
	rec.SetFilter(dateField, "<="+workDate)
	rec.SetFilter("expiration_date", "''|>="+workDate)
}

// recurringLine is a journal line with the recurring fields
type recurringLine interface {
	TestField(field string, value ...any) error
}

// CheckRecurringLine (CheckRecurringLine): a recurring journal's line has a method and a
// frequency, a variable one its quantity (or amount) field; other journals' lines have neither
func CheckRecurringLine(rec recurringLine, recurring, variable bool, variableField string) error {
	if !recurring {
		err := rec.TestField("recurring_method", 0)
		if err != nil {
			return err
		}
		return rec.TestField("recurring_frequency", types.Code(""))
	}
	err := rec.TestField("recurring_method")
	if err == nil {
		err = rec.TestField("recurring_frequency")
	}
	if err == nil && variable {
		err = rec.TestField(variableField)
	}
	return err
}

// RecurringText (MakeRecurringText): %1 day, %2 week, %3 month, %4 month name and %5 accounting
// period name of the posting date in a recurring line's document no. and description
func RecurringText(src ftables.Source, postingDate types.Date, documentNo types.Code, description types.Text, maxDocumentNo, maxDescription int) (types.Code, types.Text) {
	day := system.Date2DMY(postingDate, 1)
	week := system.Date2DWY(postingDate, 2)
	month := system.Date2DMY(postingDate, 2)
	monthText := postingDate.Month().String()
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("starting_date", types.Date{}, postingDate)
	periodName := ""
	if accountingPeriod.FindLast() {
		periodName = accountingPeriod.Name.String()
	}
	documentNo = types.Code(system.CopyStr(system.StrSubstNo(documentNo.String(), day, week, month, monthText, periodName), 1, maxDocumentNo))
	description = types.Text(system.CopyStr(system.StrSubstNo(description.String(), day, week, month, monthText, periodName), 1, maxDescription))
	return documentNo, description
}

// NextRecurringDate: the date a recurring line moves on to after posting (Recurring Frequency;
// a line without a date keeps none)
func NextRecurringDate(date types.Date, frequency types.Code) (types.Date, error) {
	if date.IsZero() {
		return date, nil
	}
	return system.CalcDate(frequency.String(), date)
}
