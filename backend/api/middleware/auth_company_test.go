package middleware

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	_ "github.com/mattn/go-sqlite3"
)

// A token naming a company that no longer exists (renamed or deleted) gives no session:
// its tables are gone, so the user logs in again instead of hitting errors.
func TestAuthMiddlewareCompanyGone(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY); INSERT INTO "Company" VALUES ('alpha')`); err != nil {
		t.Fatal(err)
	}

	config := JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	cache := NewSessionCache()
	defer cache.Stop()
	app := fiber.New()
	app.Use(AuthMiddleware(config, db, database.DBTypeSQLite, cache, nil))
	app.Get("/", func(c *fiber.Ctx) error {
		if sess, _ := c.Locals("session").(*session.Session); sess != nil {
			return c.SendString(sess.GetCompany())
		}
		return c.SendString("-")
	})

	request := func(companyName string) (string, string) {
		token, err := GenerateToken(config, "HANS", "Hans", companyName, "en-US", "", time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Cookie", config.CookieName+"="+token)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 64)
		n, _ := resp.Body.Read(body)
		return string(body[:n]), resp.Header.Get("Set-Cookie")
	}

	if got, _ := request("alpha"); got != "alpha" {
		t.Errorf("existing company: session company %q, want alpha", got)
	}
	got, cookie := request("gone")
	if got != "-" {
		t.Errorf("missing company: session company %q, want no session", got)
	}
	if !strings.Contains(cookie, config.CookieName+"=;") && !strings.Contains(cookie, "Max-Age=0") && !strings.Contains(cookie, "expires=") {
		t.Errorf("missing company: cookie not cleared: %q", cookie)
	}

	// A cached session of a renamed company is dropped with RemoveByCompany
	cache.Set("HANS:alpha", session.NewSession(nil, "alpha", nil), time.Hour)
	cache.Set("KARI:beta", session.NewSession(nil, "beta", nil), time.Hour)
	cache.RemoveByCompany("alpha")
	if cache.Get("HANS:alpha") != nil || cache.Get("KARI:beta") == nil {
		t.Error("RemoveByCompany removed the wrong entries")
	}
}

// A response that re-issues the session cookie tells the frontend to reload its session
// state (X-Session-Changed); other responses do not.
func TestSetAuthCookieSignalsSessionChange(t *testing.T) {
	config := JWTConfig{SecretKey: []byte("test"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	app := fiber.New()
	app.Get("/reissue", func(c *fiber.Ctx) error {
		SetAuthCookie(c, config, "token")
		return c.SendStatus(200)
	})
	app.Get("/plain", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest("GET", "/reissue", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get(SessionChangedHeader) == "" || !strings.Contains(resp.Header.Get("Set-Cookie"), "openerp_session=token") {
		t.Errorf("re-issued cookie: header %q, Set-Cookie %q", resp.Header.Get(SessionChangedHeader), resp.Header.Get("Set-Cookie"))
	}
	resp, err = app.Test(httptest.NewRequest("GET", "/plain", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Header.Get(SessionChangedHeader) != "" {
		t.Error("X-Session-Changed on a response that did not change the session")
	}
}
