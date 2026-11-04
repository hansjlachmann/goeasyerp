package codeunits

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"time"
)

// ProgressDemo - Codeunit 991: Progress Bar Demo
const ProgressDemoID = 991
const ProgressDemoName = "progress-demo"

func init() {
	Register(ProgressDemoID, ProgressDemoName, NewProgressDemo)
}

type ProgressDemo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewProgressDemo creates a new instance of the codeunit
func NewProgressDemo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ProgressDemo{
		db:      db,
		company: company,
		dbType:  dbType,
	}
}

// ID returns the codeunit ID
func (c *ProgressDemo) ID() int {
	return ProgressDemoID
}

// Name returns the codeunit name
func (c *ProgressDemo) Name() string {
	return ProgressDemoName
}

// SourceTable returns the table this codeunit operates on
func (c *ProgressDemo) SourceTable() string {
	return "Job_Queue"
}

// UsesProgress returns true - this codeunit uses progress dialog updates
func (c *ProgressDemo) UsesProgress() bool {
	return true
}

// Run executes the codeunit with progress updates
func (c *ProgressDemo) Run(record interface{}) (fcodeunits.Result, error) {
	// Let the user choose the speed (StrMenu), open the project page (Hyperlink), or cancel
	speed := system.StrMenu("Fast,Normal,Slow,Open the project page,Pick a customer", 2, "Start progress demo: choose the speed")
	switch speed {
	case 0:
		return fcodeunits.Message("Progress demo cancelled."), nil
	case 4:
		err := system.Hyperlink("https://github.com/hansjlachmann/goeasyerp")
		if err != nil {
			return fcodeunits.Result{}, err
		}
		return fcodeunits.Message("Project page opened."), nil
	}

	// "Pick a customer": a page variable — the Customer List in lookup mode (OK / Cancel), the
	// picked customer read into cust; then the demo runs at normal speed for that customer
	customer := "(none)"
	if speed == 5 {
		var cust gtables.Customer
		cust.InitWithDBType(c.db, c.company, c.dbType)
		custList := system.NewPage(22)
		custList.SetTableView(&cust)
		custList.SetRecord(&cust)
		custList.LookupMode(true)
		if custList.RunModal() != system.ActionLookupOK {
			return fcodeunits.Message("Lookup cancelled: no customer picked."), nil
		}
		custList.GetRecord(&cust)
		customer = system.StrSubstNo("%1 %2", cust.No, cust.Name)
		speed = 2
	}
	stepTime := []time.Duration{100, 500, 1500}[speed-1] * time.Millisecond

	jobQueue := record.(*gtables.JobQueue)

	// The job queues selected on the list (SetSelectionFilter; the current one when only
	// one is selected)
	jobQueue.SetSelectionFilter()
	selected := jobQueue.Count()

	// Simulate a long-running process in a progress window: text fields #n###, a bar @n@@@
	totalSteps := 10
	window := system.OpenWindow("Customer: #6##########\nJob queues selected: #5##\nStep #1## of #2##\n#3####################\n@4@@@@@@@@@@@@@@@@", 0, totalSteps, "", 0, selected, customer)
	defer window.Close()
	for i := 1; i <= totalSteps; i++ {
		window.Update(1, i)
		window.Update(3, getProgressMessage(i))
		window.Update(4, i*10000/totalSteps)

		// Simulate work
		time.Sleep(stepTime)
	}

	err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", types.Now())
	if err != nil {
		return fcodeunits.Message(err.Error()), nil
	}

	if customer != "(none)" {
		return system.Message("Processing completed successfully for customer %1.", customer), nil
	}
	return fcodeunits.Message("Processing completed successfully!"), nil
}

// getProgressMessage returns a message for each step
func getProgressMessage(step int) string {
	messages := []string{
		"Initializing...",
		"Loading data...",
		"Processing records...",
		"Validating entries...",
		"Updating database...",
		"Calculating totals...",
		"Generating reports...",
		"Finalizing changes...",
		"Cleaning up...",
		"Completing...",
	}

	if step > 0 && step <= len(messages) {
		return messages[step-1]
	}
	return "Processing..."
}
