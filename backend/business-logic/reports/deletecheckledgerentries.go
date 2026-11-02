package reports

import (
	"strconv"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1495 Delete Check Ledger Entries: the check ledger entries (not Printed) with a check
// date between the Starting and Ending Date of closed fiscal years are deleted; the Date Compr.
// Register records the run ("All records deleted").
const DeleteCheckLedgerEntriesID = 1495

func init() {
	Register(DeleteCheckLedgerEntriesID, "delete-check-ledger-entries", func() Report { return &deleteCheckLedgerEntries{} })
}

type deleteCheckLedgerEntries struct {
	dateCompression
	sourceCodeSetup   *gtables.SourceCodeSetup
	regExists         bool
	allRecordsDeleted string
	window            *system.Window
}

func (r *deleteCheckLedgerEntries) Definition() *Definition {
	r.tableID = gtables.CheckLedgerEntryTableID
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date", Required: true},
		},
		Transaction: true,
		OnInitReport: func(run *Run) error {
			run.SetOption("ending_date", system.Today().String())
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Check_Ledger_Entry")
			r.allRecordsDeleted = run.Text("ALL_RECORDS_DELETED")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Check_Ledger_Entry", Table: gtables.CheckLedgerEntryTableName, Key: []string{"bank_account_no", "check_date"},
			TableView:           []ftables.FieldFilter{{Field: "entry_status", Expression: "<>" + strconv.Itoa(int(gtables.CheckLedgerEntry_Entry_status.Printed))}},
			RequestFilterFields: []string{"bank_account_no", "bank_payment_type"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *deleteCheckLedgerEntries) onPreDataItem(checkLedgEntry *gtables.CheckLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_check_ledger")
	if err != nil {
		return err
	}

	checkLedgEntry2 := gtables.NewCheckLedgerEntry(run)
	checkLedgEntry2.LockTable()
	r.dateComprReg.LockTable()

	checkLedgEntry.SetRange("check_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	_, err = r.dateComprMgt.GetDateFilter(r.entrdDateComprJnl.Ending_date, r.entrdDateComprJnl, true)
	if err != nil {
		return err
	}

	r.initRegister()
	return nil
}

func (r *deleteCheckLedgerEntries) onAfterGetRecord(checkLedgEntry *gtables.CheckLedgerEntry, run *Run) error {
	checkLedgEntry2 := gtables.NewCheckLedgerEntry(run)
	checkLedgEntry2.SetCurrentKey("bank_account_no", "check_date")
	checkLedgEntry2.CopyFilters(checkLedgEntry)

	r.window.Update(1, checkLedgEntry.Bank_account_no)

	for found := checkLedgEntry2.FindFirst(); found; found = checkLedgEntry2.FindFirst() {
		err := checkLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}

	if r.due() {
		return r.insertRegisters()
	}
	return nil
}

func (r *deleteCheckLedgerEntries) onPostDataItem(checkLedgEntry *gtables.CheckLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters()
	}
	return nil
}

// initRegister (InitRegister): the Date Compr. Register row (by day, all records deleted)
func (r *deleteCheckLedgerEntries) initRegister() {
	dateComprReg := r.dateComprReg
	dateComprReg.Reset()
	no := 0
	if dateComprReg.FindLast() {
		no = dateComprReg.No
	}
	dateComprReg.Init()
	dateComprReg.No = no + 1
	dateComprReg.Table_id = r.tableID
	dateComprReg.Creation_date = system.Today()
	dateComprReg.Starting_date = r.entrdDateComprJnl.Starting_date
	dateComprReg.Ending_date = r.entrdDateComprJnl.Ending_date
	dateComprReg.Period_length = gtables.DateComprRegister_Period_length.Day
	dateComprReg.Retain_field_contents = types.Text(r.allRecordsDeleted)
	dateComprReg.Filter = types.Text(system.CopyStr(r.filter, 1, dateComprReg.MaxStrLen("filter")))
	dateComprReg.Register_no = 0
	dateComprReg.Source_code = r.sourceCodeSetup.Compress_check_ledger
	dateComprReg.User_id = types.Code(fcodeunits.CurrentUserID())
	r.regExists = false
	r.noOfDeleted = 0
}

// insertRegisters (InsertRegisters): the register row, then a new one for the entries after
func (r *deleteCheckLedgerEntries) insertRegisters() error {
	err := r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true
	r.initRegister()
	return nil
}
