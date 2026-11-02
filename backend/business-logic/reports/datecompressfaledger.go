package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5696 Date Compress FA Ledger: the FA ledger entries of closed fiscal years (not the
// disposal's proceeds, gain/loss or the entries of a posting category) become one entry per
// asset, book, posting type, part of book value/depreciable basis, posting group, document type,
// period and sign of the quantity — and per the retained Document No., Reclassification Entry,
// Index Entry, Department and Project Code; only entries of the same posting type in a row of
// the asset's book are added up. The FA depreciation book's dates follow; an FA register of the
// run.
const DateCompressFALedgerID = 5696

func init() {
	Register(DateCompressFALedgerID, "date-compress-fa-ledger", func() Report { return &dateCompressFALedger{} })
}

type dateCompressFALedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup *gtables.SourceCodeSetup
	newFALedgEntry  *gtables.FALedgerEntry
	lastEntryNo     int
	window          *system.Window
}

func (r *dateCompressFALedger) Definition() *Definition {
	r.tableID = gtables.FALedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "reclassification_entry"}, {name: "index_entry"}, {name: "department_code"}, {name: "project_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "FA_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "FA_Ledger_Entry", Table: gtables.FALedgerEntryTableName, Key: []string{"fa_no", "depreciation_book_code", "fa_posting_date"},
			RequestFilterFields: []string{"fa_no", "depreciation_book_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressFALedger) onPreDataItem(faLedgEntry *gtables.FALedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_fa_ledger")
	if err != nil {
		return err
	}

	r.newFALedgEntry = gtables.NewFALedgerEntry(run)
	r.newFALedgEntry.LockTable()
	r.reg = gtables.NewFARegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	faLedgEntry2 := gtables.NewFALedgerEntry(run)
	r.lastEntryNo = 0
	if faLedgEntry2.FindLast() {
		r.lastEntryNo = faLedgEntry2.Entry_no
	}
	faLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	faLedgEntry.SetRange("fa_posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressFALedger) onAfterGetRecord(faLedgEntry *gtables.FALedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewFALedgerEntry(run)
	if !stillThere.Get(faLedgEntry.Entry_no) {
		return Skip()
	}
	if faLedgEntry.Fa_no != "" {
		if faLedgEntry.Fa_posting_category != gtables.FALedgerEntry_Fa_posting_category.Blank {
			return Skip()
		}
		switch faLedgEntry.Fa_posting_type {
		case gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal, gtables.FALedgerEntry_Fa_posting_type.GainLoss:
			return Skip()
		}
	}

	faLedgEntry2 := gtables.NewFALedgerEntry(run)
	faLedgEntry2.Copy(faLedgEntry)
	faLedgEntry2.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	faLedgEntry2.CopyFilters(faLedgEntry)

	faLedgEntry2.SetRange("fa_no", faLedgEntry2.Fa_no)
	faLedgEntry2.SetRange("depreciation_book_code", faLedgEntry2.Depreciation_book_code)
	faLedgEntry2.SetRange("fa_posting_category", faLedgEntry2.Fa_posting_category)
	faLedgEntry2.SetRange("fa_posting_type", faLedgEntry2.Fa_posting_type)
	faLedgEntry2.SetRange("part_of_book_value", faLedgEntry2.Part_of_book_value)
	faLedgEntry2.SetRange("part_of_depreciable_basis", faLedgEntry2.Part_of_depreciable_basis)
	faLedgEntry2.SetRange("fa_posting_group", faLedgEntry2.Fa_posting_group)
	faLedgEntry2.SetRange("document_type", faLedgEntry2.Document_type)

	dateFilter, periodStart, err := r.dateFilter(faLedgEntry2.Fa_posting_date, true)
	if err != nil {
		return err
	}
	faLedgEntry2.SetFilter("fa_posting_date", dateFilter)

	r.lastEntryNo++

	newFALedgEntry := r.newFALedgEntry
	newFALedgEntry.Init()
	newFALedgEntry.Entry_no = r.lastEntryNo
	newFALedgEntry.Fa_no = faLedgEntry2.Fa_no
	newFALedgEntry.Depreciation_book_code = faLedgEntry2.Depreciation_book_code
	newFALedgEntry.Fa_posting_category = faLedgEntry2.Fa_posting_category
	newFALedgEntry.Fa_posting_type = faLedgEntry2.Fa_posting_type
	newFALedgEntry.Part_of_book_value = faLedgEntry2.Part_of_book_value
	newFALedgEntry.Part_of_depreciable_basis = faLedgEntry2.Part_of_depreciable_basis
	newFALedgEntry.Fa_posting_group = faLedgEntry2.Fa_posting_group
	newFALedgEntry.Document_type = faLedgEntry2.Document_type

	newFALedgEntry.Fa_posting_date = periodStart
	newFALedgEntry.Posting_date = periodStart
	newFALedgEntry.Description = r.description
	newFALedgEntry.Source_code = r.sourceCodeSetup.Compress_fa_ledger
	newFALedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	r.window.Update(1, newFALedgEntry.Fa_no)
	r.window.Update(2, newFALedgEntry.Fa_posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		faLedgEntry2.SetRange("document_no", faLedgEntry2.Document_no)
		newFALedgEntry.Document_no = faLedgEntry2.Document_no
	}
	if r.retainNo("reclassification_entry") {
		faLedgEntry2.SetRange("reclassification_entry", faLedgEntry2.Reclassification_entry)
		newFALedgEntry.Reclassification_entry = faLedgEntry2.Reclassification_entry
	}
	if r.retainNo("index_entry") {
		faLedgEntry2.SetRange("index_entry", faLedgEntry2.Index_entry)
		newFALedgEntry.Index_entry = faLedgEntry2.Index_entry
	}
	if r.retainNo("department_code") {
		faLedgEntry2.SetRange("department_code", faLedgEntry2.Department_code)
		newFALedgEntry.Department_code = faLedgEntry2.Department_code
	}
	if r.retainNo("project_code") {
		faLedgEntry2.SetRange("project_code", faLedgEntry2.Project_code)
		newFALedgEntry.Project_code = faLedgEntry2.Project_code
	}
	if !faLedgEntry2.Quantity.IsNegative() {
		faLedgEntry2.SetFilter("quantity", ">=0")
	} else {
		faLedgEntry2.SetFilter("quantity", "<0")
	}
	var equalPostingType map[int]bool
	if faLedgEntry2.Fa_no != "" {
		equalPostingType = r.findEqualPostingType(faLedgEntry2, run)
	}
	var group []*gtables.FALedgerEntry
	for found := faLedgEntry2.FindSetBuffered(); found; found = faLedgEntry2.Next() {
		entry := gtables.NewFALedgerEntry(run)
		entry.Copy(faLedgEntry2)
		group = append(group, entry)
	}
	for _, entry := range group {
		if equalPostingType[entry.Entry_no] || entry.Fa_no == "" {
			newFALedgEntry.Quantity = newFALedgEntry.Quantity.Add(entry.Quantity)
			newFALedgEntry.Amount = newFALedgEntry.Amount.Add(entry.Amount)
			err = entry.DeleteOrError(false)
			if err != nil {
				return err
			}
			r.deleted()
		}
	}
	err = newFALedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	if newFALedgEntry.Fa_no != "" {
		err = fajnl.SetFAPostingDate(newFALedgEntry)
		if err != nil {
			return err
		}
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressFALedger) onPostDataItem(faLedgEntry *gtables.FALedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// findEqualPostingType (FindEqualPostingType): from the entry on, the entries of the asset's
// book in a row with the same posting category, type and parts of book value / depreciable
// basis — those of the group that are added up
func (r *dateCompressFALedger) findEqualPostingType(faLedgEntry2 *gtables.FALedgerEntry, run *Run) map[int]bool {
	faLedgEntryTmp := map[int]bool{}
	faLedgEntry := gtables.NewFALedgerEntry(run)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	faLedgEntry.SetRange("fa_no", faLedgEntry2.Fa_no)
	faLedgEntry.SetRange("depreciation_book_code", faLedgEntry2.Depreciation_book_code)
	if !faLedgEntry.Get(faLedgEntry2.Entry_no) {
		return faLedgEntryTmp
	}
	for {
		equalPostingType := faLedgEntry2.Fa_posting_category == faLedgEntry.Fa_posting_category &&
			faLedgEntry2.Fa_posting_type == faLedgEntry.Fa_posting_type &&
			faLedgEntry2.Part_of_book_value == faLedgEntry.Part_of_book_value &&
			faLedgEntry2.Part_of_depreciable_basis == faLedgEntry.Part_of_depreciable_basis
		if equalPostingType {
			faLedgEntryTmp[faLedgEntry.Entry_no] = true
		}
		if !faLedgEntry.Next() || !equalPostingType {
			break
		}
	}
	return faLedgEntryTmp
}

// initRegisters (InitRegisters): the FA register (journal type Fixed Asset) of the new entries
// and the Date Compr. Register row
func (r *dateCompressFALedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_fa_ledger, r.lastEntryNo)
	faReg, _ := r.reg.(*gtables.FARegister)
	faReg.Journal_type = gtables.FARegister_Journal_type.FixedAsset
	faLedgEntry := gtables.NewFALedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_fa_ledger, func(field string) string { return faLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressFALedger) insertRegisters(run *Run) error {
	err := r.save(r.newFALedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true
	faLedgEntry2 := gtables.NewFALedgerEntry(run)
	if faLedgEntry2.FindLast() {
		r.lastEntryNo = faLedgEntry2.Entry_no
	}
	return nil
}
