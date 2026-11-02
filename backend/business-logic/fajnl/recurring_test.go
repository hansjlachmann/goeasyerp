package fajnl_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A recurring FA journal (the demo's RECURRING template, page 5634): a fixed and a variable line
// post with the month in the document no.; they stay a month later by FA Posting Date — the
// variable one without its amount
func TestRecurringFAJournal(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	tpl := gtables.NewFAJournalTemplate(src)
	must(t, tpl.GetOrError(types.Code("RECURRING")))
	if !tpl.Recurring || tpl.Page_id != 5634 {
		t.Errorf("template RECURRING: recurring %v, page %d", tpl.Recurring, tpl.Page_id)
	}
	batch := gtables.NewFAJournalBatch(src)
	must(t, batch.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "name": "DEFAULT"}))
	_, acqBatch := journal(t, src)
	addLine(t, acqBatch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "100000")
	_, err := postBatch(t, src, acqBatch)
	must(t, err)

	for _, l := range []struct {
		lineNo int
		method gtables.FAJournalLineRecurring_method
		amount string
	}{{10000, gtables.FAJournalLine_Recurring_method.FFixed, "2000"}, {20000, gtables.FAJournalLine_Recurring_method.VVariable, "500"}} {
		line := addLine(t, batch, l.lineNo, gtables.FAJournalLine_Fa_posting_type.Maintenance, "2026-10-31", l.amount)
		must(t, line.Validate("maintenance_code", types.Code("SERVICE")))
		line.Recurring_method = l.method
		line.Recurring_frequency = "1M"
		line.Document_no = "SVC-%3"
		must(t, line.ModifyOrError(true))
	}
	result, err := postBatch(t, src, batch)
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	entry := gtables.NewMaintenanceLedgerEntry(src)
	entry.SetRange("document_no", types.Code("SVC-10"))
	if entry.Count() != 2 {
		t.Errorf("%d maintenance entries SVC-10, want 2", entry.Count())
	}
	lines := gtables.NewFAJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("RECURRING"))
	for found := lines.FindSetBuffered(); found; found = lines.Next() {
		want := map[int]string{10000: "2000", 20000: "0"}[lines.Line_no]
		if !lines.Fa_posting_date.Equal(date("2026-11-30")) || !lines.Amount.Equal(dec(want)) {
			t.Errorf("line %d: %s amount %s, want 2026-11-30 and %s", lines.Line_no, lines.Fa_posting_date, lines.Amount, want)
		}
	}
}
