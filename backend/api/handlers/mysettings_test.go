package handlers

import (
	"database/sql"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// My Settings: the defaults, saving the formats and the work date, refusing unknown values; the
// loader gives them to the user's code; a login takes the work date back to today
func TestMySettings(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.UserPersonalization{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewMySettingsHandler(db, database.DBTypeSQLite)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: "HANS", Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/my-settings", h.GetMySettings)
	app.Put("/api/my-settings", h.SaveMySettings)

	_, out := getJSON(t, app, "/api/my-settings")
	data, _ := out["data"].(map[string]interface{})
	if data["date_format"] != "" || data["number_format"] != "" || data["work_date"] != "" || data["today"] != types.Today().String() {
		t.Fatalf("defaults %v", data)
	}
	code, out := putJSON(t, app, "/api/my-settings", `{"date_format":"dd.mm.yy","number_format":"1 234,56","work_date":"2026-09-05"}`)
	if code != 200 {
		t.Fatalf("save: %d %v", code, out["error"])
	}
	for _, body := range []string{`{"date_format":"dd-mm"}`, `{"number_format":"1_234"}`, `{"work_date":"5 Sept"}`} {
		if code, _ := putJSON(t, app, "/api/my-settings", body); code != 400 {
			t.Errorf("%s taken: %d", body, code)
		}
	}
	regional := RegionalLoader(db, database.DBTypeSQLite)("HANS")
	if regional.DateFormat != "dd.mm.yy" || regional.NumberFormat != "1 234,56" || regional.WorkDate.String() != "2026-09-05" {
		t.Errorf("loader %+v", regional)
	}
	ResetWorkDate(db, database.DBTypeSQLite, "HANS")
	regional = RegionalLoader(db, database.DBTypeSQLite)("HANS")
	if !regional.WorkDate.IsZero() || regional.DateFormat != "dd.mm.yy" {
		t.Errorf("after a login %+v", regional)
	}
}
