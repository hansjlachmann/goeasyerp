package bomjnl_test

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// A recurring BOM journal (the demo's RECURRING template, page 287): a variable assembly posts
// with the month in its document no. and stays a month later without quantity and cost
func TestRecurringBOMJournal(t *testing.T) {
	src := demoCompany(t)
	tpl := gtables.NewBOMJournalTemplate(src)
	must(t, tpl.GetOrError(types.Code("RECURRING")))
	if !tpl.Recurring || tpl.Page_id != 287 {
		t.Errorf("template RECURRING: recurring %v, page %d", tpl.Recurring, tpl.Page_id)
	}
	line := gtables.NewBOMJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RECURRING", "DEFAULT", 10000
	must(t, tables.SetUpNewBOMJnlLine(line, gtables.NewBOMJournalLine(src)))
	must(t, line.Validate("recurring_method", gtables.BOMJournalLine_Recurring_method.Variable))
	line.Recurring_frequency = "1M"
	must(t, line.Validate("posting_date", types.NewDate(2026, 10, 6)))
	line.Document_no = "KIT-%3"
	must(t, line.Validate("item_no", types.Code("2100")))
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.Validate("quantity", dec("2")))
	must(t, line.InsertOrError(true))

	var result bomjnl.PostBatchResult
	inTx(t, src, func(txSrc ftables.Source) error {
		l := gtables.NewBOMJournalLine(txSrc)
		l.Journal_template_name, l.Journal_batch_name = "RECURRING", "DEFAULT"
		var err error
		result, err = bomjnl.NewBOMJnlPostBatch(txSrc).Run(l)
		return err
	})
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	// (BOM ledger entries have no document no.: the item entries of the assembly carry it)
	entry := gtables.NewItemLedgerEntry(src)
	entry.SetRange("document_no", types.Code("KIT-10"))
	if entry.Count() == 0 {
		t.Error("no item ledger entries KIT-10")
	}
	must(t, line.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "line_no": 10000}))
	if !line.Posting_date.Equal(types.NewDate(2026, 11, 6)) || !line.Quantity.IsZero() || !line.Total_cost.IsZero() {
		t.Errorf("line after posting: %s qty %s cost %s", line.Posting_date, line.Quantity, line.Total_cost)
	}
}
