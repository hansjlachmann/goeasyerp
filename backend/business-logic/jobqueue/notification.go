// Package jobqueue holds the Job Queue's e-mail notifications, shared by the scheduler
// (after a run) and the "Send Test E-mail" codeunit.
package jobqueue

import (
	// The table triggers (business logic) run for the records written here
	"fmt"
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strconv"
	"strings"
	"time"

	// The table triggers (business logic) run for the records written here
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
)

// Run is the outcome of one run of a job, for its notification
type Run struct {
	Start, End time.Time
	Err        error
	// Printout: a report job's PDF, attached to the notification
	Printout *Printout
}

// Printout is a scheduled report's PDF and its file name
type Printout struct {
	FileName string
	PDF      []byte
}

// ShouldNotify reports whether a run's outcome is e-mailed, per the job's Notify On
func ShouldNotify(job *gtables.JobQueue, runErr error) bool {
	if len(mail.SplitAddressList(job.Notification_email.String())) == 0 {
		return false
	}
	switch job.Notify_on {
	case gtables.JobQueue_Notify_on.Always:
		return true
	case gtables.JobQueue_Notify_on.OnError:
		return runErr != nil
	}
	return false
}

// Recipients returns the job's notification addresses
func Recipients(job *gtables.JobQueue) []string {
	return mail.SplitAddressList(job.Notification_email.String())
}

// Language returns the translation language of the job's notifications: the
// Translation Key of its Notification Language, else the default language.
func Language(db database.Executor, dbType database.DBType, company string, job *gtables.JobQueue) string {
	ts := i18n.GetInstance()
	if job.Notification_language.IsEmpty() {
		return ts.GetDefaultLanguage()
	}
	var language gtables.Language
	language.InitWithDBType(db, company, dbType)
	if language.Get(job.Notification_language) && !language.Translation_key.IsEmpty() {
		return language.Translation_key.String()
	}
	return NormalizeLanguage(job.Notification_language.String())
}

// NormalizeLanguage turns a language code into a translation key: "NB-NO" → "nb-NO"
func NormalizeLanguage(code string) string {
	lang, region, ok := strings.Cut(code, "-")
	if ok {
		return strings.ToLower(lang) + "-" + strings.ToUpper(region)
	}
	return strings.ToLower(code)
}

// CompanyName returns the company's display name (its technical name when it has none)
func CompanyName(db database.Executor, dbType database.DBType, company string) string {
	var c gtables.Company
	c.InitWithDBType(db, "", dbType)
	if c.Get(company) && !c.Display_name.IsEmpty() {
		return c.Display_name.String()
	}
	return company
}

// Message is a notification e-mail: subject and plain-text body
type Message struct {
	Subject, Body string
}

// RunMessage is the notification of a run, in language lang. job is the job as saved
// after the run (its status and next start are reported).
func RunMessage(job *gtables.JobQueue, companyName string, run Run, lang string) Message {
	ts := i18n.GetInstance()
	subjectKey := "JOBQUEUE_MAIL_SUBJECT_SUCCESS"
	result := ts.Message("JOBQUEUE_MAIL_SUCCEEDED", lang)
	if run.Err != nil {
		subjectKey = "JOBQUEUE_MAIL_SUBJECT_ERROR"
		result = ts.Message("JOBQUEUE_MAIL_FAILED", lang)
	}

	b := newBody(lang)
	b.line(ts.TableCaption("Company", lang), companyName)
	b.job(job)
	b.line(ts.Message("JOBQUEUE_MAIL_RESULT", lang), result)
	b.line(ts.Message("JOBQUEUE_MAIL_STARTED", lang), formatTime(run.Start))
	b.line(ts.Message("JOBQUEUE_MAIL_FINISHED", lang), formatTime(run.End))
	if job.Status == gtables.JobQueue_Status.Ready && !job.Next_start.IsZero() {
		b.field(job, "next_start", formatTime(job.Next_start.Time))
	}
	if run.Err != nil {
		b.text.WriteString("\n" + ts.Message("JOBQUEUE_MAIL_ERROR", lang) + ":\n" + run.Err.Error() + "\n")
		if job.Status == gtables.JobQueue_Status.Error {
			b.text.WriteString("\n" + ts.Message("JOBQUEUE_MAIL_STOPPED", lang) + "\n")
		}
	}
	return Message{
		Subject: ts.MessageWithParams(subjectKey, lang, jobTitle(job), companyName),
		Body:    b.text.String(),
	}
}

// TestMessage is the "Send Test E-mail" message of a job, in language lang
func TestMessage(job *gtables.JobQueue, companyName string, lang string) Message {
	ts := i18n.GetInstance()
	b := newBody(lang)
	b.text.WriteString(ts.MessageWithParams("JOBQUEUE_MAIL_TEST_INTRO", lang, mail.JoinAddressList(Recipients(job))) + "\n\n")
	b.line(ts.TableCaption("Company", lang), companyName)
	b.job(job)
	b.field(job, "notify_on", job.GetOptionCaption("notify_on", optionKey(job.Notify_on.String()), lang))
	if job.Status == gtables.JobQueue_Status.Ready && !job.Next_start.IsZero() {
		b.field(job, "next_start", formatTime(job.Next_start.Time))
	}
	return Message{
		Subject: ts.MessageWithParams("JOBQUEUE_MAIL_SUBJECT_TEST", lang, jobTitle(job), companyName),
		Body:    b.text.String(),
	}
}

// jobTitle names a job in a subject: "JOB01 Daily invoices", or only its No.
func jobTitle(job *gtables.JobQueue) string {
	desc := strings.TrimSpace(job.Description.String())
	if desc != "" {
		return job.No.String() + " " + desc
	}
	return job.No.String()
}

// optionKey turns an option value into its translation key: "On Error" → "on_error"
func optionKey(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, " ", "_"))
}

// formatTime writes a time with its zone, the same in every language: e-mails are read
// without the user's locale settings.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05 MST")
}

// body builds "Caption: value" lines
type body struct {
	lang string
	text strings.Builder
}

func newBody(lang string) *body { return &body{lang: lang} }

func (b *body) line(caption, value string) {
	fmt.Fprintf(&b.text, "%s: %s\n", caption, value)
}

func (b *body) field(job *gtables.JobQueue, field, value string) {
	b.line(i18n.GetInstance().FieldCaption(gtables.JobQueueTableName, field, b.lang), value)
}

// job writes the job's No., Description, codeunit and parameter
func (b *body) job(job *gtables.JobQueue) {
	b.field(job, "no", job.No.String())
	desc := job.Description.String()
	if desc != "" {
		b.field(job, "description", desc)
	}
	b.field(job, "object_id_to_run", strconv.Itoa(job.Object_id_to_run))
	param := job.Parameter.String()
	if param != "" {
		b.field(job, "parameter", param)
	}
}

// LoadSMTPConfig reads the single SMTP_Setup record (a setup table with a
// blank primary key) and builds a mail.Config. If the record is missing or not
// enabled, a disabled config is returned. The stored password is decrypted; when that
// fails (secrets.ErrDecrypt: the encryption key changed) the config is disabled and
// the error returned — the password has to be entered again.
func LoadSMTPConfig(db database.Executor, dbType database.DBType) (mail.Config, error) {
	var setup gtables.SMTPSetup
	// SMTP_Setup is a global setup table; the company argument is ignored and the
	// single record is keyed by a blank primary key.
	setup.InitWithDBType(db, "", dbType)

	if setup.Get("") && setup.Enabled {
		password, err := setup.PlainPassword()
		if err != nil {
			return mail.Config{Enabled: false}, err
		}
		port := setup.Smtp_server_port
		if port <= 0 {
			port = 587
		}
		return mail.Config{
			Enabled:  true,
			Host:     setup.Smtp_server.String(),
			Port:     strconv.Itoa(port),
			Username: setup.User_id.String(),
			Password: password,
			From:     setup.From_address.String(),
		}, nil
	}
	return mail.Config{Enabled: false}, nil
}
