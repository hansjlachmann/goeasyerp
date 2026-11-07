package handlers

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A delete refused because the record is in use undoes what its OnDelete did: the location's
// bins are still there, for the single delete and the delete of several
func TestRefusedDeleteKeepsWhatOnDeleteRemoved(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Location{}, &gtables.Bin{}, &gtables.Customer{}} {
		if err := rec.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	location := gtables.NewLocation(src)
	location.Code = "MAIN"
	if err := location.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	bin := gtables.NewBin(src)
	bin.Location_code, bin.Code = "MAIN", "A-01"
	if err := bin.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	cust := gtables.NewCustomer(src)
	cust.No, cust.Location_code = "C1", "MAIN"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}

	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: "ADMIN", Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Delete("/api/tables/:table/delete/:id", h.DeleteRecord)
	app.Delete("/api/tables/:table/delete-many", h.DeleteRecords)

	for _, c := range []struct{ what, path, body string }{
		{"single", "/api/tables/Location/delete/MAIN", ``},
		{"several", "/api/tables/Location/delete-many", `{"keys":["MAIN"]}`},
	} {
		status, out := sendJSON(t, app, "DELETE", c.path, c.body)
		if msg, _ := out["error"].(string); status != 400 || !strings.Contains(msg, "MAIN") {
			t.Errorf("%s: %d %q, want refused", c.what, status, msg)
		}
		if gtables.NewBin(src).Count() != 1 || gtables.NewLocation(src).Count() != 1 {
			t.Fatalf("%s: the refused delete changed something", c.what)
		}
	}

	cust.Location_code = ""
	if err := cust.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	status, out := sendJSON(t, app, "DELETE", "/api/tables/Location/delete/MAIN", ``)
	if status != 200 {
		t.Fatalf("unused location: %d %v", status, out)
	}
	if gtables.NewBin(src).Count() != 0 || gtables.NewLocation(src).Count() != 0 {
		t.Error("the location or its bins are left")
	}
}
