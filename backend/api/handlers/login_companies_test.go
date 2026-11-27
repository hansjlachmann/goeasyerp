package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The company names are shown only after the password: GET /api/auth/companies needs a
// session, a login without a company goes straight into the user's one company or lists the
// user's companies, and a wrong password says nothing about the company sent (A5)
func TestLoginListsCompaniesAfterPassword(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY, display_name TEXT);
		INSERT INTO "Company" VALUES ('alpha', 'Alpha AS'), ('beta', ''), ('gamma', '')`); err != nil {
		t.Fatal(err)
	}
	for _, create := range []func() error{
		func() error { return (&gtables.User{}).CreateTableWithDBType(db, "", sqlite) },
		func() error { return (&gtables.UserMember{}).CreateTableWithDBType(db, "", sqlite) },
	} {
		if err := create(); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"ONE", "MANY"} {
		var user gtables.User
		user.InitWithDBType(db, "", sqlite)
		user.User_id = types.NewCode(id)
		user.Active = true
		if err := tables.SetPassword(&user, "long-secret-1"); err != nil {
			t.Fatal(err)
		}
		if !user.Insert(false) {
			t.Fatal(user.TriggerError())
		}
	}
	for _, m := range [][2]string{{"ONE", "alpha"}, {"MANY", "alpha"}, {"MANY", "beta"}} {
		if _, err := db.Exec(`INSERT INTO "User_Member" (user_id, role_id, company) VALUES (?, 'READER', ?)`, m[0], m[1]); err != nil {
			t.Fatal(err)
		}
	}

	config := middleware.JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	h := NewAuthHandlerFull(db, sqlite, nil, config, middleware.NewSessionCache())
	app := fiber.New()
	app.Post("/login", h.Login)
	app.Get("/companies", h.ListCompanies)
	login := func(user, password, company string) (int, map[string]interface{}) {
		t.Helper()
		body := `{"user_id":"` + user + `","password":"` + password + `","company":"` + company + `"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	t.Cleanup(func() { logins = newLoginGuard() })

	resp, err := app.Test(httptest.NewRequest("GET", "/companies", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("company list without a session: %d, want 401", resp.StatusCode)
	}

	// a wrong password: the same answer for an existing and a made-up company
	_, known := login("ONE", "wrong-pw", "alpha")
	_, unknown := login("ONE", "wrong-pw", "nosuch")
	if known["error"] != unknown["error"] {
		t.Errorf("a wrong password tells the companies apart: %v / %v", known["error"], unknown["error"])
	}

	status, out := login("ONE", "long-secret-1", "")
	data, _ := out["data"].(map[string]interface{})
	if status != 200 || data["company"] != "alpha" {
		t.Errorf("one company: logged in to it, got %d %v", status, out)
	}

	status, out = login("MANY", "long-secret-1", "")
	data, _ = out["data"].(map[string]interface{})
	list, _ := data["companies"].([]interface{})
	if status != 200 || data["choose_company"] != true || len(list) != 2 {
		t.Fatalf("several companies: the user's list, got %d %v", status, out)
	}
	if data["company"] != nil {
		t.Errorf("several companies: no session yet, got %v", data)
	}

	status, out = login("MANY", "long-secret-1", "beta")
	data, _ = out["data"].(map[string]interface{})
	if status != 200 || data["company"] != "beta" {
		t.Errorf("the picked company: got %d %v", status, out)
	}
}
