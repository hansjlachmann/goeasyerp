package handlers

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// POST /api/auth/companies makes every company table: never without a login, and only for a
// SUPER user
func TestCreateCompanyNeedsSuper(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Company{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandlerWithDBType(db, database.DBTypeSQLite)
	for _, c := range []struct {
		name string
		sess *session.Session
		want int
	}{
		{"no login", nil, 401},
		{"not SUPER", func() *session.Session {
			s := &session.Session{UserID: "CLERK", Language: "en-US"}
			s.SetPermissions(true, false, map[string]session.TablePermission{"COMPANY": {Read: true, Insert: true}})
			return s
		}(), 403},
		{"SUPER", func() *session.Session {
			s := &session.Session{UserID: "ADMIN", Language: "en-US"}
			s.SetPermissions(true, true, nil)
			return s
		}(), 200},
	} {
		app := fiber.New()
		sess := c.sess
		app.Use(func(ctx *fiber.Ctx) error {
			ctx.Locals("session", sess)
			return ctx.Next()
		})
		app.Post("/api/auth/companies", h.CreateCompany)
		req := httptest.NewRequest("POST", "/api/auth/companies", strings.NewReader(`{"name":"new_`+strings.ReplaceAll(strings.ToLower(c.name), " ", "_")+`"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != c.want {
			t.Errorf("%s: %d, want %d", c.name, resp.StatusCode, c.want)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Company"`).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d companies, want only the SUPER user's (%v)", n, err)
	}
}
