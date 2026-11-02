package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1498 Date Compress Bank Acc. Ledger: the closed bank account ledger entries of closed
// fiscal years become one entry per bank account, posting group, currency, document type, period
// and sign — and per the retained Document No., Our Contact Code, Department and Project Code. The new entries take G/L
// entry numbers (a G/L register of the run).
const DateCompressBankAccLedgerID = 1498

func init() {
	Register(DateCompressBankAccLedgerID, "date-compress-bank-acc-ledger", func() Report { return &dateCompressBank{} })
}

type dateCompressBank struct {
	dateCompression
	glNumbered
	sourceCodeSetup     *gtables.SourceCodeSetup
	newBankAccLedgEntry *gtables.BankAccountLedgerEntry
	window              *system.Window
}

func (r *dateCompressBank) Definition() *Definition {
	r.tableID = gtables.BankAccountLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "our_contact_code"}, {name: "department_code"}, {name: "project_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Bank_Account_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Bank_Account_Ledger_Entry", Table: gtables.BankAccountLedgerEntryTableName, Key: []string{"bank_account_no", "posting_date"},
			TableView:           []ftables.FieldFilter{{Field: "open", Expression: "false"}},
			RequestFilterFields: []string{"bank_account_no", "bank_acc_posting_group", "currency_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressBank) onPreDataItem(bankAccLedgEntry *gtables.BankAccountLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_bank_acc_ledger")
	if err != nil {
		return err
	}

	r.start(run, r.sourceCodeSetup.Compress_bank_acc_ledger, r.description, false)
	r.newBankAccLedgEntry = gtables.NewBankAccountLedgerEntry(run)
	r.newBankAccLedgEntry.LockTable()
	r.dateComprReg.LockTable()
	bankAccLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	bankAccLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressBank) onAfterGetRecord(bankAccLedgEntry *gtables.BankAccountLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewBankAccountLedgerEntry(run)
	if !stillThere.Get(bankAccLedgEntry.Entry_no) {
		return Skip()
	}

	bankAccLedgEntry2 := gtables.NewBankAccountLedgerEntry(run)
	bankAccLedgEntry2.Copy(bankAccLedgEntry)
	bankAccLedgEntry2.SetCurrentKey("bank_account_no", "posting_date")
	bankAccLedgEntry2.CopyFilters(bankAccLedgEntry)
	bankAccLedgEntry2.SetRange("bank_account_no", bankAccLedgEntry2.Bank_account_no)
	dateFilter, periodStart, err := r.dateFilter(bankAccLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	bankAccLedgEntry2.SetFilter("posting_date", dateFilter)
	bankAccLedgEntry2.SetRange("bank_acc_posting_group", bankAccLedgEntry2.Bank_acc_posting_group)
	bankAccLedgEntry2.SetRange("currency_code", bankAccLedgEntry2.Currency_code)
	bankAccLedgEntry2.SetRange("document_type", bankAccLedgEntry2.Document_type)

	r.lastEntryNo++

	newBankAccLedgEntry := r.newBankAccLedgEntry
	newBankAccLedgEntry.Init()
	newBankAccLedgEntry.Entry_no = r.lastEntryNo
	newBankAccLedgEntry.Bank_account_no = bankAccLedgEntry2.Bank_account_no
	newBankAccLedgEntry.Posting_date = periodStart
	newBankAccLedgEntry.Description = r.description
	newBankAccLedgEntry.Bank_acc_posting_group = bankAccLedgEntry2.Bank_acc_posting_group
	newBankAccLedgEntry.Currency_code = bankAccLedgEntry2.Currency_code
	newBankAccLedgEntry.Document_type = bankAccLedgEntry2.Document_type
	newBankAccLedgEntry.Source_code = r.sourceCodeSetup.Compress_bank_acc_ledger
	newBankAccLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	newBankAccLedgEntry.Transaction_no = r.nextTransactionNo
	r.window.Update(1, newBankAccLedgEntry.Bank_account_no)
	r.window.Update(2, newBankAccLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		bankAccLedgEntry2.SetRange("document_no", bankAccLedgEntry2.Document_no)
		newBankAccLedgEntry.Document_no = bankAccLedgEntry2.Document_no
	}
	if r.retainNo("our_contact_code") {
		bankAccLedgEntry2.SetRange("our_contact_code", bankAccLedgEntry2.Our_contact_code)
		newBankAccLedgEntry.Our_contact_code = bankAccLedgEntry2.Our_contact_code
	}
	if r.retainNo("department_code") {
		bankAccLedgEntry2.SetRange("department_code", bankAccLedgEntry2.Department_code)
		newBankAccLedgEntry.Department_code = bankAccLedgEntry2.Department_code
	}
	if r.retainNo("project_code") {
		bankAccLedgEntry2.SetRange("project_code", bankAccLedgEntry2.Project_code)
		newBankAccLedgEntry.Project_code = bankAccLedgEntry2.Project_code
	}
	if !bankAccLedgEntry2.Amount.IsNegative() {
		bankAccLedgEntry2.SetFilter("amount", ">=0")
	} else {
		bankAccLedgEntry2.SetFilter("amount", "<0")
	}
	for found := bankAccLedgEntry2.FindFirst(); found; found = bankAccLedgEntry2.FindFirst() {
		newBankAccLedgEntry.Amount = newBankAccLedgEntry.Amount.Add(bankAccLedgEntry2.Amount)
		newBankAccLedgEntry.Remaining_amount = newBankAccLedgEntry.Remaining_amount.Add(bankAccLedgEntry2.Remaining_amount)
		newBankAccLedgEntry.Amount_lcy = newBankAccLedgEntry.Amount_lcy.Add(bankAccLedgEntry2.Amount_lcy)
		newBankAccLedgEntry.Debit_amount = newBankAccLedgEntry.Debit_amount.Add(bankAccLedgEntry2.Debit_amount)
		newBankAccLedgEntry.Credit_amount = newBankAccLedgEntry.Credit_amount.Add(bankAccLedgEntry2.Credit_amount)
		newBankAccLedgEntry.Debit_amount_lcy = newBankAccLedgEntry.Debit_amount_lcy.Add(bankAccLedgEntry2.Debit_amount_lcy)
		newBankAccLedgEntry.Credit_amount_lcy = newBankAccLedgEntry.Credit_amount_lcy.Add(bankAccLedgEntry2.Credit_amount_lcy)
		err = bankAccLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	err = newBankAccLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressBank) onPostDataItem(bankAccLedgEntry *gtables.BankAccountLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the G/L register and the Date Compr. Register row
func (r *dateCompressBank) initRegisters(run *Run) {
	r.initGLReg()
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(r.glReg.No, r.sourceCodeSetup.Compress_bank_acc_ledger, func(field string) string { return bankAccLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): a G/L entry of 0 and the registers
func (r *dateCompressBank) insertRegisters(run *Run) error {
	err := r.insertGLEntry(run)
	if err != nil {
		return err
	}
	err = r.saveGLReg()
	if err != nil {
		return err
	}
	err = r.saveRegister(r.glRegExists)
	if err != nil {
		return err
	}
	r.glRegExists = true

	glEntry := gtables.NewGLEntry(run)
	newBankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
	glEntry.FindLast()
	newBankAccLedgEntry.FindLast()
	if r.lastEntryNo != glEntry.Entry_no || r.lastEntryNo != newBankAccLedgEntry.Entry_no+1 {
		r.lastEntryNo = glEntry.Entry_no
		r.nextTransactionNo = glEntry.Transaction_no + 1
		r.initRegisters(run)
	}
	return nil
}
