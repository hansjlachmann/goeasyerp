package genjnl

import (
	"strconv"
	"testing"

	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// recurringJournal: template RECURRING (recurring, General) with batch DEFAULT
func recurringJournal(t *testing.T, src ftables.Source) {
	t.Helper()
	tpl := gtables.NewGenJournalTemplate(src)
	tpl.Name = "RECURRING"
	must(t, tpl.Validate("type", gtables.GenJournalTemplate_Type.General))
	must(t, tpl.Validate("recurring", true))
	must(t, tpl.InsertOrError(true))
	if tpl.Page_id != 283 {
		t.Errorf("recurring template page %d, want 283", tpl.Page_id)
	}
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "RECURRING"
	batch.Name = "DEFAULT"
	must(t, batch.InsertOrError(true))
}

func recurringLine(t *testing.T, src ftables.Source, lineNo int, method gtables.GenJournalLineRecurring_method, accNo, amount string) *gtables.GenJournalLine {
	t.Helper()
	line := newJnlLine(src, "RECURRING", "DEFAULT", lineNo)
	must(t, line.Validate("recurring_method", method))
	line.Recurring_frequency = "1M"
	must(t, line.Validate("posting_date", date(t, 31, 10, 2026)))
	line.Document_no = "RENT-%3"
	must(t, line.Validate("account_no", types.Code(accNo)))
	line.Description = "Rent %4"
	must(t, line.InsertOrError(true))
	must(t, line.Validate("amount", dec(amount)))
	must(t, line.ModifyOrError(true))
	return line
}

func allocation(t *testing.T, line *gtables.GenJournalLine, lineNo int, accNo string, set func(a *gtables.GenJnlAllocation)) {
	t.Helper()
	a := gtables.NewGenJnlAllocation(line)
	a.Journal_template_name = line.Journal_template_name
	a.Journal_batch_name = line.Journal_batch_name
	a.Journal_line_no = line.Line_no
	a.Line_no = lineNo
	must(t, a.Validate("account_no", types.Code(accNo)))
	must(t, a.InsertOrError(true))
	set(a)
	must(t, a.ModifyOrError(true))
}

func glEntriesOf(src ftables.Source, from int) map[string]types.Decimal {
	entry := gtables.NewGLEntry(src)
	entry.SetFilter("entry_no", ">"+strconv.Itoa(from))
	got := map[string]types.Decimal{}
	for found := entry.FindSet(); found; found = entry.Next() {
		key := entry.Posting_date.String() + " " + entry.Document_no.String() + " " + entry.Gl_account_no.String()
		got[key] = got[key].Add(entry.Amount)
	}
	return got
}

// A recurring journal: a fixed line allocated by quantity (1:2) and a reversing variable line
// allocated by amount post with their allocations, the reversing line reversed the day after;
// the document no. and description get the posting date's month; the lines stay, a month later,
// the variable line and its allocation without amounts
func TestPostRecurringJournal(t *testing.T) {
	src := demoCompany(t)
	recurringJournal(t, src)
	fixed := recurringLine(t, src, 10000, gtables.GenJournalLine_Recurring_method.FFixed, "1210", "1200")
	allocation(t, fixed, 10000, "1230", func(a *gtables.GenJnlAllocation) { must(t, a.Validate("allocation_quantity", dec("1"))) })
	allocation(t, fixed, 20000, "1250", func(a *gtables.GenJnlAllocation) { must(t, a.Validate("allocation_quantity", dec("2"))) })
	variable := recurringLine(t, src, 20000, gtables.GenJournalLine_Recurring_method.RVReversingVariable, "1270", "300")
	allocation(t, variable, 10000, "1230", func(a *gtables.GenJnlAllocation) { must(t, a.Validate("amount", dec("-300"))) })

	alloc := gtables.NewGenJnlAllocation(src)
	must(t, alloc.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "journal_line_no": 10000, "line_no": 20000}))
	if !alloc.Amount.Equal(dec("-800")) || !alloc.Allocation_percent.Equal(dec("66.67")) {
		t.Errorf("allocation by quantity: amount %s, %s %%", alloc.Amount, alloc.Allocation_percent)
	}
	must(t, fixed.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "line_no": 10000}))
	fixed.CalcFields("allocated_amt_lcy")
	if !fixed.Allocated_amt_lcy.Equal(dec("-1200")) || !fixed.Balance_lcy.IsZero() {
		t.Errorf("fixed line: allocated %s, balance %s", fixed.Allocated_amt_lcy, fixed.Balance_lcy)
	}

	lastGL := gtables.NewGLEntry(src)
	lastGL.FindLast()
	result, err := postBatch(t, src, "RECURRING", "DEFAULT")
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	got := glEntriesOf(src, lastGL.Entry_no)
	want := map[string]string{
		"2026-10-31 RENT-10 1210": "1200",
		"2026-10-31 RENT-10 1230": "-700",
		"2026-10-31 RENT-10 1250": "-800",
		"2026-10-31 RENT-10 1270": "300",
		"2026-11-01 RENT-10 1270": "-300",
		"2026-11-01 RENT-10 1230": "300",
	}
	if len(got) != len(want) {
		t.Errorf("G/L entries %v", got)
	}
	for k, v := range want {
		if !got[k].Equal(dec(v)) {
			t.Errorf("%s: %s, want %s", k, got[k], v)
		}
	}
	entry := gtables.NewGLEntry(src)
	entry.SetRange("gl_account_no", types.Code("1210"))
	entry.FindLast()
	if entry.Description != "Rent October" {
		t.Errorf("description %q", entry.Description)
	}

	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("RECURRING"))
	if lines.Count() != 2 {
		t.Fatalf("%d lines left, want both", lines.Count())
	}
	for found := lines.FindSet(); found; found = lines.Next() {
		if !lines.Posting_date.Equal(date(t, 30, 11, 2026)) || lines.Document_no != "RENT-%3" {
			t.Errorf("line %d: %s %s", lines.Line_no, lines.Posting_date, lines.Document_no)
		}
		wantAmount := "1200"
		if lines.Line_no == 20000 {
			wantAmount = "0"
		}
		if !lines.Amount.Equal(dec(wantAmount)) {
			t.Errorf("line %d amount %s, want %s", lines.Line_no, lines.Amount, wantAmount)
		}
	}
	must(t, alloc.GetOrError(map[string]interface{}{"journal_template_name": "RECURRING", "journal_batch_name": "DEFAULT", "journal_line_no": 20000, "line_no": 10000}))
	if !alloc.Amount.IsZero() {
		t.Errorf("the variable line's allocation kept %s", alloc.Amount)
	}
}

// The Balance method posts the reverse of the account's balance on the posting date to it and
// allocates it; a recurring line cannot have a balancing account; allocations only in recurring
// journals
func TestRecurringBalanceAndChecks(t *testing.T) {
	src := demoCompany(t)
	recurringJournal(t, src)
	acc := gtables.NewGLAccount(src)
	acc.No = "1270"
	acc.SetRange("date_filter", types.Date{}, date(t, 31, 10, 2026))
	acc.CalcFields("net_change")
	if acc.Net_change.IsZero() {
		t.Skip("account 1270 has no balance in the demo")
	}
	line := recurringLine(t, src, 10000, gtables.GenJournalLine_Recurring_method.BBalance, "1270", "0")
	allocation(t, line, 10000, "1230", func(a *gtables.GenJnlAllocation) { must(t, a.Validate("allocation_percent", dec("100"))) })
	_, err := postBatch(t, src, "RECURRING", "DEFAULT")
	must(t, err)
	acc.CalcFields("net_change")
	if !acc.Net_change.IsZero() {
		t.Errorf("1270 balance after the balance line: %s", acc.Net_change)
	}

	bal := recurringLine(t, src, 20000, gtables.GenJournalLine_Recurring_method.FFixed, "1210", "100")
	must(t, bal.Validate("bal_account_no", types.Code("1230")))
	must(t, bal.ModifyOrError(true))
	if _, err = postBatch(t, src, "RECURRING", "DEFAULT"); err == nil {
		t.Error("a recurring line with a balancing account posted")
	}
	must(t, bal.DeleteOrError(true))

	general := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, general.Validate("posting_date", date(t, 15, 10, 2026)))
	general.Document_no = "G00001"
	must(t, general.Validate("account_no", types.Code("1210")))
	must(t, general.Validate("amount", dec("100")))
	must(t, general.InsertOrError(true))
	allocation(t, general, 10000, "1230", func(a *gtables.GenJnlAllocation) { must(t, a.Validate("amount", dec("-100"))) })
	if _, err = postBatch(t, src, "GENERAL", "DEFAULT"); err == nil {
		t.Error("allocations posted in a general journal")
	}
}
