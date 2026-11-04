package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5604 Fixed Asset - Details: per fixed asset in the filters (not inactive) its FA ledger
// entries in a depreciation book (FA Posting Date Filter: those in the dates); assets without
// entries are not printed (PrintOnlyIfDetail); New Page per Asset (main menu Fixed Assets →
// Reports).
const FixedAssetDetailsID = 5604

func init() {
	Register(FixedAssetDetailsID, "fixed-asset-details", func() Report { return &fixedAssetDetails{} })
}

type fixedAssetDetails struct {
	deprBookCode        types.Code
	printOnlyOnePerPage bool
	fa                  *gtables.FixedAsset
	printedFixedAssets  int
}

func (r *fixedAssetDetails) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "new_page_per_asset", Type: "bool", Default: "false"},
		},
		OnInitReport: func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
			OnPreDataItem:       Trigger(r.faOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:          "FALedgerEntry",
				Table:         gtables.FALedgerEntryTableName,
				Key:           []string{"fa_no", "depreciation_book_code", "fa_posting_date"},
				OnPreDataItem: Trigger(r.faLedgEntryOnPreDataItem),
			}},
		}},
	}
}

func (r *fixedAssetDetails) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.printOnlyOnePerPage = run.OptionBool("new_page_per_asset")
	setReportHeadValues(run)
	return nil
}

func (r *fixedAssetDetails) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
	setFilterText(run, rec)
	return nil
}

// faOnAfterGetRecord: not inactive assets that have entries in the book (PrintOnlyIfDetail);
// New Page per Asset (NEWPAGEPERRECORD)
func (r *fixedAssetDetails) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	if rec.Inactive {
		return Skip()
	}
	r.fa = rec
	faLedgEntry := gtables.NewFALedgerEntry(run)
	r.setEntryFilters(faLedgEntry)
	if faLedgEntry.IsEmpty() {
		return Skip()
	}
	if r.printOnlyOnePerPage && r.printedFixedAssets > 0 {
		run.NewPage()
	}
	r.printedFixedAssets++
	return nil
}

// faLedgEntryOnPreDataItem: the asset's entries in the book in its FA Posting Date Filter
func (r *fixedAssetDetails) faLedgEntryOnPreDataItem(rec *gtables.FALedgerEntry, run *Run) error {
	r.setEntryFilters(rec)
	return nil
}

func (r *fixedAssetDetails) setEntryFilters(faLedgEntry *gtables.FALedgerEntry) {
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	faLedgEntry.SetRange("fa_no", r.fa.No)
	faLedgEntry.SetRange("depreciation_book_code", r.deprBookCode)
	if r.fa.Fa_posting_date_filter != "" {
		faLedgEntry.SetFilter("fa_posting_date", r.fa.Fa_posting_date_filter)
	}
}
