package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5687 Copy Depreciation Book: the FA ledger entries of the chosen posting types in one
// book between two dates as journal lines of another book (the amounts at the books' default
// exchange rates when they use FA exchange rates in duplication) — in the journals of the other
// book's FA Journal Setup (faEntryJournal); an asset without the other book gets it (a copy of
// its FA depreciation book). Posted by the user. A processing-only report (the depreciation book
// card's and list's Depr. Book menu).
const CopyDepreciationBookID = 5687

func init() {
	Register(CopyDepreciationBookID, "copy-depreciation-book", func() Report { return &copyDepreciationBook{} })
}

type copyDepreciationBook struct {
	faEntryJournal
	fromDeprBook                *gtables.DepreciationBook // DeprBook (the classic DeprBook2 is faEntryJournal's)
	deprBookCode, deprBookCode2 types.Code
	exchangeRate                types.Decimal
}

func (r *copyDepreciationBook) Definition() *Definition {
	deprBookRelation := &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}
	options := []Option{
		{Name: "copy_from_book", Type: "text", Required: true, Relation: deprBookRelation},
		{Name: "copy_to_book", Type: "text", Required: true, Relation: deprBookRelation},
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
		ContextTable:   gtables.DepreciationBookTableName,
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

// Copy from: the book it is run from, else the FA Setup's default depreciation book
func (r *copyDepreciationBook) onInitReport(run *Run) error {
	deprBook, ok := run.Context().(*gtables.DepreciationBook)
	if ok && deprBook.Code != "" {
		run.SetOption("copy_from_book", deprBook.Code.String())
		return nil
	}
	faSetup := gtables.GetFASetup(run)
	run.SetOption("copy_from_book", faSetup.Default_depr_book.String())
	return nil
}

func (r *copyDepreciationBook) onPreReport(run *Run) error {
	err := r.readOptions(run)
	if err != nil {
		return err
	}
	r.deprBookCode = types.Code(system.UpperCase(run.Option("copy_from_book")))
	r.deprBookCode2 = types.Code(system.UpperCase(run.Option("copy_to_book")))
	r.fromDeprBook = gtables.NewDepreciationBook(run)
	err = r.fromDeprBook.GetOrError(r.deprBookCode)
	if err != nil {
		return err
	}
	r.deprBook = gtables.NewDepreciationBook(run)
	err = r.deprBook.GetOrError(r.deprBookCode2)
	if err != nil {
		return err
	}
	r.exchangeRate = r.getExchangeRate()
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

// faOnAfterGetRecord: the asset's entries in the book copied from, in date order
func (r *copyDepreciationBook) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	r.window.Update(1, fa.No)
	if fa.Inactive || fa.Blocked {
		return Skip()
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	faDeprBook2 := gtables.NewFADepreciationBook(run)
	if !faDeprBook2.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode2}) {
		faDeprBook2.Copy(faDeprBook)
		faDeprBook2.Depreciation_book_code = r.deprBookCode2
		err := faDeprBook2.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	faLedgEntry := r.faLedgEntries(run, fa.No, r.deprBookCode)
	for found := faLedgEntry.Find("-"); found; found = faLedgEntry.Next() {
		insert, gen := r.journalType(fa, faLedgEntry)
		var err error
		switch {
		case !insert:
		case gen:
			err = r.insertGenJnlLine(run, faLedgEntry, func(genJnlLine *gtables.GenJournalLine) error {
				return genJnlLine.Validate("amount", system.Round(genJnlLine.Amount.Mul(r.exchangeRate)))
			})
		default:
			err = r.insertFAJnlLine(run, faLedgEntry, func(faJnlLine *gtables.FAJournalLine) error {
				return faJnlLine.Validate("amount", system.Round(faJnlLine.Amount.Mul(r.exchangeRate)))
			})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *copyDepreciationBook) faOnPostDataItem(fa *gtables.FixedAsset, run *Run) error {
	r.window.Close()
	return nil
}

// getExchangeRate (GetExchangeRate): the from book's default exchange rate over the to book's
// (100 for a book that does not use FA exchange rates in duplication)
func (r *copyDepreciationBook) getExchangeRate() types.Decimal {
	hundred := types.NewDecimalFromInt(100)
	exchangeRate2 := r.fromDeprBook.Default_exchange_rate
	if !exchangeRate2.IsPositive() || !r.fromDeprBook.Use_fa_exch_rate_in_duplic {
		exchangeRate2 = hundred
	}
	exchangeRate3 := r.deprBook.Default_exchange_rate
	if !exchangeRate3.IsPositive() || !r.deprBook.Use_fa_exch_rate_in_duplic {
		exchangeRate3 = hundred
	}
	return exchangeRate2.Div(exchangeRate3)
}
