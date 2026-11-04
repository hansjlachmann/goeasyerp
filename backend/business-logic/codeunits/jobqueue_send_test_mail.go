package codeunits

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/jobqueue"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// JobQueueSendTestMail - Codeunit 455: Job Queue Send Test E-mail
//
// The "Send Test E-mail" action of the Job Queue: sends a test notification of the
// selected job to its Notification E-mail addresses, in its Notification Language,
// without running the job. Checks the addresses and the SMTP Setup.
const JobQueueSendTestMailID = 455
const JobQueueSendTestMailName = "jobqueue-send-test-mail"

func init() {
	Register(JobQueueSendTestMailID, JobQueueSendTestMailName, NewJobQueueSendTestMail)
}

// newTestMailer builds the mailer from the SMTP Setup (replaced by tests)
var newTestMailer = func(cfg mail.Config) mail.Sender { return mail.NewSMTPMailer(cfg) }

type JobQueueSendTestMail struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewJobQueueSendTestMail creates a new instance of the codeunit
func NewJobQueueSendTestMail(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &JobQueueSendTestMail{db: db, company: company, dbType: dbType}
}

// ID returns the codeunit ID
func (c *JobQueueSendTestMail) ID() int { return JobQueueSendTestMailID }

// Name returns the codeunit name
func (c *JobQueueSendTestMail) Name() string { return JobQueueSendTestMailName }

// SourceTable returns the table this codeunit operates on
func (c *JobQueueSendTestMail) SourceTable() string { return "Job_Queue" }

// UsesProgress returns true - connecting to the SMTP server can take a while
func (c *JobQueueSendTestMail) UsesProgress() bool { return true }

// Run sends the test e-mail of the job (as saved: the record from the page only names it)
func (c *JobQueueSendTestMail) Run(record interface{}) (fcodeunits.Result, error) {
	selected, ok := record.(*gtables.JobQueue)
	if !ok {
		return fcodeunits.Result{}, fmt.Errorf("expected *JobQueue record, got %T", record)
	}
	ts := i18n.GetInstance()
	lang := jobqueue.NormalizeLanguage(fcodeunits.CurrentLanguage())

	var job gtables.JobQueue
	job.InitWithDBType(c.db, c.company, c.dbType)
	if !job.Get(selected.No.String()) {
		return fcodeunits.Result{}, apperrors.RecordNotFound(ts.TableCaption(gtables.JobQueueTableName, lang), selected.No.String())
	}
	recipients := jobqueue.Recipients(&job)
	if len(recipients) == 0 {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("JOBQUEUE_TEST_MAIL_NO_RECIPIENTS", lang,
			job.No.String(), ts.FieldCaption(gtables.JobQueueTableName, "notification_email", lang)))
	}
	_, err := mail.ParseAddressList(job.Notification_email.String())
	if err != nil {
		var listErr *mail.AddressListError
		if errors.As(err, &listErr) && listErr.Duplicate {
			return fcodeunits.Result{}, errors.New(apperrors.EmailDuplicate(gtables.JobQueueTableName, "notification_email", listErr.Address).Message(lang))
		}
		if errors.As(err, &listErr) {
			return fcodeunits.Result{}, errors.New(apperrors.EmailInvalid(gtables.JobQueueTableName, "notification_email", listErr.Address).Message(lang))
		}
		return fcodeunits.Result{}, err
	}
	cfg, err := jobqueue.LoadSMTPConfig(c.db, c.dbType)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("JOBQUEUE_TEST_MAIL_BAD_PASSWORD", lang,
			ts.FieldCaption(gtables.SMTPSetupTableName, "password", lang), ts.TableCaption(gtables.SMTPSetupTableName, lang)))
	}
	mailer := newTestMailer(cfg)
	if !mailer.Enabled() {
		return fcodeunits.Result{}, errors.New(ts.Message("JOBQUEUE_TEST_MAIL_NO_SMTP", lang))
	}

	msg := jobqueue.TestMessage(&job, jobqueue.CompanyName(c.db, c.dbType, c.company), jobqueue.Language(c.db, c.dbType, c.company, &job))
	err = mailer.Send(recipients, msg.Subject, msg.Body)
	var rcptErr *mail.RecipientsError
	switch {
	case err == nil:
		text := ts.MessageWithParams("JOBQUEUE_TEST_MAIL_SENT", lang, mail.JoinAddressList(recipients))
		return fcodeunits.Result{Success: true, Message: text, Dialog: &fcodeunits.DialogResult{Message: text, Type: "success"}}, nil
	case errors.As(err, &rcptErr) && rcptErr.Sent > 0:
		rejected := make(map[string]bool, len(rcptErr.Rejected))
		var notSent []string
		for _, r := range rcptErr.Rejected {
			rejected[r.Recipient] = true
			notSent = append(notSent, fmt.Sprintf("%s (%v)", r.Recipient, r.Err))
		}
		var sent []string
		for _, r := range recipients {
			if !rejected[r] {
				sent = append(sent, r)
			}
		}
		text := ts.MessageWithParams("JOBQUEUE_TEST_MAIL_PARTIAL", lang, mail.JoinAddressList(sent), strings.Join(notSent, "; "))
		return fcodeunits.Result{Success: true, Message: text, Dialog: &fcodeunits.DialogResult{Message: text, Type: "warning"}}, nil
	default:
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("JOBQUEUE_TEST_MAIL_FAILED", lang, err.Error()))
	}
}
