package codeunits

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func init() {
	// Run (F9) on the Job Queue page: the job's codeunit or report now
	functions.Register("table", gtables.JobQueueTableName, "Run", functions.For(RunJobQueue), functions.WithDialog())
}

// RunJobQueue runs a job now (Run on the Job Queue page), as the user who runs it — who must be
// allowed to run the job's object: its codeunit with the job as its record, or its report with
// the request page, its options and filters filled in from the job's Parameter
// (reports.ParseJobParameter); the printout is shown as any report's
func RunJobQueue(job *gtables.JobQueue) (fcodeunits.Result, error) {
	err := tables.JobQueueCanRun(job, fcodeunits.CurrentUserID(), fcodeunits.CanExecute)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if job.Object_type_to_run == gtables.JobQueue_Object_type_to_run.Report {
		if _, ok := reports.Get(job.Object_id_to_run); !ok {
			return fcodeunits.Result{}, fmt.Errorf("report %d not found", job.Object_id_to_run)
		}
		options, filters := reports.ParseJobParameter(job.Parameter.String())
		result, err := reports.Execute(job.Object_id_to_run, job.GetDB(), job.GetCompany(), job.GetDBType(),
			reports.RunOptions{RequestPage: true, Options: options, Filters: filters})
		if err != nil {
			return fcodeunits.Result{}, err
		}
		if result.Cancelled {
			return fcodeunits.Result{Success: true}, nil
		}
		return reports.CodeunitResult(result), nil
	}
	factory, ok := Get(job.Object_id_to_run)
	if !ok {
		return fcodeunits.Result{}, fmt.Errorf("routine %d not found", job.Object_id_to_run)
	}
	return factory(job.GetDB(), job.GetCompany(), job.GetDBType()).Run(job)
}
