package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// getSession retrieves the per-request session from c.Locals.
// Falls back to the global session during migration.
func getSession(c *fiber.Ctx) *session.Session {
	if sess, ok := c.Locals("session").(*session.Session); ok {
		return sess
	}
	return session.GetCurrent()
}

// NewPermissionLoader returns a PermissionLoader that uses the same logic as AuthHandler.loadUserPermissions.
func NewPermissionLoader(db *sql.DB, dbType database.DBType) middleware.PermissionLoader {
	h := &AuthHandler{db: db, dbType: dbType}
	return func(_ *sql.DB, _ database.DBType, userID, company string) (bool, bool, map[string]session.TablePermission) {
		return h.loadUserPermissions(userID, company)
	}
}
