package docprint

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Print on Account Schedule 104 (report 25 on the page's schedule, classic SetAccSchedName) and
// on Tax Statement Names 320 (report 12 on the statement of the line)
func init() {
	functions.Register("table", gtables.AccScheduleLineTableName, "PrintSchedule", functions.For(PrintAccSchedule), functions.WithPrint())
	functions.Register("table", gtables.TaxStatementNameTableName, "PrintStatement", functions.For(PrintTaxStatement), functions.WithPrint())
}

// PrintAccSchedule prints report 25 Account Schedule on the schedule of the line (the page's
// schedule when the page is empty), with its request page
func PrintAccSchedule(accSchedLine *gtables.AccScheduleLine) (fcodeunits.Result, error) {
	scheduleName := accSchedLine.Schedule_name
	if scheduleName == "" {
		accSchedLine.FilterGroup(2)
		scheduleName = types.Code(accSchedLine.GetFilter("schedule_name"))
		accSchedLine.FilterGroup(0)
	}
	accSchedName := gtables.NewAccScheduleName(accSchedLine)
	err := accSchedName.GetOrError(scheduleName)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return printWithContext(reports.AccountScheduleID, accSchedName)
}

// PrintTaxStatement prints report 12 Tax Statement on the statement, with its request page
func PrintTaxStatement(taxStmtName *gtables.TaxStatementName) (fcodeunits.Result, error) {
	return printWithContext(reports.TaxStatementID, taxStmtName)
}

// printWithContext runs the report with its request page on the record — its filter
// (SETRECFILTER) and its context (the report's SetAccSchedName / SetTaxStmtName); the user needs
// Execute on the report
func printWithContext(reportID int, rec testReportRecord) (fcodeunits.Result, error) {
	if !fcodeunits.CanExecute(session.ObjectReport, reportID) {
		return fcodeunits.Result{}, apperrors.NoPermissionReport(reportID, reports.Caption(reportID, fcodeunits.CurrentLanguage()))
	}
	rec.SetRecFilter()
	result, err := reports.Execute(reportID, rec.GetDB(), rec.GetCompany(), rec.GetDBType(), reports.RunOptions{RequestPage: true, Record: rec, Context: rec})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if result.Cancelled {
		return fcodeunits.Result{Success: true}, nil
	}
	return reports.WithPrintouts(fcodeunits.Result{Success: true}, []reports.Result{result}), nil
}
