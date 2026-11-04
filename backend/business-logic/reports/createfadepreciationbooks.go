package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5689 Create FA Depreciation Books: every fixed asset in the filters (not inactive)
// without the depreciation book gets it — blank, or a copy of another asset's FA depreciation
// book in that book (Copy from FA No.). A processing-only report (the Fixed Asset list's
// Functions).
const CreateFADepreciationBooksID = 5689

func init() {
	Register(CreateFADepreciationBooksID, "create-fa-depreciation-books", func() Report { return &createFADepreciationBooks{} })
}

type createFADepreciationBooks struct {
	deprBookCode types.Code
	faNo         types.Code
	faDeprBook2  *gtables.FADepreciationBook
	window       *system.Window
}

func (r *createFADepreciationBooks) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "copy_from_fa_no", Type: "text", Relation: &OptionRelation{Table: "Fixed_Asset", Columns: []string{"no", "description"}}},
		},
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
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
func (r *createFADepreciationBooks) onInitReport(run *Run) error {
	faSetup := gtables.GetFASetup(run)
	run.SetOption("depreciation_book", faSetup.Default_depr_book.String())
	return nil
}

// onPreReport: the book, and the FA depreciation book copied (CheckFADeprBook: it must exist)
func (r *createFADepreciationBooks) onPreReport(run *Run) error {
	r.deprBookCode = types.Code(system.UpperCase(run.Option("depreciation_book")))
	r.faNo = types.Code(system.UpperCase(run.Option("copy_from_fa_no")))
	deprBook := gtables.NewDepreciationBook(run)
	err := deprBook.GetOrError(r.deprBookCode)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	if r.faNo != "" {
		r.faDeprBook2 = gtables.NewFADepreciationBook(run)
		err = r.faDeprBook2.GetOrError(map[string]interface{}{"fa_no": r.faNo, "depreciation_book_code": r.deprBookCode})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *createFADepreciationBooks) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	if fa.Inactive {
		return Skip()
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		r.window.Update(2, fa.No)
		return Skip()
	}
	r.window.Update(1, fa.No)
	if r.faNo != "" {
		faDeprBook.Copy(r.faDeprBook2)
	} else {
		faDeprBook.Init()
	}
	faDeprBook.Fa_no = fa.No
	faDeprBook.Depreciation_book_code = r.deprBookCode
	return faDeprBook.InsertOrError(true)
}

func (r *createFADepreciationBooks) faOnPostDataItem(fa *gtables.FixedAsset, run *Run) error {
	r.window.Close()
	return nil
}
