package handlers

import (
	"database/sql"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Preferences are per user, the same in every company (a global table): saved in one company,
// read in another.
func TestPreferencesSaveAndRead(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.UserPreferences{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewPreferencesHandlerWithDBType(db, database.DBTypeSQLite)
	appIn := func(company string) *fiber.App {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("session", &session.Session{UserID: "HANS", Company: company, Language: "en-US"})
			return c.Next()
		})
		app.Get("/api/preferences/:page_id/:type", h.GetPreferences)
		app.Post("/api/preferences/:page_id/:type", h.SavePreference)
		return app
	}

	saved := postJSON(t, appIn("cronus"), "/api/preferences/22/COLUMNS", `{"preference_name":"DEFAULT","preference_data":{"hidden":["city"]}}`)
	if saved["success"] != true {
		t.Fatalf("save: %v", saved)
	}
	resp, err := appIn("demo01").Test(httptest.NewRequest("GET", "/api/preferences/22/COLUMNS", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"success":true`) || !strings.Contains(string(body), "city") {
		t.Errorf("read in another company: %s", body)
	}

	// Saving again updates the same preference
	if r := postJSON(t, appIn("cronus"), "/api/preferences/22/COLUMNS", `{"preference_name":"DEFAULT","preference_data":{"hidden":["phone"]}}`); r["success"] != true {
		t.Fatalf("update: %v", r)
	}
	var rows int
	var data string
	if err := db.QueryRow(`SELECT COUNT(*), MAX(preference_data) FROM "User_Preferences"`).Scan(&rows, &data); err != nil || rows != 1 || !strings.Contains(data, "phone") {
		t.Errorf("%d rows, data %q, %v", rows, data, err)
	}
}
