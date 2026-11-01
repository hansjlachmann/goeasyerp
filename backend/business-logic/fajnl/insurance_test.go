package fajnl_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/insurancejnl"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// insuranceLine: a line of the demo's insurance journal INSURANCE/DEFAULT covering an asset by
// INS010
func insuranceLine(t *testing.T, src ftables.Source, lineNo int, faNo, amount string) *gtables.InsuranceJournalLine {
	t.Helper()
	l := gtables.NewInsuranceJournalLine(src)
	l.Journal_template_name = "INSURANCE"
	l.Journal_batch_name = "DEFAULT"
	l.Line_no = lineNo
	must(t, l.Validate("insurance_no", types.Code("INS010")))
	must(t, l.Validate("fa_no", types.Code(faNo)))
	l.Posting_date = date("2026-04-01")
	l.Document_no = "INS-1"
	must(t, l.Validate("amount", dec(amount)))
	must(t, l.InsertOrError(true))
	return l
}

func coverageEntries(src ftables.Source, faNo string) (int, types.Decimal) {
	entries := gtables.NewInsCoverageLedgerEntry(src)
	entries.SetRange("fa_no", types.Code(faNo))
	if entries.CalcSums("amount") != nil {
		return 0, types.Decimal{}
	}
	return entries.Count(), entries.Amount
}

// Insurance journal: two lines posted as insurance coverage ledger entries in one insurance
// register (the template's source code, the insurance's class and locations); the lines are
// deleted and an empty line is left
func TestPostInsuranceJournal(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	countBefore, _ := coverageEntries(src, "FA0040")
	first := insuranceLine(t, src, 10000, "FA0040", "50000")
	if first.Description == "" || first.Fa_description != "Pallet racking" || first.Source_code != "INSJNL" {
		t.Errorf("line from the insurance and asset: %q %q %q", first.Description, first.Fa_description, first.Source_code)
	}
	insuranceLine(t, src, 20000, "FA0040", "25000")
	result, err := insurancejnl.PostBatch(src, first)
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	count, _ := coverageEntries(src, "FA0040")
	if count != countBefore+2 {
		t.Errorf("%d coverage entries, want %d", count, countBefore+2)
	}
	reg := gtables.NewInsuranceRegister(src)
	must(t, reg.GetOrError(result.RegisterNo))
	if reg.To_entry_no-reg.From_entry_no != 1 || reg.Source_code != "INSJNL" || reg.Journal_batch_name != "DEFAULT" {
		t.Errorf("register %d..%d %s %s", reg.From_entry_no, reg.To_entry_no, reg.Source_code, reg.Journal_batch_name)
	}
	entry := gtables.NewInsCoverageLedgerEntry(src)
	must(t, entry.GetOrError(reg.To_entry_no))
	if !entry.Amount.Equal(dec("25000")) || entry.Document_no != "INS-1" || entry.Source_code != "INSJNL" || entry.Document_date.IsZero() {
		t.Errorf("entry %s %s %s %s", entry.Amount, entry.Document_no, entry.Source_code, entry.Document_date)
	}
	lines := gtables.NewInsuranceJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("INSURANCE"))
	if lines.Count() != 1 {
		t.Errorf("%d lines left, want the empty one", lines.Count())
	}
}

// An acquisition posted with an Insurance No. in the insurance book (COMPANY): with the demo's
// Automatic Insurance Posting the coverage entry is posted at once; without it a line goes into
// the book's insurance journal (FA Journal Setup)
func TestAcquisitionWithInsurance(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	newAsset(t, src)
	line := faLine(t, src, 10000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "60000", "1520")
	line.Insurance_no = "INS010"
	must(t, genjnl.NewGenJnlPostLine(src).Run(line))
	count, amount := coverageEntries(src, "FA0090")
	if count != 1 || !amount.Equal(dec("60000")) {
		t.Errorf("automatic insurance posting: %d entries, %s", count, amount)
	}

	faSetup := gtables.GetFASetup(src)
	faSetup.Automatic_insurance_posting = false
	must(t, faSetup.ModifyOrError(false))
	setup := gtables.NewFAJournalSetup(src)
	setup.Depreciation_book_code = "COMPANY"
	setup.Insurance_jnl_template_name = "INSURANCE"
	setup.Insurance_jnl_batch_name = "DEFAULT"
	must(t, setup.InsertOrError(true))
	line = faLine(t, src, 20000, gtables.GenJournalLine_Fa_posting_type.AcquisitionCost, "4000", "1520")
	line.Insurance_no = "INS010"
	must(t, genjnl.NewGenJnlPostLine(src).Run(line))
	count, _ = coverageEntries(src, "FA0090")
	if count != 1 {
		t.Errorf("%d coverage entries, want 1 (the journal line is not posted)", count)
	}
	lines := gtables.NewInsuranceJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("INSURANCE"))
	if !lines.FindFirst() || lines.Fa_no != "FA0090" || !lines.Amount.Equal(dec("4000")) || lines.Insurance_no != "INS010" || lines.Source_code != "INSJNL" {
		t.Errorf("insurance journal line %s %s %s %s", lines.Fa_no, lines.Amount, lines.Insurance_no, lines.Source_code)
	}
}
