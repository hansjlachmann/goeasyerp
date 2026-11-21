package handlers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userCodeunit returns the user its run sees (USERID)
type userCodeunit struct{}

func (userCodeunit) ID() int             { return 99991 }
func (userCodeunit) Name() string        { return "permission-test" }
func (userCodeunit) SourceTable() string { return "" }
func (userCodeunit) UsesProgress() bool  { return false }
func (userCodeunit) Run(interface{}) (fcodeunits.Result, error) {
	return fcodeunits.Result{Success: true, Message: "ran " + fmt.Sprint(fcodeunits.CanExecute(session.ObjectCodeunit, 99991))}, nil
}

// readerSession has a role with the given permission rows (not SUPER)
func readerSession(perms map[string]session.TablePermission) *session.Session {
	s := &session.Session{UserID: "READER1", Company: "TEST", Language: "en-US"}
	s.SetPermissions(true, false, perms)
	return s
}

// test functions of the permission check (registered once): a table function on Customer and
// on Job_Queue, codeunit functions on codeunits 6 and 900 (names of their own)
func init() {
	noop := functions.ForArgs(func(rec *gtables.Customer, _ functions.Args) (fcodeunits.Result, error) {
		return fcodeunits.Result{}, nil
	}, "int")
	functions.Register("table", "Customer", "PermTest", functions.For(func(rec *gtables.Customer) (fcodeunits.Result, error) { return fcodeunits.Result{}, nil }))
	functions.Register("table", "Job_Queue", "PermTest", functions.For(func(rec *gtables.JobQueue) (fcodeunits.Result, error) { return fcodeunits.Result{}, nil }))
	functions.Register("codeunit", "6", "PermTest", functions.For(func(rec *gtables.Customer) (fcodeunits.Result, error) { return fcodeunits.Result{}, nil }), functions.OnTable("Customer"))
	functions.Register("codeunit", "900", "PermTest", noop, functions.OnTable("Customer"))
}

func TestRunPermissionError(t *testing.T) {
	reader := readerSession(map[string]session.TablePermission{
		"CODEUNIT 6":   {Execute: true},
		"REPORT 93":    {Execute: true},
		"CUSTOMER":     {Read: true, Modify: true},
		"JOB_QUEUE":    {Read: true},
		"CODEUNIT 455": {Read: true},
	})
	tests := []struct {
		name       string
		codeunitID int
		reportID   int
		function   string
		allowed    bool
	}{
		{"codeunit with Execute", 6, 0, "", true},
		{"codeunit without a row", 7, 0, "", false},
		{"row without Execute", 455, 0, "", false},
		{"report with Execute", 0, 93, "", true},
		{"report without a row", 0, 94, "", false},
		{"table function, Modify on the table", 0, 0, "table:Customer.PermTest", true},
		{"table function, read only", 0, 0, "table:Job_Queue.PermTest", false},
		{"codeunit function with Execute", 0, 0, "codeunit:6.PermTest", true},
		{"codeunit function without", 0, 0, "codeunit:900.PermTest(1)", false},
		// the lookup ignores letter case and spaces: the check must too (no bypass by writing
		// the kind in capitals)
		{"table function in capitals, read only", 0, 0, "TABLE:Job_Queue.PermTest", false},
		{"table function with spaces, read only", 0, 0, " Table:Job_Queue.PermTest", false},
		{"codeunit function in capitals, without", 0, 0, "CODEUNIT:900.PermTest(1)", false},
		{"a function that is not registered", 0, 0, "table:Customer.NoSuchFunction", false},
	}
	for _, tt := range tests {
		denied := runPermissionError(reader, tt.codeunitID, tt.reportID, tt.function, "en-US")
		if (denied == nil) != tt.allowed {
			t.Errorf("%s: allowed=%v, got %v", tt.name, tt.allowed, denied)
		}
	}
	// SUPER runs everything, a user without roles nothing
	super := &session.Session{}
	super.SetPermissions(true, true, nil)
	if denied := runPermissionError(super, 7, 0, "", "en-US"); denied != nil {
		t.Errorf("SUPER denied: %v", denied)
	}
	if denied := runPermissionError(super, 0, 0, "table:Job_Queue.PermTest", "en-US"); denied != nil {
		t.Errorf("SUPER denied a table function: %v", denied)
	}
	noRoles := &session.Session{}
	if runPermissionError(noRoles, 7, 0, "", "en-US") == nil {
		t.Error("a user without roles may run a routine")
	}
	if runPermissionError(noRoles, 0, 0, "table:Job_Queue.PermTest", "en-US") == nil {
		t.Error("a user without roles may run a table function")
	}
}

// The run endpoints refuse a codeunit or report without Execute permission, and the run sees
// the user's permissions
func TestRunEndpointsCheckExecute(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	codeunits.Register(99991, "permission-test", func(database.Executor, string, database.DBType) fcodeunits.Codeunit {
		return userCodeunit{}
	})

	newApp := func(sess *session.Session) *fiber.App {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("session", sess)
			return c.Next()
		})
		app.Post("/api/codeunits/run", NewCodeunitsHandlerWithDBType(db, database.DBTypeSQLite).RunCodeunit)
		app.Post("/api/jobs/start", NewJobsHandlerWithDBType(db, database.DBTypeSQLite).StartJob)
		return app
	}

	denied := newApp(readerSession(nil))
	out := postJSON(t, denied, "/api/codeunits/run", `{"codeunit_id":99991}`)
	if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "permission to run routine 99991") {
		t.Errorf("codeunit without Execute: %v", out)
	}
	out = postJSON(t, denied, "/api/jobs/start", `{"codeunit_id":99991}`)
	if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "permission to run routine 99991") {
		t.Errorf("job start without Execute: %v", out)
	}
	out = postJSON(t, denied, "/api/jobs/start", `{"report_id":`+strconv.Itoa(reports.CreateFiscalYearID)+`,"record":{"starting_date":"2026-01-01"}}`)
	if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "permission to run report "+strconv.Itoa(reports.CreateFiscalYearID)) {
		t.Errorf("report without Execute: %v", out)
	}

	allowed := newApp(readerSession(map[string]session.TablePermission{"CODEUNIT 99991": {Execute: true}}))
	out = postJSON(t, allowed, "/api/codeunits/run", `{"codeunit_id":99991}`)
	data, _ := out["data"].(map[string]interface{})
	if out["success"] != true || data["message"] != "ran true" {
		t.Errorf("codeunit with Execute: %v", out)
	}

	// a routine of a disabled extension app is refused, as by the job API (S24)
	const app = "aaaaaaaa-0000-4000-8000-000000000024"
	extensions.SetObjectApps(func(objectType string, id int) string {
		if id == 99991 {
			return app
		}
		return ""
	})
	extensions.SetDisabled(app)
	t.Cleanup(func() { extensions.SetDisabled(); extensions.SetObjectApps(nil) })
	for _, target := range []string{"/api/codeunits/run", "/api/jobs/start"} {
		out = postJSON(t, allowed, target, `{"codeunit_id":99991}`)
		if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "disabled") {
			t.Errorf("%s: a disabled app's routine: %v", target, out)
		}
	}
}
