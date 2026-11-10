package codeunits

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Post on the batch list (codeunit 233) posts every batch in a transaction of its own: a batch
// that cannot be posted stays, is named in the message and marked (the list selects it)
func TestJnlBatchPostMarksFailedBatches(t *testing.T) {
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

	// a second batch whose line has no balancing account: it does not balance
	line := gtables.NewGenJournalLine(src)
	line.SetRange("journal_template_name", "GENERAL")
	if !line.FindFirst() {
		t.Fatal("no demo general journal line")
	}
	goodBatch := line.Journal_batch_name
	batch := gtables.NewGenJournalBatch(src)
	if !batch.Get(tables.GenJnlBatchKey("GENERAL", goodBatch)) {
		t.Fatalf("no batch %s", goodBatch)
	}
	batch.Name = "BAD"
	if err := batch.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	line.Journal_batch_name = "BAD"
	line.Bal_account_no = ""
	if err := line.InsertOrError(false); err != nil {
		t.Fatal(err)
	}

	d := fcodeunits.OpenDialogWithID("", "jnl-batch-post-test")
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

	batches := gtables.NewGenJournalBatch(src)
	batches.SetRange("journal_template_name", types.Code("GENERAL"))
	batches.SetFilter("name", goodBatch.String()+"|BAD")
	codeunit := &JnlBatchPost{id: GenJnlBPostID, name: "gen-jnl-b-post", kind: genJnlBatchKind, db: db, company: "demo", dbType: database.DBTypeSQLite}
	result, err := codeunit.Run(batches)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Mark) != 1 || result.Mark[0] != "GENERAL,BAD" {
		t.Errorf("marked %v, want [GENERAL,BAD]", result.Mark)
	}
	if result.Dialog == nil || !strings.Contains(result.Dialog.Message, "BAD") {
		t.Errorf("the message does not name the batch: %+v", result.Dialog)
	}
	// the good batch was posted (its lines are gone), the bad one kept its line
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("GENERAL"))
	lines.SetRange("journal_batch_name", goodBatch)
	if !lines.IsEmpty() {
		t.Errorf("batch %s was not posted", goodBatch)
	}
	lines.SetRange("journal_batch_name", types.Code("BAD"))
	if lines.Count() != 1 {
		t.Error("the batch that could not be posted lost its line")
	}
}
