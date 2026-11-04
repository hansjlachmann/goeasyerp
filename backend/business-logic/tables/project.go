package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectLogic is the business logic of table 167 Project.
//
// Status asks no confirmation (no dialogs in table triggers): a status entered is taken.
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

// OnDelete: a project with status Order is not deleted; its ledger entries lose the project
// (MoveEntries), its budget lines, budget entries, resource prices and comment lines go with it
func (projectLogic) OnDelete(rec *gtables.Project) error {
	if rec.Status == gtables.Project_Status.Order {
		return apperrors.ProjectDeleteStatus(gtables.ProjectTableName, rec.Status)
	}

	err := MoveProjectEntries(rec)
	if err != nil {
		return err
	}

	projectBudgetLine := gtables.NewProjectBudgetLine(rec)
	projectBudgetLine.SetRange("project_no", rec.No)
	projectBudgetLine.DeleteAll()

	projectBudgetEntry := gtables.NewProjectBudgetEntry(rec)
	projectBudgetEntry.SetCurrentKey("project_no")
	projectBudgetEntry.SetRange("project_no", rec.No)
	projectBudgetEntry.DeleteAll()

	resPrice := gtables.NewResourcePrice(rec)
	resPrice.SetRange("project_no", rec.No)
	resPrice.DeleteAll()

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

// MoveProjectEntries (codeunit 361 MoveEntries MoveJobEntries): a project whose entries are not
// all recognized, or with purchase order lines, is not deleted; its entries lose the project
func MoveProjectEntries(project *gtables.Project) error {
	projectLedgEntry := gtables.NewProjectLedgerEntry(project)
	projectLedgEntry.SetCurrentKey("project_no")
	projectLedgEntry.SetRange("project_no", project.No)

	projectLedgEntry.LockTable()
	for found := projectLedgEntry.FindSet(); found; found = projectLedgEntry.Next() {
		if !projectLedgEntry.Amt_posted_to_g_l.Equal(projectLedgEntry.Amt_recognized) {
			return apperrors.ProjectEntriesNotRecognized(gtables.ProjectTableName, project.No.String())
		}
	}

	purchOrderLine := gtables.NewPurchaseLine(project)
	purchOrderLine.SetCurrentKey("document_type")
	purchOrderLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchOrderLine.SetRange("project_no", project.No)
	if purchOrderLine.FindFirst() {
		return apperrors.ProjectOutstandingPurchOrders(gtables.ProjectTableName, project.No.String())
	}

	projectLedgEntry.ModifyAll("project_no", types.Code(""))
	return nil
}
