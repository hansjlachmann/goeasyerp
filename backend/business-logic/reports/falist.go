package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5601 Fixed Asset - List: per fixed asset in the filters (not inactive) its master data —
// main asset/component, budgeted asset, serial no., dimension 1 and 2, class, subclass, location —
// and its FA depreciation book in the chosen book: the depreciation method and its figures,
// printed only where they are set (straight-line %, years and months, fixed amount, declining-
// balance %, depreciation table, final rounding, ending book value, exchange rate, depreciation
// below zero, projected disposal, custom 1); assets without the book are not printed
// (PrintOnlyIfDetail); New Page per Asset.
const FixedAssetListID = 5601

func init() {
	Register(FixedAssetListID, "fixed-asset-list", func() Report { return &fixedAssetList{} })
}

type fixedAssetList struct {
	deprBookCode        types.Code
	printOnlyOnePerPage bool
	fa                  *gtables.FixedAsset
	printedFixedAssets  int
}

func (r *fixedAssetList) Definition() *Definition {
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
				Name:             "FADepreciationBook",
				Table:            gtables.FADepreciationBookTableName,
				Key:              []string{"fa_no", "depreciation_book_code"},
				OnPreDataItem:    Trigger(r.faDeprBookOnPreDataItem),
				OnAfterGetRecord: Trigger(r.faDeprBookOnAfterGetRecord),
			}},
		}},
	}
}

func (r *fixedAssetList) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.printOnlyOnePerPage = run.OptionBool("new_page_per_asset")
	setReportHeadValues(run)
	return nil
}

func (r *fixedAssetList) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
	setFilterText(run, rec)
	return nil
}

// faOnAfterGetRecord: not inactive assets with the book (PrintOnlyIfDetail); the captions printed
// only when the asset is a component, a budgeted asset or has a serial no.; New Page per Asset
func (r *fixedAssetList) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	if rec.Inactive {
		return Skip()
	}
	r.fa = rec
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": rec.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	lang := fcodeunits.CurrentLanguage()
	componentFieldname := ""
	if rec.Main_asset_component != gtables.FixedAsset_Main_asset_component.Blank {
		componentFieldname = rec.GetFieldCaption("component_of_main_asset", lang)
	}
	budgetedAssetFieldname := ""
	if rec.Budgeted_asset {
		budgetedAssetFieldname = rec.GetFieldCaption("budgeted_asset", lang)
	}
	run.SetValue("component_fieldname", componentFieldname)
	run.SetValue("budgeted_asset_fieldname", budgetedAssetFieldname)
	run.SetValue("is_component", rec.Main_asset_component == gtables.FixedAsset_Main_asset_component.Component)
	if r.printOnlyOnePerPage && r.printedFixedAssets > 0 {
		run.NewPage()
	}
	r.printedFixedAssets++
	return nil
}

// faDeprBookOnPreDataItem: the asset's FA depreciation book in the book
func (r *fixedAssetList) faDeprBookOnPreDataItem(rec *gtables.FADepreciationBook, run *Run) error {
	rec.SetRange("fa_no", r.fa.No)
	rec.SetRange("depreciation_book_code", r.deprBookCode)
	return nil
}

// faDeprBookOnAfterGetRecord: which of the book's sections print (their OnPreSection)
func (r *fixedAssetList) faDeprBookOnAfterGetRecord(rec *gtables.FADepreciationBook, run *Run) error {
	zero := types.Decimal{}
	run.SetValue("show_straight_line", rec.Straight_line_percent.GreaterThan(zero))
	run.SetValue("show_years", rec.No_of_depreciation_years.GreaterThan(zero))
	run.SetValue("show_fixed_amount", rec.Fixed_depr_amount.GreaterThan(zero))
	run.SetValue("show_declining_balance", rec.Declining_balance_percent.GreaterThan(zero))
	run.SetValue("show_depr_table", rec.Depreciation_table_code != "")
	run.SetValue("show_final_rounding", !rec.Final_rounding_amount.IsZero())
	run.SetValue("show_ending_book_value", !rec.Ending_book_value.IsZero())
	run.SetValue("show_fa_exchange_rate", !rec.Fa_exchange_rate.IsZero())
	run.SetValue("show_no_ledger_check", !rec.Use_fa_ledger_check)
	run.SetValue("show_depr_below_zero_percent", !rec.Use_fa_ledger_check && rec.Fixed_depr_amount_below_zero.IsZero())
	run.SetValue("show_fixed_below_zero", !rec.Use_fa_ledger_check && rec.Fixed_depr_amount_below_zero.GreaterThan(zero))
	run.SetValue("show_projected_disposal", !rec.Projected_disposal_date.IsZero())
	run.SetValue("show_custom_1", !rec.Depr_ending_date_custom_1.IsZero())
	return nil
}
