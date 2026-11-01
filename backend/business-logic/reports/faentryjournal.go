package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// faEntryChoices: the request page check boxes of reports 5687 Copy Depreciation Book and 5688
// Cancel FA Ledger Entries, by FA journal posting type (the classic CopyChoices/CancelChoices)
var faEntryChoices = []struct {
	option      string
	postingType gtables.FAJournalLineFa_posting_type
}{
	{"acquisition_cost", gtables.FAJournalLine_Fa_posting_type.AcquisitionCost},
	{"depreciation", gtables.FAJournalLine_Fa_posting_type.Depreciation},
	{"write_down", gtables.FAJournalLine_Fa_posting_type.WriteDown},
	{"appreciation", gtables.FAJournalLine_Fa_posting_type.Appreciation},
	{"custom_1", gtables.FAJournalLine_Fa_posting_type.Custom1},
	{"custom_2", gtables.FAJournalLine_Fa_posting_type.Custom2},
	{"salvage_value", gtables.FAJournalLine_Fa_posting_type.SalvageValue},
	{"disposal", gtables.FAJournalLine_Fa_posting_type.Disposal},
}

func faEntryChoiceOptions() []Option {
	var options []Option
	for _, c := range faEntryChoices {
		options = append(options, Option{Name: c.option, Type: "bool", Default: "false"})
	}
	return options
}

// faEntryJournal: the request page and the journal lines of reports 5687 and 5688 — the FA
// ledger entries (category blank, Acquisition Cost..Salvage Value) of the chosen posting types
// between two dates as lines of a book (its FA Journal Setup: the general journal when the book
// posts the type to the G/L — not for budgeted assets —, else the FA journal)
type faEntryJournal struct {
	deprBook                          *gtables.DepreciationBook // the book the lines are for
	choices                           map[gtables.FAJournalLineFa_posting_type]bool
	startingDate, endingDate2         types.Date
	documentNo                        types.Code
	postingDescription                types.Text
	balAccount                        bool
	firstGenJnl, firstFAJnl           bool
	genJnlTemplate, genJnlBatch       types.Code
	faJnlTemplate, faJnlBatch         types.Code
	genJnlNextLineNo, faJnlNextLineNo int
	documentNo2, documentNo3          types.Code
	noSeries2, noSeries3              types.Code
	window                            *system.Window
}

// readOptions (OnPreReport): the dates, document no., description, balancing account and choices
func (j *faEntryJournal) readOptions(run *Run) error {
	j.startingDate = run.OptionDate("starting_date")
	endingDate := run.OptionDate("ending_date")
	if !endingDate.IsZero() && j.startingDate.After(endingDate) {
		return system.Error(run.Text("STARTING_DATE_LATER"))
	}
	j.endingDate2 = endingDate
	if endingDate.IsZero() {
		j.endingDate2, _ = system.DMY2Date(31, 12, 9999)
	}
	j.documentNo = types.Code(system.UpperCase(run.Option("document_no")))
	j.postingDescription = types.Text(run.Option("posting_description"))
	j.balAccount = run.OptionBool("insert_bal_account")
	j.choices = map[gtables.FAJournalLineFa_posting_type]bool{}
	for _, c := range faEntryChoices {
		j.choices[c.postingType] = run.OptionBool(c.option)
	}
	j.firstGenJnl = true
	j.firstFAJnl = true
	return nil
}

// faLedgEntries (OnPreDataItem): the entries of the asset in the book the report reads
func (j *faEntryJournal) faLedgEntries(run *Run, faNo, deprBookCode types.Code) *gtables.FALedgerEntry {
	faLedgEntry := fadepr.SetFAFilter(run, faNo, deprBookCode, false)
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost, gtables.FALedgerEntry_Fa_posting_type.SalvageValue)
	faLedgEntry.SetRange("fa_posting_date", j.startingDate, j.endingDate2)
	return faLedgEntry
}

// journalType (SetJournalType): skip the entry (false), or its line in the general journal (gen)
// or the FA journal
func (j *faEntryJournal) journalType(fa *gtables.FixedAsset, faLedgEntry *gtables.FALedgerEntry) (insert, gen bool) {
	postingType := tables.FALedgEntryConvertPostingType(faLedgEntry)
	if !j.choices[postingType] {
		return false, false
	}
	return true, tables.DeprBookGLIntegration(j.deprBook, postingType) && !fa.Budgeted_asset
}

// faDocumentNo: the Document No. option, else the batch series' next number (the batch must be
// empty), else an error (GetFAJnlDocumentNo/GetGenJnlDocumentNo with CreateError)
func (j *faEntryJournal) faDocumentNo(run *Run, next func() (types.Code, error)) (types.Code, error) {
	if j.documentNo != "" {
		return j.documentNo, nil
	}
	documentNo, err := next()
	if err != nil {
		return "", err
	}
	if documentNo == "" {
		return "", system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	return documentNo, nil
}

// insertGenJnlLine (InsertGenJnlLine): the entry's line in the general journal; prepare sets
// what the report changes (the amount, cancel fields)
func (j *faEntryJournal) insertGenJnlLine(run *Run, faLedgEntry *gtables.FALedgerEntry, prepare func(genJnlLine *gtables.GenJournalLine) error) error {
	var err error
	if j.firstGenJnl {
		gtables.NewGenJournalLine(run).LockTable()
		j.genJnlTemplate, j.genJnlBatch, j.genJnlNextLineNo, err = tables.GenJnlName(run, j.deprBook.Code)
		if err != nil {
			return err
		}
		j.noSeries2 = tables.GetGenNoSeries(run, j.genJnlTemplate, j.genJnlBatch)
		j.documentNo2, err = j.faDocumentNo(run, func() (types.Code, error) {
			return tables.GetGenJnlDocumentNo(run, j.genJnlTemplate, j.genJnlBatch, faLedgEntry.Fa_posting_date)
		})
		if err != nil {
			return err
		}
	}
	j.firstGenJnl = false
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Journal_template_name = j.genJnlTemplate
	genJnlLine.Journal_batch_name = j.genJnlBatch
	err = tables.FALedgEntryMoveToGenJnl(faLedgEntry, genJnlLine)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("depreciation_book_code", j.deprBook.Code)
	if err != nil {
		return err
	}
	err = prepare(genJnlLine)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = j.documentNo2
	genJnlLine.Posting_no_series = j.noSeries2
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Blank
	genJnlLine.External_document_no = ""
	if j.postingDescription != "" {
		genJnlLine.Description = j.postingDescription
	}
	j.genJnlNextLineNo += 10000
	genJnlLine.Line_no = j.genJnlNextLineNo
	err = tables.SetGenJnlTrailCodes(genJnlLine)
	if err != nil {
		return err
	}
	err = genJnlLine.InsertOrError(true)
	if err != nil {
		return err
	}
	if j.balAccount {
		err = fajnl.GetBalAcc(genJnlLine)
		if err != nil {
			return err
		}
		last := gtables.NewGenJournalLine(run)
		last.SetRange("journal_template_name", j.genJnlTemplate)
		last.SetRange("journal_batch_name", j.genJnlBatch)
		if last.FindLast() {
			j.genJnlNextLineNo = last.Line_no
		}
	}
	return nil
}

// insertFAJnlLine (InsertFAJnlLine): the entry's line in the FA journal
func (j *faEntryJournal) insertFAJnlLine(run *Run, faLedgEntry *gtables.FALedgerEntry, prepare func(faJnlLine *gtables.FAJournalLine) error) error {
	var err error
	if j.firstFAJnl {
		gtables.NewFAJournalLine(run).LockTable()
		j.faJnlTemplate, j.faJnlBatch, j.faJnlNextLineNo, err = tables.FAJnlName(run, j.deprBook.Code)
		if err != nil {
			return err
		}
		j.noSeries3 = tables.GetFANoSeries(run, j.faJnlTemplate, j.faJnlBatch)
		j.documentNo3, err = j.faDocumentNo(run, func() (types.Code, error) {
			return tables.GetFAJnlDocumentNo(run, j.faJnlTemplate, j.faJnlBatch, faLedgEntry.Fa_posting_date)
		})
		if err != nil {
			return err
		}
	}
	j.firstFAJnl = false
	faJnlLine := gtables.NewFAJournalLine(run)
	faJnlLine.Journal_template_name = j.faJnlTemplate
	faJnlLine.Journal_batch_name = j.faJnlBatch
	err = tables.FALedgEntryMoveToFAJnl(faLedgEntry, faJnlLine)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("depreciation_book_code", j.deprBook.Code)
	if err != nil {
		return err
	}
	err = prepare(faJnlLine)
	if err != nil {
		return err
	}
	faJnlLine.Document_no = j.documentNo3
	faJnlLine.Posting_no_series = j.noSeries3
	faJnlLine.Document_type = gtables.FAJournalLine_Document_type.Blank
	faJnlLine.External_document_no = ""
	if j.postingDescription != "" {
		faJnlLine.Description = j.postingDescription
	}
	j.faJnlNextLineNo += 10000
	faJnlLine.Line_no = j.faJnlNextLineNo
	return faJnlLine.InsertOrError(true)
}
