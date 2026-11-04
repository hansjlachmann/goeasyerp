package tables

import (
	"encoding/base64"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// jobQueueEntryLogic is the business logic of table 474 Job_Queue_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/jobqueueentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type jobQueueEntryLogic struct{}

func init() {
	gtables.RegisterJobQueueEntryLogic(jobQueueEntryLogic{})
	// Functions pages call with the current line (run_object: table:Job_Queue_Entry.<name>)
	functions.Register("table", gtables.JobQueueEntryTableName, "ShowLineNo", functions.For(jobQueueEntryLogic{}.ShowLineNo))
	functions.Register("table", gtables.JobQueueEntryTableName, "Test", functions.For(jobQueueEntryLogic{}.Test))
	functions.Register("table", gtables.JobQueueEntryTableName, "ShowLine", functions.ForArgs(jobQueueEntryLogic{}.ShowLine, "int"))
	// Show Printout on the log: a scheduled report's PDF (preview, print or download)
	functions.Register("table", gtables.JobQueueEntryTableName, "ShowPrintout", functions.For(JobQueueEntryShowPrintout), functions.WithDialog())
}

// JobQueueEntryShowPrintout shows the printout a scheduled report run left in its log entry
func JobQueueEntryShowPrintout(rec *gtables.JobQueueEntry) (fcodeunits.Result, error) {
	entry := gtables.NewJobQueueEntry(rec)
	if !entry.Get(rec.Entry_no) || len(entry.Printout) == 0 {
		return fcodeunits.Result{}, system.Error(i18n.GetInstance().Message("JOBQUEUE_NO_PRINTOUT", fcodeunits.CurrentLanguage()), rec.Entry_no)
	}
	return fcodeunits.Result{Success: true, Data: map[string]interface{}{
		"pdf":      base64.StdEncoding.EncodeToString(entry.Printout),
		"filename": entry.Printout_file_name.String(),
		"output":   "preview",
	}}, nil
}

// ShowLine shows one field of the current line — the argument is an option
// "Entry No.,Description,Status" (sub-menu Line → Show Line: an example of arguments)
func (jobQueueEntryLogic) ShowLine(rec *gtables.JobQueueEntry, args functions.Args) (fcodeunits.Result, error) {
	language := fcodeunits.CurrentLanguage()
	var field string
	var value any
	switch args.Int(0) {
	case 0:
		field, value = "entry_no", rec.Entry_no
	case 1:
		field, value = "description", rec.Description
	default:
		field, value = "status", rec.Status.String()
	}
	text := i18n.GetInstance().Message("JOBQUEUEENTRY_SHOW_FIELD", language)
	return system.Message(text, rec.GetFieldCaption(field, language), value), nil
}

// Test shows a simple message (button "TEST" on the Job Queue Entries page: a plain action
// button calling a page function)
func (jobQueueEntryLogic) Test(rec *gtables.JobQueueEntry) (fcodeunits.Result, error) {
	text := i18n.GetInstance().Message("JOBQUEUEENTRY_TEST", fcodeunits.CurrentLanguage())
	return system.Message(text, rec.Entry_no), nil
}

// ShowLineNo shows the number of the current line in a message (menu item "Test Line 1" on
// the Job Queue Entries page: an example of a page function on a table)
func (jobQueueEntryLogic) ShowLineNo(rec *gtables.JobQueueEntry) (fcodeunits.Result, error) {
	text := i18n.GetInstance().Message("JOBQUEUEENTRY_LINE_NO", fcodeunits.CurrentLanguage())
	return system.Message(text, rec.Entry_no, rec.Job_queue_no), nil
}

// OnInsert runs before a new record is inserted
func (jobQueueEntryLogic) OnInsert(rec *gtables.JobQueueEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (jobQueueEntryLogic) OnModify(rec *gtables.JobQueueEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (jobQueueEntryLogic) OnDelete(rec *gtables.JobQueueEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (jobQueueEntryLogic) OnRename(rec *gtables.JobQueueEntry) error {
	return nil
}
