package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 898 Date Compress BOM Ledger: the BOM ledger entries of closed fiscal years become one
// entry per period — and per the retained Reference Type and Entry Type; a BOM register of the
// run.
const DateCompressBOMLedgerID = 898

func init() {
	Register(DateCompressBOMLedgerID, "date-compress-bom-ledger", func() Report { return &dateCompressBOMLedger{} })
}

type dateCompressBOMLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup *gtables.SourceCodeSetup
	newBOMLedgEntry *gtables.BOMLedgerEntry
	lastEntryNo     int
	window          *system.Window
}

func (r *dateCompressBOMLedger) Definition() *Definition {
	r.tableID = gtables.BOMLedgerEntryTableID
	r.fields = []retainField{{name: "entry_type"}, {name: "reference_type"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(false, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "BOM_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "BOM_Ledger_Entry", Table: gtables.BOMLedgerEntryTableName, Key: []string{"posting_date"},
			RequestFilterFields: []string{"reference_type", "entry_type"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressBOMLedger) onPreDataItem(bomLedgEntry *gtables.BOMLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_bom_ledger")
	if err != nil {
		return err
	}

	r.newBOMLedgEntry = gtables.NewBOMLedgerEntry(run)
	r.newBOMLedgEntry.LockTable()
	r.reg = gtables.NewBOMRegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	bomLedgEntry2 := gtables.NewBOMLedgerEntry(run)
	r.lastEntryNo = 0
	if bomLedgEntry2.FindLast() {
		r.lastEntryNo = bomLedgEntry2.Entry_no
	}
	bomLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	bomLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressBOMLedger) onAfterGetRecord(bomLedgEntry *gtables.BOMLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewBOMLedgerEntry(run)
	if !stillThere.Get(bomLedgEntry.Entry_no) {
		return Skip()
	}

	bomLedgEntry2 := gtables.NewBOMLedgerEntry(run)
	bomLedgEntry2.Copy(bomLedgEntry)
	bomLedgEntry2.SetCurrentKey("posting_date")
	bomLedgEntry2.CopyFilters(bomLedgEntry)
	dateFilter, periodStart, err := r.dateFilter(bomLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	bomLedgEntry2.SetFilter("posting_date", dateFilter)

	r.lastEntryNo++

	newBOMLedgEntry := r.newBOMLedgEntry
	newBOMLedgEntry.Init()
	newBOMLedgEntry.Entry_no = r.lastEntryNo
	newBOMLedgEntry.Posting_date = periodStart
	r.window.Update(1, newBOMLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(2, r.dateComprReg.No_of_new_records)

	if r.retainNo("reference_type") {
		bomLedgEntry2.SetRange("reference_type", bomLedgEntry2.Reference_type)
		newBOMLedgEntry.Reference_type = bomLedgEntry2.Reference_type
	}
	if r.retainNo("entry_type") {
		bomLedgEntry2.SetRange("entry_type", bomLedgEntry2.Entry_type)
		newBOMLedgEntry.Entry_type = bomLedgEntry2.Entry_type
	}
	for found := bomLedgEntry2.FindFirst(); found; found = bomLedgEntry2.FindFirst() {
		err = bomLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(3, r.dateComprReg.No_records_deleted)
	}
	err = newBOMLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressBOMLedger) onPostDataItem(bomLedgEntry *gtables.BOMLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the BOM register of the new entries and the Date Compr.
// Register row
func (r *dateCompressBOMLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_bom_ledger, r.lastEntryNo)
	bomLedgEntry := gtables.NewBOMLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_bom_ledger, func(field string) string { return bomLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressBOMLedger) insertRegisters(run *Run) error {
	err := r.save(r.newBOMLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true

	bomLedgEntry2 := gtables.NewBOMLedgerEntry(run)
	if bomLedgEntry2.FindLast() && r.lastEntryNo != bomLedgEntry2.Entry_no {
		r.lastEntryNo = bomLedgEntry2.Entry_no
		r.initRegisters(run)
	}
	return nil
}
