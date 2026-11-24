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

// The login answer does not tell which user IDs exist: an unknown ID and a wrong password get
// the same message after a password check of the same cost, and an inactive user is told so
// only with the right password (S24)
func TestLoginDoesNotRevealUsers(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY); INSERT INTO "Company" VALUES ('alpha')`); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.User{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	var kari gtables.User
	kari.InitWithDBType(db, "", sqlite)
	kari.User_id = types.NewCode("KARI")
	kari.Active = false
	if err := tables.SetPassword(&kari, "secret1"); err != nil {
		t.Fatal(err)
	}
	if !kari.Insert(false) {
		t.Fatal(kari.TriggerError())
	}

	config := middleware.JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	h := NewAuthHandlerFull(db, sqlite, nil, config, middleware.NewSessionCache())
	app := fiber.New()
	app.Post("/login", h.Login)
	login := func(user, password string) (string, time.Duration) {
		t.Helper()
		body := `{"user_id":"` + user + `","password":"` + password + `","company":"alpha"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		start := time.Now()
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return strings.TrimSpace(out["error"].(string)), time.Since(start)
	}

	checkDummyPassword("") // the dummy hash is made once, before the timing
	unknown, unknownTook := login("NOBODY", "secret1")
	wrong, wrongTook := login("KARI", "wrong-pw")
	if unknown != wrong {
		t.Errorf("unknown user %q, wrong password %q: want the same answer", unknown, wrong)
	}
	if unknownTook < wrongTook/3 {
		t.Errorf("an unknown user is answered in %v, a wrong password in %v: the time tells them apart", unknownTook, wrongTook)
	}
	if inactive, _ := login("KARI", "secret1"); inactive == wrong {
		t.Errorf("the right password of an inactive user: %q, want the inactive message", inactive)
	}
	logins = newLoginGuard() // the failures above must not lock or brake other tests
}
