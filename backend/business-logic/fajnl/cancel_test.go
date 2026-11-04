package fajnl_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxJournalSetup: the TAX book's batch functions fill the FA journal ASSETS/DEFAULT; FA0040
// acquired for 1,000 and written down by 100 in it
func taxJournalSetup(t *testing.T, src ftables.Source) *gtables.FAJournalBatch {
	t.Helper()
	setup := gtables.NewFAJournalSetup(src)
	setup.Depreciation_book_code = "TAX"
	setup.Fa_jnl_template_name = "ASSETS"
	setup.Fa_jnl_batch_name = "DEFAULT"
	must(t, setup.InsertOrError(true))
	_, batch := journal(t, src)
	addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
	addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.WriteDown, "2026-02-01", "-100")
	_, err := postBatch(t, src, batch)
	must(t, err)
	return batch
}

func faJnlLines(src ftables.Source) []string { // the empty line posting leaves is skipped
	lines := gtables.NewFAJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	lines.SetFilter("fa_no", "<>''")
	var got []string
	for found := lines.FindSet(); found; found = lines.Next() {
		got = append(got, lines.Fa_no.String()+" "+lines.Depreciation_book_code.String()+" "+lines.Fa_posting_type.String()+" "+lines.Amount.String())
	}
	return got
}

// Cancel Entries (codeunit 5624): the entries as FA journal lines with the opposite amounts and
// FA Error Entry No., last first; posted, the asset's book is back to zero and the entries are
// canceled — cancelling them again is refused
func TestCancelFAEntries(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	batch := taxJournalSetup(t, src)
	entries := gtables.NewFALedgerEntry(src)
	entries.SetRange("fa_no", types.Code("FA0040"))
	entries.SetRange("depreciation_book_code", types.Code("TAX"))
	var entryNos []int
	for found := entries.FindSet(); found; found = entries.Next() {
		entryNos = append(entryNos, entries.Entry_no)
	}
	must(t, fajnl.TransferLine(entries, false))
	got := faJnlLines(src)
	if len(got) != 2 || got[0] != "FA0040 TAX Write-Down 100" || got[1] != "FA0040 TAX Acquisition Cost -1000" {
		t.Fatalf("cancel lines %v", got)
	}
	lines := gtables.NewFAJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	lines.SetFilter("fa_no", "<>''")
	lines.FindFirst()
	if lines.Document_no != "" {
		t.Errorf("Document No. %q, want blank (no number series)", lines.Document_no)
	}
	if lines.Fa_error_entry_no != entryNos[1] {
		t.Errorf("FA Error Entry No. %d, want %d", lines.Fa_error_entry_no, entryNos[1])
	}
	// the batch has no number series: the user enters the document no.
	for found := lines.FindSet(); found; found = lines.Next() {
		lines.Document_no = "CANCEL-1"
		must(t, lines.ModifyOrError(true))
	}
	_, err := postBatch(t, src, batch)
	must(t, err)
	b := faDeprBook(t, src)
	wantDec(t, "acquisition cost after cancelling", b.Acquisition_cost, "0")
	wantDec(t, "book value after cancelling", b.Book_value, "0")

	again := gtables.NewFALedgerEntry(src)
	again.SetRange("entry_no", entryNos[0])
	if err := fajnl.TransferLine(again, false); err == nil {
		t.Error("a canceled entry was canceled again")
	}
}

// Report 5688 Cancel FA Ledger Entries with only Write-Down chosen: one line cancelling the
// write-down, with the report's document no. and description
func TestCancelFALedgerEntriesReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	taxJournalSetup(t, src)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"))
	_, err := reports.Execute(reports.CancelFALedgerEntriesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Record:  fa,
		Options: map[string]string{"cancel_book": "TAX", "write_down": "true", "document_no": "CANCEL-1", "posting_description": "Wrong write-down"},
	})
	must(t, err)
	got := faJnlLines(src)
	if len(got) != 1 || got[0] != "FA0040 TAX Write-Down 100" {
		t.Fatalf("cancel lines %v", got)
	}
	lines := gtables.NewFAJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	lines.SetFilter("fa_no", "<>''")
	lines.FindFirst()
	if lines.Document_no != "CANCEL-1" || lines.Description != "Wrong write-down" || lines.Fa_error_entry_no == 0 {
		t.Errorf("line %s %q error entry %d", lines.Document_no, lines.Description, lines.Fa_error_entry_no)
	}
}

// Report 5687 Copy Depreciation Book: the TAX book's acquisition and write-down as lines of a new
// book without G/L integration; the asset gets the new book from its TAX book
func TestCopyDepreciationBookReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	taxJournalSetup(t, src)
	book := gtables.NewDepreciationBook(src)
	book.Code = "IFRS"
	book.Description = "IFRS"
	must(t, book.InsertOrError(true))
	setup := gtables.NewFAJournalSetup(src)
	setup.Depreciation_book_code = "IFRS"
	setup.Fa_jnl_template_name = "ASSETS"
	setup.Fa_jnl_batch_name = "DEFAULT"
	must(t, setup.InsertOrError(true))
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FA0040"))
	_, err := reports.Execute(reports.CopyDepreciationBookID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Record: fa,
		Options: map[string]string{"copy_from_book": "TAX", "copy_to_book": "IFRS", "acquisition_cost": "true", "write_down": "true",
			"document_no": "COPY-1"},
	})
	must(t, err)
	got := faJnlLines(src)
	if len(got) != 2 || got[0] != "FA0040 IFRS Acquisition Cost 1000" || got[1] != "FA0040 IFRS Write-Down -100" {
		t.Fatalf("copied lines %v", got)
	}
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "IFRS"}))
}

// Report 5689 Create FA Depreciation Books: every asset in the filter gets the book — a copy of
// another asset's FA depreciation book in it; one that has it is left alone
func TestCreateFADepreciationBooksReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	tax := gtables.NewFADepreciationBook(src)
	must(t, tax.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "TAX"}))
	book := gtables.NewDepreciationBook(src)
	book.Code = "IFRS"
	book.Description = "IFRS"
	must(t, book.InsertOrError(true))
	template := gtables.NewFADepreciationBook(src)
	template.Fa_no = "FA0040"
	template.Depreciation_book_code = "IFRS"
	must(t, template.InsertOrError(true))
	must(t, template.Validate("fa_posting_group", tax.Fa_posting_group))
	must(t, template.ModifyOrError(true))

	fa := gtables.NewFixedAsset(src)
	fa.SetFilter("no", "FA0010..FA0040")
	_, err := reports.Execute(reports.CreateFADepreciationBooksID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Record:  fa,
		Options: map[string]string{"depreciation_book": "IFRS", "copy_from_fa_no": "FA0040"},
	})
	must(t, err)
	books := gtables.NewFADepreciationBook(src)
	books.SetRange("depreciation_book_code", types.Code("IFRS"))
	if books.Count() < 2 {
		t.Fatalf("%d assets have the IFRS book", books.Count())
	}
	for found := books.FindSet(); found; found = books.Next() {
		if books.Fa_posting_group != tax.Fa_posting_group {
			t.Errorf("%s: FA posting group %q, want %q (copied)", books.Fa_no, books.Fa_posting_group, tax.Fa_posting_group)
		}
	}
}
