package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1297 Date Compr. Project Budget Entries: the project budget entries between the
// Starting and Ending Date become one entry per project, phase, task, step, type, number,
// variant, period and sign of the quantity, adding up quantities, costs and prices; budgets need
// no closed fiscal year and write no register (as report 97; the classic report merges the
// entries of one date only)
const DateComprProjectBudgetEntriesID = 1297

func init() {
	Register(DateComprProjectBudgetEntriesID, "date-compr-project-budget-entries", func() Report { return &dateComprProjectBudgetEntries{} })
}

type dateComprProjectBudgetEntries struct {
	dateCompression
	newProjectBudgetEntry *gtables.ProjectBudgetEntry
	lastEntryNo           int
	window                *system.Window
}

func (r *dateComprProjectBudgetEntries) Definition() *Definition {
	r.tableID = gtables.ProjectBudgetEntryTableID
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Project_Budget_Entry", Table: gtables.ProjectBudgetEntryTableName,
			Key:                 []string{"project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code", "date"},
			RequestFilterFields: []string{"project_no", "phase_code", "task_code", "step_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateComprProjectBudgetEntries) onPreDataItem(projectBudgetEntry *gtables.ProjectBudgetEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.dateComprReg.Init()
	r.dateComprReg.Starting_date = r.entrdDateComprJnl.Starting_date
	r.dateComprReg.Ending_date = r.entrdDateComprJnl.Ending_date
	r.dateComprReg.Period_length = r.entrdDateComprJnl.Period_length

	projectBudgetEntry2 := gtables.NewProjectBudgetEntry(run)
	projectBudgetEntry2.LockTable()
	r.lastEntryNo = 0
	if projectBudgetEntry2.FindLast() {
		r.lastEntryNo = projectBudgetEntry2.Entry_no
	}
	r.newProjectBudgetEntry = gtables.NewProjectBudgetEntry(run)
	r.window = system.OpenWindow(run.Text("WINDOW"))

	projectBudgetEntry.SetRange("entry_no", 0, r.lastEntryNo)
	projectBudgetEntry.SetRange("date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	return nil
}

func (r *dateComprProjectBudgetEntries) onAfterGetRecord(projectBudgetEntry *gtables.ProjectBudgetEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewProjectBudgetEntry(run)
	if !stillThere.Get(projectBudgetEntry.Entry_no) {
		return Skip()
	}

	projectBudgetEntry2 := gtables.NewProjectBudgetEntry(run)
	projectBudgetEntry2.Copy(projectBudgetEntry)
	projectBudgetEntry2.SetCurrentKey("project_no", "phase_code", "task_code", "step_code", "type", "no", "variant_code", "date")
	projectBudgetEntry2.CopyFilters(projectBudgetEntry)
	dateFilter, periodStart, err := r.dateFilter(projectBudgetEntry2.Date, false)
	if err != nil {
		return err
	}
	projectBudgetEntry2.SetFilter("date", dateFilter)
	projectBudgetEntry2.SetRange("project_no", projectBudgetEntry2.Project_no)
	projectBudgetEntry2.SetRange("phase_code", projectBudgetEntry2.Phase_code)
	projectBudgetEntry2.SetRange("task_code", projectBudgetEntry2.Task_code)
	projectBudgetEntry2.SetRange("step_code", projectBudgetEntry2.Step_code)
	projectBudgetEntry2.SetRange("type", projectBudgetEntry2.Type)
	projectBudgetEntry2.SetRange("no", projectBudgetEntry2.No)
	projectBudgetEntry2.SetRange("variant_code", projectBudgetEntry2.Variant_code)

	r.lastEntryNo++

	newProjectBudgetEntry := r.newProjectBudgetEntry
	newProjectBudgetEntry.Init()
	newProjectBudgetEntry.Entry_no = r.lastEntryNo
	newProjectBudgetEntry.Project_no = projectBudgetEntry2.Project_no
	newProjectBudgetEntry.Project_status = projectBudgetEntry2.Project_status
	newProjectBudgetEntry.Date = periodStart
	newProjectBudgetEntry.Phase_code = projectBudgetEntry2.Phase_code
	newProjectBudgetEntry.Task_code = projectBudgetEntry2.Task_code
	newProjectBudgetEntry.Step_code = projectBudgetEntry2.Step_code
	newProjectBudgetEntry.Type = projectBudgetEntry2.Type
	newProjectBudgetEntry.No = projectBudgetEntry2.No
	newProjectBudgetEntry.Variant_code = projectBudgetEntry2.Variant_code
	newProjectBudgetEntry.Resource_group_no = projectBudgetEntry2.Resource_group_no
	r.window.Update(1, newProjectBudgetEntry.Project_no)
	r.window.Update(2, newProjectBudgetEntry.Date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if !projectBudgetEntry2.Quantity.IsNegative() {
		projectBudgetEntry2.SetFilter("quantity", ">=0")
	} else {
		projectBudgetEntry2.SetFilter("quantity", "<0")
	}
	for found := projectBudgetEntry2.FindFirst(); found; found = projectBudgetEntry2.FindFirst() {
		newProjectBudgetEntry.Quantity = newProjectBudgetEntry.Quantity.Add(projectBudgetEntry2.Quantity)
		newProjectBudgetEntry.Total_cost = newProjectBudgetEntry.Total_cost.Add(projectBudgetEntry2.Total_cost)
		newProjectBudgetEntry.Total_price = newProjectBudgetEntry.Total_price.Add(projectBudgetEntry2.Total_price)
		err = projectBudgetEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	newProjectBudgetEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	// InsertOrError(false): the budget entry's OnInsert would set the user again
	return newProjectBudgetEntry.InsertOrError(false)
}

func (r *dateComprProjectBudgetEntries) onPostDataItem(projectBudgetEntry *gtables.ProjectBudgetEntry, run *Run) error {
	r.window.Close()
	return nil
}
