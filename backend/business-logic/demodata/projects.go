package demodata

import (
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// demoProject: a project for a demo customer and its budget — each line's quantity per month,
// from the project's first month
type demoProject struct {
	description string
	customer    types.Code
	status      gtables.ProjectStatus
	person      types.Code
	startMonth  int // months from this month
	months      int
	lines       []demoBudgetLine
}

type demoBudgetLine struct {
	phase, task, step types.Code
	lineType          gtables.ProjectBudgetLineType
	no                types.Code
	monthly           int64
}

var demoProjects = []demoProject{
	{"Office relocation", "C00010", gtables.Project_Status.Order, "R0010", -2, 4, []demoBudgetLine{
		{"PLAN", "MEETING", "PREPARE", gtables.ProjectBudgetLine_Type.Resource, "R0010", 12},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Resource, "R0040", 40},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Resource, "R0070", 16},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Item, "1310", 5},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Item, "1300", 10},
		{"CLOSE", "MEETING", "REVIEW", gtables.ProjectBudgetLine_Type.Resource, "R0010", 4},
	}},
	{"Shop fit-out", "C00020", gtables.Project_Status.Quote, "R0020", 1, 3, []demoBudgetLine{
		{"PLAN", "MEETING", "PREPARE", gtables.ProjectBudgetLine_Type.Resource, "R0020", 8},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.GroupResource, "TECH", 60},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Item, "1320", 6},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.AccountGL, "5520", 2},
	}},
	{"Office software training", "C00030", gtables.Project_Status.Planning, "R0030", 2, 2, []demoBudgetLine{
		{"PLAN", "TRAINING", "PREPARE", gtables.ProjectBudgetLine_Type.Resource, "R0030", 10},
		{"EXEC", "TRAINING", "DELIVER", gtables.ProjectBudgetLine_Type.GroupResource, "CONSULT", 24},
	}},
	{"Warehouse shelving", "C00050", gtables.Project_Status.Completed, "R0050", -6, 2, []demoBudgetLine{
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Resource, "R0050", 30},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Resource, "R0060", 30},
		{"EXEC", "INSTALL", "DELIVER", gtables.ProjectBudgetLine_Type.Item, "1320", 12},
	}},
}

// insertDemoProjects: the demo projects (numbers from the Project Nos. series) with their
// budget lines, whose monthly quantities are budget entries on each month's first day
func insertDemoProjects(src ftables.Source, today time.Time, counts *Counts) error {
	thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local)
	for _, d := range demoProjects {
		start := thisMonth.AddDate(0, d.startMonth, 0)
		project := gtables.NewProject(src)
		project.Init()
		project.Description = types.Text(d.description)
		err := project.Validate("description", project.Description)
		if err != nil {
			return err
		}
		project.Bill_to_customer_no = d.customer
		project.Person_responsible = d.person
		project.Project_posting_group = "PROJECT"
		project.Creation_date = types.NewDateFromTime(start.AddDate(0, -1, 0))
		project.Starting_date = types.NewDateFromTime(start)
		project.Ending_date = types.NewDateFromTime(start.AddDate(0, d.months, -1))
		project.Status = d.status
		if d.status == gtables.Project_Status.Completed {
			project.Completion_date = project.Ending_date
			project.Completion_percent = types.NewDecimalFromInt(100)
		}
		err = project.InsertOrError(true)
		if err != nil {
			return err
		}
		counts.Projects++

		for _, l := range d.lines {
			projectBudgetLine := gtables.NewProjectBudgetLine(src)
			projectBudgetLine.Init()
			projectBudgetLine.Project_no = project.No
			projectBudgetLine.Phase_code = l.phase
			projectBudgetLine.Task_code = l.task
			projectBudgetLine.Step_code = l.step
			projectBudgetLine.Type = l.lineType
			err = projectBudgetLine.Validate("no", l.no)
			if err != nil {
				return err
			}
			err = projectBudgetLine.InsertOrError(true)
			if err != nil {
				return err
			}
			for month := 0; month < d.months; month++ {
				projectBudgetLine.Date_filter = types.NewDateFromTime(start.AddDate(0, month, 0)).String()
				err = tables.SetProjectBudgetLineQuantity(projectBudgetLine, types.NewDecimalFromInt(l.monthly))
				if err != nil {
					return err
				}
				counts.ProjectBudgetEntries++
			}
		}
	}
	return nil
}
