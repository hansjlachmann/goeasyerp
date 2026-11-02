package itemjnl

import (
	"testing"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func recurringItemLine(t *testing.T, src ftables.Source, lineNo int, method gtables.ItemJournalLineRecurring_method, qty string) {
	t.Helper()
	line := gtables.NewItemJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RECURRING", "DEFAULT", lineNo
	must(t, line.Validate("recurring_method", method))
	line.Recurring_frequency = "1M"
	must(t, line.Validate("posting_date", date(t, 31, 10, 2026)))
	line.Document_no = "USE-%3"
	must(t, line.Validate("entry_type", gtables.ItemJournalLine_Entry_type.NegativeAdjmt))
	must(t, line.Validate("item_no", types.Code("2000")))
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.Validate("quantity", dec(qty)))
	line.Description = "Kitchen %4"
	must(t, line.InsertOrError(true))
}

func postRecurringItems(t *testing.T, src ftables.Source) (PostBatchResult, error) {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	line := gtables.NewItemJournalLine(txSrc)
	line.Journal_template_name, line.Journal_batch_name = "RECURRING", "DEFAULT"
	result, err := NewItemJnlPostBatch(txSrc).Run(line)
	if err != nil {
		_ = ftables.Rollback(tx)
		return result, err
	}
	return result, ftables.Commit(tx)
}

// A recurring item journal (the demo's RECURRING template, page 286): a fixed and a variable
// line post with the month in the document no. and description, stay a month later — the
// variable one without its quantity —; a variable line needs a quantity; a recurring method in
// an ordinary item journal is refused
func TestRecurringItemJournal(t *testing.T) {
	src := demoCompany(t)
	tpl := gtables.NewItemJournalTemplate(src)
	must(t, tpl.GetOrError(types.Code("RECURRING")))
	if !tpl.Recurring || tpl.Page_id != 286 {
		t.Errorf("template RECURRING: recurring %v, page %d", tpl.Recurring, tpl.Page_id)
	}
	recurringItemLine(t, src, 10000, gtables.ItemJournalLine_Recurring_method.Fixed, "2")
	recurringItemLine(t, src, 20000, gtables.ItemJournalLine_Recurring_method.Variable, "1")
	result, err := postRecurringItems(t, src)
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	posted := entries(src, "2000")
	last := posted[len(posted)-1]
	if last.Document_no != "USE-10" || last.Description != "Kitchen October" || !last.Quantity.Equal(dec("-1")) {
		t.Errorf("entry %s %q %s", last.Document_no, last.Description, last.Quantity)
	}
	lines := gtables.NewItemJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("RECURRING"))
	if lines.Count() != 2 {
		t.Fatalf("%d lines left, want both", lines.Count())
	}
	for found := lines.FindSet(); found; found = lines.Next() {
		want := map[int]string{10000: "2", 20000: "0"}[lines.Line_no]
		if !lines.Posting_date.Equal(date(t, 30, 11, 2026)) || !lines.Quantity.Equal(dec(want)) || lines.Document_no != "USE-%3" {
			t.Errorf("line %d: %s qty %s doc %s, want 2026-11-30, %s, USE-%%3", lines.Line_no, lines.Posting_date, lines.Quantity, lines.Document_no, want)
		}
	}
	// the variable line has no quantity now: posted on the next date it is refused
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	variable := gtables.NewItemJournalLine(txSrc)
	must(t, variable.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "line_no": 20000}))
	if err := tables.CheckRecurringLine(variable, true, true, "quantity"); err == nil {
		t.Error("a variable line without quantity passed the check")
	}
	_ = ftables.Rollback(tx)

	// an ordinary journal refuses a recurring method
	_, err = postItemLines(t, src, itemLine{entryType: gtables.ItemJournalLine_Entry_type.PositiveAdjmt, itemNo: "2000", qty: "1"})
	must(t, err)
	line := gtables.NewItemJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "ITEM", "DEFAULT", 10000
	must(t, line.Validate("posting_date", date(t, 10, 10, 2026)))
	line.Document_no = "T00002"
	must(t, line.Validate("item_no", types.Code("2000")))
	must(t, line.Validate("quantity", dec("1")))
	line.Recurring_method = gtables.ItemJournalLine_Recurring_method.Fixed
	must(t, line.InsertOrError(true))
	if _, err = postItemLines(t, src); err == nil {
		t.Error("a recurring method posted in an ordinary item journal")
	}
}
