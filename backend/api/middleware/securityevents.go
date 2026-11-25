package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// securityEventLocal marks a request whose security event was recorded where it happened
const securityEventLocal = "security_event_recorded"

// SecurityEventRecorded marks the request's security event as recorded (or as none to record),
// so SecurityEvents does not note its 403 again
func SecurityEventRecorded(c *fiber.Ctx) {
	c.Locals(securityEventLocal, true)
}

// SecurityEvents notes every request refused with 403 that its handler did not note itself
// (S48): a permission refusal, a SUPER-only action, a page of a disabled app … — the user and
// company of the session when there is one, the method and path as details. It runs before the
// other middleware, so it sees their answers too.
func SecurityEvents() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		if fe, ok := err.(*fiber.Error); ok {
			status = fe.Code
		}
		if status != fiber.StatusForbidden || c.Locals(securityEventLocal) != nil {
			return err
		}
		event := securitylog.Event{Kind: securitylog.AccessRefused, Address: c.IP(), Details: c.Method() + " " + c.Path()}
		if sess, ok := c.Locals("session").(*session.Session); ok && sess != nil {
			event.UserID = sess.GetUserID()
			event.Company = sess.GetCompany()
		}
		securitylog.Record(event)
		return err
	}
}
