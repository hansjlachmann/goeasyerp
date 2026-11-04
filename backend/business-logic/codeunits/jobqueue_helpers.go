package codeunits

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// CreateJobQueueEntry creates a new Job Queue Entry record for the given job queue.
func CreateJobQueueEntry(db database.Executor, company string, dbType database.DBType, jobQueue *gtables.JobQueue, status gtables.JobQueueEntryStatus, errorMsg string, startTime types.DateTime) error {
	var entry gtables.JobQueueEntry
	entry.InitWithDBType(db, company, dbType)

	nextEntryNo := 1
	if entry.FindLast() {
		nextEntryNo = entry.Entry_no + 1
	}

	// Create a fresh entry to avoid stale fields from FindLast (e.g. previous error_message)
	entry = gtables.JobQueueEntry{}
	entry.InitWithDBType(db, company, dbType)
	entry.Entry_no = nextEntryNo
	entry.Status = status
	entry.User_id = types.NewCode(fcodeunits.CurrentUserID())
	entry.Description = jobQueue.Description
	entry.Job_queue_no = jobQueue.No
	entry.Start_date_time = startTime
	entry.End_date_time = types.Now()
	if errorMsg != "" {
		entry.Error_message = types.NewText(errorMsg)
	}

	return entry.InsertOrError(true)
}

// SetJobQueueStatus updates the status of a Job Queue record.
func SetJobQueueStatus(db database.Executor, company string, dbType database.DBType, jobQueueNo types.Code, status gtables.JobQueueStatus) error {
	var jq gtables.JobQueue
	jq.InitWithDBType(db, company, dbType)
	err := jq.GetOrError(jobQueueNo.String())
	if err != nil {
		return err
	}
	jq.Status = status
	return jq.ModifyOrError(true)
}
