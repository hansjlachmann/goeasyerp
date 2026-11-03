package reports

import (
	"strconv"

	"github.com/hansjlachmann/openerp/backend/business-logic/accsched"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 25 Account Schedule: each schedule's lines against the first six columns of a column
// layout (not Never shown) for the period of the Date Filter (blank: the work date's fiscal year
// up to the work date), dimensions and budget — as the overview calculates them (codeunit 8). A
// line prints by its Show (No: never, If Any Column Not Zero: with a value, Bold: bold); New Page
// starts a new page after the line; Show Division by Zero prints * ERROR * in such a cell. Report
// 900 Acc. Sched. Insert Accounts puts G/L accounts as lines into the schedule it is run from
// (classic Insert Accounts of page 104).
const (
	AccountScheduleID   = 25
	AccSchedInsertAccID = 900
	maxColumnsDisplayed = 6
)

func init() {
	Register(AccountScheduleID, "account-schedule", func() Report { return &accountSchedule{} })
	Register(AccSchedInsertAccID, "acc-sched-insert-accounts", func() Report { return &accSchedInsertAccounts{} })
}

type accountSchedule struct {
	accSchedManagement *accsched.AccSchedManagement
	columnLayoutName   types.Code
	columns            []*gtables.AccSchedColumn
	newPageNext        bool
}

func (r *accountSchedule) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "column_layout_name", Type: "text", Relation: &OptionRelation{Table: gtables.AccSchedColumnLayoutNameTableName, Columns: []string{"name", "description"}}},
			{Name: "show_division_by_zero", Type: "bool", Default: "false"},
			{Name: "use_add_rpt_curr", Type: "bool", Default: "false"},
		},
		ContextTable: gtables.AccScheduleNameTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "AccScheduleName",
			Table:               gtables.AccScheduleNameTableName,
			Key:                 []string{"name"},
			RequestFilterFields: []string{"name"},
			OnAfterGetRecord:    Trigger(r.nameOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:                "AccScheduleLine",
				Table:               gtables.AccScheduleLineTableName,
				Key:                 []string{"schedule_name", "line_no"},
				Link:                map[string]string{"schedule_name": "name"},
				RequestFilterFields: []string{"date_filter", "budget_filter", "dimension_1_filter", "dimension_2_filter"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
			}},
		}},
	}
}

// Run from a schedule: that schedule with its default column layout
func (r *accountSchedule) onInitReport(run *Run) error {
	accSchedName, ok := run.Context().(*gtables.AccScheduleName)
	if ok && accSchedName.Name != "" {
		run.SetRequestFilter("AccScheduleName", "name", accSchedName.Name.String())
		if accSchedName.Default_column_layout != "" {
			run.SetOption("column_layout_name", accSchedName.Default_column_layout.String())
		}
	}
	return nil
}

// onPreReport: with Use Additional Reporting Currency the head names the currency
func (r *accountSchedule) onPreReport(run *Run) error {
	setReportHeadValues(run)
	if run.OptionBool("use_add_rpt_curr") {
		glSetup := gtables.GetGeneralLedgerSetup(run)
		currency := gtables.NewCurrency(run)
		err := currency.GetOrError(glSetup.Additional_reporting_currency)
		if err != nil {
			return err
		}
		run.SetValue("add_curr_text", run.Text("AMOUNTS_IN_CURRENCY", currency.Description))
	}
	return nil
}

func (r *accountSchedule) nameOnAfterGetRecord(rec *gtables.AccScheduleName, run *Run) error {
	run.SetPageNo(1)
	r.columnLayoutName = types.Code(system.UpperCase(run.Option("column_layout_name")))
	if r.columnLayoutName == "" {
		r.columnLayoutName = rec.Default_column_layout
	}
	colLayoutNameRec := gtables.NewAccSchedColumnLayoutName(run)
	if !colLayoutNameRec.Get(r.columnLayoutName) {
		return apperrors.AccSchedColumnLayoutNotExists(r.columnLayoutName.String())
	}
	run.SetValue("schedule_description", rec.Description.String())
	run.SetValue("column_layout_description", colLayoutNameRec.Description.String())

	// the columns shown (at most six): their headers and the rounding of their values
	r.columns = nil
	colLayoutRec := gtables.NewAccSchedColumn(run)
	colLayoutRec.SetRange("column_layout_name", r.columnLayoutName)
	for found := colLayoutRec.FindSetBuffered(); found && len(r.columns) < maxColumnsDisplayed; found = colLayoutRec.Next() {
		if colLayoutRec.Show == gtables.AccSchedColumn_Show.Never {
			continue
		}
		column := gtables.NewAccSchedColumn(run)
		column.Copy(colLayoutRec)
		r.columns = append(r.columns, column)
	}
	for i := 1; i <= maxColumnsDisplayed; i++ {
		header, rounding := "", ""
		if i <= len(r.columns) {
			column := r.columns[i-1]
			header = column.Column_header.String()
			switch column.Rounding_factor {
			case gtables.AccSchedColumn_Rounding_factor.N1000:
				rounding = run.Text("THOUSANDS")
			case gtables.AccSchedColumn_Rounding_factor.N1000000:
				rounding = run.Text("MILLIONS")
			}
		}
		run.SetValue("header"+strconv.Itoa(i), header)
		run.SetValue("rounding"+strconv.Itoa(i), rounding)
	}
	r.accSchedManagement = accsched.New(run)
	r.accSchedManagement.SetParameters(run.OptionBool("use_add_rpt_curr"))
	r.newPageNext = false
	return nil
}

// lineOnPreDataItem: the period (blank: the work date's fiscal year up to the work date) and
// the filters in the head
func (r *accountSchedule) lineOnPreDataItem(rec *gtables.AccScheduleLine, run *Run) error {
	if rec.Date_filter == "" {
		workDate := system.WorkDate()
		fiscalStartDate := accsched.FindFiscalYear(run, workDate)
		err := rec.SetFlowFilter("date_filter", fiscalStartDate.String()+".."+workDate.String())
		if err != nil {
			return err
		}
	}
	startDate, endDate := dateFilterRange(rec.Date_filter)
	language := fcodeunits.CurrentLanguage()
	run.SetValue("period_text", run.Text("PERIOD_FROM_TO", formatDate(startDate, language), formatDate(endDate, language)))
	run.SetValue("dimension_1_text", filterText(run, "INCLUDES_DIMENSION_1", rec.Dimension_1_filter))
	run.SetValue("dimension_2_text", filterText(run, "INCLUDES_DIMENSION_2", rec.Dimension_2_filter))
	run.SetValue("budget_text", filterText(run, "INCLUDES_BUDGETS", rec.Budget_filter))
	return nil
}

func filterText(run *Run, key, filter string) string {
	if filter == "" {
		return ""
	}
	return run.Text(key, filter)
}

// lineOnAfterGetRecord: the line's cells, and whether and how it prints
func (r *accountSchedule) lineOnAfterGetRecord(rec *gtables.AccScheduleLine, run *Run) error {
	if r.newPageNext {
		run.NewPage()
	}
	r.newPageNext = false
	show := gtables.AccScheduleLine_Show
	if rec.Show == show.No {
		return Skip()
	}
	nonZero := false
	language := fcodeunits.CurrentLanguage()
	filters := accsched.LineFilters(rec)
	for i := 1; i <= maxColumnsDisplayed; i++ {
		text := ""
		if i <= len(r.columns) {
			column := r.columns[i-1]
			switch rec.Totaling_type {
			case gtables.AccScheduleLine_Totaling_type.Underline:
				text = "────────────"
			case gtables.AccScheduleLine_Totaling_type.DoubleUnderline:
				text = "════════════"
			default:
				if rec.Totaling != "" {
					value, err := r.accSchedManagement.CalcCell(filters, rec, column)
					if err != nil {
						return err
					}
					if r.accSchedManagement.GetDivisionError() {
						if run.OptionBool("show_division_by_zero") {
							text = "* ERROR *"
						}
					} else {
						nonZero = nonZero || !value.IsZero()
						value = accsched.RoundCell(column, value)
						if !value.IsZero() {
							text = formatDecimal(value, accsched.DecimalPlaces(column), language)
							if accsched.IsPercentColumn(column) {
								text += " %"
							}
						}
					}
				}
			}
		}
		run.SetValue("col"+strconv.Itoa(i), text)
	}
	if rec.Show == show.IfAnyColumnNotZero && !nonZero {
		return Skip()
	}
	run.SetValue("is_bold", rec.Show == show.Bold)
	r.newPageNext = rec.New_page
	return nil
}

// Report 900 Acc. Sched. Insert Accounts: the G/L accounts in the request filters as lines of
// the schedule it is run from, before the current line (the lines from there on move down): the
// account's name, and for posting and total accounts its number as Row No. and Totaling
type accSchedInsertAccounts struct {
	scheduleName    types.Code
	accSchedLineNo  int
	insertedLineNos int
}

func (r *accSchedInsertAccounts) Definition() *Definition {
	return &Definition{
		Transaction:  true,
		ContextTable: gtables.AccScheduleLineTableName,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "account_type", "income_balance"},
			OnPreDataItem:       Trigger(r.glAccOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.glAccOnAfterGetRecord),
			OnPostDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				run.Message(run.Text("INSERTED", r.insertedLineNos))
				return nil
			}),
		}},
	}
}

// onPreReport: the schedule and the line the accounts go before (run from page 104)
func (r *accSchedInsertAccounts) onPreReport(run *Run) error {
	accSchedLine, ok := run.Context().(*gtables.AccScheduleLine)
	if ok {
		r.scheduleName = accSchedLine.Schedule_name
		if r.scheduleName == "" {
			r.scheduleName = types.Code(accSchedLine.GetFilter("schedule_name"))
		}
		r.accSchedLineNo = accSchedLine.Line_no
	}
	if r.scheduleName == "" {
		return apperrors.FieldRequired(gtables.AccScheduleLineTableName, "schedule_name")
	}
	return nil
}

// glAccOnPreDataItem: the lines from the current one on move down to make room
func (r *accSchedInsertAccounts) glAccOnPreDataItem(rec *gtables.GLAccount, run *Run) error {
	glAccCount := rec.Count()
	if glAccCount == 0 {
		return Break()
	}
	accSchedLine := gtables.NewAccScheduleLine(run)
	accSchedLine.SetRange("schedule_name", r.scheduleName)
	if r.accSchedLineNo != 0 {
		accSchedLine.SetFilter("line_no", ">="+strconv.Itoa(r.accSchedLineNo))
		accSchedLine.SetAscending(false)
		var lines []*gtables.AccScheduleLine
		for found := accSchedLine.FindSetBuffered(); found; found = accSchedLine.Next() {
			line := gtables.NewAccScheduleLine(run)
			line.Copy(accSchedLine)
			lines = append(lines, line)
		}
		for _, line := range lines {
			if !line.Rename(line.Schedule_name, line.Line_no+10000*glAccCount) {
				return line.TriggerError()
			}
		}
	} else {
		r.accSchedLineNo = 10000
		if accSchedLine.FindLast() {
			r.accSchedLineNo = accSchedLine.Line_no + 10000
		}
	}
	return nil
}

func (r *accSchedInsertAccounts) glAccOnAfterGetRecord(rec *gtables.GLAccount, run *Run) error {
	accountType := gtables.GLAccount_Account_type
	accSchedLine := gtables.NewAccScheduleLine(run)
	accSchedLine.Init()
	accSchedLine.Schedule_name = r.scheduleName
	accSchedLine.Line_no = r.accSchedLineNo
	r.accSchedLineNo += 10000
	accSchedLine.Description = rec.Name
	if rec.Account_type == accountType.Posting || rec.Account_type == accountType.Total || rec.Account_type == accountType.EndTotal {
		accSchedLine.Totaling = types.Text(rec.No)
		accSchedLine.Row_no = types.Code(system.CopyStr(rec.No.String(), 1, 10))
	}
	if rec.Account_type == accountType.Total || rec.Account_type == accountType.EndTotal {
		accSchedLine.Totaling_type = gtables.AccScheduleLine_Totaling_type.TotalAccounts
	} else {
		accSchedLine.Totaling_type = gtables.AccScheduleLine_Totaling_type.PostingAccounts
	}
	accSchedLine.Show = gtables.AccScheduleLine_Show.Yes
	err := accSchedLine.InsertOrError(true)
	if err != nil {
		return err
	}
	r.insertedLineNos++
	return nil
}
