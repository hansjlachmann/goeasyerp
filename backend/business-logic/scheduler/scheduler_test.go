package scheduler

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/jobqueue"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/secrets"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// sentMail is one message a mockSender was asked to send
type sentMail struct {
	to            []string
	subject, body string
}

// mockSender records the emails it is asked to send.
type mockSender struct {
	sent []sentMail
}

func (m *mockSender) Send(to []string, subject, body string) error {
	m.sent = append(m.sent, sentMail{to, subject, body})
	return nil
}
func (m *mockSender) Enabled() bool { return true }

const testCompany = "test"

// newTestDB opens an in-memory SQLite DB with an empty test$Job_Queue table.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	var jq gtables.JobQueue
	if err := jq.CreateTableWithDBType(db, testCompany, database.DBTypeSQLite); err != nil {
		t.Fatalf("create Job_Queue table: %v", err)
	}
	return db
}

// seedJob inserts a Ready job and returns nothing; fields configure recurrence.
func seedJob(t *testing.T, db *sql.DB, no string, rec gtables.JobQueueRecurrence, minutes int) {
	t.Helper()
	var jq gtables.JobQueue
	jq.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	jq.No = types.NewCode(no)
	jq.Description = types.NewText("test job")
	jq.Status = gtables.JobQueue_Status.Ready
	jq.Recurrence = rec
	jq.Minutes_between_run = minutes
	jq.Next_start = types.NewDateTimeFromTime(time.Now().Add(-time.Hour)) // due
	if !jq.Insert(true) {
		t.Fatalf("failed to insert job %s", no)
	}
}

func getJob(t *testing.T, db *sql.DB, no string) gtables.JobQueue {
	t.Helper()
	var jq gtables.JobQueue
	jq.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !jq.Get(no) {
		t.Fatalf("job %s not found", no)
	}
	return jq
}

func TestFinishReschedulesByRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		rec     gtables.JobQueueRecurrence
		minutes int
		// expected next_start delta from now, with tolerance
		wantDelta time.Duration
	}{
		{"minutes", gtables.JobQueue_Recurrence.Minutes, 15, 15 * time.Minute},
		{"hourly", gtables.JobQueue_Recurrence.Hourly, 0, time.Hour},
		{"daily", gtables.JobQueue_Recurrence.Daily, 0, 24 * time.Hour},
		{"weekly", gtables.JobQueue_Recurrence.Weekly, 0, 7 * 24 * time.Hour},
		{"monthly", gtables.JobQueue_Recurrence.Monthly, 0, 28 * 24 * time.Hour}, // >= 28 days
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			defer db.Close()
			seedJob(t, db, "J", tc.rec, tc.minutes)

			s := &Scheduler{db: db, dbType: database.DBTypeSQLite}
			before := time.Now()
			s.finish(testCompany, "J", nil)

			job := getJob(t, db, "J")
			if job.Status != gtables.JobQueue_Status.Ready {
				t.Errorf("status = %v, want Ready (recurring job should stay Ready)", job.Status)
			}
			gotDelta := job.Next_start.Time.Sub(before)
			// monthly uses calendar months (28-31 days); assert lower bound only
			if tc.name == "monthly" {
				if gotDelta < 28*24*time.Hour {
					t.Errorf("monthly next_start delta = %s, want >= 28d", gotDelta)
				}
				return
			}
			diff := gotDelta - tc.wantDelta
			if diff < -time.Minute || diff > time.Minute {
				t.Errorf("next_start delta = %s, want ~%s", gotDelta, tc.wantDelta)
			}
		})
	}
}

func TestFinishOnceGoesOnHold(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	seedJob(t, db, "J", gtables.JobQueue_Recurrence.Once, 0)

	s := &Scheduler{db: db, dbType: database.DBTypeSQLite}
	s.finish(testCompany, "J", nil)

	job := getJob(t, db, "J")
	if job.Status != gtables.JobQueue_Status.OnHold {
		t.Errorf("status = %v, want OnHold after a successful Once run", job.Status)
	}
}

func TestFinishErrorSetsErrorStatus(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	seedJob(t, db, "J", gtables.JobQueue_Recurrence.Daily, 0)
	orig := getJob(t, db, "J").Next_start

	s := &Scheduler{db: db, dbType: database.DBTypeSQLite}
	s.finish(testCompany, "J", errors.New("boom"))

	job := getJob(t, db, "J")
	if job.Status != gtables.JobQueue_Status.Error {
		t.Errorf("status = %v, want Error", job.Status)
	}
	if !job.Next_start.Equal(orig) {
		t.Errorf("next_start changed on error: got %s want %s", job.Next_start, orig)
	}
}

func TestNotifyGating(t *testing.T) {
	tests := []struct {
		name     string
		notifyOn gtables.JobQueueNotify_on
		email    string
		runErr   error
		wantSent bool
	}{
		{"always/success", gtables.JobQueue_Notify_on.Always, "a@b.com", nil, true},
		{"always/error", gtables.JobQueue_Notify_on.Always, "a@b.com", errors.New("x"), true},
		{"onerror/success", gtables.JobQueue_Notify_on.OnError, "a@b.com", nil, false},
		{"onerror/error", gtables.JobQueue_Notify_on.OnError, "a@b.com", errors.New("x"), true},
		{"never/error", gtables.JobQueue_Notify_on.Never, "a@b.com", errors.New("x"), false},
		{"blank-email/always", gtables.JobQueue_Notify_on.Always, "", nil, false},
		{"separators-only/always", gtables.JobQueue_Notify_on.Always, " ; ", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := &mockSender{}
			db := newTestDB(t)
			defer func() { _ = db.Close() }()
			s := &Scheduler{db: db, dbType: database.DBTypeSQLite, mailer: ms}
			job := &gtables.JobQueue{}
			job.No = types.NewCode("J")
			job.Notification_email = types.NewText(tc.email)
			job.Notify_on = tc.notifyOn

			s.notify(testCompany, job, jobqueue.Run{Start: time.Now(), End: time.Now(), Err: tc.runErr})

			if got := len(ms.sent) == 1; got != tc.wantSent {
				t.Errorf("sent=%d, wantSent=%v", len(ms.sent), tc.wantSent)
			}
		})
	}
}

// upsertSMTPSetup writes the single blank-primary-key SMTP_Setup record.
func upsertSMTPSetup(t *testing.T, db *sql.DB, enabled bool, host string, port int) {
	t.Helper()
	var st gtables.SMTPSetup
	st.InitWithDBType(db, "", database.DBTypeSQLite)
	exists := st.Get("")
	st.Primary_key = types.NewCode("")
	st.Enabled = enabled
	st.Smtp_server = types.NewText(host)
	st.Smtp_server_port = port
	st.From_address = types.NewText("from@example.com")
	ok := false
	if exists {
		ok = st.Modify(true)
	} else {
		ok = st.Insert(true)
	}
	if !ok {
		t.Fatalf("failed to write SMTP setup")
	}
}

func TestLoadSMTPConfigFromDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var setup gtables.SMTPSetup
	if err := setup.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatalf("create SMTP_Setup: %v", err)
	}

	s := &Scheduler{db: db, dbType: database.DBTypeSQLite}

	// No record -> disabled config.
	if cfg := s.loadSMTPConfig(); cfg.Enabled {
		t.Error("expected disabled config when the setup record is missing")
	}

	// A disabled record -> still disabled.
	upsertSMTPSetup(t, db, false, "off.example.com", 25)
	if cfg := s.loadSMTPConfig(); cfg.Enabled {
		t.Error("expected disabled config when the record is not enabled")
	}

	// Enabled record with port 0 -> enabled, port defaults to 587.
	upsertSMTPSetup(t, db, true, "smtp.example.com", 0)
	cfg := s.loadSMTPConfig()
	if !cfg.Enabled {
		t.Fatal("expected enabled config")
	}
	if cfg.Host != "smtp.example.com" {
		t.Errorf("host = %q, want smtp.example.com", cfg.Host)
	}
	if cfg.Port != "587" {
		t.Errorf("port = %q, want 587 (default for 0)", cfg.Port)
	}
	if cfg.From != "from@example.com" {
		t.Errorf("from = %q", cfg.From)
	}

	// The stored (encrypted) password is decrypted for the mailer
	defer secrets.SetKeyForTest("test key")()
	var st gtables.SMTPSetup
	st.InitWithDBType(db, "", database.DBTypeSQLite)
	if !st.Get("") {
		t.Fatal("get SMTP setup")
	}
	st.Password = types.NewText("app password")
	if !st.Modify(true) {
		t.Fatal("modify SMTP setup")
	}
	if cfg := s.loadSMTPConfig(); cfg.Password != "app password" {
		t.Errorf("password = %q, want the decrypted password", cfg.Password)
	}
}

func TestIsDue(t *testing.T) {
	now := types.Now()
	if !isDue(types.DateTime{}, now) {
		t.Error("zero next_start should be due")
	}
	if !isDue(now.AddMinutes(-1), now) {
		t.Error("past next_start should be due")
	}
	if isDue(now.AddMinutes(5), now) {
		t.Error("future next_start should not be due")
	}
}

func TestNotifySendsToAllRecipients(t *testing.T) {
	db := newTestDB(t)
	defer func() { _ = db.Close() }()
	ms := &mockSender{}
	s := &Scheduler{db: db, dbType: database.DBTypeSQLite, mailer: ms}
	job := &gtables.JobQueue{}
	job.No = types.NewCode("J1")
	job.Description = types.NewText("Daily invoices")
	job.Notification_email = types.NewText("ops@example.com; Hans <hans@example.com>;")
	job.Notify_on = gtables.JobQueue_Notify_on.OnError
	job.Status = gtables.JobQueue_Status.Error

	s.notify(testCompany, job, jobqueue.Run{Start: time.Now(), End: time.Now(), Err: errors.New("disk full")})

	if len(ms.sent) != 1 {
		t.Fatalf("sent %d messages, want one message to all recipients", len(ms.sent))
	}
	m := ms.sent[0]
	if len(m.to) != 2 || m.to[0] != "ops@example.com" || m.to[1] != "Hans <hans@example.com>" {
		t.Errorf("recipients = %q", m.to)
	}
	if m.subject != "Job Queue J1 Daily invoices failed (test)" {
		t.Errorf("subject = %q", m.subject)
	}
	for _, want := range []string{"Company: test\n", "No.: J1\n", "Result: Failed\n", "Error:\ndisk full\n", "does not run again"} {
		if !strings.Contains(m.body, want) {
			t.Errorf("body lacks %q:\n%s", want, m.body)
		}
	}
}

func TestNotifyInNotificationLanguage(t *testing.T) {
	db := newTestDB(t)
	defer func() { _ = db.Close() }()
	ms := &mockSender{}
	s := &Scheduler{db: db, dbType: database.DBTypeSQLite, mailer: ms}
	job := &gtables.JobQueue{}
	job.No = types.NewCode("J1")
	job.Notification_email = types.NewText("ops@example.com")
	job.Notification_language = types.NewCode("NB-NO") // no Language record: the code itself
	job.Notify_on = gtables.JobQueue_Notify_on.Always
	job.Status = gtables.JobQueue_Status.Ready
	job.Next_start = types.NewDateTimeFromTime(time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC))

	s.notify(testCompany, job, jobqueue.Run{Start: time.Now(), End: time.Now()})

	if len(ms.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(ms.sent))
	}
	m := ms.sent[0]
	if m.subject != "Jobbkø J1 fullført (test)" {
		t.Errorf("subject = %q", m.subject)
	}
	if !strings.Contains(m.body, "Resultat: Fullført\n") || !strings.Contains(m.body, "2026-10-09 02:00:00") {
		t.Errorf("body not in Norwegian or without next start:\n%s", m.body)
	}
}
