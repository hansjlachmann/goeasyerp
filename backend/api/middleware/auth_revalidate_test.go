package middleware

import (
	"database/sql"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// A cached session is checked again — the user and the permissions read anew — after
// RevalidateAfter or a change to users, roles or permissions (session.SecurityChanged), so a
// removed role, a deactivated user or a log out takes effect without waiting for the token's
// 24 hours (S22)
func TestAuthMiddlewareRevalidates(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY); INSERT INTO "Company" VALUES ('alpha')`); err != nil {
		t.Fatal(err)
	}

	var loads atomic.Int32
	canModify := atomic.Bool{}
	canModify.Store(true)
	permLoader := func(_ *sql.DB, _ database.DBType, _, _ string) (bool, bool, map[string]session.TablePermission) {
		loads.Add(1)
		return true, false, map[string]session.TablePermission{"CUSTOMER": {Read: true, Modify: canModify.Load()}}
	}
	userOK := atomic.Bool{}
	userOK.Store(true)

	config := JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	cache := NewSessionCache()
	defer cache.Stop()
	cache.SetUserCheck(func(string, time.Time) bool { return userOK.Load() })
	app := fiber.New()
	app.Use(AuthMiddleware(config, db, database.DBTypeSQLite, cache, permLoader, nil))
	app.Get("/", func(c *fiber.Ctx) error {
		sess, _ := c.Locals("session").(*session.Session)
		switch {
		case sess == nil:
			return c.SendString("none")
		case sess.HasPermission("Customer", session.PermModify):
			return c.SendString("modify")
		}
		return c.SendString("read")
	})
	token, err := GenerateToken(config, "HANS", "Hans", "alpha", "en-US", "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	request := func() string {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Cookie", config.CookieName+"="+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 16)
		n, _ := resp.Body.Read(body)
		return string(body[:n])
	}

	if got := request(); got != "modify" || loads.Load() != 1 {
		t.Fatalf("first request: %q after %d loads", got, loads.Load())
	}
	canModify.Store(false)
	if got := request(); got != "modify" || loads.Load() != 1 {
		t.Errorf("a fresh cached session is used as it is: %q after %d loads", got, loads.Load())
	}

	// a change on this server: the next request reads the permissions again
	session.SecurityChanged()
	if got := request(); got != "read" || loads.Load() != 2 {
		t.Errorf("after a security change: %q after %d loads, want read after 2", got, loads.Load())
	}

	// a change on another server: seen after RevalidateAfter
	old := RevalidateAfter
	RevalidateAfter = 0
	defer func() { RevalidateAfter = old }()
	canModify.Store(true)
	time.Sleep(time.Millisecond)
	if got := request(); got != "modify" || loads.Load() != 3 {
		t.Errorf("after RevalidateAfter: %q after %d loads, want modify after 3", got, loads.Load())
	}

	// the user deactivated, deleted or logged out: no session
	userOK.Store(false)
	time.Sleep(time.Millisecond)
	if got := request(); got != "none" {
		t.Errorf("a token the user check refuses still gave a session (%q)", got)
	}
}
