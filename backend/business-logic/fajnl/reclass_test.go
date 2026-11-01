package fajnl_test

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/fareclass"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// reclassLine: an FA reclass. journal line moving a share of FA0040's TAX amounts to FA0050
func reclassLine(t *testing.T, src ftables.Source, lineNo int, set func(l *gtables.FAReclassJournalLine)) *gtables.FAReclassJournalLine {
	t.Helper()
	l := gtables.NewFAReclassJournalLine(src)
	l.Journal_template_name = "RECLASSIFY"
	l.Journal_batch_name = "DEFAULT"
	l.Line_no = lineNo
	must(t, l.Validate("fa_no", types.Code("FA0040")))
	must(t, l.Validate("new_fa_no", types.Code("FA0050")))
	l.Depreciation_book_code = "TAX"
	l.Fa_posting_date = date("2026-03-01")
	l.Document_no = "RECL-1"
	set(l)
	must(t, l.InsertOrError(true))
	return l
}

// FA Reclass. Journal: 40 % of FA0040's acquisition cost and write-down in the TAX book move to
// FA0050 as FA journal lines (FA Reclassification Entry) — out of the old asset write-down first,
// acquisition last, into the new one acquisition first; the reclass. lines are deleted. Posted,
// the old asset keeps 60 % and the new one has 40 %.
func TestFAReclassJournal(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	batch := taxJournalSetup(t, src)
	b := gtables.NewFADepreciationBook(src)
	b.Fa_no = "FA0050"
	b.Depreciation_book_code = "TAX"
	must(t, b.InsertOrError(true))
	must(t, b.Validate("fa_posting_group", types.Code("OFFICE")))
	must(t, b.ModifyOrError(true))

	l := reclassLine(t, src, 10000, func(l *gtables.FAReclassJournalLine) {
		must(t, l.Validate("reclassify_acq_cost_percent", dec("40")))
		l.Reclassify_acquisition_cost = true
		l.Reclassify_write_down = true
	})
	if l.Description != "Pallet racking" {
		t.Errorf("description %q", l.Description)
	}
	result, err := fareclass.TransferBatch(src, l)
	must(t, err)
	if !result.Done || result.JournalBatchName != "DEFAULT" {
		t.Errorf("result %+v", result)
	}
	got := faJnlLines(src)
	want := []string{"FA0040 TAX Write-Down 40", "FA0040 TAX Acquisition Cost -400", "FA0050 TAX Acquisition Cost 400", "FA0050 TAX Write-Down -40"}
	if len(got) != len(want) {
		t.Fatalf("journal lines %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: %s, want %s", i, got[i], want[i])
		}
	}
	lines := gtables.NewFAJournalLine(src)
	lines.SetFilter("fa_no", "<>''")
	for found := lines.FindSet(); found; found = lines.Next() {
		if !lines.Fa_reclassification_entry || lines.Document_no != "RECL-1" {
			t.Errorf("line %d: reclassification %v, document %s", lines.Line_no, lines.Fa_reclassification_entry, lines.Document_no)
		}
	}
	remaining := gtables.NewFAReclassJournalLine(src)
	remaining.SetFilter("fa_no", "<>''")
	if !remaining.IsEmpty() {
		t.Error("the reclass. lines were not deleted")
	}

	_, err = postBatch(t, src, batch)
	must(t, err)
	old := faDeprBook(t, src)
	wantDec(t, "FA0040 acquisition cost", old.Acquisition_cost, "600")
	wantDec(t, "FA0040 write-down", old.Write_down, "-60")
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0050", "depreciation_book_code": "TAX"}))
	b.CalcFields("acquisition_cost", "write_down")
	wantDec(t, "FA0050 acquisition cost", b.Acquisition_cost, "400")
	wantDec(t, "FA0050 write-down", b.Write_down, "-40")
}

// The reclassification refuses the same asset twice and an amount above the acquisition cost
func TestFAReclassJournalChecks(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	taxJournalSetup(t, src)
	b := gtables.NewFADepreciationBook(src)
	b.Fa_no = "FA0050"
	b.Depreciation_book_code = "TAX"
	must(t, b.InsertOrError(true))

	same := reclassLine(t, src, 10000, func(l *gtables.FAReclassJournalLine) {
		l.New_fa_no = "FA0040"
		l.Reclassify_acquisition_cost = true
		must(t, l.Validate("reclassify_acq_cost_percent", dec("10")))
	})
	if _, err := fareclass.TransferBatch(src, same); err == nil {
		t.Error("FA No. = New FA No. was reclassified")
	}
	must(t, same.DeleteOrError(true))
	tooLarge := reclassLine(t, src, 20000, func(l *gtables.FAReclassJournalLine) {
		l.Reclassify_acquisition_cost = true
		must(t, l.Validate("reclassify_acq_cost_amount", dec("1500")))
	})
	if _, err := fareclass.TransferBatch(src, tooLarge); err == nil {
		t.Error("more than the acquisition cost was reclassified")
	}
}

// In the COMPANY book (acquisition cost and depreciation posted to the G/L) the reclassification
// goes into the general journal of the FA Journal Setup; with Insert Bal. Account each line gets
// its balancing G/L line. Posted, the journal balances.
func TestFAReclassJournalGL(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	tpl := gtables.NewGenJournalTemplate(src)
	tpl.Name = "ASSETS"
	must(t, tpl.Validate("type", gtables.GenJournalTemplate_Type.Assets))
	must(t, tpl.InsertOrError(true))
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "ASSETS"
	batch.Name = "DEFAULT"
	must(t, batch.InsertOrError(true))
	setup := gtables.NewFAJournalSetup(src)
	setup.Depreciation_book_code = "COMPANY"
	setup.Gen_jnl_template_name = "ASSETS"
	setup.Gen_jnl_batch_name = "DEFAULT"
	must(t, setup.InsertOrError(true))

	l := reclassLine(t, src, 10000, func(l *gtables.FAReclassJournalLine) {
		l.Depreciation_book_code = "COMPANY"
		must(t, l.Validate("reclassify_acq_cost_percent", dec("25")))
		l.Reclassify_acquisition_cost = true
		l.Reclassify_depreciation = true
		l.Insert_bal_account = true
	})
	_, err := fareclass.TransferBatch(src, l)
	must(t, err)
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	var fa, gl int
	total := types.Decimal{}
	for found := lines.FindSet(); found; found = lines.Next() {
		if lines.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
			fa++
			if !lines.Fa_reclassification_entry {
				t.Errorf("line %d is not a reclassification entry", lines.Line_no)
			}
		} else {
			gl++
		}
		total = total.Add(lines.Amount)
	}
	if fa != 4 || gl != 4 || !total.IsZero() {
		t.Fatalf("%d fixed asset lines, %d G/L lines, total %s; want 4, 4, 0", fa, gl, total)
	}
	lines.FindFirst()
	_, err = genjnl.NewGenJnlPostBatch(src).Run(lines)
	must(t, err)
}
