package middleware

import (
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// The limits count per user for logged-in requests and per client address otherwise (S35):
// an office behind one address (NAT) does not share one budget, and IPv6 addresses count by
// their /64 network — the block one connection is given — so a client cannot take a new address
// for every request. The state is kept in memory per process.

// RateLimit returns the generous global limit of every request: 300 a minute per user (a
// request with a valid session cookie), else per client address (ClientKey). It runs before the
// session is read, so it checks the cookie's signature itself. Health checks are not limited.
func RateLimit(config JWTConfig) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          300,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return requestKey(c, config) },
		LimitReached: rateLimitReached,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/health"
		},
	})
}

// requestKey: "user:<ID>" for a request with a valid session cookie, else "addr:<ClientKey>"
func requestKey(c *fiber.Ctx, config JWTConfig) string {
	cookie := c.Cookies(config.CookieName)
	if cookie != "" {
		claims, err := ParseToken(config, cookie)
		if err == nil && claims.UserID != "" {
			return "user:" + strings.ToUpper(claims.UserID)
		}
	}
	return "addr:" + ClientKey(c)
}

// userOrAddressKey: the session's user (a route after the auth middleware), else the address
func userOrAddressKey(c *fiber.Ctx) string {
	sess, ok := c.Locals("session").(*session.Session)
	if ok && sess != nil && sess.UserID != "" {
		return "user:" + strings.ToUpper(sess.UserID)
	}
	return "addr:" + ClientKey(c)
}

// ClientKey is the client's address as the limits and the login guard count it: an IPv4
// address as it is, an IPv6 address as its /64 network ("2001:db8:1:2::/64")
func ClientKey(c *fiber.Ctx) string {
	return AddressKey(c.IP())
}

// AddressKey: see ClientKey (an address that does not parse is returned as it is)
func AddressKey(address string) string {
	ip := net.ParseIP(address)
	if ip == nil {
		return address
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}

// LoginRateLimit returns the strict limit of the login endpoint: 10 failed logins a minute per
// client address (successful ones do not count, so an office's users all log in at the start of
// the day). Login runs a deliberately slow bcrypt check, so unbounded failures would invite both
// password guessing and CPU exhaustion; the login guard (handlers) adds the per-user-ID locks.
func LoginRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    10,
		Expiration:             1 * time.Minute,
		KeyGenerator:           ClientKey,
		SkipSuccessfulRequests: true,
		LimitReached:           rateLimitReached,
	})
}

// ThemeInstallRateLimit returns a strict limit for installing/refreshing themes (per user):
// each request downloads up to 30 MB from GitHub and decodes the images.
func ThemeInstallRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          5,
		Expiration:   1 * time.Minute,
		KeyGenerator: userOrAddressKey,
		LimitReached: rateLimitReached,
	})
}

// rateLimitReached responds with 429 in the app's standard {success, error} envelope. The
// limiters run before the session is read, so the message is in the browser's language
// (Accept-Language), else the default language.
func rateLimitReached(c *fiber.Ctx) error {
	ts := i18n.GetInstance()
	language := acceptedLanguage(c.Get(fiber.HeaderAcceptLanguage), ts.GetSupportedLanguages())
	if language == "" {
		language = ts.GetDefaultLanguage()
	}
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"success": false,
		"error":   apperrors.TooManyRequests().Message(language),
	})
}

// acceptedLanguage returns the first language of an Accept-Language header that is supported
// (nb-NO, or nb for nb-NO), "" when none is (the default language)
func acceptedLanguage(header string, supported []string) string {
	type tag struct {
		name string
		q    float64
	}
	var tags []tag
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				q = parsed
			}
		}
		if name != "" && name != "*" && q > 0 {
			tags = append(tags, tag{name, q})
		}
	}
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].q > tags[j].q })
	sort.Strings(supported)
	for _, t := range tags {
		for _, lang := range supported {
			if strings.EqualFold(lang, t.name) {
				return lang
			}
		}
		primary, _, _ := strings.Cut(t.name, "-")
		if strings.EqualFold(primary, "no") || strings.EqualFold(primary, "nn") {
			primary = "nb" // Norwegian: Bokmål
		}
		for _, lang := range supported {
			langPrimary, _, _ := strings.Cut(lang, "-")
			if strings.EqualFold(langPrimary, primary) {
				return lang
			}
		}
	}
	return ""
}
