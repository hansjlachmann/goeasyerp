package reports

import (
	"fmt"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1296 Copy Project Budget: the budget lines of one project (the request filter) are
// copied to another project (Copy to Project; run from Project Budget 212: that page's project),
// their starting dates moved by the Date Change Formula — with each line's quantity, cost and
// price as one budget entry on its new starting date, or, with Copy Quantity Allocated, every
// budget entry of the lines on its own date moved by the formula. A processing-only report.
const CopyProjectBudgetID = 1296

func init() {
	Register(CopyProjectBudgetID, "copy-project-budget", func() Report { return &copyProjectBudget{} })
}

type copyProjectBudget struct {
	project              *gtables.Project
	changeDateExpression string
	copyQtyAlloc         bool
	nextEntryNo          int
	lastDate             types.Date
	lastDateCalc         types.Date
	copied               int
}

func (r *copyProjectBudget) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "project_no", Type: "text", Relation: &OptionRelation{Table: gtables.ProjectTableName, Columns: []string{"no", "description"}}},
			{Name: "copy_qty_alloc", Type: "bool", Default: "false"},
			{Name: "date_change_formula", Type: "text"},
		},
		Transaction:  true,
		ContextTable: gtables.ProjectBudgetLineTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "Project_Budget_Line",
			Table:               gtables.ProjectBudgetLineTableName,
			Key:                 []string{"project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code"},
			RequestFilterFields: []string{"project_no", "phase_code", "task_code", "step_code"},
			OnPreDataItem:       Trigger(r.lineOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.lineOnPostDataItem),
		}},
	}
}

// Run from Project Budget: the page's project is the one copied to (classic SetCurrentJobNo)
func (r *copyProjectBudget) onInitReport(run *Run) error {
	projectBudgetLine, ok := run.Context().(*gtables.ProjectBudgetLine)
	if ok {
		projectBudgetLine.FilterGroup(2)
		currentProjectNo := projectBudgetLine.GetFilter("project_no")
		projectBudgetLine.FilterGroup(0)
		if currentProjectNo == "" {
			currentProjectNo = projectBudgetLine.Project_no.String()
		}
		run.SetOption("project_no", currentProjectNo)
	}
	return nil
}

func (r *copyProjectBudget) onPreReport(run *Run) error {
	r.project = gtables.NewProject(run)
	err := r.project.GetOrError(types.Code(system.UpperCase(run.Option("project_no"))))
	if err != nil {
		return err
	}
	r.copyQtyAlloc = run.OptionBool("copy_qty_alloc")
	r.changeDateExpression = system.UpperCase(run.Option("date_change_formula"))
	if r.changeDateExpression != "" {
		_, err = system.CalcDate(r.changeDateExpression, system.WorkDate())
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *copyProjectBudget) lineOnPreDataItem(projectBudgetLine *gtables.ProjectBudgetLine, run *Run) error {
	fromProjectNo := fmt.Sprint(projectBudgetLine.GetRangeMin("project_no"))
	if fromProjectNo != fmt.Sprint(projectBudgetLine.GetRangeMax("project_no")) {
		return system.Error(run.Text("ONE_PROJECT"))
	}
	projectBudgetEntry := gtables.NewProjectBudgetEntry(run)
	r.nextEntryNo = 1
	if projectBudgetEntry.FindLast() {
		r.nextEntryNo = projectBudgetEntry.Entry_no + 1
	}
	if r.project.No.String() == fromProjectNo {
		return system.Error(run.Text("SAME_PROJECT", fromProjectNo, r.project.No))
	}
	return nil
}

// changeDate: a date moved by the Date Change Formula (without one the same date)
func (r *copyProjectBudget) changeDate(date types.Date) (types.Date, error) {
	if r.changeDateExpression == "" || date.IsZero() {
		return date, nil
	}
	return system.CalcDate(r.changeDateExpression, date)
}

func (r *copyProjectBudget) lineOnAfterGetRecord(projectBudgetLine *gtables.ProjectBudgetLine, run *Run) error {
	toProjectBudgetLine := gtables.NewProjectBudgetLine(run)
	toProjectBudgetLine.Copy(projectBudgetLine)
	toProjectBudgetLine.Project_no = r.project.No
	startingDate, err := r.changeDate(projectBudgetLine.Starting_date)
	if err != nil {
		return err
	}
	toProjectBudgetLine.Starting_date = startingDate
	projectBudgetLine.CalcFields("quantity", "total_cost", "total_price")
	if !r.copyQtyAlloc && !projectBudgetLine.Quantity.IsZero() {
		toProjectBudgetEntry, err := r.getProjectBudgetEntry(projectBudgetLine, run)
		if err != nil {
			return err
		}
		toProjectBudgetEntry.Entry_no = r.nextEntryNo
		toProjectBudgetEntry.Project_status = gtables.ProjectBudgetEntryProject_status(r.project.Status)
		toProjectBudgetEntry.Project_no = r.project.No
		toProjectBudgetEntry.Quantity = projectBudgetLine.Quantity
		toProjectBudgetEntry.Total_cost = projectBudgetLine.Total_cost
		toProjectBudgetEntry.Total_price = projectBudgetLine.Total_price
		toProjectBudgetEntry.Date = toProjectBudgetLine.Starting_date
		toProjectBudgetEntry.Variant_code = projectBudgetLine.Variant_code
		toProjectBudgetEntry.User_id = types.Code(system.UserId())
		err = toProjectBudgetEntry.InsertOrError(false)
		if err != nil {
			return err
		}
		r.nextEntryNo = r.nextEntryNo + 1
	}
	err = toProjectBudgetLine.InsertOrError(false)
	if err != nil {
		return err
	}
	r.copied = r.copied + 1
	return nil
}

// getProjectBudgetEntry (GetJobBudgetEntry): the line's first budget entry, a copy for the new
// line
func (r *copyProjectBudget) getProjectBudgetEntry(projectBudgetLine *gtables.ProjectBudgetLine, run *Run) (*gtables.ProjectBudgetEntry, error) {
	projectBudgetEntry2 := gtables.NewProjectBudgetEntry(run)
	setProjectBudgetLineEntryFilters(projectBudgetEntry2, projectBudgetLine)
	if !projectBudgetEntry2.FindFirst() {
		return nil, apperrors.TableRecordNotFound(gtables.ProjectBudgetEntryTableName, projectBudgetLine.No.String())
	}
	toProjectBudgetEntry := gtables.NewProjectBudgetEntry(run)
	toProjectBudgetEntry.Copy(projectBudgetEntry2)
	return toProjectBudgetEntry, nil
}

// lineOnPostDataItem: with Copy Quantity Allocated the lines' budget entries are copied, each on
// its date moved by the formula (the second data item, Project Budget Entry)
func (r *copyProjectBudget) lineOnPostDataItem(projectBudgetLine *gtables.ProjectBudgetLine, run *Run) error {
	if r.copyQtyAlloc {
		projectBudgetEntry := gtables.NewProjectBudgetEntry(run)
		projectBudgetEntry.SetCurrentKey("project_no", "resource_group_no", "date")
		for _, field := range []string{"project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code"} {
			projectBudgetLine.CopyFilter(field, projectBudgetEntry, field)
		}
		r.lastDate = types.Date{}
		for found := projectBudgetEntry.FindSetBuffered(); found; found = projectBudgetEntry.Next() {
			toProjectBudgetEntry := gtables.NewProjectBudgetEntry(run)
			toProjectBudgetEntry.Copy(projectBudgetEntry)
			toProjectBudgetEntry.Entry_no = r.nextEntryNo
			toProjectBudgetEntry.Project_status = gtables.ProjectBudgetEntryProject_status(r.project.Status)
			toProjectBudgetEntry.Project_no = r.project.No
			if projectBudgetEntry.Date != r.lastDate {
				r.lastDate = projectBudgetEntry.Date
				lastDateCalc, err := r.changeDate(projectBudgetEntry.Date)
				if err != nil {
					return err
				}
				r.lastDateCalc = lastDateCalc
			}
			toProjectBudgetEntry.Date = r.lastDateCalc
			err := toProjectBudgetEntry.InsertOrError(false)
			if err != nil {
				return err
			}
			r.nextEntryNo = r.nextEntryNo + 1
		}
	}
	run.Message(run.Text("COPIED", r.copied))
	return nil
}

// setProjectBudgetLineEntryFilters: the budget entries of a budget line
func setProjectBudgetLineEntryFilters(projectBudgetEntry *gtables.ProjectBudgetEntry, projectBudgetLine *gtables.ProjectBudgetLine) {
	projectBudgetEntry.SetCurrentKey("project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code", "date")
	projectBudgetEntry.SetRange("project_no", projectBudgetLine.Project_no)
	projectBudgetEntry.SetRange("phase_code", projectBudgetLine.Phase_code)
	projectBudgetEntry.SetRange("task_code", projectBudgetLine.Task_code)
	projectBudgetEntry.SetRange("step_code", projectBudgetLine.Step_code)
	projectBudgetEntry.SetRange("type", projectBudgetLine.Type)
	projectBudgetEntry.SetRange("no", projectBudgetLine.No)
	projectBudgetEntry.SetRange("variant_code", projectBudgetLine.Variant_code)
}
