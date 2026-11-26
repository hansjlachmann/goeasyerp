package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
)

// recordSecurity notes a security event of the request (S48): its client address, and marks the
// request so the 403 catch-all (middleware.SecurityEvents) does not note it a second time
func recordSecurity(c *fiber.Ctx, kind securitylog.Kind, userID, company, details string) {
	middleware.SecurityEventRecorded(c)
	securitylog.Record(securitylog.Event{Kind: kind, UserID: userID, Address: c.IP(), Company: company, Details: details})
}

// failedLogin notes a failed login (and the lockout when it locks the address)
func failedLogin(c *fiber.Ctx, kind securitylog.Kind, userID, company, details string) {
	recordSecurity(c, kind, userID, company, details)
	if logins.failed(userID, middleware.ClientKey(c)) {
		recordSecurity(c, securitylog.LoginLocked, userID, company, "")
	}
}
