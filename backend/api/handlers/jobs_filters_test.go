package handlers

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// filtersCodeunit reports the filters its record carries: group 2 (fixed) and group 0 (user)
type filtersCodeunit struct{}

func (filtersCodeunit) ID() int             { return 99993 }
func (filtersCodeunit) Name() string        { return "filters-test" }
func (filtersCodeunit) SourceTable() string { return "Customer" }
func (filtersCodeunit) UsesProgress() bool  { return false }
func (filtersCodeunit) Run(record interface{}) (fcodeunits.Result, error) {
	rec := record.(*gtables.Customer)
	rec.FilterGroup(2)
	fixed := rec.GetFilter("country_code")
	rec.FilterGroup(0)
	return fcodeunits.Result{Success: true, Message: fmt.Sprintf("%s|%s", fixed, rec.GetFilter("name"))}, nil
}

// A codeunit run from a list gets the list's filters on its record (a journal's Post works on
// the batch the list shows)
func TestJobGetsListFilters(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	codeunits.Register(99993, "filters-test", func(database.Executor, string, database.DBType) fcodeunits.Codeunit {
		return filtersCodeunit{}
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/jobs/start", NewJobsHandlerWithDBType(db, database.DBTypeSQLite).StartJob)
	out := postJSON(t, app, "/api/jobs/start", `{"codeunit_id":99993,"record":{"no":"C1"},
		"fixed_filters":[{"field":"country_code","value":"NO"}],"filters":[{"field":"name","expression":"A*"}]}`)
	data, _ := out["data"].(map[string]interface{})
	if out["success"] != true || data["message"] != "NO|A*" {
		t.Errorf("codeunit saw %v", out)
	}
	out = postJSON(t, app, "/api/jobs/start", `{"codeunit_id":99993,"record":{},"fixed_filters":[{"field":"password_hash","value":"x"}]}`)
	if out["success"] != false {
		t.Errorf("a filter on an unknown field must be refused: %v", out)
	}
}
