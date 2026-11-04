package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5698 Date Compress Maint. Ledger: the maintenance ledger entries of closed fiscal years
// become one entry per asset, book, posting group, document type, period and sign of the
// quantity — and per the retained Document No., Maintenance Code, Index Entry, Dimension 1 and
// Dimension 2 Code; the asset's Last Maintenance Date follows; the FA register of the run holds the
// maintenance entry range.
const DateCompressMaintLedgerID = 5698

func init() {
	Register(DateCompressMaintLedgerID, "date-compress-maint-ledger", func() Report { return &dateCompressMaintLedger{} })
}

type dateCompressMaintLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup         *gtables.SourceCodeSetup
	newMaintenanceLedgEntry *gtables.MaintenanceLedgerEntry
	lastEntryNo             int
	window                  *system.Window
}

func (r *dateCompressMaintLedger) Definition() *Definition {
	r.tableID = gtables.MaintenanceLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "maintenance_code"}, {name: "index_entry"}, {name: "dimension_1_code"}, {name: "dimension_2_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Maintenance_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Maintenance_Ledger_Entry", Table: gtables.MaintenanceLedgerEntryTableName, Key: []string{"fa_no", "depreciation_book_code", "fa_posting_date"},
			RequestFilterFields: []string{"fa_no", "depreciation_book_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressMaintLedger) onPreDataItem(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_maintenance_ledger")
	if err != nil {
		return err
	}

	r.newMaintenanceLedgEntry = gtables.NewMaintenanceLedgerEntry(run)
	r.newMaintenanceLedgEntry.LockTable()
	r.reg = gtables.NewFARegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	maintenanceLedgEntry2 := gtables.NewMaintenanceLedgerEntry(run)
	r.lastEntryNo = 0
	if maintenanceLedgEntry2.FindLast() {
		r.lastEntryNo = maintenanceLedgEntry2.Entry_no
	}
	maintenanceLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	maintenanceLedgEntry.SetRange("fa_posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressMaintLedger) onAfterGetRecord(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewMaintenanceLedgerEntry(run)
	if !stillThere.Get(maintenanceLedgEntry.Entry_no) {
		return Skip()
	}
	maintenanceLedgEntry2 := gtables.NewMaintenanceLedgerEntry(run)
	maintenanceLedgEntry2.Copy(maintenanceLedgEntry)
	maintenanceLedgEntry2.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry2.CopyFilters(maintenanceLedgEntry)

	maintenanceLedgEntry2.SetRange("fa_no", maintenanceLedgEntry2.Fa_no)
	maintenanceLedgEntry2.SetRange("depreciation_book_code", maintenanceLedgEntry2.Depreciation_book_code)
	maintenanceLedgEntry2.SetRange("fa_posting_group", maintenanceLedgEntry2.Fa_posting_group)
	maintenanceLedgEntry2.SetRange("document_type", maintenanceLedgEntry2.Document_type)

	dateFilter, periodStart, err := r.dateFilter(maintenanceLedgEntry2.Fa_posting_date, true)
	if err != nil {
		return err
	}
	maintenanceLedgEntry2.SetFilter("fa_posting_date", dateFilter)

	r.lastEntryNo++

	newMaintenanceLedgEntry := r.newMaintenanceLedgEntry
	newMaintenanceLedgEntry.Init()
	newMaintenanceLedgEntry.Entry_no = r.lastEntryNo
	newMaintenanceLedgEntry.Fa_no = maintenanceLedgEntry2.Fa_no
	newMaintenanceLedgEntry.Depreciation_book_code = maintenanceLedgEntry2.Depreciation_book_code
	newMaintenanceLedgEntry.Fa_posting_group = maintenanceLedgEntry2.Fa_posting_group
	newMaintenanceLedgEntry.Document_type = maintenanceLedgEntry2.Document_type

	newMaintenanceLedgEntry.Fa_posting_date = periodStart
	newMaintenanceLedgEntry.Posting_date = periodStart
	newMaintenanceLedgEntry.Description = r.description
	newMaintenanceLedgEntry.Source_code = r.sourceCodeSetup.Compress_maintenance_ledger
	newMaintenanceLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	r.window.Update(1, newMaintenanceLedgEntry.Fa_no)
	r.window.Update(2, newMaintenanceLedgEntry.Fa_posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		maintenanceLedgEntry2.SetRange("document_no", maintenanceLedgEntry2.Document_no)
		newMaintenanceLedgEntry.Document_no = maintenanceLedgEntry2.Document_no
	}
	if r.retainNo("maintenance_code") {
		maintenanceLedgEntry2.SetRange("maintenance_code", maintenanceLedgEntry2.Maintenance_code)
		newMaintenanceLedgEntry.Maintenance_code = maintenanceLedgEntry2.Maintenance_code
	}
	if r.retainNo("index_entry") {
		maintenanceLedgEntry2.SetRange("index_entry", maintenanceLedgEntry2.Index_entry)
		newMaintenanceLedgEntry.Index_entry = maintenanceLedgEntry2.Index_entry
	}
	if r.retainNo("dimension_1_code") {
		maintenanceLedgEntry2.SetRange("dimension_1_code", maintenanceLedgEntry2.Dimension_1_code)
		newMaintenanceLedgEntry.Dimension_1_code = maintenanceLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		maintenanceLedgEntry2.SetRange("dimension_2_code", maintenanceLedgEntry2.Dimension_2_code)
		newMaintenanceLedgEntry.Dimension_2_code = maintenanceLedgEntry2.Dimension_2_code
	}
	if !maintenanceLedgEntry2.Quantity.IsNegative() {
		maintenanceLedgEntry2.SetFilter("quantity", ">=0")
	} else {
		maintenanceLedgEntry2.SetFilter("quantity", "<0")
	}
	for found := maintenanceLedgEntry2.FindFirst(); found; found = maintenanceLedgEntry2.FindFirst() {
		newMaintenanceLedgEntry.Quantity = newMaintenanceLedgEntry.Quantity.Add(maintenanceLedgEntry2.Quantity)
		newMaintenanceLedgEntry.Amount = newMaintenanceLedgEntry.Amount.Add(maintenanceLedgEntry2.Amount)
		err = maintenanceLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
	}
	err = newMaintenanceLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	if newMaintenanceLedgEntry.Fa_no != "" {
		err = fajnl.SetMaintenanceLastDate(newMaintenanceLedgEntry)
		if err != nil {
			return err
		}
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressMaintLedger) onPostDataItem(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the FA register (journal type Fixed Asset) of the new
// maintenance entries and the Date Compr. Register row
func (r *dateCompressMaintLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_maintenance_ledger, r.lastEntryNo)
	faReg, _ := r.reg.(*gtables.FARegister)
	faReg.Journal_type = gtables.FARegister_Journal_type.FixedAsset
	faReg.From_entry_no = 0
	faReg.From_maintenance_entry_no = r.lastEntryNo + 1
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_maintenance_ledger, func(field string) string { return maintenanceLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressMaintLedger) insertRegisters(run *Run) error {
	faReg, _ := r.reg.(*gtables.FARegister)
	faReg.To_maintenance_entry_no = r.newMaintenanceLedgEntry.Entry_no
	err := r.save(0)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true
	maintenanceLedgEntry2 := gtables.NewMaintenanceLedgerEntry(run)
	if maintenanceLedgEntry2.FindLast() {
		r.lastEntryNo = maintenanceLedgEntry2.Entry_no
	}
	return nil
}
