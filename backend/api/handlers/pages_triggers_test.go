package handlers

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A card's OnAfterGetRecord switches fields per record (here a test trigger on Customer
// Statistics 151: Name editable only for a customer that is not blocked); the page says it has
// such a trigger
func TestPageFieldStates(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Skipf("pages: %v", err)
	}
	pages.OnAfterGetRecord(151, func(rec *gtables.Customer, page *pages.Control) {
		page.SetEditable("name", !rec.Blocked)
	})
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/pages/:id/field-states", NewPageTriggersHandler(db, database.DBTypeSQLite).FieldStates)

	states := func(body string) map[string]interface{} {
		t.Helper()
		out := postJSON(t, app, "/api/pages/151/field-states", body)
		data, _ := out["data"].(map[string]interface{})
		s, _ := data["field_states"].(map[string]interface{})
		if s == nil {
			t.Fatalf("field states: %v", out)
		}
		return s
	}
	editable := func(s map[string]interface{}, field string) interface{} {
		f, _ := s[field].(map[string]interface{})
		return f["editable"]
	}
	if s := states(`{"no":"C1","blocked":false}`); editable(s, "name") != true {
		t.Errorf("not blocked: %v", s)
	}
	if s := states(`{"no":"C1","blocked":true}`); editable(s, "name") != false {
		t.Errorf("blocked: %v", s)
	}
	if !pages.HasRecordTriggers(151) || pages.HasRecordTriggers(22) {
		t.Error("HasRecordTriggers")
	}

	// A trigger for the wrong record type is an error, not a panic
	pages.OnAfterGetRecord(99991, func(rec *gtables.Customer, page *pages.Control) { page.SetVisible("name", false) })
	if _, err := pages.RunAfterGetRecord(99991, &gtables.JobQueue{}); err == nil {
		t.Error("wrong record type accepted")
	}
	if st, err := pages.RunAfterGetRecord(99991, &gtables.Customer{}); err != nil || *st["name"].Visible {
		t.Errorf("SetVisible: %v %v", st, err)
	}
}

// A journal line entered on its page gets its read-only fields from the page's OnNewRecord
// (the template's Source Code), not from the payload — the browser cannot send them
func TestInsertTakesReadOnlyFieldsFromNewRecord(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	template := gtables.NewResJournalTemplate(src)
	template.Name, template.Page_id, template.Source_code = "RESOURCES", 207, "RESJNL"
	batch := gtables.NewResJournalBatch(src)
	batch.Journal_template_name, batch.Name = "RESOURCES", "DEFAULT"
	for _, err := range []error{template.InsertOrError(false), batch.InsertOrError(false)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", NewTablesHandler(db).InsertRecord)

	line := `{"journal_template_name":"RESOURCES","journal_batch_name":"DEFAULT","line_no":%d,"source_code":"HACK"}`
	status, out := sendJSON(t, app, "POST", "/api/tables/Res_Journal_Line/insert?page_id=207", fmt.Sprintf(line, 10000))
	if status != 200 || out["data"].(map[string]interface{})["source_code"] != "RESJNL" {
		t.Errorf("insert on page 207: %d %v", status, out)
	}
	// without the page the read-only field stays blank (only business logic sets it)
	status, out = sendJSON(t, app, "POST", "/api/tables/Res_Journal_Line/insert", fmt.Sprintf(line, 20000))
	if status != 200 || out["data"].(map[string]interface{})["source_code"] != "" {
		t.Errorf("insert without a page: %d %v", status, out)
	}
}
