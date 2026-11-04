package codeunits

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// HelloWorld - Codeunit 990: Hello World Test
const HelloWorldID = 990
const HelloWorldName = "hello-world"

func init() {
	Register(HelloWorldID, HelloWorldName, NewHelloWorld)
}

type HelloWorld struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewHelloWorld creates a new instance of the codeunit
func NewHelloWorld(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &HelloWorld{
		db:      db,
		company: company,
		dbType:  dbType,
	}
}

// ID returns the codeunit ID
func (c *HelloWorld) ID() int {
	return HelloWorldID
}

// Name returns the codeunit name
func (c *HelloWorld) Name() string {
	return HelloWorldName
}

// SourceTable returns the table this codeunit operates on
func (c *HelloWorld) SourceTable() string {
	return "Job_Queue"
}

// UsesProgress returns false - HelloWorld shows a message dialog, not progress
func (c *HelloWorld) UsesProgress() bool {
	return false
}

// Run executes the codeunit with the given record
func (c *HelloWorld) Run(record interface{}) (fcodeunits.Result, error) {
	jobQueue := record.(*gtables.JobQueue)

	err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", types.Now())
	if err != nil {
		return fcodeunits.Message(err.Error()), nil
	}

	return fcodeunits.Message("Hello World!"), nil
}
