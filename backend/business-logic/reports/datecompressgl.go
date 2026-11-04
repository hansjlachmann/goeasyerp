package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 98 Date Compress General Ledger: the G/L entries of closed fiscal years between the
// Starting and Ending Date become one entry per account, general posting type and groups,
// period (day … year) and sign — and per the retained Document Type/No., Job No., Business
// Unit, Dimension 1 and Dimension 2 Code — dated on the period's first day (Prior-Year Entry), in a
// new transaction and G/L register; the Date Compr. Register records the run.
const DateCompressGeneralLedgerID = 98

func init() {
	Register(DateCompressGeneralLedgerID, "date-compress-general-ledger", func() Report { return &dateCompressGL{} })
}

type dateCompressGL struct {
	dateCompression
	sourceCodeSetup   *gtables.SourceCodeSetup
	glReg             *gtables.GLRegister
	newGLEntry        *gtables.GLEntry
	lastEntryNo       int
	nextTransactionNo int
	glRegExists       bool
	window            *system.Window
}

func (r *dateCompressGL) Definition() *Definition {
	r.tableID = gtables.GLEntryTableID
	r.fields = []retainField{{name: "document_type"}, {name: "document_no"}, {name: "project_no"}, {name: "business_unit_code"},
		{name: "dimension_1_code"}, {name: "dimension_2_code"}, {name: "quantity", total: true}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "G_L_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "G_L_Entry", Table: gtables.GLEntryTableName, Key: []string{"gl_account_no", "posting_date"},
			RequestFilterFields: []string{"gl_account_no"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressGL) onPreDataItem(glEntry *gtables.GLEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_g_l")
	if err != nil {
		return err
	}

	r.newGLEntry = gtables.NewGLEntry(run)
	r.newGLEntry.LockTable()
	r.glReg = gtables.NewGLRegister(run)
	r.glReg.LockTable()
	r.dateComprReg.LockTable()

	glEntry2 := gtables.NewGLEntry(run)
	if glEntry2.FindLast() {
		r.lastEntryNo = glEntry2.Entry_no
		r.nextTransactionNo = glEntry2.Transaction_no + 1
	} else {
		r.nextTransactionNo = 1
	}
	glEntry.SetRange("entry_no", 0, r.lastEntryNo)
	glEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressGL) onAfterGetRecord(glEntry *gtables.GLEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewGLEntry(run)
	if !stillThere.Get(glEntry.Entry_no) {
		return Skip()
	}

	glEntry2 := gtables.NewGLEntry(run)
	glEntry2.Copy(glEntry)
	glEntry2.SetCurrentKey("gl_account_no", "posting_date")
	glEntry2.CopyFilters(glEntry)
	dateFilter, _, err := r.dateFilter(glEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	glEntry2.SetFilter("posting_date", dateFilter)
	for found := true; found; found = glEntry2.FindFirst() {
		glEntry2.SetRange("gl_account_no", glEntry2.Gl_account_no)
		glEntry2.SetRange("gen_posting_type", glEntry2.Gen_posting_type)
		glEntry2.SetRange("gen_bus_posting_group", glEntry2.Gen_bus_posting_group)
		glEntry2.SetRange("gen_prod_posting_group", glEntry2.Gen_prod_posting_group)

		r.lastEntryNo++

		newGLEntry := r.newGLEntry
		newGLEntry.Init()
		newGLEntry.Entry_no = r.lastEntryNo
		newGLEntry.Gl_account_no = glEntry2.Gl_account_no
		_, newGLEntry.Posting_date, err = r.dateFilter(glEntry2.Posting_date, true)
		if err != nil {
			return err
		}
		newGLEntry.Description = r.description
		newGLEntry.Gen_posting_type = glEntry2.Gen_posting_type
		newGLEntry.Gen_bus_posting_group = glEntry2.Gen_bus_posting_group
		newGLEntry.Gen_prod_posting_group = glEntry2.Gen_prod_posting_group
		newGLEntry.System_created_entry = true
		newGLEntry.Prior_year_entry = true
		newGLEntry.Source_code = r.sourceCodeSetup.Compress_g_l
		newGLEntry.User_id = types.Code(fcodeunits.CurrentUserID())
		newGLEntry.Transaction_no = r.nextTransactionNo
		r.window.Update(1, newGLEntry.Gl_account_no)
		r.window.Update(2, newGLEntry.Posting_date)
		r.newRecord()
		r.window.Update(3, r.dateComprReg.No_of_new_records)

		if r.retainNo("document_type") {
			glEntry2.SetRange("document_type", glEntry2.Document_type)
			newGLEntry.Document_type = glEntry2.Document_type
		}
		if r.retainNo("document_no") {
			glEntry2.SetRange("document_no", glEntry2.Document_no)
			newGLEntry.Document_no = glEntry2.Document_no
		}
		if r.retainNo("project_no") {
			glEntry2.SetRange("project_no", glEntry2.Project_no)
			newGLEntry.Project_no = glEntry2.Project_no
		}
		if r.retainNo("business_unit_code") {
			glEntry2.SetRange("business_unit_code", glEntry2.Business_unit_code)
			newGLEntry.Business_unit_code = glEntry2.Business_unit_code
		}
		if r.retainNo("dimension_1_code") {
			glEntry2.SetRange("dimension_1_code", glEntry2.Dimension_1_code)
			newGLEntry.Dimension_1_code = glEntry2.Dimension_1_code
		}
		if r.retainNo("dimension_2_code") {
			glEntry2.SetRange("dimension_2_code", glEntry2.Dimension_2_code)
			newGLEntry.Dimension_2_code = glEntry2.Dimension_2_code
		}
		if !glEntry2.Amount.IsZero() {
			if glEntry2.Amount.IsPositive() {
				glEntry2.SetFilter("amount", ">0")
			} else {
				glEntry2.SetFilter("amount", "<0")
			}
		} else {
			glEntry2.SetRange("amount", types.Decimal{})
			if !glEntry2.Additional_currency_amount.IsNegative() {
				glEntry2.SetFilter("additional_currency_amount", ">=0")
			} else {
				glEntry2.SetFilter("additional_currency_amount", "<0")
			}
		}
		for found := glEntry2.FindFirst(); found; found = glEntry2.FindFirst() {
			newGLEntry.Amount = newGLEntry.Amount.Add(glEntry2.Amount)
			newGLEntry.Tax_amount = newGLEntry.Tax_amount.Add(glEntry2.Tax_amount)
			newGLEntry.Debit_amount = newGLEntry.Debit_amount.Add(glEntry2.Debit_amount)
			newGLEntry.Credit_amount = newGLEntry.Credit_amount.Add(glEntry2.Credit_amount)
			newGLEntry.Additional_currency_amount = newGLEntry.Additional_currency_amount.Add(glEntry2.Additional_currency_amount)
			newGLEntry.Add_currency_debit_amount = newGLEntry.Add_currency_debit_amount.Add(glEntry2.Add_currency_debit_amount)
			newGLEntry.Add_currency_credit_amount = newGLEntry.Add_currency_credit_amount.Add(glEntry2.Add_currency_credit_amount)
			if r.retainNo("quantity") {
				newGLEntry.Quantity = newGLEntry.Quantity.Add(glEntry2.Quantity)
			}
			err = glEntry2.DeleteOrError(false)
			if err != nil {
				return err
			}
			r.deleted()
			r.window.Update(4, r.dateComprReg.No_records_deleted)
		}
		err = newGLEntry.InsertOrError(false)
		if err != nil {
			return err
		}

		glEntry2.Reset()
		glEntry2.SetCurrentKey("gl_account_no", "posting_date")
		glEntry2.CopyFilters(glEntry)
		glEntry2.SetFilter("posting_date", dateFilter)
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressGL) onPostDataItem(glEntry *gtables.GLEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the G/L register of the new entries and the Date Compr.
// Register row
func (r *dateCompressGL) initRegisters(run *Run) {
	glReg := r.glReg
	glReg.Reset()
	no := 0
	if glReg.FindLast() {
		no = glReg.No
	}
	glReg.Init()
	glReg.No = no + 1
	glReg.Creation_date = system.Today()
	glReg.Source_code = r.sourceCodeSetup.Compress_g_l
	glReg.User_id = types.Code(fcodeunits.CurrentUserID())
	glReg.From_entry_no = r.lastEntryNo + 1

	glEntry := gtables.NewGLEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(glReg.No, r.sourceCodeSetup.Compress_g_l, func(field string) string { return glEntry.FieldCaption(field, language) })
	r.glRegExists = false
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressGL) insertRegisters(run *Run) error {
	r.glReg.To_entry_no = r.newGLEntry.Entry_no
	var err error
	if r.glRegExists {
		err = r.glReg.ModifyOrError(false)
	} else {
		err = r.glReg.InsertOrError(false)
	}
	if err != nil {
		return err
	}
	err = r.saveRegister(r.glRegExists)
	if err != nil {
		return err
	}
	r.glRegExists = true

	glEntry2 := gtables.NewGLEntry(run)
	if glEntry2.FindLast() && r.lastEntryNo != glEntry2.Entry_no {
		r.lastEntryNo = glEntry2.Entry_no
		r.nextTransactionNo = glEntry2.Transaction_no + 1
		r.initRegisters(run)
	}
	return nil
}
