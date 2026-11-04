package fajnl_test

import (
	"bytes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// printFA prints a fixed asset report (PDF) and returns its texts, | between them
func printFA(t *testing.T, src ftables.Source, id int, record ftables.Table, options map[string]string) string {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Record: record, Options: options})
	must(t, err)
	if !bytes.HasPrefix(result.PDF, []byte("%PDF-")) {
		t.Fatal("no PDF")
	}
	if dir := os.Getenv("FA_PDF_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, strconv.Itoa(id)+".pdf"), result.PDF, 0o644)
	}
	var texts []string
	for _, text := range result.Texts {
		texts = append(texts, text.Text)
	}
	return "|" + strings.Join(texts, "|") + "|"
}

func wantTexts(t *testing.T, texts string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(texts, "|"+w+"|") {
			t.Errorf("%q not printed in %s", w, texts)
		}
	}
}

// Report 5603 Fixed Asset Register: the register of a TAX journal posting with its entries,
// the asset's description and the total
func TestFixedAssetRegisterReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	taxJournalSetup(t, src)
	faReg := gtables.NewFARegister(src)
	if !faReg.FindLast() {
		t.Fatal("no FA register")
	}
	reg := gtables.NewFARegister(src)
	reg.SetRange("no", faReg.No)
	texts := printFA(t, src, reports.FixedAssetRegisterID, reg, nil)
	wantTexts(t, texts, "Fixed Asset Register", "Register No.", "FA0040", "Pallet racking", "Write-Down", "1,000.00", "-100.00", "900.00")
}

// Report 5604 Fixed Asset - Details: FA0040's entries in the COMPANY book (the demo's acquisition
// and depreciations); an asset without entries in the book is not printed
func TestFixedAssetDetailsReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"), types.Code("FA0090"))
	texts := printFA(t, src, reports.FixedAssetDetailsID, fa, map[string]string{"depreciation_book": "COMPANY"})
	wantTexts(t, texts, "Fixed Asset - Details", "Depreciation Book: COMPANY", "FA0040", "Pallet racking", "Acquisition Cost", "Depreciation")
	if strings.Contains(texts, "|FA0090|") {
		t.Error("FA0090 has no entries and was printed")
	}
}

// Report 5605 Fixed Asset - Book Value 01 per asset with group totals by FA class: FA0040's line
// with its book value at the ending date (the FA depreciation book's), the group total and the total
func TestFixedAssetBookValue01Report(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "COMPANY"}))
	must(t, b.SetFlowFilter("fa_posting_date_filter", "..2026-06-30"))
	b.CalcFields("book_value")
	bookValue := formatted(b.Book_value)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"))
	texts := printFA(t, src, reports.FixedAssetBookValue01ID, fa, map[string]string{"depreciation_book": "COMPANY",
		"starting_date": "2026-01-01", "ending_date": "2026-06-30", "group_totals": "2", "print_per_fixed_asset": "true"})
	wantTexts(t, texts, "Fixed Asset - Book Value 01", "FA0040", "Pallet racking", "Total", bookValue)
	wantTexts(t, texts, "Group Totals: FA Class Code", "Group Total: TANGIBLE", "Acquisition", "12/31/2025", "06/30/2026")
}

// Report 5606 Fixed Asset - Book Value 02: the posting types of FA0040 one under the other and
// its book value at the ending date
func TestFixedAssetBookValue02Report(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "COMPANY"}))
	must(t, b.SetFlowFilter("fa_posting_date_filter", "..2026-06-30"))
	b.CalcFields("book_value")
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"))
	texts := printFA(t, src, reports.FixedAssetBookValue02ID, fa, map[string]string{"depreciation_book": "COMPANY",
		"starting_date": "2026-01-01", "ending_date": "2026-06-30", "print_per_fixed_asset": "true", "include_reclassification": "true"})
	wantTexts(t, texts, "Fixed Asset - Book Value 02", "FA0040", "Acquisition Cost", "Depreciation", "Book Value", "Reclassification", formatted(b.Book_value))
}

// formatted: a decimal as the printouts show it (en-US, 2 decimals)
func formatted(d types.Decimal) string {
	return format.Decimal(d, "2:2", "en-US")
}
