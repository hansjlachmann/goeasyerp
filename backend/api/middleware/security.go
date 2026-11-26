package middleware

import (
	"net"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
)

// SecurityHeaders sets the headers every API answer carries (S25, S27): never framed, never
// sniffed into another type, no referrer to other sites, and not stored by the browser or a
// proxy — the answers hold company data, which must not stay in a shared computer's cache after
// log out, and not read by other sites' pages (Cross-Origin-Resource-Policy; the app's own pages
// are the same origin). A handler whose answer may be kept (theme images) sets its own
// Cache-Control.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "same-origin")
		c.Set("Cross-Origin-Resource-Policy", "same-origin")
		c.Set("Cache-Control", "no-store")
		return c.Next()
	}
}

// SameOriginWrites refuses a change (POST, PUT, PATCH, DELETE) that another web site sends
// with the user's session (S26, cross-site request forgery): a form or script on another site —
// also a sibling subdomain, which the session cookie's SameSite=Lax lets through — or a login
// form that logs the user into someone else's account. Browsers say where a request comes from
// in Sec-Fetch-Site; without it (older browsers) the Origin must name this host. Requests from
// programs (no Sec-Fetch-Site, no Origin) are not browsers acting for a user and pass. allowed
// are origins that may (CORS_ORIGINS).
func SameOriginWrites(allowed []string) fiber.Handler {
	allowedOrigins := map[string]bool{}
	for _, origin := range allowed {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			allowedOrigins[strings.ToLower(origin)] = true
		}
	}
	return func(c *fiber.Ctx) error {
		if crossSiteWrite(c.Method(), c.Get("Sec-Fetch-Site"), c.Get(fiber.HeaderOrigin), c.Hostname(), allowedOrigins) {
			SecurityEventRecorded(c)
			securitylog.Record(securitylog.Event{Kind: securitylog.CrossSiteRefused, Address: c.IP(),
				Details: c.Method() + " " + c.Path() + " from " + originOrSite(c.Get(fiber.HeaderOrigin), c.Get("Sec-Fetch-Site"))})
			ts := i18n.GetInstance()
			language := acceptedLanguage(c.Get(fiber.HeaderAcceptLanguage), ts.GetSupportedLanguages())
			if language == "" {
				language = ts.GetDefaultLanguage()
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   apperrors.CrossSiteRequest().Message(language),
			})
		}
		return c.Next()
	}
}

// crossSiteWrite: the request changes something and comes from another site
func crossSiteWrite(method, fetchSite, origin, host string, allowed map[string]bool) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return false
	}
	origin = strings.ToLower(strings.TrimRight(origin, "/"))
	if allowed[origin] {
		return false
	}
	switch strings.ToLower(fetchSite) {
	case "same-origin", "none": // the app itself, or the user typed/bookmarked it
		return false
	case "same-site", "cross-site":
		return true
	}
	if origin == "" {
		return false // not from a browser page
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return true // "null" (a sandboxed frame, a redirect) or garbage
	}
	return !strings.EqualFold(u.Hostname(), hostOnly(host))
}

// hostOnly: a Host header without its port (nginx passes $host, which has none)
func hostOnly(host string) string {
	h, _, err := net.SplitHostPort(host)
	if err == nil {
		return h
	}
	return strings.Trim(host, "[]")
}

// originOrSite: where a refused cross-site request came from, for the security event
func originOrSite(origin, fetchSite string) string {
	if origin != "" {
		return origin
	}
	if fetchSite != "" {
		return fetchSite
	}
	return "?"
}
