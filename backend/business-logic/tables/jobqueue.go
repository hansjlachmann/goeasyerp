package tables

import (
	"errors"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// jobQueueLogic is the business logic of table 472 Job_Queue. The notification addresses
// are checked by OnValidate_Notification_email when a user changes them, not on Modify: a
// list saved before that check must not stop the scheduler from rescheduling the job (it
// sends to the valid addresses and logs the others).
type jobQueueLogic struct{}

func init() {
	gtables.RegisterJobQueueLogic(jobQueueLogic{})
	// Functions of the Job Queue Card's "Job Queue" menu (examples of card page functions)
	functions.Register("table", gtables.JobQueueTableName, "ShowStatus", functions.For(jobQueueLogic{}.ShowStatus))
	functions.Register("table", gtables.JobQueueTableName, "AddTestEntry", functions.For(jobQueueLogic{}.AddTestEntry))
}

// OnInsert: the job runs as the user who sets it up (setJobQueueUser)
func (jobQueueLogic) OnInsert(rec *gtables.JobQueue) error {
	return setJobQueueUser(rec)
}

// OnModify: the job runs as the user who saves it (setJobQueueUser)
func (jobQueueLogic) OnModify(rec *gtables.JobQueue) error {
	return setJobQueueUser(rec)
}

// setJobQueueUser: a user who saves a job must be allowed to run its codeunit or report
// (Execute permission), and the job runs as that user. The scheduler's own changes (status, next start)
// have no user and leave the job's user as it is.
func setJobQueueUser(rec *gtables.JobQueue) error {
	userID := system.UserId()
	if userID == "" || userID == "SCHEDULER" {
		return nil
	}
	if rec.Object_id_to_run != 0 {
		err := JobQueueCanRun(rec, userID, fcodeunits.CanExecute)
		if err != nil {
			return err
		}
	}
	rec.User_id = types.NewCode(userID)
	return nil
}

// JobQueueCanRun: the user may run the job's codeunit or report (Execute permission, checked by
// canExecute — the current user's, or a scheduled job user's session)
func JobQueueCanRun(rec *gtables.JobQueue, userID string, canExecute func(objectType string, id int) bool) error {
	if rec.Object_type_to_run == gtables.JobQueue_Object_type_to_run.Report {
		if !canExecute(session.ObjectReport, rec.Object_id_to_run) {
			return apperrors.JobNoReportPermission(userID, rec.Object_id_to_run)
		}
		return nil
	}
	if !canExecute(session.ObjectCodeunit, rec.Object_id_to_run) {
		return apperrors.JobNoPermission(userID, rec.Object_id_to_run)
	}
	return nil
}

// ShowStatus shows the job queue's status in a message (a card page function)
func (jobQueueLogic) ShowStatus(rec *gtables.JobQueue) (fcodeunits.Result, error) {
	text := i18n.GetInstance().Message("JOBQUEUE_STATUS_IS", fcodeunits.CurrentLanguage())
	return system.Message(text, rec.No, rec.Status.String()), nil
}

// AddTestEntry adds a log entry for the job queue and asks the card to refresh, so the new
// line shows in its Log Entries subpage at once (a header function that changes the lines)
func (jobQueueLogic) AddTestEntry(rec *gtables.JobQueue) (fcodeunits.Result, error) {
	entry := gtables.NewJobQueueEntry(rec)
	next := 1
	if entry.FindLast() {
		next = entry.Entry_no + 1
	}
	entry.Init()
	entry.Entry_no = next
	entry.Job_queue_no = rec.No
	entry.Status = gtables.JobQueueEntry_Status.Success
	entry.Description = types.NewText(i18n.GetInstance().Message("JOBQUEUE_TEST_ENTRY", fcodeunits.CurrentLanguage()))
	entry.Start_date_time = types.Now()
	entry.End_date_time = types.Now()
	err := entry.InsertOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(i18n.GetInstance().Message("JOBQUEUE_TEST_ENTRY_ADDED", fcodeunits.CurrentLanguage()), next)
	result.Refresh = true
	return result, nil
}

// OnValidate_Notification_email checks the notification address list (one or more
// addresses separated by ";") and stores it tidied up: "a@x.no; b@x.no".
func (jobQueueLogic) OnValidate_Notification_email(rec *gtables.JobQueue) error {
	items, err := mail.ParseAddressList(rec.Notification_email.String())
	if err != nil {
		var listErr *mail.AddressListError
		if errors.As(err, &listErr) && listErr.Duplicate {
			return apperrors.EmailDuplicate(gtables.JobQueueTableName, "notification_email", listErr.Address)
		}
		if errors.As(err, &listErr) {
			return apperrors.EmailInvalid(gtables.JobQueueTableName, "notification_email", listErr.Address)
		}
		return err
	}
	rec.Notification_email = types.NewText(mail.JoinAddressList(items))
	return ftables.CheckMaxLength(gtables.JobQueueTableName, "notification_email", string(rec.Notification_email), 250)
}
