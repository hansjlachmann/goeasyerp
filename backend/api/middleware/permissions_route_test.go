package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// List copy reads records, list paste inserts them: the permission check must know both
func TestMapRouteToOperationCopyPaste(t *testing.T) {
	app := fiber.New()
	app.All("/*", func(c *fiber.Ctx) error { return c.SendString(string(mapRouteToOperation(c))) })
	for _, tc := range []struct{ method, path, want string }{
		{"POST", "/api/tables/Customer/insert-many", "insert"},
		{"POST", "/api/tables/Customer/copy", "read"},
		{"POST", "/api/tables/Customer/insert", "insert"},
		{"DELETE", "/api/tables/Customer/delete-many", "delete"},
	} {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.method, tc.path, body, tc.want)
		}
	}
}

// A table route the permission check does not know is refused, never let through
func TestPermissionCheckRefusesUnknownRoute(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: "CLERK", HasRoles: true})
		return c.Next()
	})
	group := app.Group("/api/tables/:table", PermissionCheck())
	group.Post("/something-new", func(c *fiber.Ctx) error { return c.SendString("ran") })
	resp, err := app.Test(httptest.NewRequest("POST", "/api/tables/Customer/something-new", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("an unknown route answered %d, want 403", resp.StatusCode)
	}
}
