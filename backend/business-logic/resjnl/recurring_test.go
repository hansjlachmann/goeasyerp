package resjnl_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A recurring resource journal (the demo's RECURRING template, page 290): a variable usage line
// posts with the week in its document no. and stays a week later without its quantity and
// amounts
func TestRecurringResourceJournal(t *testing.T) {
	src := demoCompany(t)
	tpl := gtables.NewResJournalTemplate(src)
	must(t, tpl.GetOrError(types.Code("RECURRING")))
	if !tpl.Recurring || tpl.Page_id != 290 {
		t.Errorf("template RECURRING: recurring %v, page %d", tpl.Recurring, tpl.Page_id)
	}
	line := gtables.NewResJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RECURRING", "DEFAULT", 20000
	must(t, line.Validate("recurring_method", gtables.ResJournalLine_Recurring_method.Variable))
	line.Recurring_frequency = "1W"
	must(t, line.Validate("posting_date", types.NewDate(2026, 10, 5)))
	line.Document_no = "WEEK-%2"
	must(t, line.Validate("entry_type", gtables.ResJournalLine_Entry_type.Usage))
	must(t, line.Validate("resource_no", types.Code("R0010")))
	must(t, line.Validate("work_type_code", types.Code("NORMAL")))
	must(t, line.Validate("quantity", dec("3")))
	must(t, line.InsertOrError(true))

	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	post := gtables.NewResJournalLine(txSrc)
	post.Journal_template_name, post.Journal_batch_name = "RECURRING", "DEFAULT"
	result, err := resjnl.NewResJnlPostBatch(txSrc).Run(post)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatal(err)
	}
	must(t, ftables.Commit(tx))
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	entry := gtables.NewResLedgerEntry(src)
	entry.SetRange("document_no", types.Code("WEEK-41"))
	if entry.Count() != 1 {
		t.Errorf("%d entries WEEK-41, want 1", entry.Count())
	}
	must(t, line.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "line_no": 20000}))
	if !line.Posting_date.Equal(types.NewDate(2026, 10, 12)) || !line.Quantity.IsZero() || !line.Total_cost.IsZero() || !line.Total_price.IsZero() {
		t.Errorf("line after posting: %s qty %s cost %s price %s", line.Posting_date, line.Quantity, line.Total_cost, line.Total_price)
	}
}
