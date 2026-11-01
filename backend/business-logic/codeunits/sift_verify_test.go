package codeunits

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func TestSIFTVerifyCodeunit(t *testing.T) {
	const company = "demo01"
	db, err := sql.Open("sqlite3", "file:siftverify?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	// every company table (the demo data's and the Job Queue's)
	if err := tables.CreateCompanyTables(db, company, database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, company, database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}

	run := func(parameter string) (string, error) {
		job := &gtables.JobQueue{}
		job.InitWithDBType(db, company, database.DBTypeSQLite)
		job.No = types.NewCode("SIFTCHECK")
		job.Description = types.NewText("Verify SIFT")
		job.Parameter = types.NewText(parameter)
		result, err := NewSIFTVerify(db, company, database.DBTypeSQLite).Run(job)
		msg := ""
		if result.Dialog != nil {
			msg = result.Dialog.Message
		}
		return msg, err
	}
	lastEntryStatus := func() gtables.JobQueueEntryStatus {
		var e gtables.JobQueueEntry
		e.InitWithDBType(db, company, database.DBTypeSQLite)
		if !e.FindLast() {
			t.Fatal("no Job Queue Entry logged")
		}
		return e.Status
	}

	// Correct totals: success, the 27 keys checked (customer, vendor, bank, FA, FA allocation, gen. journal allocation, maintenance, item and
	// insurance coverage ledger, G/L entries, resource capacity and ledger)
	msg, err := run("")
	if err != nil || !strings.Contains(msg, "27 SIFT keys") {
		t.Fatalf("verify on correct totals: %q, %v", msg, err)
	}
	if lastEntryStatus() != gtables.JobQueueEntry_Status.Success {
		t.Error("entry not logged as Success")
	}

	// Corrupt a total behind the triggers' back
	if _, err := db.Exec(`UPDATE "demo01$Customer Ledger Entry$SIFT$customer_open" SET remaining_amt_lcy = remaining_amt_lcy + 100 WHERE customer_no = 'C00010'`); err != nil {
		t.Fatal(err)
	}
	if _, err := run("VERIFY"); err == nil || !strings.Contains(err.Error(), "customer_open") {
		t.Fatalf("verify on corrupted totals: err = %v, want the differing key named", err)
	}
	if lastEntryStatus() != gtables.JobQueueEntry_Status.Error {
		t.Error("entry not logged as Error")
	}

	// REPAIR rebuilds, then everything verifies again
	if msg, err := run("repair"); err != nil || !strings.Contains(msg, "customer_open") {
		t.Fatalf("repair: %q, %v", msg, err)
	}
	if _, err := run("VERIFY"); err != nil {
		t.Fatalf("verify after repair: %v", err)
	}

	if _, err := run("FIXIT"); err == nil {
		t.Error("unknown parameter accepted")
	}
}
