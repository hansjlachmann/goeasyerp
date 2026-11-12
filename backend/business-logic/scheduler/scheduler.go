// Package scheduler runs Job Queue jobs automatically on a recurring schedule.
//
// It is an in-process poller (no cron dependency): a ticker wakes periodically,
// acquires a DB-backed distributed lock (so only one instance runs across a
// multi-pod deployment), scans each company's Job_Queue for records that are
// Ready and due (next_start <= now), runs the referenced codeunit, reschedules
// the job per its recurrence, and optionally emails a notification.
package scheduler

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/jobqueue"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Scheduler polls Job_Queue tables and runs due jobs.
type Scheduler struct {
	db        database.Executor
	dbType    database.DBType
	companies func() ([]string, error)
	mailer    mail.Sender

	enabled  bool
	interval time.Duration
	lockTTL  time.Duration
	podID    string

	stop chan struct{}
	done chan struct{}
}

// New builds a Scheduler. Behavior is configured from the environment:
//
//	JOB_QUEUE_ENABLED        "false" to disable the scheduler (default enabled)
//	JOB_QUEUE_POLL_INTERVAL  poll interval in seconds (default 60, min 5)
//
// companies enumerates company names (e.g. companyMgr.ListCompanies). SMTP
// configuration for notifications is read from the global SMTP_Setup table.
func New(db database.Executor, dbType database.DBType, companies func() ([]string, error)) *Scheduler {
	enabled := !strings.EqualFold(os.Getenv("JOB_QUEUE_ENABLED"), "false")

	interval := 60 * time.Second
	v := os.Getenv("JOB_QUEUE_POLL_INTERVAL")
	if v != "" {
		secs, err := strconv.Atoi(v)
		if err == nil && secs >= 5 {
			interval = time.Duration(secs) * time.Second
		}
	}

	podID := os.Getenv("HOSTNAME")
	if podID == "" {
		h, err := os.Hostname()
		if err == nil {
			podID = h
		} else {
			podID = "scheduler"
		}
	}

	return &Scheduler{
		db:        db,
		dbType:    dbType,
		companies: companies,
		enabled:   enabled,
		interval:  interval,
		lockTTL:   5 * time.Minute,
		podID:     podID,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start launches the background poll loop. It is a no-op when disabled.
func (s *Scheduler) Start() {
	if !s.enabled {
		log.Println("Job Queue scheduler disabled (JOB_QUEUE_ENABLED=false)")
		return
	}
	err := s.ensureLockTable()
	if err != nil {
		log.Printf("Job Queue scheduler: failed to create lock table, not starting: %v", err)
		return
	}
	log.Printf("Job Queue scheduler started (interval %s, pod %s)", s.interval, s.podID)
	go s.loop()
}

// Stop signals the loop to exit and waits for it to finish.
func (s *Scheduler) Stop() {
	if !s.enabled {
		return
	}
	close(s.stop)
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		log.Println("Job Queue scheduler: stop timed out")
	}
}

func (s *Scheduler) loop() {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stop:
			return
		}
	}
}

// tick runs one scheduling pass: acquire the lock, then process every company.
func (s *Scheduler) tick() {
	defer func() {
		r := recover()
		if r != nil {
			log.Printf("Job Queue scheduler: recovered from panic in tick: %v", r)
		}
	}()

	acquired, err := s.tryAcquireLock()
	if err != nil {
		log.Printf("Job Queue scheduler: lock error: %v", err)
		return
	}
	if !acquired {
		return // another instance owns this tick
	}
	defer s.releaseLock()

	companies, err := s.companies()
	if err != nil {
		log.Printf("Job Queue scheduler: failed to list companies: %v", err)
		return
	}
	for _, company := range companies {
		s.processCompany(company)
	}
}

// processCompany scans one company's Job_Queue for due jobs and runs them.
func (s *Scheduler) processCompany(company string) {
	var jq gtables.JobQueue
	jq.InitWithDBType(s.db, company, s.dbType)

	now := types.Now()
	var dueNos []string
	for found := jq.FindSet(); found; found = jq.Next() {
		if jq.Status == gtables.JobQueue_Status.Ready && isDue(jq.Next_start, now) {
			dueNos = append(dueNos, jq.No.String())
		}
	}

	// Run after the scan completes so running a job (which may query/modify the
	// same table) can't disturb the FindSet cursor. Re-Get each job fresh.
	for _, no := range dueNos {
		var job gtables.JobQueue
		job.InitWithDBType(s.db, company, s.dbType)
		if job.Get(no) {
			s.runJob(company, &job)
		}
	}
}

// isDue reports whether a job with the given next_start is due to run now.
// A zero next_start means "run as soon as possible".
func isDue(nextStart, now types.DateTime) bool {
	return nextStart.IsZero() || !nextStart.After(now)
}

// runJob executes one job's codeunit and applies the outcome (reschedule + notify).
func (s *Scheduler) runJob(company string, job *gtables.JobQueue) {
	// A job runs as the user who set it up (Job Queue User ID), with that user's permissions:
	// without Execute permission on the codeunit the run fails (the job goes to Error). A job
	// saved before jobs had a user runs as SCHEDULER, unchecked.
	start := time.Now()
	var runErr error
	userID := job.User_id.String()
	if userID == "" {
		fcodeunits.SetCurrentContext("SCHEDULER", "SCHEDULER", company, "en-US")
		fcodeunits.SetCurrentWorkDate(types.Date{}) // a scheduled job runs on today
	} else {
		access := &session.Session{UserID: userID, Company: company}
		access.SetPermissions(session.LoadPermissions(s.db, s.dbType, userID, company))
		fcodeunits.SetCurrentContext(userID, userID, company, "en-US")
		fcodeunits.SetCurrentWorkDate(types.Date{}) // a scheduled job runs on today
		fcodeunits.SetCurrentAccess(access)
		var appErr *apperrors.AppError
		if errors.As(tables.JobQueueCanRun(job, userID, access.CanExecute), &appErr) {
			runErr = errors.New(appErr.Message("en-US"))
		}
	}
	defer fcodeunits.ClearCurrentContext()

	var printout *jobqueue.Printout
	if runErr == nil {
		printout, runErr = s.execute(company, job)
	}
	end := time.Now()
	// Rescheduling is the scheduler's own work: saving the job must not check the job user's
	// permissions again (a user without Execute could otherwise not get the job set to Error)
	fcodeunits.SetCurrentContext("SCHEDULER", "SCHEDULER", company, "en-US")
	fcodeunits.SetCurrentWorkDate(types.Date{}) // a scheduled job runs on today

	s.finish(company, job.No.String(), runErr)
	// The notification reports the job as saved after the run (status, next start)
	var after gtables.JobQueue
	after.InitWithDBType(s.db, company, s.dbType)
	if !after.Get(job.No.String()) {
		after = *job
	}
	s.notify(company, &after, jobqueue.Run{Start: start, End: end, Err: runErr, Printout: printout})
}

// execute runs the job's object: its codeunit, or its report (executeReport)
func (s *Scheduler) execute(company string, job *gtables.JobQueue) (*jobqueue.Printout, error) {
	if job.Object_type_to_run == gtables.JobQueue_Object_type_to_run.Report {
		return s.executeReport(company, job)
	}
	factory, ok := codeunits.Get(job.Object_id_to_run)
	if !ok {
		return nil, fmt.Errorf("routine %d not found", job.Object_id_to_run)
	}
	cu := factory(s.db, company, s.dbType)

	result, err := cu.Run(job)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		msg := result.Message
		if result.Dialog != nil && result.Dialog.Message != "" {
			msg = result.Dialog.Message
		}
		if msg == "" {
			msg = "job reported failure"
		}
		return nil, errors.New(msg)
	}
	return nil, nil
}

// executeReport runs the job's report without a request page — its options and filters from the
// job's Parameter (reports.ParseJobParameter) — and logs the run as a Job Queue Entry: a printed
// report's PDF goes into the entry (Show Printout) and to the notification
func (s *Scheduler) executeReport(company string, job *gtables.JobQueue) (*jobqueue.Printout, error) {
	if _, ok := reports.Get(job.Object_id_to_run); !ok {
		return nil, fmt.Errorf("report %d not found", job.Object_id_to_run)
	}
	options, filters := reports.ParseJobParameter(job.Parameter.String())
	start := types.Now()
	result, err := reports.Execute(job.Object_id_to_run, s.db, company, s.dbType, reports.RunOptions{Options: options, Filters: filters, Output: reports.OutputPDF})
	if err != nil {
		return nil, err
	}
	var printout *jobqueue.Printout
	if result.PDF != nil {
		printout = &jobqueue.Printout{FileName: result.FileName, PDF: result.PDF}
	}
	entry := gtables.NewJobQueueEntry(ftables.DBSource(s.db, company, s.dbType))
	entryNo := 1
	if entry.FindLast() {
		entryNo = entry.Entry_no + 1
	}
	entry.Init()
	entry.Entry_no = entryNo
	entry.Job_queue_no = job.No
	entry.Status = gtables.JobQueueEntry_Status.Success
	entry.User_id = types.Code(system.CopyStr(fcodeunits.CurrentUserID(), 1, 20))
	description := result.Message
	if description == "" {
		description = reports.Caption(job.Object_id_to_run, "en-US")
	}
	entry.Description = types.Text(system.CopyStr(description, 1, 100))
	entry.Start_date_time = start
	entry.End_date_time = types.Now()
	if printout != nil {
		entry.Printout = printout.PDF
		entry.Printout_file_name = types.Text(system.CopyStr(printout.FileName, 1, 250))
	}
	err = entry.InsertOrError(true)
	if err != nil {
		log.Printf("Job Queue scheduler: log entry of %s/%s: %v", company, job.No.String(), err)
	}
	return printout, nil
}

// finish re-reads the job and updates its status / next_start based on the run
// outcome and its recurrence. On error the job is set to Error (stops re-runs
// until a user resets it). A successful Once job is set to On Hold.
func (s *Scheduler) finish(company, jobNo string, runErr error) {
	var job gtables.JobQueue
	job.InitWithDBType(s.db, company, s.dbType)
	if !job.Get(jobNo) {
		return
	}

	if runErr != nil {
		job.Status = gtables.JobQueue_Status.Error
		err := modify(&job)
		if err != nil {
			log.Printf("Job Queue scheduler: failed to set Error status on %s/%s: %v", company, jobNo, err)
		}
		return
	}

	now := types.Now()
	switch job.Recurrence {
	case gtables.JobQueue_Recurrence.Once:
		job.Status = gtables.JobQueue_Status.OnHold // do not reschedule
	case gtables.JobQueue_Recurrence.Minutes:
		mins := job.Minutes_between_run
		if mins < 1 {
			mins = 1
		}
		job.Next_start = now.AddMinutes(mins)
	case gtables.JobQueue_Recurrence.Hourly:
		job.Next_start = now.AddHours(1)
	case gtables.JobQueue_Recurrence.Daily:
		job.Next_start = types.NewDateTimeFromTime(now.Time.AddDate(0, 0, 1))
	case gtables.JobQueue_Recurrence.Weekly:
		job.Next_start = types.NewDateTimeFromTime(now.Time.AddDate(0, 0, 7))
	case gtables.JobQueue_Recurrence.Monthly:
		job.Next_start = types.NewDateTimeFromTime(now.Time.AddDate(0, 1, 0))
	default:
		// Unknown/Once-like: don't reschedule to avoid a busy loop.
		job.Status = gtables.JobQueue_Status.OnHold
	}

	err := modify(&job)
	if err != nil {
		log.Printf("Job Queue scheduler: failed to reschedule %s/%s: %v", company, jobNo, err)
	}
}

// notify e-mails the run's outcome to the job's notification addresses according to
// notify_on, in the job's notification language. A rejected address is logged and
// does not stop the others.
func (s *Scheduler) notify(company string, job *gtables.JobQueue, run jobqueue.Run) {
	if !jobqueue.ShouldNotify(job, run.Err) {
		return
	}
	lang := jobqueue.Language(s.db, s.dbType, company, job)
	msg := jobqueue.RunMessage(job, jobqueue.CompanyName(s.db, s.dbType, company), run, lang)
	mailer := s.currentMailer()
	var err error
	attachments, ok := mailer.(mail.AttachmentSender)
	if ok && run.Printout != nil {
		err = attachments.SendWithAttachments(jobqueue.Recipients(job), msg.Subject, msg.Body,
			[]mail.Attachment{{Name: run.Printout.FileName, ContentType: "application/pdf", Data: run.Printout.PDF}})
	} else {
		err = mailer.Send(jobqueue.Recipients(job), msg.Subject, msg.Body)
	}
	if err != nil {
		log.Printf("Job Queue scheduler: notification e-mail of %s/%s: %v", company, job.No.String(), err)
	}
}

// modify persists changes to a job, returning an error if the write fails.
func modify(job *gtables.JobQueue) error {
	return job.ModifyOrError(true)
}
