package handlers

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	_ "github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Every function a page action or menu item calls (run_object "<type>:<object>.<function>")
// is registered and takes the record of the page's table
func TestPageFunctionsAreRegistered(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Fatal(err)
	}
	var check func(id int, table string, actions []pages.Action)
	found := 0
	check = func(id int, table string, actions []pages.Action) {
		for _, a := range actions {
			check(id, table, a.Items)
			obj := a.RunObject
			if obj == "" || !strings.Contains(obj, ".") {
				continue
			}
			found++
			// Registered, and the call's arguments fit the function
			f, _, err := functions.Resolve(obj)
			if err != nil {
				t.Errorf("page %d action %s: %v", id, a.Name, err)
				continue
			}
			if factory, ok := gtables.GetTableFactory(f.Table); !ok || factory().GetTableName() != tableName(table) {
				t.Errorf("page %d action %s: %s takes a %s record, the page shows %s", id, a.Name, obj, f.Table, table)
			}
		}
	}
	for id, p := range pages.GetRegistry().GetAllPages() {
		check(id, p.Page.SourceTable, p.Page.Actions)
		// A field's OnAfterValidate: a function of the page's table
		fields := []pages.Field{}
		if p.Page.Layout.Repeater != nil {
			fields = append(fields, p.Page.Layout.Repeater.Fields...)
		}
		for _, s := range p.Page.Layout.Sections {
			fields = append(fields, s.Fields...)
		}
		for _, f := range fields {
			if f.OnAfterValidate == "" {
				continue
			}
			found++
			fn, _, err := functions.Resolve(f.OnAfterValidate)
			if err != nil {
				t.Errorf("page %d field %s: %v", id, f.Source, err)
			} else if factory, ok := gtables.GetTableFactory(fn.Table); !ok || factory().GetTableName() != tableName(p.Page.SourceTable) {
				t.Errorf("page %d field %s: %s takes a %s record", id, f.Source, f.OnAfterValidate, fn.Table)
			}
		}
		// A field's AssistEdit: registered as one, on the page's table
		for _, s := range p.Page.Layout.Sections {
			for _, f := range s.Fields {
				if f.AssistEdit == "" {
					continue
				}
				fn, ok := functions.Lookup(f.AssistEdit)
				if !ok || !fn.AssistEdit {
					t.Errorf("page %d field %s: %s is not a registered AssistEdit function", id, f.Source, f.AssistEdit)
				} else if factory, ok := gtables.GetTableFactory(fn.Table); !ok || factory().GetTableName() != tableName(p.Page.SourceTable) {
					t.Errorf("page %d field %s: %s takes a %s record", id, f.Source, f.AssistEdit, fn.Table)
				}
			}
		}
	}
	if found == 0 {
		t.Error("no page function found (Job Queue Entries: Line → Test Line 1)")
	}
}

func tableName(apiName string) string {
	if factory, ok := gtables.GetTableFactory(apiName); ok {
		return factory().GetTableName()
	}
	return apiName
}

// A page function runs on the stored record (read by its key) and returns its message
func TestRunPageFunction(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.JobQueueEntry{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	e := &gtables.JobQueueEntry{}
	e.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	e.Entry_no, e.Job_queue_no = 7, types.NewCode("JQ1")
	if !e.Insert(false) {
		t.Fatal("insert")
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/jobs/start", NewJobsHandlerWithDBType(db, database.DBTypeSQLite).StartJob)

	// The browser's other values do not count: the function sees the stored line
	out := postJSON(t, app, "/api/jobs/start", `{"function":"table:Job_Queue_Entry.ShowLineNo","record":{"entry_no":7,"job_queue_no":"WRONG"}}`)
	data, _ := out["data"].(map[string]interface{})
	dialog, _ := data["dialog"].(map[string]interface{})
	if msg := fmt.Sprint(dialog["message"]); !strings.Contains(msg, "7") || !strings.Contains(msg, "JQ1") {
		t.Errorf("message: %v", out)
	}
	if out := postJSON(t, app, "/api/jobs/start", `{"function":"table:Job_Queue_Entry.ShowLineNo","record":{"entry_no":99}}`); out["success"] != false {
		t.Errorf("a line that does not exist: %v", out)
	}
	if out := postJSON(t, app, "/api/jobs/start", `{"function":"table:Job_Queue_Entry.Nope","record":{"entry_no":7}}`); out["success"] != false {
		t.Errorf("an unknown function: %v", out)
	}
}
