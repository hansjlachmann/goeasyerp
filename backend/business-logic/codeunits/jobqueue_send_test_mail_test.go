package codeunits

import (
	"database/sql"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strings"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/secrets"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	_ "github.com/mattn/go-sqlite3"
)

// testMailer records what it sends; rejects maps recipients it refuses.
type testMailer struct {
	enabled bool
	rejects map[string]bool
	to      []string
	subject string
	body    string
}

func (m *testMailer) Enabled() bool { return m.enabled }

func (m *testMailer) Send(to []string, subject, body string) error {
	m.subject, m.body = subject, body
	var rejected []mail.RecipientError
	for _, r := range to {
		if m.rejects[r] {
			rejected = append(rejected, mail.RecipientError{Recipient: r, Err: errTestRejected})
			continue
		}
		m.to = append(m.to, r)
	}
	if len(rejected) > 0 {
		return &mail.RecipientsError{Rejected: rejected, Sent: len(m.to)}
	}
	return nil
}

type testRejectedError struct{}

func (testRejectedError) Error() string { return "550 no such user" }

var errTestRejected = testRejectedError{}

func TestJobQueueSendTestMail(t *testing.T) {
	const company = "demo01"
	db, err := sql.Open("sqlite3", "file:sendtestmail?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.JobQueue{}, &gtables.Company{}, &gtables.SMTPSetup{}} {
		if err := tbl.CreateTableWithDBType(db, company, database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	fcodeunits.SetCurrentContext("HANS", "Hans", company, "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)

	mailer := &testMailer{}
	newTestMailer = func(mail.Config) mail.Sender { return mailer }
	t.Cleanup(func() { newTestMailer = func(cfg mail.Config) mail.Sender { return mail.NewSMTPMailer(cfg) } })

	setJob := func(emails string) {
		t.Helper()
		var job gtables.JobQueue
		job.InitWithDBType(db, company, database.DBTypeSQLite)
		exists := job.Get("J1")
		job.No = types.NewCode("J1")
		job.Description = types.NewText("Daily invoices")
		job.Notification_email = types.NewText(emails)
		if (exists && !job.Modify(true)) || (!exists && !job.Insert(true)) {
			t.Fatal("save job")
		}
	}
	run := func() (string, error) {
		t.Helper()
		selected := &gtables.JobQueue{}
		selected.No = types.NewCode("J1")
		result, err := NewJobQueueSendTestMail(db, company, database.DBTypeSQLite).Run(selected)
		return result.Message, err
	}

	setJob("")
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "has no Notification E-mail") {
		t.Errorf("no recipients: error %v", err)
	}

	setJob("ops@example.com; ops@")
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "'ops@' is not a valid e-mail address") {
		t.Errorf("bad address: error %v", err)
	}

	setJob("ops@example.com; hans@example.com")
	if _, err := run(); err == nil || !strings.Contains(err.Error(), "E-mail is not set up") {
		t.Errorf("SMTP disabled: error %v", err)
	}

	mailer.enabled = true
	msg, err := run()
	if err != nil || msg != "Test e-mail sent to ops@example.com; hans@example.com." {
		t.Errorf("sent: %q, %v", msg, err)
	}
	if mailer.subject != "Job Queue J1 Daily invoices: test e-mail (demo01)" || !strings.Contains(mailer.body, "sent to: ops@example.com; hans@example.com") {
		t.Errorf("message: %q\n%s", mailer.subject, mailer.body)
	}

	mailer.to = nil
	mailer.rejects = map[string]bool{"hans@example.com": true}
	msg, err = run()
	if err != nil || msg != "Test e-mail sent to ops@example.com. Not sent to: hans@example.com (550 no such user)" {
		t.Errorf("partly sent: %q, %v", msg, err)
	}

	mailer.to = nil
	mailer.rejects = map[string]bool{"ops@example.com": true, "hans@example.com": true}
	if _, err := run(); err == nil || !strings.HasPrefix(err.Error(), "The test e-mail could not be sent") {
		t.Errorf("all rejected: error %v", err)
	}

	// The SMTP password was encrypted with a key that is no longer set
	restore := secrets.SetKeyForTest("old key")
	var setup gtables.SMTPSetup
	setup.InitWithDBType(db, "", database.DBTypeSQLite)
	setup.Enabled = true
	setup.Smtp_server = types.NewText("smtp.example.com")
	setup.From_address = types.NewText("jobs@example.com")
	setup.Password = types.NewText("app password")
	if !setup.Insert(true) {
		t.Fatal("insert SMTP setup")
	}
	restore()
	defer secrets.SetKeyForTest("new key")()
	if _, err := run(); err == nil || err.Error() != "The Password in SMTP Setup cannot be read: the encryption key has changed. Enter the password again." {
		t.Errorf("undecryptable password: error %v", err)
	}
}
