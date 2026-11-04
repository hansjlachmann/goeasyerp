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
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
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
		{"table function, Modify on the table", 0, 0, "table:Customer.AssistEdit", true},
		{"table function, read only", 0, 0, "table:Job_Queue.ShowLineNo", false},
		{"codeunit function with Execute", 0, 0, "codeunit:6.PostLine", true},
		{"codeunit function without", 0, 0, "codeunit:900.PostLine(1)", false},
	}
	for _, tt := range tests {
		denied := runPermissionError(reader, tt.codeunitID, tt.reportID, tt.function, "en-US")
		if (denied == nil) != tt.allowed {
			t.Errorf("%s: allowed=%v, got %v", tt.name, tt.allowed, denied)
		}
	}
	// SUPER and users without roles run everything
	super := &session.Session{}
	super.SetPermissions(true, true, nil)
	for _, s := range []*session.Session{super, {}} {
		if denied := runPermissionError(s, 7, 0, "", "en-US"); denied != nil {
			t.Errorf("unchecked user denied: %v", denied)
		}
		if denied := runPermissionError(s, 0, 0, "table:Job_Queue.ShowLineNo", "en-US"); denied != nil {
			t.Errorf("unchecked user denied a table function: %v", denied)
		}
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
	if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "permission to run codeunit 99991") {
		t.Errorf("codeunit without Execute: %v", out)
	}
	out = postJSON(t, denied, "/api/jobs/start", `{"codeunit_id":99991}`)
	if out["success"] != false || !strings.Contains(fmt.Sprint(out["error"]), "permission to run codeunit 99991") {
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
}
