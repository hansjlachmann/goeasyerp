package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1298 Date Compress Project Ledger: the project ledger entries of closed fiscal years
// become one entry per project, type, posting groups, entry type, period and sign of the
// quantity — and per the retained Document No., No., Variant, Resource Group No., Unit of
// Measure, Location, Bin, Chargeable, Dimension 1, Dimension 2, Work Type and Phase/Task/Step
// Code — adding up quantities, costs, prices, remaining and G/L amounts; a project register of
// the run.
const DateCompressProjectLedgerID = 1298

func init() {
	Register(DateCompressProjectLedgerID, "date-compress-project-ledger", func() Report { return &dateCompressProjectLedger{} })
}

type dateCompressProjectLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup     *gtables.SourceCodeSetup
	newProjectLedgEntry *gtables.ProjectLedgerEntry
	lastEntryNo         int
	window              *system.Window
}

func (r *dateCompressProjectLedger) Definition() *Definition {
	r.tableID = gtables.ProjectLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "no"}, {name: "variant_code"}, {name: "resource_group_no"}, {name: "unit_of_measure_code"},
		{name: "location_code"}, {name: "bin_code"}, {name: "chargeable"}, {name: "dimension_1_code"}, {name: "dimension_2_code"},
		{name: "work_type_code"}, {name: "phase_code"}, {name: "task_code"}, {name: "step_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Project_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Project_Ledger_Entry", Table: gtables.ProjectLedgerEntryTableName, Key: []string{"project_no", "posting_date"},
			RequestFilterFields: []string{"project_no", "project_posting_group"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressProjectLedger) onPreDataItem(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_project_ledger")
	if err != nil {
		return err
	}

	r.newProjectLedgEntry = gtables.NewProjectLedgerEntry(run)
	r.newProjectLedgEntry.LockTable()
	r.reg = gtables.NewProjectRegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	projectLedgEntry2 := gtables.NewProjectLedgerEntry(run)
	r.lastEntryNo = 0
	if projectLedgEntry2.FindLast() {
		r.lastEntryNo = projectLedgEntry2.Entry_no
	}
	projectLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	projectLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressProjectLedger) onAfterGetRecord(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewProjectLedgerEntry(run)
	if !stillThere.Get(projectLedgEntry.Entry_no) {
		return Skip()
	}

	projectLedgEntry2 := gtables.NewProjectLedgerEntry(run)
	projectLedgEntry2.Copy(projectLedgEntry)
	projectLedgEntry2.SetCurrentKey("project_no", "posting_date")
	projectLedgEntry2.CopyFilters(projectLedgEntry)
	projectLedgEntry2.SetRange("project_no", projectLedgEntry2.Project_no)
	projectLedgEntry2.SetRange("type", projectLedgEntry2.Type)
	dateFilter, periodStart, err := r.dateFilter(projectLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	projectLedgEntry2.SetFilter("posting_date", dateFilter)
	projectLedgEntry2.SetRange("project_posting_group", projectLedgEntry2.Project_posting_group)
	projectLedgEntry2.SetRange("gen_bus_posting_group", projectLedgEntry2.Gen_bus_posting_group)
	projectLedgEntry2.SetRange("gen_prod_posting_group", projectLedgEntry2.Gen_prod_posting_group)
	projectLedgEntry2.SetRange("entry_type", projectLedgEntry2.Entry_type)

	r.lastEntryNo++

	newProjectLedgEntry := r.newProjectLedgEntry
	newProjectLedgEntry.Init()
	newProjectLedgEntry.Entry_no = r.lastEntryNo
	newProjectLedgEntry.Project_no = projectLedgEntry2.Project_no
	newProjectLedgEntry.Type = projectLedgEntry2.Type
	newProjectLedgEntry.Posting_date = periodStart
	newProjectLedgEntry.Description = r.description
	newProjectLedgEntry.Source_code = r.sourceCodeSetup.Compress_project_ledger
	newProjectLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	newProjectLedgEntry.Project_posting_group = projectLedgEntry2.Project_posting_group
	newProjectLedgEntry.Gen_bus_posting_group = projectLedgEntry2.Gen_bus_posting_group
	newProjectLedgEntry.Gen_prod_posting_group = projectLedgEntry2.Gen_prod_posting_group
	newProjectLedgEntry.Entry_type = projectLedgEntry2.Entry_type
	r.window.Update(1, newProjectLedgEntry.Project_no)
	r.window.Update(2, newProjectLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		projectLedgEntry2.SetRange("document_no", projectLedgEntry2.Document_no)
		newProjectLedgEntry.Document_no = projectLedgEntry2.Document_no
	}
	if r.retainNo("no") {
		projectLedgEntry2.SetRange("no", projectLedgEntry2.No)
		newProjectLedgEntry.No = projectLedgEntry2.No
	}
	if r.retainNo("variant_code") {
		projectLedgEntry2.SetRange("variant_code", projectLedgEntry2.Variant_code)
		newProjectLedgEntry.Variant_code = projectLedgEntry2.Variant_code
	}
	if r.retainNo("resource_group_no") {
		projectLedgEntry2.SetRange("resource_group_no", projectLedgEntry2.Resource_group_no)
		newProjectLedgEntry.Resource_group_no = projectLedgEntry2.Resource_group_no
	}
	if r.retainNo("unit_of_measure_code") {
		projectLedgEntry2.SetRange("unit_of_measure_code", projectLedgEntry2.Unit_of_measure_code)
		newProjectLedgEntry.Unit_of_measure_code = projectLedgEntry2.Unit_of_measure_code
	}
	if r.retainNo("location_code") {
		projectLedgEntry2.SetRange("location_code", projectLedgEntry2.Location_code)
		newProjectLedgEntry.Location_code = projectLedgEntry2.Location_code
	}
	if r.retainNo("bin_code") {
		projectLedgEntry2.SetRange("bin_code", projectLedgEntry2.Bin_code)
		newProjectLedgEntry.Bin_code = projectLedgEntry2.Bin_code
	}
	if r.retainNo("chargeable") {
		projectLedgEntry2.SetRange("chargeable", projectLedgEntry2.Chargeable)
		newProjectLedgEntry.Chargeable = projectLedgEntry2.Chargeable
	}
	if r.retainNo("dimension_1_code") {
		projectLedgEntry2.SetRange("dimension_1_code", projectLedgEntry2.Dimension_1_code)
		newProjectLedgEntry.Dimension_1_code = projectLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		projectLedgEntry2.SetRange("dimension_2_code", projectLedgEntry2.Dimension_2_code)
		newProjectLedgEntry.Dimension_2_code = projectLedgEntry2.Dimension_2_code
	}
	if r.retainNo("work_type_code") {
		projectLedgEntry2.SetRange("work_type_code", projectLedgEntry2.Work_type_code)
		newProjectLedgEntry.Work_type_code = projectLedgEntry2.Work_type_code
	}
	if r.retainNo("phase_code") {
		projectLedgEntry2.SetRange("phase_code", projectLedgEntry2.Phase_code)
		newProjectLedgEntry.Phase_code = projectLedgEntry2.Phase_code
	}
	if r.retainNo("task_code") {
		projectLedgEntry2.SetRange("task_code", projectLedgEntry2.Task_code)
		newProjectLedgEntry.Task_code = projectLedgEntry2.Task_code
	}
	if r.retainNo("step_code") {
		projectLedgEntry2.SetRange("step_code", projectLedgEntry2.Step_code)
		newProjectLedgEntry.Step_code = projectLedgEntry2.Step_code
	}
	if !projectLedgEntry2.Quantity.IsNegative() {
		projectLedgEntry2.SetFilter("quantity", ">=0")
	} else {
		projectLedgEntry2.SetFilter("quantity", "<0")
	}
	for found := projectLedgEntry2.FindFirst(); found; found = projectLedgEntry2.FindFirst() {
		newProjectLedgEntry.Quantity = newProjectLedgEntry.Quantity.Add(projectLedgEntry2.Quantity)
		newProjectLedgEntry.Remaining_amount = newProjectLedgEntry.Remaining_amount.Add(projectLedgEntry2.Remaining_amount)
		newProjectLedgEntry.Total_cost = newProjectLedgEntry.Total_cost.Add(projectLedgEntry2.Total_cost)
		newProjectLedgEntry.Total_price = newProjectLedgEntry.Total_price.Add(projectLedgEntry2.Total_price)
		newProjectLedgEntry.Amt_to_post_to_g_l = newProjectLedgEntry.Amt_to_post_to_g_l.Add(projectLedgEntry2.Amt_to_post_to_g_l)
		newProjectLedgEntry.Amt_posted_to_g_l = newProjectLedgEntry.Amt_posted_to_g_l.Add(projectLedgEntry2.Amt_posted_to_g_l)
		newProjectLedgEntry.Amt_to_recognize = newProjectLedgEntry.Amt_to_recognize.Add(projectLedgEntry2.Amt_to_recognize)
		newProjectLedgEntry.Amt_recognized = newProjectLedgEntry.Amt_recognized.Add(projectLedgEntry2.Amt_recognized)
		err = projectLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	// the new entry is open with a remaining amount, of the sign of its total price
	newProjectLedgEntry.Open = !newProjectLedgEntry.Remaining_amount.IsZero()
	newProjectLedgEntry.Positive = newProjectLedgEntry.Total_price.IsPositive()
	err = newProjectLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressProjectLedger) onPostDataItem(projectLedgEntry *gtables.ProjectLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the project register of the new entries and the Date Compr.
// Register row
func (r *dateCompressProjectLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_project_ledger, r.lastEntryNo)
	projectLedgEntry := gtables.NewProjectLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_project_ledger, func(field string) string { return projectLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressProjectLedger) insertRegisters(run *Run) error {
	err := r.save(r.newProjectLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true

	projectLedgEntry2 := gtables.NewProjectLedgerEntry(run)
	if projectLedgEntry2.FindLast() && r.lastEntryNo != projectLedgEntry2.Entry_no {
		r.lastEntryNo = projectLedgEntry2.Entry_no
		r.initRegisters(run)
	}
	return nil
}
