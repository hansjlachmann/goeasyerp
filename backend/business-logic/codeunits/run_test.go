package codeunits

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func TestRun(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, table := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.JobQueue{}, &gtables.JobQueueEntry{}} {
		if err := table.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	var job gtables.JobQueue
	job.InitWithDBType(db, "c1", database.DBTypeSQLite)
	job.No = types.NewCode("HELLO")
	result, err := Run(HelloWorldID, &job)
	if err != nil || !result.Success || result.Dialog == nil || result.Dialog.Message != "Hello World!" {
		t.Errorf("Run(Hello World) = %+v, %v", result, err)
	}

	var customer gtables.Customer
	customer.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if _, err := Run(HelloWorldID, &customer); err == nil || !strings.Contains(err.Error(), "runs on Job_Queue") {
		t.Errorf("record of another table: %v", err)
	}
	if _, err := Run(12345, &job); err == nil {
		t.Error("a codeunit that does not exist ran")
	}
}
