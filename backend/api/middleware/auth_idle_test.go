package middleware

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// A session ends after the idle limit without requests (S39): a token lasts IdleTimeout and is
// renewed while the user works, never past TokenExpiry after the login, and never for a token
// the user check refuses
func TestTokenIdleLimitAndRenewal(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE "Company" (name TEXT PRIMARY KEY); INSERT INTO "Company" VALUES ('alpha')`); err != nil {
		t.Fatal(err)
	}
	config := JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: 24 * time.Hour, IdleTimeout: time.Hour}
	userOK := atomic.Bool{}
	userOK.Store(true)
	cache := NewSessionCache()
	defer cache.Stop()
	cache.SetUserCheck(func(string, time.Time) bool { return userOK.Load() })
	app := fiber.New()
	app.Use(AuthMiddleware(config, db, database.DBTypeSQLite, cache, nil, nil))
	app.Get("/", func(c *fiber.Ctx) error {
		if sess, _ := c.Locals("session").(*session.Session); sess != nil {
			return c.SendString("session")
		}
		return c.SendString("none")
	})
	now := time.Now()
	token := func(loginAt, issued, expires time.Time) string {
		claims := &Claims{UserID: "HANS", Company: "alpha", LoginAt: loginAt.Unix(), RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(issued), ExpiresAt: jwt.NewNumericDate(expires)}}
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(config.SecretKey)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	// request answers the body and the renewed token's expiry (zero: not renewed)
	request := func(tok string) (string, time.Time) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Cookie", config.CookieName+"="+tok)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Header.Get(SessionChangedHeader) != "" {
			t.Error("a renewal told the frontend the session changed")
		}
		body := make([]byte, 16)
		n, _ := resp.Body.Read(body)
		var expires time.Time
		for _, cookie := range resp.Header.Values("Set-Cookie") {
			value := strings.TrimPrefix(strings.Split(cookie, ";")[0], config.CookieName+"=")
			if value == "" {
				continue
			}
			claims, err := ParseToken(config, value)
			if err != nil {
				t.Fatalf("renewed token: %v", err)
			}
			expires = claims.ExpiresAt.Time
		}
		return string(body[:n]), expires
	}
	near := func(a, b time.Time) bool { return a.Sub(b).Abs() < 5*time.Second }

	// a new token lasts the idle limit
	fresh, err := GenerateToken(config, "HANS", "Hans", "alpha", "en-US", "", now)
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := ParseToken(config, fresh)
	if !near(claims.ExpiresAt.Time, now.Add(time.Hour)) {
		t.Errorf("a new token ends %s, want in an hour", claims.ExpiresAt.Time)
	}
	// ... and near the absolute limit at that limit
	late, _ := GenerateToken(config, "HANS", "Hans", "alpha", "en-US", "", now.Add(-23*time.Hour-30*time.Minute))
	claims, _ = ParseToken(config, late)
	if !near(claims.ExpiresAt.Time, now.Add(30*time.Minute)) {
		t.Errorf("a token 23.5 h after the login ends %s, want in 30 minutes", claims.ExpiresAt.Time)
	}

	// a token issued a minute ago is not renewed; one issued 10 minutes ago is, by an hour
	if body, expires := request(token(now, now.Add(-time.Minute), now.Add(59*time.Minute))); body != "session" || !expires.IsZero() {
		t.Errorf("a minute old: %q, renewed to %s; want a session, not renewed", body, expires)
	}
	if body, expires := request(token(now.Add(-time.Hour), now.Add(-10*time.Minute), now.Add(50*time.Minute))); body != "session" || !near(expires, now.Add(time.Hour)) {
		t.Errorf("10 minutes old: %q, renewed to %s; want renewed to in an hour", body, expires)
	}
	// at the absolute limit: not renewed past it
	login := now.Add(-23*time.Hour - 50*time.Minute)
	if _, expires := request(token(login, now.Add(-10*time.Minute), login.Add(24*time.Hour))); !expires.IsZero() {
		t.Errorf("at the absolute limit the token was renewed to %s", expires)
	}
	// idle past the limit: the token has expired, no session
	if body, _ := request(token(now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour))); body != "none" {
		t.Errorf("an expired token gave %q", body)
	}
	// a token the user check refuses (revoked on another server) is not renewed, even while its
	// session is still cached here
	cached := token(now.Add(-time.Hour), now.Add(-10*time.Minute), now.Add(50*time.Minute))
	request(cached)
	userOK.Store(false)
	if _, expires := request(cached); !expires.IsZero() {
		t.Errorf("a refused token was renewed to %s", expires)
	}
}
