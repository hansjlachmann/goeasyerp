package codeunits

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 904 on the demo company: balanced (a message, the run logged as Success); an amount
// changed in the database: the run fails with the findings as details (logged as Error)
func TestGLBalanceCheckCodeunit(t *testing.T) {
	const company = "demo01"
	db, err := sql.Open("sqlite3", "file:glbalancecheck?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, company, database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, company, database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}
	job := &gtables.JobQueue{}
	job.InitWithDBType(db, company, database.DBTypeSQLite)
	job.No = types.NewCode("GLCHECK")
	job.Description = types.NewText("G/L Balance Check")
	lastEntryStatus := func() gtables.JobQueueEntryStatus {
		var e gtables.JobQueueEntry
		e.InitWithDBType(db, company, database.DBTypeSQLite)
		if !e.FindLast() {
			t.Fatal("no Job Queue Entry logged")
		}
		return e.Status
	}

	result, err := NewGLBalanceCheck(db, company, database.DBTypeSQLite).Run(job)
	if err != nil || result.Dialog == nil || !strings.Contains(result.Dialog.Message, "The G/L balances") {
		t.Fatalf("balanced: %+v, %v", result, err)
	}
	if lastEntryStatus() != gtables.JobQueueEntry_Status.Success {
		t.Error("a balanced run is logged as Success")
	}

	if _, err := db.Exec(`UPDATE "demo01$G_L_Entry" SET amount = amount + 1 WHERE entry_no = 3`); err != nil {
		t.Fatal(err)
	}
	_, err = NewGLBalanceCheck(db, company, database.DBTypeSQLite).Run(job)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrGLCheckFailed {
		t.Fatalf("unbalanced: %v", err)
	}
	if !strings.Contains(appErr.Message("en-US"), "posting dates that do not balance: 1, transactions that do not balance: 1") {
		t.Errorf("message: %s", appErr.Message("en-US"))
	}
	if appErr.Details == nil || len(appErr.Details.Sections) != 2 || appErr.Details.Sections[0].Rows[0][1] != "1.00" {
		t.Errorf("details: %+v", appErr.Details)
	}
	if lastEntryStatus() != gtables.JobQueueEntry_Status.Error {
		t.Error("a run with findings is logged as Error")
	}
}
