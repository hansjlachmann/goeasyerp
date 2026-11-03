package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 97 Date Compr. G/L Budget Entries: the budget entries between the Starting and Ending
// Date become one entry per budget, account, period and sign — and per the retained Dimension 1
// and Dimension 2 Code (Business Unit is not migrated); budgets need no closed fiscal year and
// write no register.
const DateComprGLBudgetEntriesID = 97

func init() {
	Register(DateComprGLBudgetEntriesID, "date-compr-g-l-budget-entries", func() Report { return &dateComprGLBudgetEntries{} })
}

type dateComprGLBudgetEntries struct {
	dateCompression
	newGLBudgetEntry *gtables.GLBudgetEntry
	lastEntryNo      int
	window           *system.Window
}

func (r *dateComprGLBudgetEntries) Definition() *Definition {
	r.tableID = gtables.GLBudgetEntryTableID
	r.fields = []retainField{{name: "dimension_1_code"}, {name: "dimension_2_code"}}
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
			Name: "G_L_Budget_Entry", Table: gtables.GLBudgetEntryTableName, Key: []string{"budget_name", "gl_account_no", "date"},
			RequestFilterFields: []string{"budget_name", "gl_account_no"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateComprGLBudgetEntries) onPreDataItem(glBudgetEntry *gtables.GLBudgetEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.dateComprReg.Init()
	r.dateComprReg.Starting_date = r.entrdDateComprJnl.Starting_date
	r.dateComprReg.Ending_date = r.entrdDateComprJnl.Ending_date
	r.dateComprReg.Period_length = r.entrdDateComprJnl.Period_length

	glBudgetEntry2 := gtables.NewGLBudgetEntry(run)
	glBudgetEntry2.LockTable()
	r.lastEntryNo = 0
	if glBudgetEntry2.FindLast() {
		r.lastEntryNo = glBudgetEntry2.Entry_no
	}
	r.newGLBudgetEntry = gtables.NewGLBudgetEntry(run)
	r.window = system.OpenWindow(run.Text("WINDOW"))

	glBudgetEntry.SetRange("entry_no", 0, r.lastEntryNo)
	glBudgetEntry.SetRange("date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	return nil
}

func (r *dateComprGLBudgetEntries) onAfterGetRecord(glBudgetEntry *gtables.GLBudgetEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewGLBudgetEntry(run)
	if !stillThere.Get(glBudgetEntry.Entry_no) {
		return Skip()
	}

	glBudgetEntry2 := gtables.NewGLBudgetEntry(run)
	glBudgetEntry2.Copy(glBudgetEntry)
	glBudgetEntry2.SetCurrentKey("budget_name", "gl_account_no", "date")
	glBudgetEntry2.CopyFilters(glBudgetEntry)
	dateFilter, periodStart, err := r.dateFilter(glBudgetEntry2.Date, false)
	if err != nil {
		return err
	}
	glBudgetEntry2.SetFilter("date", dateFilter)
	glBudgetEntry2.SetRange("budget_name", glBudgetEntry2.Budget_name)
	glBudgetEntry2.SetRange("gl_account_no", glBudgetEntry2.Gl_account_no)

	r.lastEntryNo++

	newGLBudgetEntry := r.newGLBudgetEntry
	newGLBudgetEntry.Init()
	newGLBudgetEntry.Entry_no = r.lastEntryNo
	newGLBudgetEntry.Budget_name = glBudgetEntry2.Budget_name
	newGLBudgetEntry.Gl_account_no = glBudgetEntry2.Gl_account_no
	newGLBudgetEntry.Date = periodStart
	newGLBudgetEntry.Description = r.description
	r.window.Update(1, newGLBudgetEntry.Budget_name)
	r.window.Update(2, newGLBudgetEntry.Gl_account_no)
	r.window.Update(3, newGLBudgetEntry.Date)
	r.newRecord()
	r.window.Update(4, r.dateComprReg.No_of_new_records)

	if r.retainNo("dimension_1_code") {
		glBudgetEntry2.SetRange("dimension_1_code", glBudgetEntry2.Dimension_1_code)
		newGLBudgetEntry.Dimension_1_code = glBudgetEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		glBudgetEntry2.SetRange("dimension_2_code", glBudgetEntry2.Dimension_2_code)
		newGLBudgetEntry.Dimension_2_code = glBudgetEntry2.Dimension_2_code
	}
	if !glBudgetEntry2.Amount.IsNegative() {
		glBudgetEntry2.SetFilter("amount", ">=0")
	} else {
		glBudgetEntry2.SetFilter("amount", "<0")
	}
	for found := glBudgetEntry2.FindFirst(); found; found = glBudgetEntry2.FindFirst() {
		newGLBudgetEntry.Amount = newGLBudgetEntry.Amount.Add(glBudgetEntry2.Amount)
		err = glBudgetEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(5, r.dateComprReg.No_records_deleted)
	}
	newGLBudgetEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	// InsertOrError(false): the budget entry's OnInsert would set the date and user again
	return newGLBudgetEntry.InsertOrError(false)
}

func (r *dateComprGLBudgetEntries) onPostDataItem(glBudgetEntry *gtables.GLBudgetEntry, run *Run) error {
	r.window.Close()
	return nil
}
