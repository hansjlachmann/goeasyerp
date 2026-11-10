package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunits 65 Sales-Get Phase/Task/Step, 75 Purch.-Get Phase/Task/Step and 215 Project Jnl.-Get
// Phase/Task/Step (Functions › Get Phase/Task/Step on the sales and purchase documents' lines and
// the project journals): page 216 Project Budget (Phase/Task/Step) opens in lookup mode on the
// budget lines of the line's project, positioned on the line's phase, task and step; the line
// takes the picked budget line's Phase, Task and Step Code.
const ProjectBudgetPhaseTaskStepPageID = 216

func init() {
	functions.Register("table", gtables.SalesLineTableName, "GetPhaseTaskStep", functions.For(SalesLineGetPhaseTaskStep), functions.WithDialog())
	functions.Register("table", gtables.PurchaseLineTableName, "GetPhaseTaskStep", functions.For(PurchLineGetPhaseTaskStep), functions.WithDialog())
	functions.Register("table", gtables.ProjectJournalLineTableName, "GetPhaseTaskStep", functions.For(ProjectJnlLineGetPhaseTaskStep), functions.WithDialog())
}

// getPhaseTaskStep (the codeunits' OnRun): the budget lines of the project in lookup mode; ok when
// the user picked one, with its phase, task and step
func getPhaseTaskStep(src ftables.Source, projectNo, phaseCode, taskCode, stepCode types.Code) (types.Code, types.Code, types.Code, bool) {
	projectBudgetLine := gtables.NewProjectBudgetLine(src)
	projectBudgetLine.Project_no = projectNo
	projectBudgetLine.Phase_code = phaseCode
	projectBudgetLine.Task_code = taskCode
	projectBudgetLine.Step_code = stepCode
	projectBudgetLine.SetRange("project_no", projectNo)
	page := system.NewPage(ProjectBudgetPhaseTaskStepPageID)
	page.SetTableView(projectBudgetLine)
	page.SetRecord(projectBudgetLine)
	page.LookupMode(true)
	if page.RunModal() == system.ActionLookupOK {
		page.GetRecord(projectBudgetLine)
		return projectBudgetLine.Phase_code, projectBudgetLine.Task_code, projectBudgetLine.Step_code, true
	}
	return phaseCode, taskCode, stepCode, false
}

// SalesLineGetPhaseTaskStep is codeunit 65 Sales-Get Phase/Task/Step
func SalesLineGetPhaseTaskStep(rec *gtables.SalesLine) (fcodeunits.Result, error) {
	phaseCode, taskCode, stepCode, ok := getPhaseTaskStep(rec, rec.Project_no, rec.Phase_code, rec.Task_code, rec.Step_code)
	if !ok {
		return fcodeunits.Result{Success: true}, nil
	}
	rec.Phase_code = phaseCode
	rec.Task_code = taskCode
	rec.Step_code = stepCode
	err := rec.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// PurchLineGetPhaseTaskStep is codeunit 75 Purch.-Get Phase/Task/Step
func PurchLineGetPhaseTaskStep(rec *gtables.PurchaseLine) (fcodeunits.Result, error) {
	phaseCode, taskCode, stepCode, ok := getPhaseTaskStep(rec, rec.Project_no, rec.Phase_code, rec.Task_code, rec.Step_code)
	if !ok {
		return fcodeunits.Result{Success: true}, nil
	}
	rec.Phase_code = phaseCode
	rec.Task_code = taskCode
	rec.Step_code = stepCode
	err := rec.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// ProjectJnlLineGetPhaseTaskStep is codeunit 215 Project Jnl.-Get Phase/Task/Step
func ProjectJnlLineGetPhaseTaskStep(rec *gtables.ProjectJournalLine) (fcodeunits.Result, error) {
	phaseCode, taskCode, stepCode, ok := getPhaseTaskStep(rec, rec.Project_no, rec.Phase_code, rec.Task_code, rec.Step_code)
	if !ok {
		return fcodeunits.Result{Success: true}, nil
	}
	rec.Phase_code = phaseCode
	rec.Task_code = taskCode
	rec.Step_code = stepCode
	err := rec.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
