package handlers

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// captureSecurityEvents starts the security log with a writer that keeps the events
func captureSecurityEvents(t *testing.T) func() []securitylog.Event {
	t.Helper()
	securitylog.Reset()
	var mu sync.Mutex
	var got []securitylog.Event
	stop := securitylog.Start(func(events []securitylog.Event) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, events...)
		return nil
	})
	t.Cleanup(func() {
		stop()
		securitylog.Reset()
	})
	return func() []securitylog.Event {
		securitylog.Flush()
		mu.Lock()
		defer mu.Unlock()
		return append([]securitylog.Event(nil), got...)
	}
}

// Failed logins are security events with the reason; the failure that locks the address is
// followed by a Login Locked event, and a login while locked is a Login Refused event (S48)
func TestLoginSecurityEvents(t *testing.T) {
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
	kari.Active = true
	if err := tables.SetPassword(&kari, "long-secret-1"); err != nil {
		t.Fatal(err)
	}
	if !kari.Insert(false) {
		t.Fatal(kari.TriggerError())
	}
	logins = newLoginGuard()
	t.Cleanup(func() { logins = newLoginGuard() })
	events := captureSecurityEvents(t)

	config := middleware.JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	h := NewAuthHandlerFull(db, sqlite, nil, config, middleware.NewSessionCache())
	app := fiber.New()
	app.Post("/login", h.Login)
	login := func(user, password string) int {
		t.Helper()
		body := `{"user_id":"` + user + `","password":"` + password + `","company":"alpha"}`
		req := httptest.NewRequest("POST", "/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}

	login("NOBODY", "long-secret-1")
	got := events()
	if len(got) != 1 || got[0].Kind != securitylog.LoginFailed || got[0].UserID != "NOBODY" || got[0].Company != "alpha" ||
		got[0].Details != "unknown user ID" || got[0].Address == "" {
		t.Fatalf("an unknown user ID: %+v", got)
	}

	for range loginMaxFailures {
		login("KARI", "wrong-password")
	}
	if status := login("KARI", "long-secret-1"); status != fiber.StatusTooManyRequests {
		t.Fatalf("after %d failures: %d, want 429", loginMaxFailures, status)
	}
	kinds := map[securitylog.Kind]int{}
	for _, e := range events()[1:] {
		if e.UserID != "KARI" {
			t.Errorf("an event of another user: %+v", e)
		}
		kinds[e.Kind]++
	}
	// the failures are held back after five a minute (the rest are counted when the minute ends)
	if kinds[securitylog.LoginFailed] != 5 || kinds[securitylog.LoginLocked] != 1 || kinds[securitylog.LoginRefused] != 1 {
		t.Errorf("events of the lockout: %v", kinds)
	}
}

// Every 403 answer is an event unless its handler noted one itself; changes from other web
// sites are Cross-Site Refused events (S48)
func TestRefusedRequestsAreSecurityEvents(t *testing.T) {
	events := captureSecurityEvents(t)
	app := fiber.New()
	app.Use(middleware.SecurityEvents())
	app.Use(middleware.SameOriginWrites(nil))
	app.Get("/api/tables/User/list", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) })
	app.Post("/api/auth/init", func(c *fiber.Ctx) error {
		middleware.SecurityEventRecorded(c) // the login page's question
		return c.SendStatus(fiber.StatusForbidden)
	})
	app.Get("/api/ok", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	send := func(method, path string, header map[string]string) {
		req := httptest.NewRequest(method, path, nil)
		for k, v := range header {
			req.Header.Set(k, v)
		}
		if _, err := app.Test(req, 5000); err != nil {
			t.Fatal(err)
		}
	}
	send("GET", "/api/tables/User/list", nil)
	send("POST", "/api/auth/init", nil)
	send("GET", "/api/ok", nil)
	send("POST", "/api/ok", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})

	got := events()
	if len(got) != 2 {
		t.Fatalf("%d events, want 2: %+v", len(got), got)
	}
	if got[0].Kind != securitylog.AccessRefused || got[0].Details != "GET /api/tables/User/list" {
		t.Errorf("the refused request: %+v", got[0])
	}
	if got[1].Kind != securitylog.CrossSiteRefused || !strings.Contains(got[1].Details, "https://evil.example") {
		t.Errorf("the cross-site change: %+v", got[1])
	}
}
