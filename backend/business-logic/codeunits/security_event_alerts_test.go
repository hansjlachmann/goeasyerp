package codeunits

import (
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Routine 909 reports lockouts, wrong setup tokens and cross-site changes one by one and users
// with many refusals; a scheduled run marks what it checked and deletes old events
func TestSecurityEventAlerts(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	securityEventAlertsOn(t, db, database.DBTypeSQLite)
}

// The same on PostgreSQL (TEST_POSTGRES_DSN): the time filter of the deletion, the batch insert
func TestSecurityEventAlertsPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DROP TABLE IF EXISTS "Security_Event"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TABLE IF EXISTS "Security_Event"`) })
	securityEventAlertsOn(t, db, database.DBTypePostgres)
}

func securityEventAlertsOn(t *testing.T, db *sql.DB, sqlite database.DBType) {
	t.Helper()
	if err := (&gtables.SecurityEvent{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	write := tables.SecurityEventWriter(db, sqlite)
	now := time.Now()
	var events []securitylog.Event
	events = append(events, securitylog.Event{Time: now.AddDate(0, 0, -400), Kind: securitylog.Login, UserID: "OLD"})
	for range 3 {
		events = append(events, securitylog.Event{Time: now, Kind: securitylog.AccessRefused, UserID: "KARI"})
	}
	events = append(events,
		securitylog.Event{Time: now, Kind: securitylog.LoginLocked, UserID: "ADMIN", Address: "198.51.100.7"},
		securitylog.Event{Time: now, Kind: securitylog.Login, UserID: "ADMIN"})
	if err := write(events); err != nil {
		t.Fatal(err)
	}

	cu := NewSecurityEventAlerts(db, "", sqlite).(*SecurityEventAlerts)
	src := ftables.DBSource(db, "", sqlite)
	limits := securityAlertLimits{refusals: 3, failedLogins: 50, keepDays: 365}
	found := cu.findings(src)
	if found.checked != 6 || found.lastEntryNo != 6 || len(found.alerts) != 1 || found.refusals["KARI"] != 3 {
		t.Fatalf("findings: %+v", found)
	}
	reported := cu.report(found, limits, i18n.GetInstance(), "en-US")
	if reported == nil || reported.Code != apperrors.ErrSecurityEventsFound || len(reported.Details.Sections) != 2 {
		t.Fatalf("the report: %+v", reported)
	}
	if got := cu.report(found, securityAlertLimits{refusals: 4}, i18n.GetInstance(), "en-US"); got == nil || len(got.Details.Sections) != 1 {
		t.Errorf("below the refusal limit the lockout alone: %+v", got)
	}

	// a manual run (no Job Queue record) shows the findings and marks nothing
	_, err := cu.Run(nil)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrSecurityEventsFound {
		t.Fatalf("a manual run: %v", err)
	}
	if again := cu.findings(src); again.checked != 6 {
		t.Errorf("a manual run marked events: %d left unchecked", again.checked)
	}

	// a scheduled run marks them checked and deletes the event older than a year
	if err := cu.markChecked(src, found.lastEntryNo, limits.keepDays); err != nil {
		t.Fatal(err)
	}
	if again := cu.findings(src); again.checked != 0 {
		t.Errorf("%d events still unchecked", again.checked)
	}
	all := gtables.NewSecurityEvent(src)
	if n := all.Count(); n != 5 {
		t.Errorf("%d events kept, want 5 (the old one deleted)", n)
	}
	// later events are reported by the next run
	if err := write([]securitylog.Event{{Time: now, Kind: securitylog.SetupTokenRefused}}); err != nil {
		t.Fatal(err)
	}
	if again := cu.findings(src); again.checked != 1 || len(again.alerts) != 1 {
		t.Errorf("the next run: %+v", again)
	}
}

// The parameter's limits, and parts the routine does not understand
func TestSecurityAlertParameter(t *testing.T) {
	limits, err := parseSecurityAlertLimits("REFUSALS=5; failed_logins=10 KEEP_DAYS=0")
	if err != nil || limits != (securityAlertLimits{refusals: 5, failedLogins: 10, keepDays: 0}) {
		t.Errorf("limits %+v, %v", limits, err)
	}
	for _, bad := range []string{"REFUSALS", "REFUSALS=x", "OTHER=1", "KEEP_DAYS=-1", "KEEP_DAYS=1", "KEEP_DAYS=29"} {
		if _, err := parseSecurityAlertLimits(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
