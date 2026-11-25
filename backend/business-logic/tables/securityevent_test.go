package tables

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	_ "github.com/mattn/go-sqlite3"
)

// The table's Event options are the recorder's kinds, in the same order
func TestSecurityEventOptionsMatchKinds(t *testing.T) {
	names := securitylog.Names()
	if !gtables.SecurityEventEvent(len(names)-1).IsValid() || gtables.SecurityEventEvent(len(names)).IsValid() {
		t.Fatalf("the table has another number of options than the %d kinds", len(names))
	}
	for i, name := range names {
		if got := gtables.SecurityEventEvent(i).String(); got != name {
			t.Errorf("option %d is %q, the kind is %q", i, got, name)
		}
	}
}

// The writer numbers the events after the last one, in batches
func TestSecurityEventWriter(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if err := (&gtables.SecurityEvent{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	write := SecurityEventWriter(db, sqlite)
	at := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	if err := write([]securitylog.Event{
		{Time: at, Kind: securitylog.LoginFailed, UserID: "kari", Address: "192.0.2.10", Company: "main", Details: "wrong password"},
		{Time: at, Kind: securitylog.LoginLocked, UserID: "kari", Address: "192.0.2.10"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := write([]securitylog.Event{{Time: at, Kind: securitylog.Login, UserID: "KARI"}}); err != nil {
		t.Fatal(err)
	}

	rec := &gtables.SecurityEvent{}
	rec.InitWithDBType(db, "", sqlite)
	var got []gtables.SecurityEvent
	for found := rec.FindSet(); found; found = rec.Next() {
		got = append(got, *rec)
	}
	if len(got) != 3 {
		t.Fatalf("%d events, want 3", len(got))
	}
	if got[0].Entry_no != 1 || got[2].Entry_no != 3 || got[0].Event != gtables.SecurityEvent_Event.LoginFailed ||
		got[0].User_id != "KARI" || got[0].Address != "192.0.2.10" || got[0].Details != "wrong password" ||
		got[1].Event != gtables.SecurityEvent_Event.LoginLocked || got[2].Event != gtables.SecurityEvent_Event.Login {
		t.Errorf("events: %+v", got)
	}
}
