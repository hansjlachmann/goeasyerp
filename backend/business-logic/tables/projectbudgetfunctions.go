package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages of a project budget line's resource availability: 227 Resource Availability (Project)
// and 231 Res. Gr. Availability (Project)
const (
	ResourceAvailabilityProjectPageID = 227
	ResGrAvailabilityProjectPageID    = 231
)

func init() {
	// The budget pages' (212, 216) functions on the current line
	functions.Register("table", gtables.ProjectBudgetLineTableName, "ResourceAvailability", functions.For(func(rec *gtables.ProjectBudgetLine) (fcodeunits.Result, error) {
		return openOnBudgetLine(rec, ResourceAvailabilityProjectPageID), nil
	}))
	functions.Register("table", gtables.ProjectBudgetLineTableName, "ResGrAvailability", functions.For(func(rec *gtables.ProjectBudgetLine) (fcodeunits.Result, error) {
		return openOnBudgetLine(rec, ResGrAvailabilityProjectPageID), nil
	}))
	functions.Register("table", gtables.ProjectBudgetLineTableName, "ChangeDates", functions.For(ProjectBudgetLineChangeDates), functions.WithDialog())
}

// openOnBudgetLine: the page opens on the budget line (its key — blank phase, task or step
// codes too) and closes back to the budget
func openOnBudgetLine(rec *gtables.ProjectBudgetLine, pageID int) fcodeunits.Result {
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: pageID, Key: ftables.KeyString(rec), Return: true}}
}

// ProjectBudgetLineChangeDates (form 366 Change Job Budget Dates): the date formula asked for
// moves the dates of the line's budget entries and its Starting Date (OK: ChangeDate)
func ProjectBudgetLineChangeDates(rec *gtables.ProjectBudgetLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	answer := fcodeunits.RequestInput(ts.Message("PROJECT_CHANGE_BUDGET_DATES", lang), []fcodeunits.InputField{
		{Name: "date_formula", Label: ts.Message("PROJECT_CHANGE_DATE_FORMULA", lang), Type: "text", Required: true},
	})
	if answer == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	formula := answer["date_formula"]
	_, err := system.CalcDate(formula, system.WorkDate())
	if err != nil {
		return fcodeunits.Result{}, apperrors.DateFormulaInvalid(gtables.ProjectBudgetEntryTableName, "date", formula)
	}
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = ChangeProjectBudgetDates(ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType()), rec, formula)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// ChangeProjectBudgetDates (ChangeDate): every budget entry of the line gets its date moved by the
// date formula, and so does the line's Starting Date
func ChangeProjectBudgetDates(src ftables.Source, rec *gtables.ProjectBudgetLine, formula string) error {
	projectBudgetLine := gtables.NewProjectBudgetLine(src)
	err := projectBudgetLine.GetOrError(map[string]interface{}{"project_no": rec.Project_no, "phase_code": rec.Phase_code, "task_code": rec.Task_code,
		"step_code": rec.Step_code, "type": rec.Type, "no": rec.No, "variant_code": rec.Variant_code})
	if err != nil {
		return err
	}
	projectBudgetEntry := gtables.NewProjectBudgetEntry(src)
	projectBudgetEntry.SetCurrentKey("project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code", "date")
	projectBudgetEntry.SetRange("project_no", projectBudgetLine.Project_no)
	projectBudgetEntry.SetRange("phase_code", projectBudgetLine.Phase_code)
	projectBudgetEntry.SetRange("task_code", projectBudgetLine.Task_code)
	projectBudgetEntry.SetRange("step_code", projectBudgetLine.Step_code)
	projectBudgetEntry.SetRange("type", projectBudgetLine.Type)
	projectBudgetEntry.SetRange("no", projectBudgetLine.No)
	projectBudgetEntry.SetRange("variant_code", projectBudgetLine.Variant_code)
	for found := projectBudgetEntry.FindSetBuffered(); found; found = projectBudgetEntry.Next() {
		date, err := system.CalcDate(formula, projectBudgetEntry.Date)
		if err != nil {
			return err
		}
		projectBudgetEntry.Date = date
		err = projectBudgetEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	if !projectBudgetLine.Starting_date.IsZero() {
		projectBudgetLine.Starting_date, err = system.CalcDate(formula, projectBudgetLine.Starting_date)
		if err != nil {
			return err
		}
	}
	return projectBudgetLine.ModifyOrError(true)
}
