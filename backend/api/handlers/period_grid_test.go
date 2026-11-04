package handlers

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The period grid of Item Availability by Periods (157): the item's inventory per month (Net
// Change) or up to each month's end (Balance at Date); by Location (492) per location
func TestPeriodGrid(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Skipf("pages: %v", err)
	}
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
	item := gtables.NewItem(src)
	item.No = "1000"
	for _, loc := range []types.Code{"BLUE", "RED"} {
		location := gtables.NewLocation(src)
		location.Code, location.Name = loc, types.Text("Location "+string(loc))
		if err := location.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	if err := item.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	for i, e := range []struct {
		date types.Date
		qty  int64
		loc  types.Code
	}{{types.NewDate(2026, 1, 10), 10, "BLUE"}, {types.NewDate(2026, 1, 20), 5, "RED"}, {types.NewDate(2026, 3, 5), -4, "BLUE"}} {
		entry := gtables.NewItemLedgerEntry(src)
		entry.Entry_no, entry.Item_no, entry.Posting_date, entry.Quantity, entry.Location_code = i+1, "1000", e.date, types.NewDecimalFromInt(e.qty), e.loc
		if err := entry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/pages/:id/period-grid", NewPageTriggersHandler(db, database.DBTypeSQLite).PeriodGrid)

	grid := func(page int, body string) []map[string]interface{} {
		t.Helper()
		status, out := sendJSON(t, app, "POST", fmt.Sprintf("/api/pages/%d/period-grid", page), body)
		if status != 200 {
			t.Fatalf("period grid: %d %v", status, out)
		}
		var rows []map[string]interface{}
		for _, r := range out["data"].(map[string]interface{})["rows"].([]interface{}) {
			rows = append(rows, r.(map[string]interface{}))
		}
		return rows
	}
	value := func(row map[string]interface{}) string {
		return fmt.Sprint(row["values"].(map[string]interface{})["net_change"])
	}

	// by month from January: 15, 0, -4 (Net Change); 15, 15, 11 (Balance at Date)
	rows := grid(157, `{"section":"Periods","record":{"no":"1000"},"view_by":2,"view_as":0,"date":"2026-01-15","rows":3}`)
	if len(rows) != 3 || rows[0]["period_start"] != "2026-01-01" || rows[2]["period_end"] != "2026-03-31" {
		t.Fatalf("months: %v", rows)
	}
	if value(rows[0]) != "15" || value(rows[1]) != "0" || value(rows[2]) != "-4" {
		t.Errorf("net change by month: %s %s %s", value(rows[0]), value(rows[1]), value(rows[2]))
	}
	rows = grid(157, `{"section":"Periods","record":{"no":"1000"},"view_by":2,"view_as":1,"date":"2026-01-15","rows":3}`)
	if value(rows[0]) != "15" || value(rows[1]) != "15" || value(rows[2]) != "11" {
		t.Errorf("balance by month: %s %s %s", value(rows[0]), value(rows[1]), value(rows[2]))
	}
	// a filter: location BLUE
	rows = grid(157, `{"section":"Periods","record":{"no":"1000"},"filters":[{"field":"location_filter","expression":"BLUE"}],"view_by":4,"view_as":0,"date":"2026-06-01","rows":1}`)
	if len(rows) != 1 || value(rows[0]) != "6" || rows[0]["period_name"] != "2026" {
		t.Errorf("2026 at BLUE: %v", rows)
	}
	// step: one year back
	rows = grid(157, `{"section":"Periods","record":{"no":"1000"},"view_by":4,"view_as":0,"date":"2026-06-01","step":-1,"rows":1}`)
	if rows[0]["period_start"] != "2025-01-01" || value(rows[0]) != "0" {
		t.Errorf("2025: %v", rows)
	}
	// by location in 2026
	rows = grid(492, `{"section":"Locations","record":{"no":"1000"},"view_by":4,"view_as":0,"date":"2026-06-01"}`)
	if len(rows) != 2 || rows[0]["key"] != "BLUE" || value(rows[0]) != "6" || rows[1]["key"] != "RED" || value(rows[1]) != "5" || rows[1]["caption"] != "Location RED" {
		t.Errorf("by location: %v", rows)
	}
}
