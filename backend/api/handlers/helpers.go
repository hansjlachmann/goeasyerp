package handlers

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// getSession retrieves the per-request session from c.Locals.
// nil: the request is not logged in.
func getSession(c *fiber.Ctx) *session.Session {
	sess, _ := c.Locals("session").(*session.Session)
	return sess
}

// NewCompanyAccess returns the check whether a user may work in a company (role memberships;
// a user without roles everywhere), for the tabs that work in their own company
func NewCompanyAccess(db *sql.DB, dbType database.DBType) middleware.CompanyAccess {
	h := &AuthHandler{db: db, dbType: dbType}
	return func(userID, company string) bool {
		allowed, _ := h.userHasCompanyAccess(userID, company)
		return allowed
	}
}

// NewUserCheck returns the auth middleware's check of a token's user: the user exists, is
// active, and the token was issued at or after the user's Sessions Valid From (seconds, as the
// token's time)
func NewUserCheck(db *sql.DB, dbType database.DBType) middleware.UserCheck {
	return func(userID string, issuedAt time.Time) bool {
		var user gtables.User
		user.InitWithDBType(db, "", dbType)
		if !user.Get(userID) || !user.Active {
			return false
		}
		validFrom := user.Sessions_valid_from.Truncate(time.Second)
		return user.Sessions_valid_from.IsZero() || !issuedAt.Before(validFrom)
	}
}

// NewPermissionLoader returns a PermissionLoader that uses the same logic as AuthHandler.loadUserPermissions.
func NewPermissionLoader(db *sql.DB, dbType database.DBType) middleware.PermissionLoader {
	h := &AuthHandler{db: db, dbType: dbType}
	return func(_ *sql.DB, _ database.DBType, userID, company string) (bool, bool, map[string]session.TablePermission) {
		return h.loadUserPermissions(userID, company)
	}
}
