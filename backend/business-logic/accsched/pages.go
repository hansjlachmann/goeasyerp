package accsched

import (
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page numbers of the account schedules
const (
	ScheduleNamesPageID     = 103 // Account Schedule Names
	SchedulePageID          = 104 // Account Schedule (the lines)
	ColumnLayoutNamesPageID = 488 // Acc. Sched. Column Layout Names
	ColumnLayoutPageID      = 489 // Account Schedule Column Layout
	OverviewPageID          = 490 // Acc. Schedule Overview
	AccountScheduleReportID = 25  // Account Schedule (the printout)
)

// OpenSchedulePage is page 104's OnOpenPage (classic OpenSchedule): the schedule of the page
// parameter "schedule" — or the first one, DEFAULT created when there is none — filters the
// lines
func OpenSchedulePage(rec *gtables.AccScheduleLine, page *pages.Open) error {
	currentSchedName, err := OpenSchedule(rec, types.Code(page.Param("schedule")))
	if err != nil {
		return err
	}
	page.SetParam("schedule", currentSchedName.String())
	page.SetFilter("schedule_name", currentSchedName.String())
	return nil
}

// OpenColumnsPage is page 489's OnOpenPage (classic OpenColumns): the column layout of the page
// parameter "layout" — or the first one, DEFAULT created when there is none — filters the
// columns
func OpenColumnsPage(rec *gtables.AccSchedColumn, page *pages.Open) error {
	currentColumnName, err := OpenColumns(rec, types.Code(page.Param("layout")))
	if err != nil {
		return err
	}
	page.SetParam("layout", currentColumnName.String())
	page.SetFilter("column_layout_name", currentColumnName.String())
	return nil
}

// UseAddRptCurrFilter: the overview's Use Add. Reporting Currency, a Yes/No FlowFilter (Yes:
// the amounts in the additional reporting currency)
func UseAddRptCurrFilter(expression string) bool {
	switch system.UpperCase(system.DelChr(expression, "<>", " ")) {
	case "YES", "TRUE", "1":
		return true
	}
	return false
}

// Overview calculates the overview of a schedule (page 490): its column layout's columns (those
// not Never shown) and every line's cells for the line record's filters (the period, the
// dimensions, the budget); a column formula with a percent shows a percent, a division by zero
// and a line without totaling nothing, an underline a line
func Overview(accSchedName *gtables.AccScheduleName, accSchedLine *gtables.AccScheduleLine) ([]pages.MatrixColumn, map[string][]string, error) {
	currentColumnName, err := OpenColumns(accSchedLine, accSchedName.Default_column_layout)
	if err != nil {
		return nil, nil, err
	}
	var columns []*gtables.AccSchedColumn
	var headers []pages.MatrixColumn
	accSchedColumn := gtables.NewAccSchedColumn(accSchedLine)
	accSchedColumn.SetRange("column_layout_name", currentColumnName)
	for found := accSchedColumn.FindSetBuffered(); found; found = accSchedColumn.Next() {
		if accSchedColumn.Show == gtables.AccSchedColumn_Show.Never {
			continue
		}
		column := gtables.NewAccSchedColumn(accSchedLine)
		column.Copy(accSchedColumn)
		columns = append(columns, column)
		headers = append(headers, pages.MatrixColumn{Name: column.Column_header.String(),
			DecimalPlaces: DecimalPlaces(column), Percent: IsPercentColumn(column)})
	}
	filters := LineFilters(accSchedLine)
	values := map[string][]string{}
	if len(columns) == 0 {
		return headers, values, nil
	}
	accSchedManagement := New(accSchedLine)
	accSchedManagement.SetParameters(UseAddRptCurrFilter(accSchedLine.Use_add_rpt_curr))
	line := gtables.NewAccScheduleLine(accSchedLine)
	line.Copy(accSchedLine)
	for found := line.FindSetBuffered(); found; found = line.Next() {
		row := make([]string, len(columns))
		for i, column := range columns {
			text, err := CellText(accSchedManagement, filters, line, column)
			if err != nil {
				return nil, nil, err
			}
			row[i] = text
		}
		values[ftables.KeyString(line)] = row
	}
	return headers, values, nil
}

// CellText: a line's cell in a column as shown — the value rounded by the column's Rounding
// Factor, blank for zero, a division by zero and a line without totaling; an underline line a
// line of dashes
func CellText(m *AccSchedManagement, filters Filters, line *gtables.AccScheduleLine, column *gtables.AccSchedColumn) (string, error) {
	switch line.Totaling_type {
	case gtables.AccScheduleLine_Totaling_type.Underline:
		return "────────────", nil
	case gtables.AccScheduleLine_Totaling_type.DoubleUnderline:
		return "════════════", nil
	}
	if line.Totaling == "" {
		return "", nil
	}
	value, err := m.CalcCell(filters, line, column)
	if err != nil {
		return "", err
	}
	if m.GetDivisionError() {
		return "", nil
	}
	value = RoundCell(column, value)
	if value.IsZero() {
		return "", nil
	}
	return value.String(), nil
}

// DecimalPlaces: how a column's values show (by its Rounding Factor)
func DecimalPlaces(column *gtables.AccSchedColumn) string {
	switch column.Rounding_factor {
	case gtables.AccSchedColumn_Rounding_factor.N1:
		return "0:0"
	case gtables.AccSchedColumn_Rounding_factor.N1000, gtables.AccSchedColumn_Rounding_factor.N1000000:
		return "1:1"
	}
	return "2:2"
}
