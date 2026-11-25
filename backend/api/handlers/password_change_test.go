package handlers

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A user changes their own password (S38): the current one is asked, the rules apply, the
// other sessions end and this browser gets a new token that stays good
func TestChangeOwnPassword(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if err := (&gtables.User{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	kari := &gtables.User{}
	kari.InitWithDBType(db, "", sqlite)
	kari.User_id = types.NewCode("KARI")
	kari.Active = true
	if err := tables.SetPassword(kari, "long-secret-1"); err != nil {
		t.Fatal(err)
	}
	if !kari.Insert(true) {
		t.Fatal(kari.TriggerError())
	}
	logins = newLoginGuard()
	t.Cleanup(func() { logins = newLoginGuard() })

	config := middleware.JWTConfig{SecretKey: []byte("test-secret"), CookieName: "openerp_session", TokenExpiry: time.Hour}
	h := &AuthHandler{db: db, dbType: sqlite, jwtConfig: config, sessionCache: middleware.NewSessionCache()}
	app := fiber.New()
	app.Post("/api/auth/password", func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: "KARI", Company: "alpha", Language: "en-US"})
		return c.Next()
	}, h.ChangePassword)
	change := func(current, next string) (int, string) {
		body := `{"current_password":"` + current + `","new_password":"` + next + `"}`
		req := httptest.NewRequest("POST", "/api/auth/password", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, resp.Header.Get("Set-Cookie")
	}
	stored := func() *gtables.User {
		u := &gtables.User{}
		u.InitWithDBType(db, "", sqlite)
		u.Get("KARI")
		return u
	}

	if status, _ := change("wrong-password", "brand-new-secret"); status != 400 {
		t.Errorf("wrong current password: %d, want 400", status)
	}
	if status, _ := change("long-secret-1", "short"); status != 400 {
		t.Errorf("too short: %d, want 400", status)
	}
	if status, _ := change("long-secret-1", "password123"); status != 400 {
		t.Errorf("common password: %d, want 400", status)
	}
	if status, _ := change("long-secret-1", "kari-likes-tea"); status != 400 {
		t.Errorf("password with the user ID: %d, want 400", status)
	}
	if !tables.CheckPassword(stored(), "long-secret-1") {
		t.Fatal("a refused change changed the password")
	}

	before := time.Now().Add(-time.Second).Truncate(time.Second)
	status, cookie := change("long-secret-1", "brand-new-secret")
	if status != 200 || !strings.Contains(cookie, "openerp_session=") {
		t.Fatalf("change: %d, cookie %q; want 200 and a new session cookie", status, cookie)
	}
	if !tables.CheckPassword(stored(), "brand-new-secret") {
		t.Error("the new password was not stored")
	}
	check := NewUserCheck(db, sqlite)
	if check("KARI", before) {
		t.Error("a token issued before the change was accepted")
	}
	token := strings.TrimPrefix(strings.Split(cookie, ";")[0], "openerp_session=")
	claims, err := middleware.ParseToken(config, token)
	if err != nil || !check("KARI", claims.IssuedAt.Time) {
		t.Errorf("the new token is refused (%v)", err)
	}

	// wrong current passwords count as failed logins: 10 lock the address
	for range 10 {
		change("wrong-password", "another-new-secret")
	}
	if status, _ := change("brand-new-secret", "another-new-secret"); status != 429 {
		t.Errorf("after 10 wrong current passwords: %d, want 429", status)
	}
}
