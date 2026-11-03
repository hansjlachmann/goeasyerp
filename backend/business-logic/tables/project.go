package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectLogic is the business logic of table 167 Project.
//
// Not migrated yet (their tables come with the project ledger): OnDelete's MoveEntries of the
// project's ledger entries and its deletion of the project's resource prices. Status asks no
// confirmation (no dialogs in table triggers): a status entered is taken.
type projectLogic struct{}

func init() {
	gtables.RegisterProjectLogic(projectLogic{})
	// the No. field's "…" (Project Card): pick another number series
	functions.Register("table", gtables.ProjectTableName, "AssistEdit", functions.ForAssistEdit(AssistEditProject), functions.WithAssistEdit())
}

// OnInsert: a project without No. gets the next number of the Project Nos. series of the
// Projects Setup
func (projectLogic) OnInsert(rec *gtables.Project) error {
	if rec.No == "" {
		projectsSetup := gtables.GetProjectsSetup(rec)
		err := projectsSetup.TestField("project_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(projectsSetup.Project_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnModify sets Last Date Modified
func (projectLogic) OnModify(rec *gtables.Project) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete: a project with status Order is not deleted; its budget lines, budget entries and
// comment lines go with it
func (projectLogic) OnDelete(rec *gtables.Project) error {
	if rec.Status == gtables.Project_Status.Order {
		return apperrors.ProjectDeleteStatus(gtables.ProjectTableName, rec.Status)
	}

	projectBudgetLine := gtables.NewProjectBudgetLine(rec)
	projectBudgetLine.SetRange("project_no", rec.No)
	projectBudgetLine.DeleteAll()

	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	projectBudgetEntry.SetCurrentKey("project_no")
	projectBudgetEntry.SetRange("project_no", rec.No)
	projectBudgetEntry.DeleteAll()

	DeleteComments(rec, gtables.CommentLine_Table_name.Project, rec.No)
	return nil
}

// OnRename sets Last Date Modified
func (projectLogic) OnRename(rec *gtables.Project) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Project Nos. series (Manual
// Nos.) and takes the project out of its series; a new project saved with the number its
// series just issued (AssistEdit) keeps that series
func (projectLogic) OnValidate_No(rec *gtables.Project) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		projectsSetup := gtables.GetProjectsSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(projectsSetup.Project_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(projectsSetup.Project_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Description: Search Description follows the description while it was blank or
// the old description
func (projectLogic) OnValidate_Description(rec *gtables.Project) error {
	xRec := rec.XRec()
	if string(rec.Search_description) == system.UpperCase(xRec.Description.String()) || rec.Search_description == "" {
		rec.Search_description = types.NewCode(system.CopyStr(rec.Description.String(), 1, 50))
	}
	return nil
}

// OnValidate_Status: the status only moves on; the project's budget entries take it (the
// resources' and budget lines' Qty. Quoted/on Order follow), and Completed sets the
// Completion Date to the work date
func (projectLogic) OnValidate_Status(rec *gtables.Project) error {
	xRec := rec.XRec()
	if rec.Status == xRec.Status {
		return nil
	}

	if rec.Status < xRec.Status {
		return apperrors.ProjectPreviousStatus(gtables.ProjectTableName)
	}

	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	projectBudgetEntry.SetCurrentKey("project_no")
	projectBudgetEntry.SetRange("project_no", rec.No)
	for found := projectBudgetEntry.FindSetBuffered(); found; found = projectBudgetEntry.Next() { // modified in the loop: read all first
		projectBudgetEntry.Project_status = gtables.ProjectBudgetEntryProject_status(rec.Status)
		err := projectBudgetEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}

	if rec.Status == gtables.Project_Status.Completed {
		rec.Completion_date = system.WorkDate()
	}
	return nil
}

// AssistEditProject (Project AssistEdit, the No. field's "…"): the Project Nos. series or
// one related to it gives the new project its number
func AssistEditProject(rec *gtables.Project) (bool, error) {
	projectsSetup := gtables.GetProjectsSetup(rec)
	err := projectsSetup.TestField("project_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(projectsSetup.Project_nos, xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
	}
	return picked, nil
}
