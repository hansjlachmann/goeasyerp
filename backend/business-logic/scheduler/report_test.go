package scheduler

import (
	"bytes"
	"database/sql"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// attachmentSender records the messages and their attachments
type attachmentSender struct {
	mockSender
	attachments [][]mail.Attachment
}

func (m *attachmentSender) SendWithAttachments(to []string, subject, body string, attachments []mail.Attachment) error {
	m.attachments = append(m.attachments, attachments)
	return m.Send(to, subject, body)
}

// A job with Object Type to Run Report runs the report without a request page — its options and
// filters from the Parameter —, logs the run as a Job Queue Entry with the printout, and attaches
// the PDF to the notification; a report that does not exist sets the job to Error
func TestRunReportJob(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, testCompany, database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, testCompany, database.DBTypeSQLite)
	for _, no := range []string{"C1", "C2", "D1"} {
		cust := gtables.NewCustomer(src)
		cust.No, cust.Name = types.Code(no), types.Text("Customer "+no)
		if err := cust.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	ms := &attachmentSender{}
	s := &Scheduler{db: db, dbType: database.DBTypeSQLite, mailer: ms}

	seedJob(t, db, "CUSTLIST", gtables.JobQueue_Recurrence.Once, 0)
	job := getJob(t, db, "CUSTLIST")
	job.Object_type_to_run = gtables.JobQueue_Object_type_to_run.Report
	job.Object_id_to_run = reports.CustomerListID
	job.Parameter = "Customer.no=C*"
	job.Notification_email = "ops@example.com"
	job.Notify_on = gtables.JobQueue_Notify_on.Always
	if !job.Modify(false) {
		t.Fatal("modify job")
	}
	s.runJob(testCompany, &job)

	after := getJob(t, db, "CUSTLIST")
	if after.Status != gtables.JobQueue_Status.OnHold {
		t.Errorf("status after the run %v, want On Hold (Once)", after.Status)
	}
	entry := gtables.NewJobQueueEntry(src)
	entry.SetRange("job_queue_no", types.Code("CUSTLIST"))
	if !entry.FindFirst() || entry.Entry_no != 1 || entry.Status != gtables.JobQueueEntry_Status.Success || !bytes.HasPrefix(entry.Printout, []byte("%PDF-")) || entry.Printout_file_name == "" {
		t.Fatalf("log entry: %+v (printout %d bytes)", entry.ToMap(), len(entry.Printout))
	}
	// the filter of the parameter: C1 and C2, not D1
	options, filters := reports.ParseJobParameter(job.Parameter.String())
	printed, err := reports.Execute(reports.CustomerListID, db, testCompany, database.DBTypeSQLite, reports.RunOptions{Options: options, Filters: filters, Output: reports.OutputPDF})
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]bool{}
	for _, text := range printed.Texts {
		texts[text.Text] = true
	}
	if !texts["C1"] || !texts["C2"] || texts["D1"] {
		t.Errorf("the parameter's filter: C1 %v, C2 %v, D1 %v", texts["C1"], texts["C2"], texts["D1"])
	}
	result, err := tables.JobQueueEntryShowPrintout(entry)
	if err != nil || result.Data["pdf"] == "" {
		t.Errorf("Show Printout: %v %v", err, result)
	}
	if len(ms.attachments) != 1 || len(ms.attachments[0]) != 1 || !bytes.Equal(ms.attachments[0][0].Data, entry.Printout) || ms.attachments[0][0].ContentType != "application/pdf" {
		t.Errorf("notification attachments: %d", len(ms.attachments))
	}

	seedJob(t, db, "NOREPORT", gtables.JobQueue_Recurrence.Once, 0)
	job = getJob(t, db, "NOREPORT")
	job.Object_type_to_run = gtables.JobQueue_Object_type_to_run.Report
	job.Object_id_to_run = 99999
	if !job.Modify(false) {
		t.Fatal("modify job")
	}
	s.runJob(testCompany, &job)
	if after := getJob(t, db, "NOREPORT"); after.Status != gtables.JobQueue_Status.Error {
		t.Errorf("a missing report: status %v, want Error", after.Status)
	}
}

// The parameter's options and data item filters
func TestParseJobParameter(t *testing.T) {
	options, filters := reports.ParseJobParameter("print_details=true; Customer.no=C*\nCustomer.date_filter=..2026-12-31;bad")
	if options["print_details"] != "true" || len(options) != 1 {
		t.Errorf("options %v", options)
	}
	if f := filters["Customer"]; len(f) != 2 || f[0].Field != "no" || f[0].Expression != "C*" || f[1].Field != "date_filter" || f[1].Expression != "..2026-12-31" {
		t.Errorf("filters %v", filters)
	}
}
