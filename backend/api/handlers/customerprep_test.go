package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 432 Reminder Levels: a new level gets the next No. of the terms the page is filtered on
// (the values /init gets); page 456 No. Series lists each series' current line
func TestReminderLevelNewRecordAndNoSeriesList(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.ReminderLevel{}, &gtables.NoSeries{}, &gtables.NoSeriesLine{}} {
		if err := tbl.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO "TEST$Reminder_Level" (reminder_terms_code, no) VALUES ('DOM', 1), ('DOM', 2), ('FOR', 1)`,
		`INSERT INTO "TEST$No_Series" (code, description) VALUES ('CUST', 'Customers'), ('NONE', 'No lines')`,
		`INSERT INTO "TEST$No_Series_Line" (series_code, line_no, starting_no, last_no_used, increment_by_no, open) VALUES ('CUST', 10000, 'C001', 'C007', 1, 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/init", h.InitRecord)
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Post("/api/tables/:table/validate", h.ValidateField)

	// A row of the No. Series list carries the page's calculated columns: saved all the same
	if status, out := sendJSON(t, app, "POST", "/api/tables/No_Series/insert", `{"code":"NEW","description":"New series","starting_date":"","starting_no":"","ending_no":"","last_date_used":"","last_no_used":"","warning_no":"","increment_by_no":""}`); status != 200 {
		t.Errorf("insert with calculated columns: %d %v", status, out["error"])
	}
	if status, out := sendJSON(t, app, "PUT", "/api/tables/No_Series/modify/NEW", `{"code":"NEW","description":"Renamed","ending_no":"X9"}`); status != 200 {
		t.Errorf("modify with calculated columns: %d %v", status, out["error"])
	}
	if status, out := sendJSON(t, app, "POST", "/api/tables/No_Series/validate", `{"field":"description","value":"D","record":{"code":"NEW","ending_no":""}}`); status != 200 {
		t.Errorf("validate with calculated columns: %d %v", status, out["error"])
	}
	if status, _ := sendJSON(t, app, "POST", "/api/tables/No_Series/insert", `{"code":"BAD","no_such_field":"x"}`); status == 200 {
		t.Error("an unknown field is accepted")
	}

	status, out := sendJSON(t, app, "POST", "/api/tables/Reminder_Level/init", `{"page_id":432,"below_x_rec":true,"values":{"reminder_terms_code":"DOM"}}`)
	if data, _ := out["data"].(map[string]interface{}); status != 200 || data["no"] != float64(3) || data["reminder_terms_code"] != "DOM" {
		t.Errorf("new DOM level: %d %v", status, out)
	}

	resp, err := app.Test(httptest.NewRequest("GET", "/api/tables/No_Series/list?page_id=456", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var list struct {
		Data struct {
			Records []map[string]interface{} `json:"records"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	got := map[string]map[string]interface{}{}
	for _, r := range list.Data.Records {
		got[r["code"].(string)] = r
	}
	if r := got["CUST"]; r == nil || r["last_no_used"] != "C007" || r["starting_no"] != "C001" {
		t.Errorf("CUST row = %v (%s)", r, body)
	}
	if r := got["NONE"]; r == nil || r["last_no_used"] != "" {
		t.Errorf("NONE row = %v", r)
	}
}

// Records with a number in their key (No. Series Line: Series Code, Line No.) are found by the
// key the pages send as text ("CUSTOMER1,10000") — edit and delete
func TestIntegerKeyPartFromURL(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.ReminderTerms{}, &gtables.ReminderLevel{}, &gtables.ReminderText{}} {
		if err := tbl.CreateTableWithDBType(db, "Test migration01", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "Test migration01", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Delete("/api/tables/:table/delete/:id", h.DeleteRecord)

	for _, step := range []struct{ method, target, body string }{
		{"POST", "/api/tables/No_Series/insert", `{"code":"CUSTOMER1"}`},
		{"POST", "/api/tables/No_Series_Line/insert", `{"series_code":"CUSTOMER1","line_no":10000,"starting_date":"2026-10-22","starting_no":""}`},
		{"PUT", "/api/tables/No_Series_Line/modify/CUSTOMER1,10000", `{"series_code":"CUSTOMER1","line_no":10000,"starting_no":"C00001"}`},
		{"POST", "/api/tables/Reminder_Terms/insert", `{"code":"DOM"}`},
		{"POST", "/api/tables/Reminder_Level/insert", `{"reminder_terms_code":"DOM","no":1}`},
		{"PUT", "/api/tables/Reminder_Level/modify/DOM,1", `{"reminder_terms_code":"DOM","no":1,"grace_period":"5D"}`},
		{"DELETE", "/api/tables/Reminder_Level/delete/DOM,1", ``},
	} {
		if status, out := sendJSON(t, app, step.method, step.target, step.body); status != 200 {
			t.Errorf("%s %s: %d %v", step.method, step.target, status, out["error"])
		}
	}
	var startingNo string
	if err := db.QueryRow(`SELECT starting_no FROM "Test migration01$No_Series_Line" WHERE series_code = 'CUSTOMER1' AND line_no = 10000`).Scan(&startingNo); err != nil || startingNo != "C00001" {
		t.Errorf("starting no = %q, %v", startingNo, err)
	}
}

// A customer numbered from a series whose next number is taken: refused with "Customer …
// already exists", and the series does not move on (the insert is one transaction)
func TestCustomerNumberTaken(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Customer{}, &gtables.SalesReceivablesSetup{}, &gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{}} {
		_ = tbl.CreateTableWithDBType(db, "T", database.DBTypeSQLite)
	}
	for _, q := range []string{
		`INSERT INTO "T$No_Series" (code, default_nos) VALUES ('CUST', 1)`,
		`INSERT INTO "T$No_Series_Line" (series_code, line_no, starting_no, increment_by_no, open) VALUES ('CUST', 10000, 'C00001', 1, 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var setup gtables.SalesReceivablesSetup
	setup.InitWithDBType(db, "T", database.DBTypeSQLite)
	setup.InitRecord()
	setup.Customer_nos = "CUST"
	setup.Insert(false)
	var cust gtables.Customer
	cust.InitWithDBType(db, "T", database.DBTypeSQLite)
	cust.InitRecord()
	cust.No, cust.Name = "C00001", "Existing"
	cust.Insert(false)
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "T", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	if status, out := sendJSON(t, app, "POST", "/api/tables/Customer/insert", `{}`); status != 409 || out["error"] != "Customer C00001 already exists" {
		t.Errorf("taken number: %d %v", status, out)
	}
	var name string
	_ = db.QueryRow(`SELECT name FROM "T$Customer" WHERE no = 'C00001'`).Scan(&name)
	var last string
	_ = db.QueryRow(`SELECT last_no_used FROM "T$No_Series_Line"`).Scan(&last)
	if name != "Existing" || last != "" {
		t.Errorf("after the refused insert: C00001 %q, Last No. Used %q (want unchanged)", name, last)
	}
}
