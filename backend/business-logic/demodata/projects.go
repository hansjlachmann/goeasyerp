package demodata

import (
	"fmt"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
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

// ProjectJnlPoster posts a project journal line (codeunit 202 Project Jnl.-Post Line)
type ProjectJnlPoster interface {
	Run(projectJnlLine *gtables.ProjectJournalLine) error
}

// postDemoProjectUsage: the project on Order uses its resources and items every month since it
// started (project journal lines posted through codeunit 202 — the resource usage, the items
// taken from inventory), and the customer is invoiced the consultant's hours of the first month
// with the project on the invoice lines (codeunit 80: the project's sale, applied to the usage)
func postDemoProjectUsage(src ftables.Source, open func(src ftables.Source) ProjectJnlPoster, openSales func(src ftables.Source) SalesPoster, today time.Time, counts *Counts) error {
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		return nil
	}
	poster := open(src)
	usage := []struct {
		lineType gtables.ProjectJournalLineType
		no       types.Code
		workType types.Code
		qty      int64
	}{
		{gtables.ProjectJournalLine_Type.Resource, "R0010", "NORMAL", 10},
		{gtables.ProjectJournalLine_Type.Resource, "R0040", "NORMAL", 30},
		{gtables.ProjectJournalLine_Type.Item, "1310", "", 2},
	}
	start := project.Starting_date.Time
	for month := start; !month.After(today); month = month.AddDate(0, 1, 0) {
		date := types.NewDateFromTime(month.AddDate(0, 0, 14))
		if date.Time.After(today) {
			date = types.NewDateFromTime(today)
		}
		for _, u := range usage {
			l := gtables.NewProjectJournalLine(src)
			l.Init()
			l.Journal_template_name = "PROJECT"
			l.Journal_batch_name = "DEFAULT"
			l.Posting_date = date
			l.Document_date = date
			l.Document_no = types.NewCode("PRJ-" + month.Format("200601"))
			l.Source_code = "PROJJNL"
			l.Entry_type = gtables.ProjectJournalLine_Entry_type.Usage
			l.Type = u.lineType
			err := set(l, "project_no", project.No, "no", u.no)
			if err == nil && u.workType != "" {
				err = set(l, "work_type_code", u.workType)
			}
			if err == nil && u.lineType == gtables.ProjectJournalLine_Type.Item {
				err = set(l, "location_code", types.Code("MAIN"))
			}
			if err == nil {
				err = set(l, "quantity", types.NewDecimalFromInt(u.qty))
			}
			if err == nil {
				err = poster.Run(l)
			}
			if err != nil {
				return fmt.Errorf("demo project usage %s %s: %w", project.No, u.no, err)
			}
			counts.ProjectLedgerEntries++
		}
	}

	if openSales == nil {
		return nil
	}
	invoiceDate := types.NewDateFromTime(start.AddDate(0, 1, 0))
	if invoiceDate.Time.After(today) {
		invoiceDate = types.NewDateFromTime(today)
	}
	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	err := invoice.InsertOrError(true)
	if err != nil {
		return err
	}
	err = set(invoice, "sell_to_customer_no", project.Bill_to_customer_no, "posting_date", invoiceDate, "document_date", invoiceDate, "project_no", project.No)
	if err == nil {
		err = invoice.ModifyOrError(true)
	}
	if err != nil {
		return fmt.Errorf("demo project invoice: %w", err)
	}
	line := gtables.NewSalesLine(src)
	line.Document_type = gtables.SalesLine_Document_type.Invoice
	line.Document_no = invoice.No
	line.Line_no = 10000
	err = line.InsertOrError(true)
	if err == nil {
		err = set(line, "type", gtables.SalesLine_Type.Resource, "no", types.Code("R0010"), "work_type_code", types.Code("NORMAL"), "quantity", types.NewDecimalFromInt(10))
	}
	if err == nil {
		err = line.ModifyOrError(true)
	}
	if err != nil {
		return fmt.Errorf("demo project invoice line: %w", err)
	}
	invoice.Ship, invoice.Invoice = true, true
	err = openSales(src).Run(invoice)
	if err != nil {
		return fmt.Errorf("demo project invoice posting: %w", err)
	}
	counts.ProjectLedgerEntries++
	return nil
}
