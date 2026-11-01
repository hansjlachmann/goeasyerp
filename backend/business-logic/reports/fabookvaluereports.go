package reports

import (
	"strconv"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5605 Fixed Asset - Book Value 01: per asset (or group) one line — acquisition cost and
// depreciation at the starting date, their additions and disposals in the period and at the
// ending date (write-downs … custom 2 counted as their FA Posting Type Setup says), the book value
// at both dates (landscape; main menu Fixed Assets → Reports).
const FixedAssetBookValue01ID = 5605

// Report 5606 Fixed Asset - Book Value 02: per asset (or group) the posting types one under the
// other, each at the starting date, its additions and disposals in the period and at the ending
// date — with Include Reclassification also the reclassified amounts —, and the book value.
const FixedAssetBookValue02ID = 5606

func init() {
	Register(FixedAssetBookValue01ID, "fixed-asset-book-value-01", func() Report {
		return &fixedAssetBookValue{faBookValue: faBookValue{addPostingTypes: true}, id: FixedAssetBookValue01ID}
	})
	Register(FixedAssetBookValue02ID, "fixed-asset-book-value-02", func() Report {
		return &fixedAssetBookValue{id: FixedAssetBookValue02ID}
	})
}

type fixedAssetBookValue struct {
	faBookValue
	id int
}

func (r *fixedAssetBookValue) Definition() *Definition {
	options := faBookValueOptions()
	if r.id == FixedAssetBookValue02ID {
		options = append(options, Option{Name: "include_reclassification", Type: "bool", Default: "false"})
	}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        options,
		OnInitReport:   func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
			OnPreDataItem:       Trigger(r.faOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
		}},
	}
}

func (r *fixedAssetBookValue) onPreReport(run *Run) error {
	r.reclassify = run.OptionBool("include_reclassification")
	err := r.faBookValue.onPreReport(run)
	if err != nil {
		return err
	}
	run.SetValue("reclassify", r.reclassify)
	if r.id == FixedAssetBookValue01ID {
		r.makeHeadLine(run)
	}
	return nil
}

// makeHeadLine (report 5605 MakeHeadLine): the column heads with the dates
func (r *fixedAssetBookValue) makeHeadLine(run *Run) {
	lang := fcodeunits.CurrentLanguage()
	startText := formatDate(r.startingDate.AddDays(-1), lang)
	endText := formatDate(r.endingDate, lang)
	heads := []string{
		run.Text("ACQ_COST_AT", startText), run.Text("ADDITION_IN_PERIOD"), run.Text("DISPOSAL_IN_PERIOD"), run.Text("ACQ_COST_AT", endText),
		run.Text("DEPRECIATION_AT", startText), run.Text("DEPRECIATION_IN_PERIOD"), run.Text("DISPOSAL_DEPRECIATION_IN_PERIOD"), run.Text("DEPRECIATION_AT", endText),
		run.Text("BOOK_VALUE_AT", startText), run.Text("BOOK_VALUE_AT", endText),
	}
	for i, head := range heads {
		run.SetValue("head_"+itoa(i+1), head)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }
