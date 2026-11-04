package handlers

import (
	"database/sql"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// A report without data items (Create Fiscal Year) has no source table: it starts without a
// record
func TestStartReportWithoutDataItems(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/jobs/start", NewJobsHandlerWithDBType(db, database.DBTypeSQLite).StartJob)
	out := postJSON(t, app, "/api/jobs/start", `{"report_id":`+strconv.Itoa(reports.CreateFiscalYearID)+`,"record":{"starting_date":"2026-01-01"}}`)
	data, _ := out["data"].(map[string]interface{})
	if out["success"] != true || data["job_id"] == nil {
		t.Errorf("start: %v", out)
	}
}
