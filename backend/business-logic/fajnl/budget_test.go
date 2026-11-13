package fajnl_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// budgetedAsset: FAB01, a budgeted asset in the TAX book (straight-line over 5 years from
// 2026-01-01) with a budgeted acquisition of 60,000 on 2026-01-01
func budgetedAsset(t *testing.T, src ftables.Source, batch *gtables.FAJournalBatch) {
	t.Helper()
	fa := gtables.NewFixedAsset(src)
	fa.No = "FAB01"
	fa.Description = "Budget: new forklift"
	must(t, fa.InsertOrError(true))
	must(t, fa.Validate("budgeted_asset", true))
	must(t, fa.ModifyOrError(true))
	faDeprBook := gtables.NewFADepreciationBook(src)
	faDeprBook.Fa_no = "FAB01"
	faDeprBook.Depreciation_book_code = "TAX"
	must(t, faDeprBook.InsertOrError(true))
	must(t, faDeprBook.Validate("fa_posting_group", types.Code("EQUIPMENT")))
	must(t, faDeprBook.Validate("depreciation_starting_date", date("2026-01-01")))
	must(t, faDeprBook.Validate("no_of_depreciation_years", dec("5")))
	must(t, faDeprBook.ModifyOrError(true))

	line := gtables.NewFAJournalLine(batch)
	line.Journal_template_name = batch.Journal_template_name
	line.Journal_batch_name = batch.Name
	line.Line_no = 90000
	line.Depreciation_book_code = "TAX"
	must(t, line.Validate("fa_no", types.Code("FAB01")))
	must(t, line.Validate("fa_posting_type", gtables.FAJournalLine_Fa_posting_type.AcquisitionCost))
	line.Fa_posting_date = date("2026-01-01")
	line.Document_no = "BUDGET-1"
	must(t, line.Validate("amount", dec("60000")))
	must(t, line.InsertOrError(true))
	_, err := postBatch(t, src, batch)
	must(t, err)
}

func budgetBook(t *testing.T, src ftables.Source) *gtables.FADepreciationBook {
	t.Helper()
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FAB01", "depreciation_book_code": "TAX"}))
	b.CalcFields("acquisition_cost", "salvage_value", "book_value", "maintenance")
	return b
}

// A posting with Budgeted FA No. marks its entry with the budgeted asset and posts its amount —
// and salvage value — reversed to the budgeted asset (an automatic entry naming the asset); a
// maintenance the same on the maintenance ledger
func TestPostWithBudgetedAsset(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	budgetedAsset(t, src, batch)
	wantDec(t, "budgeted acquisition", budgetBook(t, src).Acquisition_cost, "60000")
	// the budget's salvage value (a budgeted asset's salvage value must not become positive)
	salvage := addLine(t, batch, 40000, gtables.FAJournalLine_Fa_posting_type.SalvageValue, "2026-01-01", "-6000")
	must(t, salvage.Validate("fa_no", types.Code("FAB01")))
	must(t, salvage.Validate("amount", dec("-6000")))
	must(t, salvage.ModifyOrError(true))
	_, err := postBatch(t, src, batch)
	must(t, err)

	acq := addLine(t, batch, 50000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-03-01", "45000")
	acq.Salvage_value = dec("-5000")
	must(t, acq.Validate("budgeted_fa_no", types.Code("FAB01")))
	must(t, acq.ModifyOrError(true))
	maint := addLine(t, batch, 60000, gtables.FAJournalLine_Fa_posting_type.Maintenance, "2026-03-15", "1200")
	must(t, maint.Validate("maintenance_code", types.Code("SERVICE")))
	must(t, maint.Validate("budgeted_fa_no", types.Code("FAB01")))
	must(t, maint.ModifyOrError(true))
	_, err = postBatch(t, src, batch)
	must(t, err)

	b := budgetBook(t, src)
	wantDec(t, "budget left: acquisition cost", b.Acquisition_cost, "15000")
	wantDec(t, "budget: salvage value left", b.Salvage_value, "-1000")
	wantDec(t, "budget: maintenance", b.Maintenance, "-1200")
	wantDec(t, "the asset's acquisition cost", faDeprBook(t, src).Acquisition_cost, "45000")

	posted := gtables.NewFALedgerEntry(src)
	posted.SetRange("fa_no", types.Code("FA0040"))
	posted.SetRange("depreciation_book_code", types.Code("TAX"))
	posted.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost)
	if !posted.FindFirst() || posted.Fa_no_budgeted_fa_no != "FAB01" || posted.Automatic_entry {
		t.Errorf("the asset's entry: budgeted FA %q automatic %v", posted.Fa_no_budgeted_fa_no, posted.Automatic_entry)
	}
	budget := gtables.NewFALedgerEntry(src)
	budget.SetRange("fa_no", types.Code("FAB01"))
	budget.SetRange("fa_no_budgeted_fa_no", types.Code("FA0040"))
	if budget.Count() != 2 {
		t.Errorf("%d budget entries from FA0040, want 2 (salvage value, acquisition cost)", budget.Count())
	}
	for found := budget.FindSet(); found; found = budget.Next() {
		if !budget.Automatic_entry || budget.G_l_entry_no != 0 || budget.Document_no != "FA-TAX-1" {
			t.Errorf("budget entry %d: automatic %v G/L %d document %s", budget.Entry_no, budget.Automatic_entry, budget.G_l_entry_no, budget.Document_no)
		}
	}
	maintenance := gtables.NewMaintenanceLedgerEntry(src)
	maintenance.SetRange("fa_no", types.Code("FAB01"))
	if !maintenance.FindFirst() || maintenance.Fa_no_budgeted_fa_no != "FA0040" || !maintenance.Amount.Equal(dec("-1200")) || !maintenance.Automatic_entry {
		t.Errorf("budget maintenance entry: from %q amount %s automatic %v", maintenance.Fa_no_budgeted_fa_no, maintenance.Amount, maintenance.Automatic_entry)
	}
}

// The checks of Budgeted FA No.: it names a budgeted asset (the field's trigger and the posting),
// not on a disposal, not on a budgeted asset's own line; a budgeted asset is posted in the FA
// journal even in a book that posts to the G/L, and never in the general journal
func TestBudgetedAssetChecks(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	budgetedAsset(t, src, batch)

	line := addLine(t, batch, 50000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-03-01", "1000")
	if err := line.Validate("budgeted_fa_no", types.Code("FA0050")); err == nil {
		t.Error("Budgeted FA No. took an asset that is not budgeted")
	}
	line.Budgeted_fa_no = "FA0050" // as an import would
	must(t, line.ModifyOrError(false))
	if _, err := postBatch(t, src, batch); err == nil || !strings.Contains(err.Error(), "not a budgeted asset") {
		t.Errorf("posting to a budget that is not a budgeted asset: %v", err)
	}
	must(t, line.DeleteOrError(true))

	var check fajnl.FAJnlCheckLine
	disposal := addLine(t, batch, 60000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-03-01", "1000")
	disposal.Fa_posting_type = gtables.FAJournalLine_Fa_posting_type.Disposal
	disposal.Budgeted_fa_no = "FAB01"
	if err := check.CheckFAJnlLine(disposal); err == nil {
		t.Error("a disposal with Budgeted FA No. passed the check")
	}
	own := gtables.NewFAJournalLine(batch)
	own.Copy(disposal)
	own.Fa_no = "FAB01"
	own.Fa_posting_type = gtables.FAJournalLine_Fa_posting_type.AcquisitionCost
	if err := check.CheckFAJnlLine(own); err == nil {
		t.Error("a budgeted asset's line with Budgeted FA No. passed the check")
	}

	// the COMPANY book posts acquisitions to the G/L: a budgeted asset still goes through the FA journal
	companyBook := gtables.NewFADepreciationBook(src)
	companyBook.Fa_no = "FAB01"
	companyBook.Depreciation_book_code = "COMPANY"
	must(t, companyBook.InsertOrError(true))
	must(t, companyBook.Validate("fa_posting_group", types.Code("EQUIPMENT")))
	must(t, companyBook.ModifyOrError(true))
	own.Depreciation_book_code = "COMPANY"
	own.Budgeted_fa_no = ""
	if err := check.CheckFAJnlLine(own); err != nil {
		t.Errorf("a budgeted asset in a book with G/L integration: %v", err)
	}
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.FixedAsset
	genJnlLine.Account_no = "FAB01"
	genJnlLine.Depreciation_book_code = "COMPANY"
	genJnlLine.Fa_posting_type = gtables.GenJournalLine_Fa_posting_type.AcquisitionCost
	genJnlLine.Posting_date = date("2026-03-01")
	genJnlLine.Bal_account_type = gtables.GenJournalLine_Bal_account_type.GLAccount
	if err := check.CheckGenJnlLine(genJnlLine); err == nil || !strings.Contains(err.Error(), "budgeted asset") {
		t.Errorf("a budgeted asset in the general journal: %v", err)
	}
	if err := genJnlLine.Validate("account_no", types.Code("FAB01")); err == nil {
		t.Error("the general journal took a budgeted asset as account")
	}
}

// Report 5605 as a budget report: the budgeted asset's depreciation to come (codeunit 5615) is in
// the depreciation and the book value — 60,000 over 5 years is 6,000 in the first half year
func TestFixedAssetBookValueBudgetReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	budgetedAsset(t, src, batch)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FAB01"))
	options := map[string]string{"depreciation_book": "TAX", "starting_date": "2026-01-01", "ending_date": "2026-06-30", "print_per_fixed_asset": "true"}
	texts := printFA(t, src, reports.FixedAssetBookValue01ID, fa, options)
	wantTexts(t, texts, "Fixed Asset - Book Value 01", "FAB01", "60,000.00")
	if strings.Contains(texts, "|54,000.00|") {
		t.Errorf("the book value without budget report includes the depreciation to come: %s", texts)
	}
	options["budget_report"] = "true"
	texts = printFA(t, src, reports.FixedAssetBookValue01ID, fa, options)
	wantTexts(t, texts, "Fixed Asset - Book Value 01 (Budget Report)", "FAB01", "-6,000.00", "54,000.00")
}

// Report 5607 Fixed Asset - Projected Value: the budgeted asset depreciated quarter by quarter
// (90 days from the First Depreciation Date, the first period up to it) — 3,000 a quarter, the
// book value after each —, the asset's total and the total;
// with Projected Disposal the projection ends on the projected disposal date with the projected
// proceeds and gain; the amounts per date
func TestFixedAssetProjectedValueReport(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	budgetedAsset(t, src, batch)
	fa := gtables.NewFixedAsset(src)
	fa.SetRange("no", types.Code("FAB01"))
	options := map[string]string{"depreciation_book": "TAX", "first_depreciation_date": "2026-03-31", "last_depreciation_date": "2026-12-31",
		"number_of_days": "90", "print_per_fixed_asset": "true", "print_amounts_per_date": "true", "posted_entries_from": "2026-01-01"}
	texts := printFA(t, src, reports.FixedAssetProjectedValueID, fa, options)
	wantTexts(t, texts, "Fixed Asset - Projected Value", "FAB01", "Budget: new forklift", "Posted Entry", "60,000.00",
		"-3,000.00", "57,000.00", "54,000.00", "51,000.00", "48,000.00", "-12,000.00", "Total", "Projected Amounts per Date")
	if n := strings.Count(texts, "|-3,000.00|"); n != 8 { // four quarters, and again per date
		t.Errorf("-3,000.00 printed %d times, want 8: %s", n, texts)
	}

	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FAB01", "depreciation_book_code": "TAX"}))
	b.Projected_disposal_date = date("2026-06-30")
	b.Projected_proceeds_on_disposal = dec("55000")
	must(t, b.ModifyOrError(false))
	options["projected_disposal"] = "true"
	options["print_amounts_per_date"] = "false"
	texts = printFA(t, src, reports.FixedAssetProjectedValueID, fa, options)
	// two quarters: book value 54,000; proceeds 55,000 give a gain of 1,000 (negative)
	wantTexts(t, texts, "54,000.00", "-6,000.00", "55,000.00", "-1,000.00", "Projected Gain/Loss")
	if strings.Contains(texts, "|51,000.00|") {
		t.Errorf("depreciated past the projected disposal date: %s", texts)
	}
}
