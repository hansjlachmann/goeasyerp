package reports

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5688 Cancel FA Ledger Entries: the FA ledger entries of the chosen posting types in a
// book between two dates, last first, as journal lines that cancel them (the opposite amount,
// FA Error Entry No., Correction per the book's Mark Errors as Corrections) — in the journals of
// the book's FA Journal Setup (faEntryJournal); posted by the user. A processing-only report (the
// Fixed Asset list's Functions).
const CancelFALedgerEntriesID = 5688

func init() {
	Register(CancelFALedgerEntriesID, "cancel-fa-ledger-entries", func() Report { return &cancelFALedgerEntries{} })
}

type cancelFALedgerEntries struct {
	faEntryJournal
	deprBookCode types.Code
}

func (r *cancelFALedgerEntries) Definition() *Definition {
	options := []Option{
		{Name: "cancel_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
		{Name: "starting_date", Type: "date"},
		{Name: "ending_date", Type: "date"},
		{Name: "document_no", Type: "text"},
		{Name: "posting_description", Type: "text"},
		{Name: "insert_bal_account", Type: "bool", Default: "false"},
	}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        append(options, faEntryChoiceOptions()...),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "fixed_asset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.faOnPostDataItem),
		}},
	}
}

// The FA Setup's default depreciation book (OnOpenForm)
func (r *cancelFALedgerEntries) onInitReport(run *Run) error {
	faSetup := gtables.GetFASetup(run)
	run.SetOption("cancel_book", faSetup.Default_depr_book.String())
	return nil
}

func (r *cancelFALedgerEntries) onPreReport(run *Run) error {
	err := r.readOptions(run)
	if err != nil {
		return err
	}
	r.deprBookCode = types.Code(system.UpperCase(run.Option("cancel_book")))
	r.deprBook = gtables.NewDepreciationBook(run)
	err = r.deprBook.GetOrError(r.deprBookCode)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

// faOnAfterGetRecord: the asset's entries in the book, last first
func (r *cancelFALedgerEntries) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	r.window.Update(1, fa.No)
	if fa.Inactive || fa.Blocked {
		return Skip()
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	faLedgEntry := r.faLedgEntries(run, fa.No, r.deprBookCode)
	for found := faLedgEntry.Find("+"); found; found = faLedgEntry.Next(-1) {
		insert, gen := r.journalType(fa, faLedgEntry)
		var err error
		switch {
		case !insert:
		case gen:
			err = r.insertGenJnlLine(run, faLedgEntry, func(genJnlLine *gtables.GenJournalLine) error {
				genJnlLine.Department_code = faLedgEntry.Department_code
				genJnlLine.Project_code = faLedgEntry.Project_code
				err := genJnlLine.Validate("amount", genJnlLine.Amount.Neg())
				if err != nil {
					return err
				}
				genJnlLine.Fa_error_entry_no = faLedgEntry.Entry_no
				return genJnlLine.Validate("correction", r.deprBook.Mark_errors_as_corrections)
			})
		default:
			err = r.insertFAJnlLine(run, faLedgEntry, func(faJnlLine *gtables.FAJournalLine) error {
				faJnlLine.Department_code = faLedgEntry.Department_code
				faJnlLine.Project_code = faLedgEntry.Project_code
				err := faJnlLine.Validate("amount", faJnlLine.Amount.Neg())
				if err != nil {
					return err
				}
				faJnlLine.Fa_error_entry_no = faLedgEntry.Entry_no
				return faJnlLine.Validate("correction", r.deprBook.Mark_errors_as_corrections)
			})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *cancelFALedgerEntries) faOnPostDataItem(fa *gtables.FixedAsset, run *Run) error {
	r.window.Close()
	return nil
}
