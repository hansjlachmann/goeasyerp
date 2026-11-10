package reports_test

import (
	"fmt"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The fixed asset overviews on the demo data: Main Asset Statistics adds up the book values of
// the main asset's components, FA Posting Types Overview shows each book's FlowFields, Total
// Value Insured per FA each asset's coverage per insurance
func TestFAOverviews(t *testing.T) {
	src := listCompany(t)

	// 5603: the demo main asset and its components in the book
	mainBook := gtables.NewFADepreciationBook(src)
	mainBook.SetRange("main_asset_component", gtables.FADepreciationBook_Main_asset_component.MainAsset)
	if !mainBook.FindFirst() {
		t.Fatal("no main asset in the demo data")
	}
	components := gtables.NewFADepreciationBook(src)
	components.SetRange("depreciation_book_code", mainBook.Depreciation_book_code)
	components.SetRange("component_of_main_asset", mainBook.Component_of_main_asset)
	var wantBookValue types.Decimal
	n := 0
	for found := components.FindSet(); found; found = components.Next() {
		n++
		if components.Disposal_date.IsZero() && !components.Last_acquisition_cost_date.IsZero() {
			components.CalcFields("book_value")
			wantBookValue = wantBookValue.Add(components.Book_value)
		}
	}
	control, err := pages.RunAfterGetRecordControl(5603, mainBook)
	mustOK(t, err)
	got := fmt.Sprint(control.Values()["total_book_value"])
	if n < 2 || wantBookValue.IsZero() || got != wantBookValue.String() {
		t.Errorf("main asset %s: %d books, book value %v, want %s", mainBook.Fa_no, n, control.Values()["total_book_value"], wantBookValue)
	}

	// 5662: a cell is the book's FlowField
	faDeprBook := gtables.NewFADepreciationBook(src)
	columns, values, err := pages.RunMatrixCalculate(5662, faDeprBook, faDeprBook)
	mustOK(t, err)
	if len(columns) != 11 {
		t.Fatalf("posting type columns %d", len(columns))
	}
	check := gtables.NewFADepreciationBook(src)
	mustOK(t, check.GetOrError(map[string]interface{}{"fa_no": mainBook.Fa_no, "depreciation_book_code": mainBook.Depreciation_book_code}))
	check.CalcFields("book_value", "acquisition_cost")
	cells := values[ftables.KeyString(check)]
	if len(cells) != 11 || cells[0] != check.Book_value.String() || cells[1] != check.Acquisition_cost.String() {
		t.Errorf("overview of %s: %v, want book value %s, acquisition cost %s", check.Fa_no, cells, check.Book_value, check.Acquisition_cost)
	}

	// 5622: a cell is the coverage of the asset by the insurance
	entry := gtables.NewInsCoverageLedgerEntry(src)
	entry.SetRange("disposed_fa", false)
	if !entry.FindFirst() {
		t.Fatal("no insurance coverage in the demo data")
	}
	entry.SetRange("insurance_no", entry.Insurance_no)
	entry.SetRange("fa_no", entry.Fa_no)
	mustOK(t, entry.CalcSums("amount"))
	fa := gtables.NewFixedAsset(src)
	columns, values, err = pages.RunMatrixCalculate(5622, fa, fa)
	mustOK(t, err)
	col := -1
	for i, c := range columns {
		if c.Name == entry.Insurance_no.String() {
			col = i
		}
	}
	if col < 0 || values[entry.Fa_no.String()][col] != entry.Amount.String() {
		t.Errorf("coverage of %s by %s: columns %v, cells %v, want %s", entry.Fa_no, entry.Insurance_no, columns, values[entry.Fa_no.String()], entry.Amount)
	}
}
