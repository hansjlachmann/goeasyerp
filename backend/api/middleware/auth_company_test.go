package middleware

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
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
	app.Use(AuthMiddleware(config, db, database.DBTypeSQLite, cache, nil, nil))
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
		resp, err := app.Test(req, -1)
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

// A tab works in its own company (X-Company, or ?company= where a request cannot carry a
// header): an allowed company gets its own session; one the user may not enter, or one that
// is gone, is refused (409, marked) — the request never runs in another company silently
func TestAuthMiddlewareTabCompany(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY); INSERT INTO "Company" VALUES ('alpha'), ('beta'), ('secret')`); err != nil {
		t.Fatal(err)
	}
	config := JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	cache := NewSessionCache()
	defer cache.Stop()
	access := func(userID, company string) bool { return company != "secret" }
	app := fiber.New()
	app.Use(AuthMiddleware(config, db, database.DBTypeSQLite, cache, nil, access))
	app.Get("/", func(c *fiber.Ctx) error {
		if sess, _ := c.Locals("session").(*session.Session); sess != nil {
			return c.SendString(sess.GetCompany())
		}
		return c.SendString("-")
	})
	token, err := GenerateToken(config, "HANS", "Hans", "alpha", "en-US", "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	request := func(path, tab string) (int, string, string) {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Cookie", config.CookieName+"="+token)
		if tab != "" {
			req.Header.Set(TabCompanyHeader, tab)
		}
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 256)
		n, _ := resp.Body.Read(body)
		return resp.StatusCode, string(body[:n]), resp.Header.Get(TabCompanyUnavailableHeader)
	}

	for _, c := range []struct {
		path, tab, want string
	}{
		{"/", "", "alpha"},             // no tab company: the cookie's
		{"/", "beta", "beta"},          // the tab's own
		{"/", "beta", "beta"},          // … again, from the cache
		{"/", "alpha", "alpha"},        // the cookie's, sent by the tab
		{"/?company=beta", "", "beta"}, // an image or stream: the parameter
	} {
		if code, got, _ := request(c.path, c.tab); code != 200 || got != c.want {
			t.Errorf("%s tab %q: %d %q, want %q", c.path, c.tab, code, got, c.want)
		}
	}
	for _, tab := range []string{"secret", "gone"} {
		code, body, marker := request("/", tab)
		if code != fiber.StatusConflict || marker != "unavailable" || !strings.Contains(body, "error") {
			t.Errorf("tab %q: %d %q marker %q", tab, code, body, marker)
		}
	}
	if cache.Get("HANS:secret") != nil {
		t.Error("a refused company got a session")
	}
}
