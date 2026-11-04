package codeunits

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pdf"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// journalTest is a journal of the demo data: the line its page shows first, its test report,
// its Post and Print codeunit and the register table its posting report prints
type journalTest struct {
	name      string
	line      func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool)
	report    int
	title     string
	registers func(src ftables.Source) int
}

// The journals of the demo data (every one ready to post) print their test report without a
// warning, and Post and Print posts each and returns the printout of its register — the
// template's posting report (the demo templates get theirs from their type)
func TestJournalTestReportsAndPostAndPrint(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: openItemJnlPostLine, BOMJnlPostLine: openBOMJnlPostLine, SalesPost: openSalesPost, PurchPost: openPurchPost,
		GenJnlPostLine: openGenJnlPostLine, ProjectJnlPostLine: openProjectJnlPostLine, Journals: true,
		ReminderIssue: openReminderIssue, FinChrgMemoIssue: openFinChrgMemoIssue, AdjustExchRates: adjustExchRates}); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)

	journals := []journalTest{
		{"general", func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool) {
			line := gtables.NewGenJournalLine(src)
			line.SetRange("journal_template_name", "GENERAL")
			found := line.FindFirst()
			pageFilters(line, line.Journal_template_name.String(), line.Journal_batch_name.String())
			return NewGenJnlPostPrint(db, "demo", database.DBTypeSQLite), line, found
		}, reports.GenJnlTestID, "General Journal - Test", func(src ftables.Source) int { return gtables.NewGLRegister(src).Count() }},
		{"item", func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool) {
			line := gtables.NewItemJournalLine(src)
			line.SetRange("journal_template_name", "ITEM")
			found := line.FindFirst()
			pageFilters(line, line.Journal_template_name.String(), line.Journal_batch_name.String())
			return NewItemJnlPostPrint(db, "demo", database.DBTypeSQLite), line, found
		}, reports.ItemJnlTestID, "Inventory Posting - Test", func(src ftables.Source) int { return gtables.NewItemRegister(src).Count() }},
		{"resource", func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool) {
			line := gtables.NewResJournalLine(src)
			line.SetRange("journal_template_name", "RESOURCES")
			found := line.FindFirst()
			pageFilters(line, line.Journal_template_name.String(), line.Journal_batch_name.String())
			return NewResJnlPostPrint(db, "demo", database.DBTypeSQLite), line, found
		}, reports.ResJnlTestID, "Resource Journal - Test", func(src ftables.Source) int { return gtables.NewResourceRegister(src).Count() }},
		{"BOM", func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool) {
			line := gtables.NewBOMJournalLine(src)
			line.SetRange("journal_template_name", "BOM")
			found := line.FindFirst()
			pageFilters(line, line.Journal_template_name.String(), line.Journal_batch_name.String())
			return NewBOMJnlPostPrint(db, "demo", database.DBTypeSQLite), line, found
		}, reports.BOMJnlTestID, "BOM Journal - Test", func(src ftables.Source) int { return gtables.NewBOMRegister(src).Count() }},
		{"project", func(src ftables.Source) (fcodeunits.Codeunit, interface{}, bool) {
			line := gtables.NewProjectJournalLine(src)
			line.SetRange("journal_template_name", "PROJECT")
			found := line.FindFirst()
			pageFilters(line, line.Journal_template_name.String(), line.Journal_batch_name.String())
			return NewProjectJnlPostPrint(db, "demo", database.DBTypeSQLite), line, found
		}, reports.ProjectJnlTestID, "Project Journal - Test", func(src ftables.Source) int { return gtables.NewProjectRegister(src).Count() }},
	}
	for _, j := range journals {
		t.Run(j.name, func(t *testing.T) {
			// Every question is answered Yes (the dialog is the goroutine's: the subtest's)
			d := fcodeunits.OpenDialogWithID("", "jnl-post-print-test-"+j.name)
			fcodeunits.SetCurrentDialog(d)
			defer fcodeunits.ClearCurrentDialog()
			go func() {
				for ev := range d.Events() {
					if ev.EventType == "confirm" {
						for !d.RespondToConfirm(ev.ConfirmID, true) {
							time.Sleep(time.Millisecond)
						}
					}
				}
			}()
			codeunit, line, found := j.line(src)
			if !found {
				t.Fatalf("no demo %s journal line", j.name)
			}
			result, err := reports.Execute(j.report, db, "demo", database.DBTypeSQLite, reports.RunOptions{Output: reports.OutputPDF, Context: line})
			if err != nil {
				t.Fatal(err)
			}
			texts := allTexts(result.Texts)
			if !strings.Contains(texts, j.title) {
				t.Errorf("report %d does not print its title:\n%s", j.report, texts)
			}
			if strings.Contains(texts, "Warning!") {
				t.Errorf("report %d warns about the demo %s journal:\n%s", j.report, j.name, texts)
			}

			registersBefore := j.registers(src)
			posted, err := codeunit.Run(line)
			if err != nil {
				t.Fatal(err)
			}
			if j.registers(src) != registersBefore+1 {
				t.Errorf("%s journal: %d registers after posting, want %d", j.name, j.registers(src), registersBefore+1)
			}
			if posted.Data["pdf"] == nil || posted.Data["output"] != reports.OutputPrint {
				t.Errorf("%s journal: no printout of the register: %v", j.name, posted.Data["filename"])
			}
		})
	}
}

// pageFilters: the journal page's filters on the template and batch (filter group 2)
func pageFilters(rec interface {
	FilterGroup(...int) int
	SetRange(string, ...interface{})
}, template, batch string) {
	rec.FilterGroup(2)
	rec.SetRange("journal_template_name", template)
	rec.SetRange("journal_batch_name", batch)
	rec.FilterGroup(0)
	rec.SetRange("journal_template_name")
}

func allTexts(placed []pdf.PlacedText) string {
	var b strings.Builder
	for _, p := range placed {
		b.WriteString(p.Text)
		b.WriteString("\n")
	}
	return b.String()
}
