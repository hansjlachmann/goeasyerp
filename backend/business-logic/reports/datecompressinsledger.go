package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5697 Date Compress Insurance Ledger: the insurance coverage ledger entries of closed
// fiscal years (with Only Index Entries only the index entries) become one entry per insurance, asset, document
// type, index entry and period — and per the retained Document No., Dimension 1 and Dimension 2
// Code; an insurance register of the run.
const DateCompressInsuranceLedgerID = 5697

func init() {
	Register(DateCompressInsuranceLedgerID, "date-compress-insurance-ledger", func() Report { return &dateCompressInsLedger{} })
}

type dateCompressInsLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup         *gtables.SourceCodeSetup
	newInsCoverageLedgEntry *gtables.InsCoverageLedgerEntry
	lastEntryNo             int
	onlyIndexEntries        bool
	window                  *system.Window
}

func (r *dateCompressInsLedger) Definition() *Definition {
	r.tableID = gtables.InsCoverageLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "dimension_1_code"}, {name: "dimension_2_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        append(dateComprOptions(true, r.fields), Option{Name: "only_index_entries", Type: "bool", Default: "false"}),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Ins_Coverage_Ledger_Entry")
			r.onlyIndexEntries = run.OptionBool("only_index_entries")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Ins_Coverage_Ledger_Entry", Table: gtables.InsCoverageLedgerEntryTableName, Key: []string{"fa_no", "insurance_no", "disposed_fa"},
			RequestFilterFields: []string{"insurance_no", "fa_no"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressInsLedger) onPreDataItem(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_insurance_ledger")
	if err != nil {
		return err
	}

	r.newInsCoverageLedgEntry = gtables.NewInsCoverageLedgerEntry(run)
	r.newInsCoverageLedgEntry.LockTable()
	r.reg = gtables.NewInsuranceRegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	insCoverageLedgEntry2 := gtables.NewInsCoverageLedgerEntry(run)
	r.lastEntryNo = 0
	if insCoverageLedgEntry2.FindLast() {
		r.lastEntryNo = insCoverageLedgEntry2.Entry_no
	}
	insCoverageLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	insCoverageLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressInsLedger) onAfterGetRecord(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewInsCoverageLedgerEntry(run)
	if !stillThere.Get(insCoverageLedgEntry.Entry_no) {
		return Skip()
	}
	if insCoverageLedgEntry.Insurance_no != "" && r.onlyIndexEntries && !insCoverageLedgEntry.Index_entry {
		return Skip()
	}

	insCoverageLedgEntry2 := gtables.NewInsCoverageLedgerEntry(run)
	insCoverageLedgEntry2.Copy(insCoverageLedgEntry)
	insCoverageLedgEntry2.SetCurrentKey("fa_no", "insurance_no")
	insCoverageLedgEntry2.CopyFilters(insCoverageLedgEntry)
	insCoverageLedgEntry2.SetRange("insurance_no", insCoverageLedgEntry2.Insurance_no)
	insCoverageLedgEntry2.SetRange("fa_no", insCoverageLedgEntry2.Fa_no)
	insCoverageLedgEntry2.SetRange("document_type", insCoverageLedgEntry2.Document_type)
	insCoverageLedgEntry2.SetRange("index_entry", insCoverageLedgEntry2.Index_entry)
	dateFilter, periodStart, err := r.dateFilter(insCoverageLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	insCoverageLedgEntry2.SetFilter("posting_date", dateFilter)

	r.lastEntryNo++

	newInsCoverageLedgEntry := r.newInsCoverageLedgEntry
	newInsCoverageLedgEntry.Init()
	newInsCoverageLedgEntry.Entry_no = r.lastEntryNo
	newInsCoverageLedgEntry.Insurance_no = insCoverageLedgEntry2.Insurance_no
	newInsCoverageLedgEntry.Fa_no = insCoverageLedgEntry2.Fa_no
	newInsCoverageLedgEntry.Document_type = insCoverageLedgEntry2.Document_type
	newInsCoverageLedgEntry.Index_entry = insCoverageLedgEntry2.Index_entry
	newInsCoverageLedgEntry.Posting_date = periodStart
	newInsCoverageLedgEntry.Description = r.description
	newInsCoverageLedgEntry.Source_code = r.sourceCodeSetup.Compress_insurance_ledger
	newInsCoverageLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	r.window.Update(1, newInsCoverageLedgEntry.Insurance_no)
	r.window.Update(2, newInsCoverageLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		insCoverageLedgEntry2.SetRange("document_no", insCoverageLedgEntry2.Document_no)
		newInsCoverageLedgEntry.Document_no = insCoverageLedgEntry2.Document_no
	}
	if r.retainNo("dimension_1_code") {
		insCoverageLedgEntry2.SetRange("dimension_1_code", insCoverageLedgEntry2.Dimension_1_code)
		newInsCoverageLedgEntry.Dimension_1_code = insCoverageLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		insCoverageLedgEntry2.SetRange("dimension_2_code", insCoverageLedgEntry2.Dimension_2_code)
		newInsCoverageLedgEntry.Dimension_2_code = insCoverageLedgEntry2.Dimension_2_code
	}
	for found := insCoverageLedgEntry2.FindFirst(); found; found = insCoverageLedgEntry2.FindFirst() {
		newInsCoverageLedgEntry.Amount = newInsCoverageLedgEntry.Amount.Add(insCoverageLedgEntry2.Amount)
		err = insCoverageLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	err = newInsCoverageLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressInsLedger) onPostDataItem(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the insurance register of the new entries and the Date Compr.
// Register row
func (r *dateCompressInsLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_insurance_ledger, r.lastEntryNo)
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_insurance_ledger, func(field string) string { return insCoverageLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressInsLedger) insertRegisters(run *Run) error {
	err := r.save(r.newInsCoverageLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true

	insCoverageLedgEntry2 := gtables.NewInsCoverageLedgerEntry(run)
	if insCoverageLedgEntry2.FindLast() && r.lastEntryNo != insCoverageLedgEntry2.Entry_no {
		r.lastEntryNo = insCoverageLedgEntry2.Entry_no
		r.initRegisters(run)
	}
	return nil
}
