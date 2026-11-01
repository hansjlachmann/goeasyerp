package fajnl_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A write-down posted in the TAX book with Duplicate in Depreciation Book COMPANY: the same line
// in COMPANY's general journal (the book posts write-downs to the G/L), not posted
func TestDuplicateInDeprBook(t *testing.T) {
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

	_, faBatch := journal(t, src)
	addLine(t, faBatch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
	wd := addLine(t, faBatch, 20000, gtables.FAJournalLine_Fa_posting_type.WriteDown, "2026-02-01", "-100")
	must(t, wd.Validate("duplicate_in_depreciation_book", types.Code("COMPANY")))
	must(t, wd.ModifyOrError(true))
	_, err := postBatch(t, src, faBatch)
	must(t, err)
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("ASSETS"))
	if !lines.FindFirst() || lines.Account_no != "FA0040" || lines.Depreciation_book_code != "COMPANY" ||
		lines.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.WriteDown || !lines.Amount.Equal(dec("-100")) {
		t.Errorf("duplicated line: %s %s %v %s", lines.Account_no, lines.Depreciation_book_code, lines.Fa_posting_type, lines.Amount)
	}
	if lines.Count() != 1 {
		t.Errorf("%d duplicated lines", lines.Count())
	}
}
