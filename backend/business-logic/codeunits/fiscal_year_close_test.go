package codeunits

import (
	"database/sql"
	"testing"
	"time"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 6 closes the oldest open fiscal year: its periods closed, they and the next year's
// first period date locked. Without a next year, or with nothing open, it is refused.
func TestFiscalYearClose(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.AccountingPeriod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	// 2025 and 2026 by month, and January 2027
	for i := 0; i <= 24; i++ {
		p := gtables.NewAccountingPeriod(src)
		p.Starting_date = types.NewDate(2025+i/12, time.Month(1+i%12), 1)
		p.New_fiscal_year = i%12 == 0
		p.Date_locked = i == 0
		if err := p.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	// Every question is answered Yes
	d := fcodeunits.OpenDialogWithID("", "fiscal-year-close-test")
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
	count := func(field string) int {
		p := gtables.NewAccountingPeriod(src)
		p.SetRange(field, true)
		return p.Count()
	}
	closeYear := func() error {
		_, err := NewFiscalYearClose(db, "c1", database.DBTypeSQLite).Run(gtables.NewAccountingPeriod(src))
		return err
	}

	if err := closeYear(); err != nil {
		t.Fatal(err)
	}
	if count("closed") != 12 || count("date_locked") != 13 {
		t.Errorf("after closing 2025: %d closed, %d date locked", count("closed"), count("date_locked"))
	}
	// Then 2026; January 2027 has no next fiscal year after it: refused
	if err := closeYear(); err != nil {
		t.Fatal(err)
	}
	if count("closed") != 24 || count("date_locked") != 25 {
		t.Errorf("after closing 2026: %d closed, %d date locked", count("closed"), count("date_locked"))
	}
	if err := closeYear(); err == nil {
		t.Error("closed 2027 without a next fiscal year")
	}
}
